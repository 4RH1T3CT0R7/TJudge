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

	token, err := lock.Lock(ctx, "test-lock", 5*time.Second)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// занятый лок второй раз не берётся
	_, err = lock.Lock(ctx, "test-lock", 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock already held")

	require.NoError(t, lock.Unlock(ctx, "test-lock", token))
}

func TestDistributedLock_TryLock(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()
	lock := NewDistributedLock(cache)
	ctx := context.Background()

	// свободный лок берётся с первой попытки
	token, err := lock.TryLock(ctx, "test-trylock", 5*time.Second, 3, 50*time.Millisecond)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// пока держим — второй TryLock исчерпает попытки и вернёт ошибку с текстом
	_, err = lock.TryLock(ctx, "test-trylock", 5*time.Second, 2, 10*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to acquire lock after")

	_ = lock.Unlock(ctx, "test-trylock", token)
}

// Unlock: снять может только владелец правильным токеном. чужой токен лок
// не трогает, а снятие уже протухшего ключа — безопасный no-op
func TestDistributedLock_Unlock(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()
	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("правильный токен снимает лок", func(t *testing.T) {
		token, err := lock.Lock(ctx, "test-unlock", 5*time.Second)
		require.NoError(t, err)

		require.NoError(t, lock.Unlock(ctx, "test-unlock", token))

		locked, err := lock.IsLocked(ctx, "test-unlock")
		require.NoError(t, err)
		assert.False(t, locked)
	})

	t.Run("чужой токен не снимает лок", func(t *testing.T) {
		// A держит лок
		tokenA, err := lock.Lock(ctx, "test-unlock-2", 5*time.Second)
		require.NoError(t, err)

		// B пытается снять чужим токеном — облом, ключ жив
		err = lock.Unlock(ctx, "test-unlock-2", "wrong-token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lock token mismatch")

		// лок всё ещё за A, B захватить не может
		_, err = lock.Lock(ctx, "test-unlock-2", 5*time.Second)
		assert.Error(t, err)

		// A снимает своим токеном, теперь B заходит
		require.NoError(t, lock.Unlock(ctx, "test-unlock-2", tokenA))
		tokenB, err := lock.Lock(ctx, "test-unlock-2", 5*time.Second)
		require.NoError(t, err)
		_ = lock.Unlock(ctx, "test-unlock-2", tokenB)
	})

	t.Run("снятие протухшего лока это no-op", func(t *testing.T) {
		cacheMR, mr := setupTestCacheWithMR(t)
		defer cacheMR.Close()
		lockMR := NewDistributedLock(cacheMR)

		token, err := lockMR.Lock(ctx, "test-unlock-3", 100*time.Millisecond)
		require.NoError(t, err)

		// ttl протух, ключа уже нет
		mr.FastForward(200 * time.Millisecond)

		// снятие пропавшего ключа не ошибка
		require.NoError(t, lockMR.Unlock(ctx, "test-unlock-3", token))
	})
}

// WithLock держит лок пока работает fn и снимает после — на любом исходе.
// фоновое продление не даёт локу протухнуть на долгой операции
func TestDistributedLock_WithLock(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()
	lock := NewDistributedLock(cache)
	ctx := context.Background()

	t.Run("выполняет fn и снимает лок", func(t *testing.T) {
		executed := false
		err := lock.WithLock(ctx, "test-withlock", 5*time.Second, func(ctx context.Context) error {
			executed = true
			return nil
		})
		require.NoError(t, err)
		assert.True(t, executed)

		locked, err := lock.IsLocked(ctx, "test-withlock")
		require.NoError(t, err)
		assert.False(t, locked)
	})

	t.Run("снимает лок если fn вернула ошибку", func(t *testing.T) {
		err := lock.WithLock(ctx, "test-withlock-2", 5*time.Second, func(ctx context.Context) error {
			return assert.AnError
		})
		assert.Error(t, err)

		locked, err := lock.IsLocked(ctx, "test-withlock-2")
		require.NoError(t, err)
		assert.False(t, locked)
	})

	t.Run("снимает лок если fn паникует", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r, "паника должна пробросится наружу")

			locked, err := lock.IsLocked(ctx, "test-withlock-3")
			require.NoError(t, err)
			assert.False(t, locked)
		}()

		_ = lock.WithLock(ctx, "test-withlock-3", 5*time.Second, func(ctx context.Context) error {
			panic("test panic")
		})
	})

	t.Run("продление держит лок дольше ttl", func(t *testing.T) {
		// ttl 600мс, fn работает 750мс. без продления лок бы протух посреди
		// работы, но renewLoop тикает и продлевает пока fn не вернётся
		start := time.Now()
		err := lock.WithLock(ctx, "test-renew", 600*time.Millisecond, func(ctx context.Context) error {
			time.Sleep(750 * time.Millisecond)
			return nil
		})
		require.NoError(t, err)
		assert.Greater(t, time.Since(start), 700*time.Millisecond)

		// после возврата fn лок снят
		locked, err := lock.IsLocked(ctx, "test-renew")
		require.NoError(t, err)
		assert.False(t, locked)
	})
}

func TestDistributedLock_ConcurrentAccess(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()
	lock := NewDistributedLock(cache)
	ctx := context.Background()

	// 5 горутин лезут в один лок. в критической секции одновременно
	// должна быть максимум одна
	var inCriticalSection int64
	var maxConcurrent int64
	var wg sync.WaitGroup

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = lock.WithLock(ctx, "test-concurrent", 2*time.Second, func(ctx context.Context) error {
				current := atomic.AddInt64(&inCriticalSection, 1)
				for {
					m := atomic.LoadInt64(&maxConcurrent)
					if current <= m || atomic.CompareAndSwapInt64(&maxConcurrent, m, current) {
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

	assert.LessOrEqual(t, maxConcurrent, int64(1), "доступ должен быть сериализован")
}
