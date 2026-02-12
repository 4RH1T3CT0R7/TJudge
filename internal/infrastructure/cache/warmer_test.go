package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockTournamentRepository struct {
	mock.Mock
}

func (m *MockTournamentRepository) List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tournament), args.Error(1)
}

func (m *MockTournamentRepository) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	args := m.Called(ctx, tournamentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LeaderboardEntry), args.Error(1)
}

type MockMatchRepository struct {
	mock.Mock
}

func (m *MockMatchRepository) List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

// --- Helpers ---

func newTestWarmer(t *testing.T) (*CacheWarmer, *MockTournamentRepository, *MockMatchRepository) {
	t.Helper()
	c := setupTestCache(t)
	log, _ := logger.New("error", "json")

	leaderboardCache := NewLeaderboardCache(c)
	matchCache := NewMatchCache(c)
	tournamentCache := NewTournamentCache(c)

	tournamentRepo := new(MockTournamentRepository)
	matchRepo := new(MockMatchRepository)

	warmer := NewCacheWarmer(
		c, leaderboardCache, matchCache, tournamentCache,
		tournamentRepo, matchRepo, log, 100*time.Millisecond,
	)

	return warmer, tournamentRepo, matchRepo
}

// --- WarmUp orchestration ---

func TestCacheWarmer_WarmUp_AllSuccess(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	// Active tournaments
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{}, nil)
	// Pending tournaments
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	// Running matches
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{}, nil)
	// Pending matches
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)
	tournamentRepo.AssertExpectations(t)
	matchRepo.AssertExpectations(t)
}

func TestCacheWarmer_WarmUp_PartialFailureContinues(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	// Active tournaments fail
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return(nil, errors.New("db error"))
	// Pending tournaments succeed
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	// Running matches fail — warmActiveMatches returns early, pending match List not called
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return(nil, errors.New("redis error"))

	// WarmUp always returns nil (logs errors internally)
	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)
	tournamentRepo.AssertExpectations(t)
	matchRepo.AssertExpectations(t)
}

// --- warmActiveTournaments ---

func TestCacheWarmer_WarmActiveTournaments_Success(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	t1 := &domain.Tournament{ID: uuid.New(), Name: "T1", Status: domain.TournamentActive}
	t2 := &domain.Tournament{ID: uuid.New(), Name: "T2", Status: domain.TournamentActive}

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{t1, t2}, nil)
	// Pending
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	// Leaderboard - called once per active tournament
	tournamentRepo.On("GetLeaderboard", ctx, t1.ID, 100).Return([]*domain.LeaderboardEntry{}, nil)
	tournamentRepo.On("GetLeaderboard", ctx, t2.ID, 100).Return([]*domain.LeaderboardEntry{}, nil)
	// Matches
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)

	// Verify tournaments were cached
	cached1, err := warmer.tournamentCache.Get(ctx, t1.ID)
	require.NoError(t, err)
	require.NotNil(t, cached1)
	assert.Equal(t, t1.Name, cached1.Name)

	cached2, err := warmer.tournamentCache.Get(ctx, t2.ID)
	require.NoError(t, err)
	require.NotNil(t, cached2)
	assert.Equal(t, t2.Name, cached2.Name)
}

func TestCacheWarmer_WarmActiveTournaments_Empty(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{}, nil)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)
}

func TestCacheWarmer_WarmActiveTournaments_Error(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return(nil, errors.New("connection refused"))
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	// WarmUp doesn't fail — just logs
	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)
}

// --- warmPendingTournaments ---

func TestCacheWarmer_WarmPendingTournaments_Success(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	t1 := &domain.Tournament{ID: uuid.New(), Name: "Pending1", Status: domain.TournamentPending}

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{}, nil)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{t1}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)

	cached, err := warmer.tournamentCache.Get(ctx, t1.ID)
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, "Pending1", cached.Name)
}

// --- warmLeaderboards ---

func TestCacheWarmer_WarmLeaderboards_Success(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	tid := uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	tournament := &domain.Tournament{ID: tid, Status: domain.TournamentActive}

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{tournament}, nil)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	tournamentRepo.On("GetLeaderboard", ctx, tid, 100).Return([]*domain.LeaderboardEntry{
		{Rank: 1, ProgramID: p1, Rating: 1600},
		{Rank: 2, ProgramID: p2, Rating: 1400},
	}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)

	// Verify leaderboard entries were cached
	top, err := warmer.leaderboardCache.GetTop(ctx, tid, 10)
	require.NoError(t, err)
	require.Len(t, top, 2)
	assert.Equal(t, p1, top[0].ProgramID)
	assert.Equal(t, 1600, top[0].Rating)
	assert.Equal(t, p2, top[1].ProgramID)
	assert.Equal(t, 1400, top[1].Rating)
}

