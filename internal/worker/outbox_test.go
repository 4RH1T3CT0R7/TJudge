package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Моки ---

type MockOutboxStore struct {
	mock.Mock
}

func (m *MockOutboxStore) ClaimPending(ctx context.Context, olderThan time.Duration, limit int) ([]*db.OutboxEntry, error) {
	args := m.Called(ctx, olderThan, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.OutboxEntry), args.Error(1)
}

func (m *MockOutboxStore) MarkDone(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOutboxStore) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	args := m.Called(ctx, id, errMsg)
	return args.Error(0)
}

type MockOutboxMatchRepo struct {
	mock.Mock
}

func (m *MockOutboxMatchRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Match), args.Error(1)
}

type MockOutboxRatingRepo struct {
	mock.Mock
}

func (m *MockOutboxRatingRepo) GetParticipantRatings(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID) (int, int, error) {
	args := m.Called(ctx, tournamentID, program1ID, program2ID)
	return args.Int(0), args.Int(1), args.Error(2)
}

func (m *MockOutboxRatingRepo) GetByMatchID(ctx context.Context, matchID uuid.UUID) ([]*domain.RatingHistory, error) {
	args := m.Called(ctx, matchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RatingHistory), args.Error(1)
}

// capturingBus собирает опубликованные события.
type capturingBus struct {
	published []any
}

func (b *capturingBus) Publish(_ context.Context, event any) {
	b.published = append(b.published, event)
}
func (b *capturingBus) Subscribe(_ events.Handler, _ ...any) {}

func newTestDispatcher(t *testing.T) (*OutboxDispatcher, *MockOutboxStore, *MockOutboxMatchRepo, *MockOutboxRatingRepo, *MockProcessorRatingService, *capturingBus) {
	t.Helper()
	outbox := new(MockOutboxStore)
	matchRepo := new(MockOutboxMatchRepo)
	ratingRepo := new(MockOutboxRatingRepo)
	ratingService := new(MockProcessorRatingService)
	bus := &capturingBus{}
	log, _ := logger.New("error", "json")

	d := NewOutboxDispatcher(outbox, matchRepo, ratingRepo, ratingService, bus, log)
	return d, outbox, matchRepo, ratingRepo, ratingService, bus
}

func completedMatch() *domain.Match {
	winner := 1
	return &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		Status:       domain.MatchCompleted,
		Winner:       &winner,
	}
}

// --- Тесты ---

func TestOutboxDispatcher_RunOnce_ProcessesStaleEntry(t *testing.T) {
	d, outbox, matchRepo, ratingRepo, ratingService, _ := newTestDispatcher(t)
	match := completedMatch()
	entry := &db.OutboxEntry{ID: 1, MatchID: match.ID, Kind: db.OutboxKindRatingUpdate, Attempts: 1}

	outbox.On("ClaimPending", mock.Anything, mock.Anything, mock.Anything).
		Return([]*db.OutboxEntry{entry}, nil)
	matchRepo.On("GetByID", mock.Anything, match.ID).Return(match, nil)
	// Рейтинг ещё не применялся - history пустая.
	ratingRepo.On("GetByMatchID", mock.Anything, match.ID).Return([]*domain.RatingHistory{}, nil)
	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(1200, 1000, nil)
	ratingService.On("ProcessMatchResult", mock.Anything, match, 1200, 1000).Return(nil)
	outbox.On("MarkDone", mock.Anything, int64(1)).Return(nil)

	processed := d.RunOnce(context.Background())
	assert.Equal(t, 1, processed)
	outbox.AssertExpectations(t)
	ratingService.AssertExpectations(t)
}

