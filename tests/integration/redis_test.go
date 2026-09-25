//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// RedisTestSuite is the integration test suite for Redis operations
type RedisTestSuite struct {
	suite.Suite
	cache            *cache.Cache
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

	s.leaderboardCache = cache.NewLeaderboardCache(s.cache)
}

func (s *RedisTestSuite) TearDownSuite() {
	if s.cache != nil {
		// Clean up test keys
		_ = s.cache.Del(s.ctx, "test:*")
		s.cache.Close()
	}
}

func (s *RedisTestSuite) SetupTest() {
	// Clean up test data before each test
	_ = s.cache.Del(s.ctx, "match:*")
	_ = s.cache.Del(s.ctx, "leaderboard:*")
	_ = s.cache.Del(s.ctx, "test:*")
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
// Leaderboard Cache Tests
// =============================================================================

// кэш лидерборда на живом редисе: hash с ttl (EXPIRE NX) и инвалидация одним DEL
func (s *RedisTestSuite) TestLeaderboardCache_Invalidate() {
	tournamentID := uuid.New()
	entries := []*models.LeaderboardEntry{{Rank: 1, ProgramID: uuid.New(), Rating: 1500}}

	require.NoError(s.T(), s.leaderboardCache.SetFullLeaderboard(s.ctx, tournamentID, 100, entries))
	require.NoError(s.T(), s.leaderboardCache.SetFullLeaderboard(s.ctx, tournamentID, 50, entries))

	found, err := s.leaderboardCache.GetFullLeaderboard(s.ctx, tournamentID, 50)
	require.NoError(s.T(), err)
	assert.Len(s.T(), found, 1)

	require.NoError(s.T(), s.leaderboardCache.InvalidateFullLeaderboard(s.ctx, tournamentID))

	for _, limit := range []int{100, 50} {
		found, err := s.leaderboardCache.GetFullLeaderboard(s.ctx, tournamentID, limit)
		require.NoError(s.T(), err)
		assert.Nil(s.T(), found)
	}
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
	const numGoroutines = 5 // Reduced for faster test execution
	lockKey := "test:lock:concurrent"
	counter := 0
	done := make(chan bool, numGoroutines)
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		go func() {
			lock := cache.NewDistributedLock(s.cache)
			// Use TryLock with more retries for concurrent test
			token, err := lock.TryLock(s.ctx, lockKey, 2*time.Second, 10, 200*time.Millisecond)
			if err != nil {
				s.T().Logf("Lock error: %v", err)
				done <- true
				return
			}

			// Critical section
			mu.Lock()
			current := counter
			mu.Unlock()
			time.Sleep(10 * time.Millisecond) // Simulate work
			mu.Lock()
			counter = current + 1
			mu.Unlock()

			_ = lock.Unlock(s.ctx, lockKey, token)
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

func TestRedisSuite(t *testing.T) {
	suite.Run(t, new(RedisTestSuite))
}
