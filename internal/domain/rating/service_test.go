package rating

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
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

func (m *MockRatingRepository) UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int) error {
	return m.Called(ctx, tournamentID, programID, ratingDelta).Error(0)
}

func (m *MockRatingRepository) UpdateParticipantStats(ctx context.Context, tournamentID, programID uuid.UUID, won bool, draw bool) error {
	return m.Called(ctx, tournamentID, programID, won, draw).Error(0)
}

func (m *MockRatingRepository) UpdateParticipantRatingAndStats(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int, won bool, draw bool) error {
	return m.Called(ctx, tournamentID, programID, ratingDelta, won, draw).Error(0)
}

func (m *MockRatingRepository) ProcessMatchResultAtomic(ctx context.Context, update1, update2 *ParticipantUpdate) error {
	return m.Called(ctx, update1, update2).Error(0)
}

func newTestRatingService(t *testing.T) (*Service, *MockRatingRepository) {
	repo := new(MockRatingRepository)
	log, _ := logger.New("error", "json")
	return NewService(repo, events.NoopBus{}, log), repo
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

	// 200 point difference -> ~0.76
	assert.InDelta(t, 0.76, scoreA, 0.01)
}

// --- ProcessMatchResult ---

func newTestRatingServiceWithCache(t *testing.T) (*Service, *MockRatingRepository) {
	t.Helper()
	return newTestRatingService(t)
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

	// Equal ratings (1500 vs 1500), p1 wins: delta1=+16, delta2=-16
	repo.On("ProcessMatchResultAtomic", ctx, mock.MatchedBy(func(u *ParticipantUpdate) bool {
		return u.ProgramID == p1 && u.RatingDelta == 16 && u.Won && !u.Draw
	}), mock.MatchedBy(func(u *ParticipantUpdate) bool {
		return u.ProgramID == p2 && u.RatingDelta == -16 && !u.Won && !u.Draw
	})).Return(nil)

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

	// Equal ratings (1500 vs 1500), p2 wins: delta1=-16, delta2=+16
	repo.On("ProcessMatchResultAtomic", ctx, mock.MatchedBy(func(u *ParticipantUpdate) bool {
		return u.ProgramID == p1 && u.RatingDelta == -16 && !u.Won && !u.Draw
	}), mock.MatchedBy(func(u *ParticipantUpdate) bool {
		return u.ProgramID == p2 && u.RatingDelta == 16 && u.Won && !u.Draw
	})).Return(nil)

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

	// Equal ratings, draw: no change (delta=0)
	repo.On("ProcessMatchResultAtomic", ctx, mock.MatchedBy(func(u *ParticipantUpdate) bool {
		return u.ProgramID == p1 && u.RatingDelta == 0 && !u.Won && u.Draw
	}), mock.MatchedBy(func(u *ParticipantUpdate) bool {
		return u.ProgramID == p2 && u.RatingDelta == 0 && !u.Won && u.Draw
	})).Return(nil)

	err := svc.ProcessMatchResult(ctx, match, 1500, 1500)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_ProcessMatchResult_AtomicError(t *testing.T) {
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

	// ProcessMatchResultAtomic fails - both updates should be rolled back
	repo.On("ProcessMatchResultAtomic", ctx,
		mock.AnythingOfType("*rating.ParticipantUpdate"),
		mock.AnythingOfType("*rating.ParticipantUpdate"),
	).Return(errors.ErrInternal)

	err := svc.ProcessMatchResult(ctx, match, 1500, 1500)

	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestService_ProcessMatchResult_ExtremeRatings(t *testing.T) {
	svc, repo := newTestRatingServiceWithCache(t)
	ctx := context.Background()

	tID := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	winner := 1 // Higher rated wins - small change expected
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tID,
		Program1ID:   p1,
		Program2ID:   p2,
		Winner:       &winner,
	}

	// 2800 vs 400: expected score for 2800 ≈ 1.0, so change ≈ 0
	repo.On("ProcessMatchResultAtomic", ctx,
		mock.AnythingOfType("*rating.ParticipantUpdate"),
		mock.AnythingOfType("*rating.ParticipantUpdate"),
	).Return(nil)

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
