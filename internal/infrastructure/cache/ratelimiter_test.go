package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()

	allowed, err := rl.Allow(ctx, "test:allow", 5, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed, "first request within limit should be allowed")
}

func TestRateLimiter_Allow_ExceedsLimit(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()
	limit := 3

	// Make exactly `limit` requests -- all should be allowed.
	for i := 0; i < limit; i++ {
		allowed, err := rl.Allow(ctx, "test:exceed", limit, time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	// The next request should be denied.
	allowed, err := rl.Allow(ctx, "test:exceed", limit, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed, "request exceeding limit should be denied")
}

func TestRateLimiter_Allow_DifferentKeys(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()
	limit := 1

	// Exhaust the limit for key A.
	allowed, err := rl.Allow(ctx, "test:keyA", limit, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)

	denied, err := rl.Allow(ctx, "test:keyA", limit, time.Minute)
	require.NoError(t, err)
	assert.False(t, denied, "key A should be rate-limited")

	// Key B should still be allowed (independent counter).
	allowed, err = rl.Allow(ctx, "test:keyB", limit, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed, "key B should have its own independent limit")
}

func TestRateLimiter_Allow_WindowExpiry(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()
	limit := 2
	window := 10 * time.Second

	// Exhaust the limit.
	for i := 0; i < limit; i++ {
		allowed, err := rl.Allow(ctx, "test:expiry", limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	// Verify the limit is enforced.
	allowed, err := rl.Allow(ctx, "test:expiry", limit, window)
	require.NoError(t, err)
	assert.False(t, allowed)

	// Fast-forward past the window duration so the key expires.
	mr.FastForward(window + time.Second)

	// Now the limit should be reset and requests should be allowed again.
	allowed, err = rl.Allow(ctx, "test:expiry", limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "request should be allowed after window expiry")
}

func TestRateLimiter_Reset(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	rl := NewRateLimiter(c)
	ctx := context.Background()
	limit := 1

	// Exhaust the limit.
	allowed, err := rl.Allow(ctx, "test:reset", limit, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)

	denied, err := rl.Allow(ctx, "test:reset", limit, time.Minute)
	require.NoError(t, err)
	assert.False(t, denied)

	// Reset the counter.
	err = rl.Reset(ctx, "test:reset")
	require.NoError(t, err)

	// Requests should be allowed again.
	allowed, err = rl.Allow(ctx, "test:reset", limit, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed, "request should be allowed after reset")
}
