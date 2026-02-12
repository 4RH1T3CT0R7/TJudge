package rating

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRatingRepository implements RatingRepository
type MockRatingRepository struct {
	mock.Mock
}

func (m *MockRatingRepository) Create(ctx context.Context, history *domain.RatingHistory) error {
	return m.Called(ctx, history).Error(0)
}

func (m *MockRatingRepository) GetByProgramID(ctx context.Context, programID uuid.UUID) ([]*domain.RatingHistory, error) {
	args := m.Called(ctx, programID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RatingHistory), args.Error(1)
}

func (m *MockRatingRepository) UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, newRating int) error {
	return m.Called(ctx, tournamentID, programID, newRating).Error(0)
}

func (m *MockRatingRepository) UpdateParticipantStats(ctx context.Context, tournamentID, programID uuid.UUID, won bool, draw bool) error {
	return m.Called(ctx, tournamentID, programID, won, draw).Error(0)
}

func newTestRatingService(t *testing.T) (*Service, *MockRatingRepository) {
	repo := new(MockRatingRepository)
	log, _ := logger.New("error", "json")
	// Create service without leaderboard cache (nil) - we test private methods directly
	return &Service{
		calculator:       NewDefaultEloCalculator(),
		repo:             repo,
		leaderboardCache: nil,
		log:              log,
	}, repo
}

// --- updateParticipantRating ---

func TestService_updateParticipantRating_Success(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	matchID := uuid.New()
	programID := uuid.New()
	tID := uuid.New()

	match := &domain.Match{ID: matchID, TournamentID: tID}

	repo.On("Create", ctx, mock.AnythingOfType("*domain.RatingHistory")).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, programID, 1016).Return(nil)

	err := svc.updateParticipantRating(ctx, match, programID, 1000, 1016, 16)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_updateParticipantRating_CreateError(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()

	match := &domain.Match{ID: uuid.New(), TournamentID: uuid.New()}
	repo.On("Create", ctx, mock.Anything).Return(errors.ErrInternal)

	err := svc.updateParticipantRating(ctx, match, uuid.New(), 1000, 1016, 16)
	assert.Error(t, err)
}

func TestService_updateParticipantRating_UpdateError(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	tID := uuid.New()
	programID := uuid.New()

	match := &domain.Match{ID: uuid.New(), TournamentID: tID}
	repo.On("Create", ctx, mock.Anything).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, programID, 1016).Return(errors.ErrInternal)

	err := svc.updateParticipantRating(ctx, match, programID, 1000, 1016, 16)
	assert.Error(t, err)
}

// --- updateMatchStats ---

func TestService_updateMatchStats_Player1Wins(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 1

	match := &domain.Match{TournamentID: tID, Program1ID: p1, Program2ID: p2, Winner: &winner}

	repo.On("UpdateParticipantStats", ctx, tID, p1, true, false).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p2, false, false).Return(nil)

	err := svc.updateMatchStats(ctx, match)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_updateMatchStats_Player2Wins(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 2

	match := &domain.Match{TournamentID: tID, Program1ID: p1, Program2ID: p2, Winner: &winner}

	repo.On("UpdateParticipantStats", ctx, tID, p1, false, false).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p2, true, false).Return(nil)

	err := svc.updateMatchStats(ctx, match)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_updateMatchStats_Draw(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 0

	match := &domain.Match{TournamentID: tID, Program1ID: p1, Program2ID: p2, Winner: &winner}

	repo.On("UpdateParticipantStats", ctx, tID, p1, false, true).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p2, false, true).Return(nil)

	err := svc.updateMatchStats(ctx, match)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_updateMatchStats_Program1Error(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 1

	match := &domain.Match{TournamentID: tID, Program1ID: p1, Program2ID: p2, Winner: &winner}

	repo.On("UpdateParticipantStats", ctx, tID, p1, true, false).Return(errors.ErrInternal)
	// Program2 should not be called

	err := svc.updateMatchStats(ctx, match)
	assert.Error(t, err)
}

// --- GetRatingHistory ---

