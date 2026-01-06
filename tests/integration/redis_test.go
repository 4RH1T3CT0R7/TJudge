//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
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
	port := getEnvInt("REDIS_PORT", 6379)
	password := getEnv("REDIS_PASSWORD", "")

	log, _ := logger.New("debug", "json")
	m := metrics.New()

	var err error
	s.cache, err = cache.New(&config.RedisConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       1, // Use DB 1 for tests
		PoolSize: 10,
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
		MatchID:  matchID,
		Winner:   1,
		Score1:   10,
		Score2:   5,
		Duration: 5 * time.Second,
	}

	err := s.matchCache.Set(s.ctx, matchID, result)
	require.NoError(s.T(), err)

	found, err := s.matchCache.Get(s.ctx, matchID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), found)
	assert.Equal(s.T(), result.MatchID, found.MatchID)
	assert.Equal(s.T(), result.Winner, found.Winner)
	assert.Equal(s.T(), result.Score1, found.Score1)
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

func (s *RedisTestSuite) TestLeaderboardCache_UpdateAndGetTop() {
	tournamentID := uuid.New()
	program1 := uuid.New()
	program2 := uuid.New()
	program3 := uuid.New()

	// Update ratings for programs
	err := s.leaderboardCache.UpdateRating(s.ctx, tournamentID, program1, 1500)
	require.NoError(s.T(), err)
	err = s.leaderboardCache.UpdateRating(s.ctx, tournamentID, program2, 1400)
	require.NoError(s.T(), err)
	err = s.leaderboardCache.UpdateRating(s.ctx, tournamentID, program3, 1300)
	require.NoError(s.T(), err)

	// Get top entries
	entries, err := s.leaderboardCache.GetTop(s.ctx, tournamentID, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), entries, 3)
	assert.Equal(s.T(), 1500, entries[0].Rating)
}

func (s *RedisTestSuite) TestLeaderboardCache_IncrementRating() {
	tournamentID := uuid.New()
	programID := uuid.New()

	// Set initial rating
	err := s.leaderboardCache.UpdateRating(s.ctx, tournamentID, programID, 1500)
	require.NoError(s.T(), err)

	// Increment rating
	err = s.leaderboardCache.IncrementRating(s.ctx, tournamentID, programID, 100)
	require.NoError(s.T(), err)

	// Verify
	entries, err := s.leaderboardCache.GetTop(s.ctx, tournamentID, 1)
	require.NoError(s.T(), err)
	require.Len(s.T(), entries, 1)
	assert.Equal(s.T(), 1600, entries[0].Rating)
}

func (s *RedisTestSuite) TestLeaderboardCache_ClearTournament() {
	tournamentID := uuid.New()
	programID := uuid.New()

	err := s.leaderboardCache.UpdateRating(s.ctx, tournamentID, programID, 1500)
	require.NoError(s.T(), err)

	err = s.leaderboardCache.Clear(s.ctx, tournamentID)
	require.NoError(s.T(), err)

	entries, err := s.leaderboardCache.GetTop(s.ctx, tournamentID, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), entries, 0)
}

// =============================================================================
// Distributed Lock Tests
// =============================================================================

func (s *RedisTestSuite) TestDistributedLock_LockUnlock() {
	lock := cache.NewDistributedLock(s.cache)
	lockKey := "test:lock:basic"
	ttl := 5 * time.Second

	token, err := lock.Lock(s.ctx, lockKey, ttl)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), token)

	// Try to acquire again - should fail
	_, err = lock.Lock(s.ctx, lockKey, ttl)
	assert.Error(s.T(), err)

	// Unlock
	err = lock.Unlock(s.ctx, lockKey, token)
	require.NoError(s.T(), err)

	// Should be able to acquire again
	token2, err := lock.Lock(s.ctx, lockKey, ttl)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), token2)

	_ = lock.Unlock(s.ctx, lockKey, token2)
}

func (s *RedisTestSuite) TestDistributedLock_WithLock() {
	lock := cache.NewDistributedLock(s.cache)

	executed := false
	err := lock.WithLock(s.ctx, "test:lock:withlock", 5*time.Second, func(ctx context.Context) error {
		executed = true
		return nil
	})
	require.NoError(s.T(), err)
	assert.True(s.T(), executed)
}

func (s *RedisTestSuite) TestDistributedLock_TTLExpiration() {
	lock := cache.NewDistributedLock(s.cache)
	lockKey := "test:lock:ttl"
	ttl := 100 * time.Millisecond

	token, err := lock.Lock(s.ctx, lockKey, ttl)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), token)

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Lock should have expired, can acquire again
	token2, err := lock.Lock(s.ctx, lockKey, ttl)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), token2)

	_ = lock.Unlock(s.ctx, lockKey, token2)
}

func (s *RedisTestSuite) TestDistributedLock_ConcurrentAccess() {
	const numGoroutines = 10
	lockKey := "test:lock:concurrent"
	counter := 0
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			lock := cache.NewDistributedLock(s.cache)
			err := lock.WithLock(s.ctx, lockKey, 5*time.Second, func(ctx context.Context) error {
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
