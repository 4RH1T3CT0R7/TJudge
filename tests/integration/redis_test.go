//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// RedisTestSuite is the integration test suite for Redis operations
type RedisTestSuite struct {
	suite.Suite
	cache            *cache.Cache
	matchCache       *cache.MatchCache
	leaderboardCache *cache.LeaderboardCache
	ctx              context.Context
}

func (s *RedisTestSuite) SetupSuite() {
	if os.Getenv("RUN_INTEGRATION") != "true" {
		s.T().Skip("Skipping integration tests (set RUN_INTEGRATION=true)")
	}

	s.ctx = context.Background()

	host := getEnv("REDIS_HOST", "localhost")
	port := getEnv("REDIS_PORT", "6379")
	password := getEnv("REDIS_PASSWORD", "")

	log, _ := logger.New("debug", "json")
	m := metrics.New()

	var err error
	s.cache, err = cache.New(cache.Config{
		Host:         host,
		Port:         port,
		Password:     password,
		DB:           1, // Use DB 1 for tests
		PoolSize:     10,
		MinIdleConns: 5,
	}, log, m)
	require.NoError(s.T(), err)

	s.matchCache = cache.NewMatchCache(s.cache)
	s.leaderboardCache = cache.NewLeaderboardCache(s.cache)
}

func (s *RedisTestSuite) TearDownSuite() {
	if s.cache != nil {
		// Clean up test keys
		s.cache.Del(s.ctx, "test:*")
		s.cache.Close()
	}
}

func (s *RedisTestSuite) SetupTest() {
	// Clean up test data before each test
	s.cache.Del(s.ctx, "match:*")
	s.cache.Del(s.ctx, "leaderboard:*")
	s.cache.Del(s.ctx, "test:*")
}

// =============================================================================
// Basic Cache Operations Tests
// =============================================================================

func (s *RedisTestSuite) TestCache_SetGet() {
	key := "test:basic:setget"
	value := []byte("test_value")

	err := s.cache.Set(s.ctx, key, value, time.Minute)
	require.NoError(s.T(), err)

	result, err := s.cache.Get(s.ctx, key)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), string(value), result)
}

func (s *RedisTestSuite) TestCache_GetNonExistent() {
	result, err := s.cache.Get(s.ctx, "test:nonexistent:key")
	require.NoError(s.T(), err)
	assert.Empty(s.T(), result)
}

func (s *RedisTestSuite) TestCache_Delete() {
	key := "test:delete:key"
	err := s.cache.Set(s.ctx, key, []byte("value"), time.Minute)
	require.NoError(s.T(), err)

	err = s.cache.Del(s.ctx, key)
	require.NoError(s.T(), err)

	result, err := s.cache.Get(s.ctx, key)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), result)
}

func (s *RedisTestSuite) TestCache_TTLExpiration() {
	key := "test:ttl:key"
	err := s.cache.Set(s.ctx, key, []byte("value"), 100*time.Millisecond)
	require.NoError(s.T(), err)

	// Value should exist
	result, err := s.cache.Get(s.ctx, key)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), result)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Value should be gone
	result, err = s.cache.Get(s.ctx, key)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), result)
}

func (s *RedisTestSuite) TestCache_Exists() {
	key := "test:exists:key"

	exists, err := s.cache.Exists(s.ctx, key)
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)

	err = s.cache.Set(s.ctx, key, []byte("value"), time.Minute)
	require.NoError(s.T(), err)

	exists, err = s.cache.Exists(s.ctx, key)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

// =============================================================================
// Match Cache Tests
// =============================================================================

func (s *RedisTestSuite) TestMatchCache_SetGetMatch() {
	match := &domain.Match{
		ID:       uuid.New(),
		Status:   domain.MatchPending,
		GameType: "tictactoe",
		Priority: domain.PriorityMedium,
	}

	err := s.matchCache.SetMatch(s.ctx, match)
	require.NoError(s.T(), err)

	found, err := s.matchCache.GetMatch(s.ctx, match.ID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), found)
	assert.Equal(s.T(), match.ID, found.ID)
	assert.Equal(s.T(), match.Status, found.Status)
}

