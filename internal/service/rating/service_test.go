package rating

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRatingRepository - ручной мок RatingRepository
type MockRatingRepository struct {
	mock.Mock
	// рейтинги «из бд» для calc и то, что calc посчитал
	rating1, rating2 int
	update1, update2 *ParticipantUpdate
}

func (m *MockRatingRepository) ApplyMatchResult(ctx context.Context, match *models.Match, calc func(rating1, rating2 int) (*ParticipantUpdate, *ParticipantUpdate)) (bool, error) {
	args := m.Called(ctx, match)
	// репозиторий отдаёт в calc рейтинги, прочитанные под блокировкой
	if args.Bool(0) {
		m.update1, m.update2 = calc(m.rating1, m.rating2)
	}
	return args.Bool(0), args.Error(1)
}

// capturingNotifier запоминает события результата матча, чтобы проверить их в тесте.
// остальные методы берутся из NoopNotifier - они тут не нужны
type capturingNotifier struct {
	events.NoopNotifier
	events []events.MatchResultProcessed
}

func (n *capturingNotifier) MatchResultProcessed(_ context.Context, e events.MatchResultProcessed) {
	n.events = append(n.events, e)
}

func newTestRatingService(t *testing.T) (*Service, *MockRatingRepository) {
	repo := new(MockRatingRepository)
	log, _ := logger.New("error", "json")
	return NewService(repo, events.NoopNotifier{}, log), repo
}

// --- ProcessMatchResult ---

func testMatch(winner *int) *models.Match {
	return &models.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		Winner:       winner,
	}
}

func TestService_ProcessMatchResult_Player1Wins(t *testing.T) {
	repo := &MockRatingRepository{rating1: 1500, rating2: 1500}
	log, _ := logger.New("error", "json")
	bus := &capturingNotifier{}
	svc := NewService(repo, bus, log)
	ctx := context.Background()

	winner := 1
	match := testMatch(&winner)
	repo.On("ApplyMatchResult", ctx, match).Return(true, nil)

	err := svc.ProcessMatchResult(ctx, match)
	require.NoError(t, err)
	repo.AssertExpectations(t)

	// равные рейтинги (1500 vs 1500), выиграл первый: delta1=+16, delta2=-16
	assert.Equal(t, match.Program1ID, repo.update1.ProgramID)
	assert.Equal(t, 16, repo.update1.RatingDelta)
	assert.True(t, repo.update1.Won)
	assert.Equal(t, 1500, repo.update1.History.OldRating)
	assert.Equal(t, 1516, repo.update1.History.NewRating)
	assert.Equal(t, match.Program2ID, repo.update2.ProgramID)
	assert.Equal(t, -16, repo.update2.RatingDelta)
	assert.False(t, repo.update2.Won)
	assert.False(t, repo.update2.Draw)

	// после успешного апдейта должно уйти событие с версией 1
	require.Len(t, bus.events, 1)
	evt := bus.events[0]
	assert.Equal(t, 1, evt.Version)
	assert.Equal(t, match.ID, evt.MatchID)
	assert.Equal(t, 1, evt.Winner)
	assert.Equal(t, 1516, evt.NewRating1)
	assert.Equal(t, 1484, evt.NewRating2)
}

func TestService_ProcessMatchResult_Player2Wins(t *testing.T) {
	svc, repo := newTestRatingService(t)
	repo.rating1, repo.rating2 = 1500, 1500
	ctx := context.Background()

	winner := 2
	match := testMatch(&winner)
	repo.On("ApplyMatchResult", ctx, match).Return(true, nil)

	require.NoError(t, svc.ProcessMatchResult(ctx, match))

	// равные рейтинги, выиграл второй: delta1=-16, delta2=+16
	assert.Equal(t, -16, repo.update1.RatingDelta)
	assert.False(t, repo.update1.Won)
	assert.Equal(t, 16, repo.update2.RatingDelta)
	assert.True(t, repo.update2.Won)
}

func TestService_ProcessMatchResult_Draw(t *testing.T) {
	svc, repo := newTestRatingService(t)
	repo.rating1, repo.rating2 = 1500, 1500
	ctx := context.Background()

	winner := 0
	match := testMatch(&winner)
	repo.On("ApplyMatchResult", ctx, match).Return(true, nil)

	require.NoError(t, svc.ProcessMatchResult(ctx, match))

	// равные рейтинги, ничья: изменения нет, у обоих Draw=true
	assert.Equal(t, 0, repo.update1.RatingDelta)
	assert.True(t, repo.update1.Draw)
	assert.Equal(t, 0, repo.update2.RatingDelta)
	assert.True(t, repo.update2.Draw)
}

// дельта считается от рейтингов, которые репозиторий прочитал под блокировкой
func TestService_ProcessMatchResult_UsesLockedRatings(t *testing.T) {
	svc, repo := newTestRatingService(t)
	repo.rating1, repo.rating2 = 2800, 400
	ctx := context.Background()

	winner := 1 // фаворит выигрывает - изменение около нуля
	match := testMatch(&winner)
	repo.On("ApplyMatchResult", ctx, match).Return(true, nil)

	require.NoError(t, svc.ProcessMatchResult(ctx, match))
	assert.Equal(t, 2800, repo.update1.History.OldRating)
	assert.Equal(t, 400, repo.update2.History.OldRating)
	assert.Equal(t, 0, repo.update1.RatingDelta)
}

func TestService_ProcessMatchResult_AtomicError(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()

	winner := 1
	match := testMatch(&winner)
	repo.On("ApplyMatchResult", ctx, match).Return(false, errors.ErrInternal)

	err := svc.ProcessMatchResult(ctx, match)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// рейтинг уже применён (fast path и диспетчер разошлись): не ошибка и без события
func TestService_ProcessMatchResult_AlreadyApplied(t *testing.T) {
	repo := new(MockRatingRepository)
	log, _ := logger.New("error", "json")
	bus := &capturingNotifier{}
	svc := NewService(repo, bus, log)
	ctx := context.Background()

	winner := 1
	match := testMatch(&winner)
	repo.On("ApplyMatchResult", ctx, match).Return(false, nil)

	require.NoError(t, svc.ProcessMatchResult(ctx, match))
	assert.Nil(t, repo.update1)
	assert.Empty(t, bus.events)
}

func TestService_ProcessMatchResult_NilWinner(t *testing.T) {
	svc, _ := newTestRatingService(t)
	ctx := context.Background()

	err := svc.ProcessMatchResult(ctx, testMatch(nil))
	assert.Error(t, err)
	assert.True(t, errors.IsAppError(err))
	appErr := errors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, 400, appErr.Code)
	assert.Contains(t, appErr.Message, "no winner")
}