func TestService_GetRatingHistory_Success(t *testing.T) {
	svc, repo := newTestRatingService(t)
	ctx := context.Background()
	programID := uuid.New()

	expected := []*domain.RatingHistory{{ID: uuid.New(), ProgramID: programID}}
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

func TestService_CalculateExpectedScore_HigherRating(t *testing.T) {
	svc, _ := newTestRatingService(t)

	score := svc.CalculateExpectedScore(1800, 1500)
	assert.Greater(t, score, 0.5)
	assert.Less(t, score, 1.0)
}

func TestService_CalculateExpectedScore_LowerRating(t *testing.T) {
	svc, _ := newTestRatingService(t)

	score := svc.CalculateExpectedScore(1200, 1500)
	assert.Less(t, score, 0.5)
	assert.Greater(t, score, 0.0)
}

func TestService_CalculateExpectedScore_Symmetry(t *testing.T) {
	svc, _ := newTestRatingService(t)

	scoreA := svc.CalculateExpectedScore(1600, 1400)
	scoreB := svc.CalculateExpectedScore(1400, 1600)

	// Expected scores should sum to ~1.0
	assert.InDelta(t, 1.0, scoreA+scoreB, 0.001)

	// 200 point difference → ~0.76
	assert.InDelta(t, 0.76, scoreA, 0.01)
}

// --- ProcessMatchResult (requires LeaderboardCache via miniredis) ---

func newTestRatingServiceWithCache(t *testing.T) (*Service, *MockRatingRepository) {
	t.Helper()
	repo := new(MockRatingRepository)
	log, _ := logger.New("error", "json")

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	testCache := cache.NewFromClient(client)
	leaderboardCache := cache.NewLeaderboardCache(testCache)

	return &Service{
		calculator:       NewDefaultEloCalculator(),
		repo:             repo,
		leaderboardCache: leaderboardCache,
		log:              log,
	}, repo
}

func TestService_ProcessMatchResult_Player1Wins(t *testing.T) {
	svc, repo := newTestRatingServiceWithCache(t)
	ctx := context.Background()

	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 1
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tID,
		Program1ID:   p1,
		Program2ID:   p2,
		Winner:       &winner,
	}

	// Equal ratings (1500 vs 1500), p1 wins: new1=1516, new2=1484
	repo.On("Create", ctx, mock.AnythingOfType("*domain.RatingHistory")).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p1, 1516).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p2, 1484).Return(nil)
	// updateMatchStats
	repo.On("UpdateParticipantStats", ctx, tID, p1, true, false).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p2, false, false).Return(nil)

	err := svc.ProcessMatchResult(ctx, match, 1500, 1500)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_ProcessMatchResult_Player2Wins(t *testing.T) {
	svc, repo := newTestRatingServiceWithCache(t)
	ctx := context.Background()

	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 2
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tID,
		Program1ID:   p1,
		Program2ID:   p2,
		Winner:       &winner,
	}

	// Equal ratings (1500 vs 1500), p2 wins: new1=1484, new2=1516
	repo.On("Create", ctx, mock.AnythingOfType("*domain.RatingHistory")).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p1, 1484).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p2, 1516).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p1, false, false).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p2, true, false).Return(nil)

	err := svc.ProcessMatchResult(ctx, match, 1500, 1500)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_ProcessMatchResult_Draw(t *testing.T) {
	svc, repo := newTestRatingServiceWithCache(t)
	ctx := context.Background()

	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 0
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tID,
		Program1ID:   p1,
		Program2ID:   p2,
		Winner:       &winner,
	}

	// Equal ratings, draw: no change (1500+32*(0.5-0.5)=1500)
	repo.On("Create", ctx, mock.AnythingOfType("*domain.RatingHistory")).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p1, 1500).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p2, 1500).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p1, false, true).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p2, false, true).Return(nil)

	err := svc.ProcessMatchResult(ctx, match, 1500, 1500)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_ProcessMatchResult_UpdateRatingError_Program1(t *testing.T) {
	svc, repo := newTestRatingServiceWithCache(t)
	ctx := context.Background()

	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 1
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tID,
		Program1ID:   p1,
		Program2ID:   p2,
		Winner:       &winner,
	}

	// First updateParticipantRating (p1) fails at Create
	repo.On("Create", ctx, mock.AnythingOfType("*domain.RatingHistory")).Return(errors.ErrInternal).Once()

	err := svc.ProcessMatchResult(ctx, match, 1500, 1500)

	assert.Error(t, err)
}

func TestService_ProcessMatchResult_ExtremeRatings(t *testing.T) {
	svc, repo := newTestRatingServiceWithCache(t)
	ctx := context.Background()

	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 1 // Higher rated wins — small change expected
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tID,
		Program1ID:   p1,
		Program2ID:   p2,
		Winner:       &winner,
	}

	// 2800 vs 400: expected score for 2800 ≈ 1.0, so change ≈ 0
	// K=32, expected ≈ 0.9999..., new1 ≈ 2800, new2 ≈ 400
	repo.On("Create", ctx, mock.AnythingOfType("*domain.RatingHistory")).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p1, mock.AnythingOfType("int")).Return(nil)
	repo.On("UpdateParticipantRating", ctx, tID, p2, mock.AnythingOfType("int")).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p1, true, false).Return(nil)
	repo.On("UpdateParticipantStats", ctx, tID, p2, false, false).Return(nil)

	err := svc.ProcessMatchResult(ctx, match, 2800, 400)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_ProcessMatchResult_NilWinner(t *testing.T) {
	svc, _ := newTestRatingServiceWithCache(t)
	ctx := context.Background()

	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		Winner:       nil,
	}

	err := svc.ProcessMatchResult(ctx, match, 1500, 1500)
	assert.Error(t, err)
	assert.True(t, errors.IsAppError(err))
	appErr := errors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, 400, appErr.Code)
	assert.Contains(t, appErr.Message, "no winner")
}
