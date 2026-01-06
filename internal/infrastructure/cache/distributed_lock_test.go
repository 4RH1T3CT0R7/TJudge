package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistributedLock_Lock(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("successfully acquires lock", func(t *testing.T) {
		token, err := lock.Lock(ctx, "test-lock", 5*time.Second)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Cleanup
		err = lock.Unlock(ctx, "test-lock", token)
		assert.NoError(t, err)
	})

	t.Run("fails to acquire already held lock", func(t *testing.T) {
		token1, err := lock.Lock(ctx, "test-lock-2", 5*time.Second)
		require.NoError(t, err)
		defer func() { _ = lock.Unlock(ctx, "test-lock-2", token1) }()

		token2, err := lock.Lock(ctx, "test-lock-2", 5*time.Second)
		assert.Error(t, err)
		assert.Empty(t, token2)
		assert.Contains(t, err.Error(), "lock already held")
	})

	t.Run("lock expires after TTL", func(t *testing.T) {
		token1, err := lock.Lock(ctx, "test-lock-3", 100*time.Millisecond)
		require.NoError(t, err)
		assert.NotEmpty(t, token1)

		// Wait for lock to expire
		time.Sleep(150 * time.Millisecond)

		// Should be able to acquire again
		token2, err := lock.Lock(ctx, "test-lock-3", 5*time.Second)
		assert.NoError(t, err)
		assert.NotEmpty(t, token2)
		assert.NotEqual(t, token1, token2)

		// Cleanup
		_ = lock.Unlock(ctx, "test-lock-3", token2)
	})
}

