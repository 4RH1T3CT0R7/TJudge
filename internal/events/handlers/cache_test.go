package handlers

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	return log
}

// --- Мок TournamentCacheWriter ---

type mockTournamentCache struct {
	mock.Mock
}

func (m *mockTournamentCache) Set(ctx context.Context, tournament *domain.Tournament) error {
	args := m.Called(ctx, tournament)
	return args.Error(0)
}

func (m *mockTournamentCache) Invalidate(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

// --- Мок LeaderboardCacheWriter ---

type mockLeaderboardCache struct {
	mock.Mock
}

func (m *mockLeaderboardCache) UpdateRating(ctx context.Context, tournamentID, programID uuid.UUID, rating int) error {
	args := m.Called(ctx, tournamentID, programID, rating)
	return args.Error(0)
}

func (m *mockLeaderboardCache) UpdateRatingsBatch(ctx context.Context, updates []cache.RatingUpdate) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *mockLeaderboardCache) Clear(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

func (m *mockLeaderboardCache) InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

// --- Тесты TournamentCacheHandler ---

func TestTournamentCacheHandler_TournamentCreated(t *testing.T) {
	tc := &mockTournamentCache{}
	lc := &mockLeaderboardCache{}
	h := NewTournamentCacheHandler(tc, lc, newTestLogger(t))

	tournament := &domain.Tournament{ID: uuid.New(), Name: "Test"}
	tc.On("Set", mock.Anything, tournament).Return(nil)

	err := h.Handle(context.Background(), events.TournamentCreated{Tournament: tournament})
	assert.NoError(t, err)
	tc.AssertExpectations(t)
}

func TestTournamentCacheHandler_TournamentStarted(t *testing.T) {
	tc := &mockTournamentCache{}
	lc := &mockLeaderboardCache{}
	h := NewTournamentCacheHandler(tc, lc, newTestLogger(t))

	id := uuid.New()
	tc.On("Invalidate", mock.Anything, id).Return(nil)

	err := h.Handle(context.Background(), events.TournamentStarted{TournamentID: id})
	assert.NoError(t, err)
	tc.AssertExpectations(t)
}

func TestTournamentCacheHandler_TournamentCompleted(t *testing.T) {
	tc := &mockTournamentCache{}
	lc := &mockLeaderboardCache{}
	h := NewTournamentCacheHandler(tc, lc, newTestLogger(t))

	id := uuid.New()
	tc.On("Invalidate", mock.Anything, id).Return(nil)

	err := h.Handle(context.Background(), events.TournamentCompleted{TournamentID: id})
	assert.NoError(t, err)
	tc.AssertExpectations(t)
}

func TestTournamentCacheHandler_TournamentDeleted(t *testing.T) {
	tc := &mockTournamentCache{}
	lc := &mockLeaderboardCache{}
	h := NewTournamentCacheHandler(tc, lc, newTestLogger(t))

	id := uuid.New()
	tc.On("Invalidate", mock.Anything, id).Return(nil)
	lc.On("Clear", mock.Anything, id).Return(nil)

	err := h.Handle(context.Background(), events.TournamentDeleted{TournamentID: id})
	assert.NoError(t, err)
	tc.AssertExpectations(t)
	lc.AssertExpectations(t)
}

func TestTournamentCacheHandler_ParticipantJoined(t *testing.T) {
	tc := &mockTournamentCache{}
	lc := &mockLeaderboardCache{}
	h := NewTournamentCacheHandler(tc, lc, newTestLogger(t))

	id := uuid.New()
	tc.On("Invalidate", mock.Anything, id).Return(nil)

	err := h.Handle(context.Background(), events.ParticipantJoined{TournamentID: id})
	assert.NoError(t, err)
	tc.AssertExpectations(t)
}

func TestTournamentCacheHandler_GameRoundReset(t *testing.T) {
	tc := &mockTournamentCache{}
	lc := &mockLeaderboardCache{}
	h := NewTournamentCacheHandler(tc, lc, newTestLogger(t))

	id := uuid.New()
	gid := uuid.New()
	tc.On("Invalidate", mock.Anything, id).Return(nil)
	lc.On("Clear", mock.Anything, id).Return(nil)

	err := h.Handle(context.Background(), events.GameRoundReset{TournamentID: id, GameID: gid})
	assert.NoError(t, err)
	tc.AssertExpectations(t)
	lc.AssertExpectations(t)
}

func TestTournamentCacheHandler_UnexpectedEvent(t *testing.T) {
	tc := &mockTournamentCache{}
	lc := &mockLeaderboardCache{}
	h := NewTournamentCacheHandler(tc, lc, newTestLogger(t))

	err := h.Handle(context.Background(), "not an event")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected event type")
}

// --- Тесты LeaderboardCacheHandler ---

func TestLeaderboardCacheHandler_ParticipantJoined(t *testing.T) {
	lc := &mockLeaderboardCache{}
	h := NewLeaderboardCacheHandler(lc, newTestLogger(t))

	tid := uuid.New()
	pid := uuid.New()
	lc.On("UpdateRating", mock.Anything, tid, pid, 1500).Return(nil)
	lc.On("InvalidateFullLeaderboard", mock.Anything, tid).Return(nil)

	err := h.Handle(context.Background(), events.ParticipantJoined{
		TournamentID:  tid,
		ProgramID:     pid,
		InitialRating: 1500,
	})
	assert.NoError(t, err)
	lc.AssertExpectations(t)
}

func TestLeaderboardCacheHandler_MatchResultProcessed(t *testing.T) {
	lc := &mockLeaderboardCache{}
	h := NewLeaderboardCacheHandler(lc, newTestLogger(t))

	tid := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	expected := []cache.RatingUpdate{
		{TournamentID: tid, ProgramID: p1, Rating: 1520},
		{TournamentID: tid, ProgramID: p2, Rating: 1480},
	}
	lc.On("UpdateRatingsBatch", mock.Anything, expected).Return(nil)
	lc.On("InvalidateFullLeaderboard", mock.Anything, tid).Return(nil)

	err := h.Handle(context.Background(), events.MatchResultProcessed{
		TournamentID: tid,
		Program1ID:   p1,
		Program2ID:   p2,
		NewRating1:   1520,
		NewRating2:   1480,
		Winner:       1,
	})
	assert.NoError(t, err)
	lc.AssertExpectations(t)
}

func TestLeaderboardCacheHandler_GameRoundReset(t *testing.T) {
	lc := &mockLeaderboardCache{}
	h := NewLeaderboardCacheHandler(lc, newTestLogger(t))

	tid := uuid.New()
	gid := uuid.New()
	lc.On("Clear", mock.Anything, tid).Return(nil)

	err := h.Handle(context.Background(), events.GameRoundReset{TournamentID: tid, GameID: gid})
	assert.NoError(t, err)
	lc.AssertExpectations(t)
}

func TestLeaderboardCacheHandler_UnexpectedEvent(t *testing.T) {
	lc := &mockLeaderboardCache{}
	h := NewLeaderboardCacheHandler(lc, newTestLogger(t))

	err := h.Handle(context.Background(), "not an event")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected event type")
}
