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

func (m *MockRatingRepository) Create(ctx context.Context, history *models.RatingHistory) error {
	return m.Called(ctx, history).Error(0)
}

func (m *MockRatingRepository) GetByProgramID(ctx context.Context, programID uuid.UUID) ([]*models.RatingHistory, error) {
	args := m.Called(ctx, programID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.RatingHistory), args.Error(1)
}

func (m *MockRatingRepository) UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int) error {
	return m.Called(ctx, tournamentID, programID, ratingDelta).Error(0)
}

func (m *MockRatingRepository) UpdateParticipantStats(ctx context.Context, tournamentID, programID uuid.UUID, won bool, draw bool) error {
	return m.Called(ctx, tournamentID, programID, won, draw).Error(0)
}

func (m *MockRatingRepository) UpdateParticipantRatingAndStats(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int, won bool, draw bool) error {
	return m.Called(ctx, tournamentID, programID, ratingDelta, won, draw).Error(0)
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

// --- GetRatingHistory ---

func TestService_GetRatingHistory_Success(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	programID := uuid.New()

	expected := []*models.RatingHistory{{ID: uuid.New(), ProgramID: programID}}
	repo.On("GetByProgramID", ctx, programID).Return(expected, nil)

	result, err := svc.GetRatingHistory(ctx, programID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestService_GetRatingHistory_Error(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	programID := uuid.New()

	repo.On("GetByProgramID", ctx, programID).Return(nil, errors.ErrInternal)

	_, err := svc.GetRatingHistory(ctx, programID)
	assert.Error(t, err)
}

// --- CalculateExpectedScore ---

func TestService_CalculateExpectedScore_EqualRatings(t *testing.T) {
	svc, _ := newTestRatingService(t)

	score := svc.CalculateExpectedScore(1500, 1500)
	assert.InDelta(t, 0.5, score, 0.001)
}

func TestService_CalculateExpectedScore_Asymmetric(t *testing.T) {
	svc, _ := newTestRatingService(t)

	// у кого рейтинг выше - ожидание больше 0.5, у кого ниже - меньше
	higher := svc.CalculateExpectedScore(1800, 1500)
	assert.Greater(t, higher, 0.5)
	assert.Less(t, higher, 1.0)

	lower := svc.CalculateExpectedScore(1200, 1500)
	assert.Less(t, lower, 0.5)
	assert.Greater(t, lower, 0.0)
}

func TestService_CalculateExpectedScore_Symmetry(t *testing.T) {
	svc, _ := newTestRatingService(t)

	scoreA := svc.CalculateExpectedScore(1600, 1400)
	scoreB := svc.CalculateExpectedScore(1400, 1600)

	// два ожидания в сумме дают ~1.0
	assert.InDelta(t, 1.0, scoreA+scoreB, 0.001)
	// разница в 200 очков -> ~0.76
	assert.InDelta(t, 0.76, scoreA, 0.01)
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