func TestCacheWarmer_WarmLeaderboards_GetLeaderboardError(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	tid := uuid.New()
	tournament := &domain.Tournament{ID: tid, Status: domain.TournamentActive}

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{tournament}, nil)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	tournamentRepo.On("GetLeaderboard", ctx, tid, 100).Return(nil, errors.New("db error"))
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	// Continues despite error
	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)
}

// --- warmActiveMatches ---

func TestCacheWarmer_WarmActiveMatches_Success(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	m1 := &domain.Match{ID: uuid.New(), Status: domain.MatchRunning}
	m2 := &domain.Match{ID: uuid.New(), Status: domain.MatchPending}

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{}, nil)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{m1}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{m2}, nil)

	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)

	// Verify matches were cached
	cached1, err := warmer.matchCache.GetMatch(ctx, m1.ID)
	require.NoError(t, err)
	require.NotNil(t, cached1)
	assert.Equal(t, m1.ID, cached1.ID)

	cached2, err := warmer.matchCache.GetMatch(ctx, m2.ID)
	require.NoError(t, err)
	require.NotNil(t, cached2)
	assert.Equal(t, m2.ID, cached2.ID)
}

func TestCacheWarmer_WarmActiveMatches_RunningError(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{}, nil)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return(nil, errors.New("timeout"))

	// warmActiveMatches returns error early, pending not fetched
	err := warmer.WarmUp(ctx)
	assert.NoError(t, err)
}

// --- Start / Stop lifecycle ---

func TestCacheWarmer_StartStop(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)

	// Setup repos to return empty for all calls
	tournamentRepo.On("List", mock.Anything, mock.Anything).Return([]*domain.Tournament{}, nil)
	matchRepo.On("List", mock.Anything, mock.Anything).Return([]*domain.Match{}, nil)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		warmer.Start(ctx)
		close(done)
	}()

	// Let a couple warmup cycles run
	time.Sleep(250 * time.Millisecond)

	warmer.Stop()

	select {
	case <-done:
		// Stopped properly
	case <-time.After(2 * time.Second):
		t.Fatal("CacheWarmer.Start did not return after Stop")
	}
}

func TestCacheWarmer_StartStop_ContextCancellation(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)

	tournamentRepo.On("List", mock.Anything, mock.Anything).Return([]*domain.Tournament{}, nil)
	matchRepo.On("List", mock.Anything, mock.Anything).Return([]*domain.Match{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		warmer.Start(ctx)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Stopped via context cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("CacheWarmer.Start did not return after context cancellation")
	}
}

func TestCacheWarmer_DoubleStop(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)

	tournamentRepo.On("List", mock.Anything, mock.Anything).Return([]*domain.Tournament{}, nil)
	matchRepo.On("List", mock.Anything, mock.Anything).Return([]*domain.Match{}, nil)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		warmer.Start(ctx)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)

	// Double stop should not panic (uses sync.Once)
	warmer.Stop()
	warmer.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CacheWarmer did not stop")
	}
}

// --- Full integration warmup ---

func TestCacheWarmer_WarmUp_FullIntegration(t *testing.T) {
	warmer, tournamentRepo, matchRepo := newTestWarmer(t)
	ctx := context.Background()

	tid := uuid.New()
	p1 := uuid.New()
	tournament := &domain.Tournament{ID: tid, Name: "Full", Status: domain.TournamentActive}
	m1 := &domain.Match{ID: uuid.New(), Status: domain.MatchRunning, TournamentID: tid}

	// Active tournaments (called twice: warmActiveTournaments + warmLeaderboards)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentActive, Limit: 100}).
		Return([]*domain.Tournament{tournament}, nil)
	tournamentRepo.On("List", ctx, domain.TournamentFilter{Status: domain.TournamentPending, Limit: 50}).
		Return([]*domain.Tournament{}, nil)
	tournamentRepo.On("GetLeaderboard", ctx, tid, 100).Return([]*domain.LeaderboardEntry{
		{Rank: 1, ProgramID: p1, Rating: 1500},
	}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchRunning, Limit: 200}).
		Return([]*domain.Match{m1}, nil)
	matchRepo.On("List", ctx, domain.MatchFilter{Status: domain.MatchPending, Limit: 500}).
		Return([]*domain.Match{}, nil)

	err := warmer.WarmUp(ctx)
	require.NoError(t, err)

	// Tournament cached
	cached, err := warmer.tournamentCache.Get(ctx, tid)
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, "Full", cached.Name)

	// Leaderboard cached
	top, err := warmer.leaderboardCache.GetTop(ctx, tid, 10)
	require.NoError(t, err)
	require.Len(t, top, 1)
	assert.Equal(t, p1, top[0].ProgramID)

	// Match cached
	cachedMatch, err := warmer.matchCache.GetMatch(ctx, m1.ID)
	require.NoError(t, err)
	require.NotNil(t, cachedMatch)
	assert.Equal(t, m1.ID, cachedMatch.ID)

	tournamentRepo.AssertExpectations(t)
	matchRepo.AssertExpectations(t)
}