func TestOutboxDispatcher_RunOnce_IdempotentSkipRepublishesEvent(t *testing.T) {
	d, outbox, matchRepo, ratingRepo, ratingService, bus := newTestDispatcher(t)
	match := completedMatch()
	entry := &db.OutboxEntry{ID: 2, MatchID: match.ID, Kind: db.OutboxKindRatingUpdate, Attempts: 1}

	history := []*domain.RatingHistory{
		{ProgramID: match.Program1ID, NewRating: 1216, MatchID: &match.ID},
		{ProgramID: match.Program2ID, NewRating: 984, MatchID: &match.ID},
	}

	outbox.On("ClaimPending", mock.Anything, mock.Anything, mock.Anything).
		Return([]*db.OutboxEntry{entry}, nil)
	matchRepo.On("GetByID", mock.Anything, match.ID).Return(match, nil)
	// Рейтинг уже применён (краш после коммита) - повторять нельзя.
	ratingRepo.On("GetByMatchID", mock.Anything, match.ID).Return(history, nil)
	outbox.On("MarkDone", mock.Anything, int64(2)).Return(nil)

	processed := d.RunOnce(context.Background())
	assert.Equal(t, 1, processed)

	// Рейтинг НЕ пересчитывается...
	ratingService.AssertNotCalled(t, "ProcessMatchResult", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	// ...но потерянное событие переотправляется с точными рейтингами из history.
	assert.Len(t, bus.published, 1)
	evt, ok := bus.published[0].(events.MatchResultProcessed)
	assert.True(t, ok)
	assert.Equal(t, 1216, evt.NewRating1)
	assert.Equal(t, 984, evt.NewRating2)
	assert.Equal(t, match.ID, evt.MatchID)
}

func TestOutboxDispatcher_RunOnce_MatchDeleted(t *testing.T) {
	d, outbox, matchRepo, _, _, _ := newTestDispatcher(t)
	matchID := uuid.New()
	entry := &db.OutboxEntry{ID: 3, MatchID: matchID, Kind: db.OutboxKindRatingUpdate, Attempts: 1}

	outbox.On("ClaimPending", mock.Anything, mock.Anything, mock.Anything).
		Return([]*db.OutboxEntry{entry}, nil)
	matchRepo.On("GetByID", mock.Anything, matchID).Return(nil, errors.ErrNotFound)
	// Матч удалён - задача закрывается как неактуальная.
	outbox.On("MarkDone", mock.Anything, int64(3)).Return(nil)

	processed := d.RunOnce(context.Background())
	assert.Equal(t, 1, processed)
	outbox.AssertExpectations(t)
}

func TestOutboxDispatcher_RunOnce_ErrorMarksFailed(t *testing.T) {
	d, outbox, matchRepo, ratingRepo, ratingService, _ := newTestDispatcher(t)
	match := completedMatch()
	entry := &db.OutboxEntry{ID: 4, MatchID: match.ID, Kind: db.OutboxKindRatingUpdate, Attempts: 3}

	outbox.On("ClaimPending", mock.Anything, mock.Anything, mock.Anything).
		Return([]*db.OutboxEntry{entry}, nil)
	matchRepo.On("GetByID", mock.Anything, match.ID).Return(match, nil)
	ratingRepo.On("GetByMatchID", mock.Anything, match.ID).Return([]*domain.RatingHistory{}, nil)
	ratingRepo.On("GetParticipantRatings", mock.Anything, match.TournamentID, match.Program1ID, match.Program2ID).
		Return(1200, 1000, nil)
	ratingService.On("ProcessMatchResult", mock.Anything, match, 1200, 1000).
		Return(fmt.Errorf("db connection lost"))
	outbox.On("MarkFailed", mock.Anything, int64(4), "db connection lost").Return(nil)

	processed := d.RunOnce(context.Background())
	assert.Equal(t, 0, processed)
	outbox.AssertExpectations(t)
	outbox.AssertNotCalled(t, "MarkDone", mock.Anything, mock.Anything)
}

func TestOutboxDispatcher_RunOnce_FailedMatchSkipped(t *testing.T) {
	d, outbox, matchRepo, _, ratingService, _ := newTestDispatcher(t)
	match := completedMatch()
	match.Status = domain.MatchFailed
	entry := &db.OutboxEntry{ID: 5, MatchID: match.ID, Kind: db.OutboxKindRatingUpdate, Attempts: 1}

	outbox.On("ClaimPending", mock.Anything, mock.Anything, mock.Anything).
		Return([]*db.OutboxEntry{entry}, nil)
	matchRepo.On("GetByID", mock.Anything, match.ID).Return(match, nil)
	outbox.On("MarkDone", mock.Anything, int64(5)).Return(nil)

	processed := d.RunOnce(context.Background())
	assert.Equal(t, 1, processed)
	ratingService.AssertNotCalled(t, "ProcessMatchResult", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestOutboxDispatcher_StartStop(t *testing.T) {
	d, outbox, _, _, _, _ := newTestDispatcher(t)
	d.interval = 10 * time.Millisecond

	outbox.On("ClaimPending", mock.Anything, mock.Anything, mock.Anything).
		Return([]*db.OutboxEntry{}, nil).Maybe()

	d.Start()
	time.Sleep(35 * time.Millisecond)
	d.Stop() // не должен зависнуть

	select {
	case <-d.done:
	default:
		t.Fatal("dispatcher goroutine not finished after Stop")
	}
}