func (s *RedisTestSuite) TestMatchCache_SetGetResult() {
	matchID := uuid.New()
	result := &domain.MatchResult{
		MatchID:     matchID,
		WinnerID:    uuid.New(),
		ScoreP1:     10,
		ScoreP2:     5,
		DetailsJSON: `{"moves": 42}`,
	}

	err := s.matchCache.Set(s.ctx, matchID, result)
	require.NoError(s.T(), err)

	found, err := s.matchCache.Get(s.ctx, matchID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), found)
	assert.Equal(s.T(), result.MatchID, found.MatchID)
	assert.Equal(s.T(), result.WinnerID, found.WinnerID)
	assert.Equal(s.T(), result.ScoreP1, found.ScoreP1)
}

func (s *RedisTestSuite) TestMatchCache_Delete() {
	match := &domain.Match{
		ID:       uuid.New(),
		Status:   domain.MatchPending,
		GameType: "tictactoe",
	}

	err := s.matchCache.SetMatch(s.ctx, match)
	require.NoError(s.T(), err)

	err = s.matchCache.Delete(s.ctx, match.ID)
	require.NoError(s.T(), err)

	found, err := s.matchCache.GetMatch(s.ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Nil(s.T(), found)
}

func (s *RedisTestSuite) TestMatchCache_Exists() {
	matchID := uuid.New()

	exists, err := s.matchCache.Exists(s.ctx, matchID)
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)

	match := &domain.Match{
		ID:       matchID,
		Status:   domain.MatchPending,
		GameType: "tictactoe",
	}
	err = s.matchCache.SetMatch(s.ctx, match)
	require.NoError(s.T(), err)

	exists, err = s.matchCache.Exists(s.ctx, matchID)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

// =============================================================================
// Leaderboard Cache Tests
// =============================================================================

func (s *RedisTestSuite) TestLeaderboardCache_SetGetTournament() {
	tournamentID := uuid.New()
	entries := []domain.LeaderboardEntry{
		{ProgramID: uuid.New(), ProgramName: "Program 1", Rating: 1500, Rank: 1, Wins: 10, Losses: 2, Draws: 1},
		{ProgramID: uuid.New(), ProgramName: "Program 2", Rating: 1400, Rank: 2, Wins: 8, Losses: 4, Draws: 1},
		{ProgramID: uuid.New(), ProgramName: "Program 3", Rating: 1300, Rank: 3, Wins: 6, Losses: 5, Draws: 2},
	}

	err := s.leaderboardCache.SetTournamentLeaderboard(s.ctx, tournamentID, entries)
	require.NoError(s.T(), err)

	found, err := s.leaderboardCache.GetTournamentLeaderboard(s.ctx, tournamentID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), found)
	assert.Len(s.T(), found, 3)
	assert.Equal(s.T(), entries[0].Rating, found[0].Rating)
}

func (s *RedisTestSuite) TestLeaderboardCache_SetGetGlobal() {
	entries := []domain.LeaderboardEntry{
		{ProgramID: uuid.New(), ProgramName: "Top 1", Rating: 2000, Rank: 1},
		{ProgramID: uuid.New(), ProgramName: "Top 2", Rating: 1900, Rank: 2},
	}

	err := s.leaderboardCache.SetGlobalLeaderboard(s.ctx, entries)
	require.NoError(s.T(), err)

	found, err := s.leaderboardCache.GetGlobalLeaderboard(s.ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), found)
	assert.Len(s.T(), found, 2)
}

func (s *RedisTestSuite) TestLeaderboardCache_InvalidateTournament() {
	tournamentID := uuid.New()
	entries := []domain.LeaderboardEntry{
		{ProgramID: uuid.New(), ProgramName: "Program 1", Rating: 1500, Rank: 1},
	}

	err := s.leaderboardCache.SetTournamentLeaderboard(s.ctx, tournamentID, entries)
	require.NoError(s.T(), err)

	err = s.leaderboardCache.InvalidateTournamentLeaderboard(s.ctx, tournamentID)
	require.NoError(s.T(), err)

	found, err := s.leaderboardCache.GetTournamentLeaderboard(s.ctx, tournamentID)
	require.NoError(s.T(), err)
	assert.Nil(s.T(), found)
}