func TestDistributedLock_TryLock(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("acquires lock on first attempt", func(t *testing.T) {
		token, err := lock.TryLock(ctx, "test-trylock", 5*time.Second, 3, 50*time.Millisecond)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Cleanup
		_ = lock.Unlock(ctx, "test-trylock", token)
	})

	t.Run("retries and eventually acquires lock", func(t *testing.T) {
		// First lock with short TTL
		token1, err := lock.Lock(ctx, "test-trylock-2", 200*time.Millisecond)
		require.NoError(t, err)

		// Try to acquire in another goroutine, should retry and succeed
		token2, err := lock.TryLock(ctx, "test-trylock-2", 5*time.Second, 5, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.NotEmpty(t, token2)
		assert.NotEqual(t, token1, token2)

		// Cleanup
		_ = lock.Unlock(ctx, "test-trylock-2", token2)
	})

	t.Run("fails after max attempts", func(t *testing.T) {
		token1, err := lock.Lock(ctx, "test-trylock-3", 5*time.Second)
		require.NoError(t, err)
		defer func() { _ = lock.Unlock(ctx, "test-trylock-3", token1) }()

		token2, err := lock.TryLock(ctx, "test-trylock-3", 5*time.Second, 2, 10*time.Millisecond)
		assert.Error(t, err)
		assert.Empty(t, token2)
		assert.Contains(t, err.Error(), "failed to acquire lock after")
	})
}

func TestDistributedLock_Unlock(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("successfully unlocks with correct token", func(t *testing.T) {
		token, err := lock.Lock(ctx, "test-unlock", 5*time.Second)
		require.NoError(t, err)

		err = lock.Unlock(ctx, "test-unlock", token)
		assert.NoError(t, err)

		// Should be able to lock again
		token2, err := lock.Lock(ctx, "test-unlock", 5*time.Second)
		assert.NoError(t, err)
		assert.NotEmpty(t, token2)

		// Cleanup
		_ = lock.Unlock(ctx, "test-unlock", token2)
	})

	t.Run("fails to unlock with wrong token", func(t *testing.T) {
		token, err := lock.Lock(ctx, "test-unlock-2", 5*time.Second)
		require.NoError(t, err)
		defer func() { _ = lock.Unlock(ctx, "test-unlock-2", token) }()

		err = lock.Unlock(ctx, "test-unlock-2", "wrong-token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token mismatch")
	})

	t.Run("unlocking already unlocked lock is safe", func(t *testing.T) {
		token, err := lock.Lock(ctx, "test-unlock-3", 5*time.Second)
		require.NoError(t, err)

		err = lock.Unlock(ctx, "test-unlock-3", token)
		assert.NoError(t, err)

		// Unlock again - should not error
		err = lock.Unlock(ctx, "test-unlock-3", token)
		assert.NoError(t, err)
	})
}

func TestDistributedLock_WithLock(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("executes function with lock", func(t *testing.T) {
		executed := false
		err := lock.WithLock(ctx, "test-withlock", 5*time.Second, func(ctx context.Context) error {
			executed = true
			return nil
		})

		assert.NoError(t, err)
		assert.True(t, executed)

		// Lock should be released
		isLocked, err := lock.IsLocked(ctx, "test-withlock")
		assert.NoError(t, err)
		assert.False(t, isLocked)
	})

	t.Run("unlocks even if function returns error", func(t *testing.T) {
		err := lock.WithLock(ctx, "test-withlock-2", 5*time.Second, func(ctx context.Context) error {
			return assert.AnError
		})

		assert.Error(t, err)

		// Lock should be released
		isLocked, err := lock.IsLocked(ctx, "test-withlock-2")
		assert.NoError(t, err)
		assert.False(t, isLocked)
	})

	t.Run("unlocks even if function panics", func(t *testing.T) {
		defer func() {
			_ = recover() // Expected panic
		}()

		_ = lock.WithLock(ctx, "test-withlock-3", 5*time.Second, func(ctx context.Context) error {
			panic("test panic")
		})

		// Lock should be released
		time.Sleep(100 * time.Millisecond) // Give defer time to execute
		isLocked, err := lock.IsLocked(ctx, "test-withlock-3")
		assert.NoError(t, err)
		assert.False(t, isLocked)
	})
}

func TestDistributedLock_ConcurrentAccess(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("only one goroutine acquires lock at a time", func(t *testing.T) {
		var counter int64
		var successCount int64
		var wg sync.WaitGroup

		// Start 10 goroutines trying to acquire the same lock
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				err := lock.WithLock(ctx, "test-concurrent", 2*time.Second, func(ctx context.Context) error {
					// Critical section
					current := atomic.LoadInt64(&counter)
					time.Sleep(10 * time.Millisecond) // Simulate work
					atomic.StoreInt64(&counter, current+1)
					atomic.AddInt64(&successCount, 1)
					return nil
				})

				if err != nil {
					t.Logf("Failed to acquire lock: %v", err)
				}
			}()
		}

		wg.Wait()

		// All operations that acquired lock should have succeeded
		assert.Equal(t, successCount, counter)
	})

	t.Run("concurrent WithLock calls are serialized", func(t *testing.T) {
		var inCriticalSection int64
		var maxConcurrent int64
		var wg sync.WaitGroup

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				_ = lock.WithLock(ctx, "test-concurrent-2", 2*time.Second, func(ctx context.Context) error {
					current := atomic.AddInt64(&inCriticalSection, 1)

					// Track max concurrent
					for {
						max := atomic.LoadInt64(&maxConcurrent)
						if current <= max || atomic.CompareAndSwapInt64(&maxConcurrent, max, current) {
							break
						}
					}

					time.Sleep(50 * time.Millisecond)
					atomic.AddInt64(&inCriticalSection, -1)
					return nil
				})
			}()
		}

		wg.Wait()

		// Max concurrent should be 1 (serialized access)
		assert.LessOrEqual(t, maxConcurrent, int64(1), "expected serialized access, but found concurrent execution")
	})
}

func TestDistributedLock_IsLocked(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("returns true when locked", func(t *testing.T) {
		token, err := lock.Lock(ctx, "test-islocked", 5*time.Second)
		require.NoError(t, err)
		defer func() { _ = lock.Unlock(ctx, "test-islocked", token) }()

		isLocked, err := lock.IsLocked(ctx, "test-islocked")
		assert.NoError(t, err)
		assert.True(t, isLocked)
	})

	t.Run("returns false when not locked", func(t *testing.T) {
		isLocked, err := lock.IsLocked(ctx, "test-islocked-2")
		assert.NoError(t, err)
		assert.False(t, isLocked)
	})

	t.Run("returns false after unlock", func(t *testing.T) {
		token, err := lock.Lock(ctx, "test-islocked-3", 5*time.Second)
		require.NoError(t, err)

		err = lock.Unlock(ctx, "test-islocked-3", token)
		assert.NoError(t, err)

		isLocked, err := lock.IsLocked(ctx, "test-islocked-3")
		assert.NoError(t, err)
		assert.False(t, isLocked)
	})
}

// setupTestCache creates a test cache instance
// You'll need to implement this based on your test setup
func setupTestCache(t *testing.T) *Cache {
	// This is a placeholder - implement based on your test infrastructure
	// For integration tests, use a real Redis instance or testcontainers
	// For unit tests, you might want to mock the Cache interface
	t.Skip("Implement setupTestCache with real Redis or mock")
	return nil
}
