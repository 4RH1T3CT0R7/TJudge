package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow_ExceedsLimit(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()
	limit := 3

	// ровно limit запросов проходят
	for i := range limit {
		allowed, err := rl.Allow(ctx, "test:exceed", limit, time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed, "запрос %d должен пройти", i+1)
	}

	// следующий за лимитом отклоняется
	allowed, err := rl.Allow(ctx, "test:exceed", limit, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestRateLimiter_Allow_DifferentKeys(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()

	// выбираем лимит для ключа A
	allowed, err := rl.Allow(ctx, "test:keyA", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)

	denied, err := rl.Allow(ctx, "test:keyA", 1, time.Minute)
	require.NoError(t, err)
	assert.False(t, denied)

	// у ключа B свой счётчик
	allowed, err = rl.Allow(ctx, "test:keyB", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiter_Allow_WindowExpiry(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()
	limit := 2
	window := 10 * time.Second

	for range limit {
		allowed, err := rl.Allow(ctx, "test:expiry", limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	allowed, err := rl.Allow(ctx, "test:expiry", limit, window)
	require.NoError(t, err)
	assert.False(t, allowed)

	// ttl окна ставится на первом запросе -> после проматывания счётчик сброшен
	mr.FastForward(window + time.Second)

	allowed, err = rl.Allow(ctx, "test:expiry", limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "после истечения окна снова можно")
}

func TestRateLimiter_AllowWithIncr_ExceedsLimit(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()
	limit := 2

	for i := range limit {
		allowed, err := rl.AllowWithIncr(ctx, "test:incr-exceed", limit, time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed, "запрос %d должен пройти", i+1)
	}

	allowed, err := rl.AllowWithIncr(ctx, "test:incr-exceed", limit, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestRateLimiter_Reset(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()

	allowed, err := rl.Allow(ctx, "test:reset", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)

	denied, err := rl.Allow(ctx, "test:reset", 1, time.Minute)
	require.NoError(t, err)
	assert.False(t, denied)

	require.NoError(t, rl.Reset(ctx, "test:reset"))

	allowed, err = rl.Allow(ctx, "test:reset", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed, "после reset снова можно")
}