// =============================================================================
// Distributed Lock Tests
// =============================================================================

func (s *RedisTestSuite) TestDistributedLock_LockUnlock() {
	lock := cache.NewDistributedLock(s.cache, "test:lock:basic", 5*time.Second)

	acquired, err := lock.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), acquired)

	// Try to acquire again - should fail
	acquired, err = lock.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.False(s.T(), acquired)

	// Unlock
	err = lock.Unlock(s.ctx)
	require.NoError(s.T(), err)

	// Should be able to acquire again
	acquired, err = lock.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), acquired)

	lock.Unlock(s.ctx)
}

func (s *RedisTestSuite) TestDistributedLock_WithLock() {
	lock := cache.NewDistributedLock(s.cache, "test:lock:withlock", 5*time.Second)

	executed := false
	err := lock.WithLock(s.ctx, func() error {
		executed = true
		return nil
	})
	require.NoError(s.T(), err)
	assert.True(s.T(), executed)
}

func (s *RedisTestSuite) TestDistributedLock_TTLExpiration() {
	lock := cache.NewDistributedLock(s.cache, "test:lock:ttl", 100*time.Millisecond)

	acquired, err := lock.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), acquired)

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Lock should have expired, can acquire again
	acquired, err = lock.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), acquired)

	lock.Unlock(s.ctx)
}

func (s *RedisTestSuite) TestDistributedLock_ConcurrentAccess() {
	const numGoroutines = 10
	lockKey := "test:lock:concurrent"
	counter := 0
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			lock := cache.NewDistributedLock(s.cache, lockKey, 5*time.Second)
			err := lock.WithLock(s.ctx, func() error {
				// Critical section
				current := counter
				time.Sleep(10 * time.Millisecond) // Simulate work
				counter = current + 1
				return nil
			})
			if err != nil {
				s.T().Logf("Lock error: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Counter should be exactly numGoroutines if locking works
	assert.Equal(s.T(), numGoroutines, counter)
}

// =============================================================================
// Sorted Set Operations Tests (for Queue)
// =============================================================================

func (s *RedisTestSuite) TestSortedSet_Operations() {
	key := "test:sortedset"

	// Add items with scores (priorities)
	err := s.cache.ZAdd(s.ctx, key, 1.0, "item1")
	require.NoError(s.T(), err)
	err = s.cache.ZAdd(s.ctx, key, 2.0, "item2")
	require.NoError(s.T(), err)
	err = s.cache.ZAdd(s.ctx, key, 3.0, "item3")
	require.NoError(s.T(), err)

	// Get count
	count, err := s.cache.ZCard(s.ctx, key)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), count)

	// Pop highest priority (lowest score)
	items, err := s.cache.ZPopMin(s.ctx, key, 1)
	require.NoError(s.T(), err)
	require.Len(s.T(), items, 1)
	assert.Equal(s.T(), "item1", items[0].Member)

	// Verify count decreased
	count, err = s.cache.ZCard(s.ctx, key)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), count)
}

// =============================================================================
// List Operations Tests (for Queue)
// =============================================================================

func (s *RedisTestSuite) TestList_Operations() {
	key := "test:list"

	// Push items
	err := s.cache.LPush(s.ctx, key, "item1")
	require.NoError(s.T(), err)
	err = s.cache.LPush(s.ctx, key, "item2")
	require.NoError(s.T(), err)

	// Get length
	length, err := s.cache.LLen(s.ctx, key)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), length)

	// Pop (blocking)
	result, err := s.cache.BRPop(s.ctx, time.Second, key)
	require.NoError(s.T(), err)
	require.Len(s.T(), result, 2)
	assert.Equal(s.T(), "item1", result[1]) // First pushed = first popped (FIFO)
}

func TestRedisSuite(t *testing.T) {
	suite.Run(t, new(RedisTestSuite))
}
