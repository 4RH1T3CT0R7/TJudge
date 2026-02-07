package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_Get_Miss(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	val, err := c.Get(ctx, "nonexistent-key")

	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestCache_Set_Get_RoundTrip(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "test-key", "test-value", 5*time.Minute)
	require.NoError(t, err)

	val, err := c.Get(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "test-value", val)
}

func TestCache_Get_Hit(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "hit-key", "hit-value", time.Minute))

	val, err := c.Get(ctx, "hit-key")
	require.NoError(t, err)
	assert.Equal(t, "hit-value", val)
}

func TestCache_Del(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "del-key", "value", time.Minute))

	err := c.Del(ctx, "del-key")
	require.NoError(t, err)

	val, err := c.Get(ctx, "del-key")
	require.NoError(t, err)
	assert.Equal(t, "", val) // Gone
}

func TestCache_Exists_True(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "exists-key", "value", time.Minute))

	exists, err := c.Exists(ctx, "exists-key")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCache_Exists_False(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	exists, err := c.Exists(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCache_Expire_FastForward(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "expire-key", "value", 0)) // No TTL
	require.NoError(t, c.Expire(ctx, "expire-key", 2*time.Second))

	// Key exists before expiry
	exists, err := c.Exists(ctx, "expire-key")
	require.NoError(t, err)
	assert.True(t, exists)

	// Fast forward past TTL
	mr.FastForward(3 * time.Second)

	exists, err = c.Exists(ctx, "expire-key")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCache_RPop_EmptyList(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	val, err := c.RPop(ctx, "empty-list")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestCache_LPush_RPop(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "list-key", "item1", "item2"))

	val, err := c.RPop(ctx, "list-key")
	require.NoError(t, err)
	assert.Equal(t, "item1", val)

	val, err = c.RPop(ctx, "list-key")
	require.NoError(t, err)
	assert.Equal(t, "item2", val)
}

func TestCache_LPush_LRange(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "range-list", "a", "b", "c"))

	result, err := c.LRange(ctx, "range-list", 0, -1)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestCache_LLen(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "len-list", "a", "b"))

	length, err := c.LLen(ctx, "len-list")
	require.NoError(t, err)
	assert.Equal(t, int64(2), length)
}

func TestCache_Health(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	err := c.Health(ctx)
	assert.NoError(t, err)
}

func TestCache_Publish(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	err := c.Publish(ctx, "test-channel", "test-message")
	assert.NoError(t, err)
}

func TestCache_SetNX(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	// First SetNX should succeed
	ok, err := c.SetNX(ctx, "nx-key", "value", time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)

	// Second SetNX should fail
	ok, err = c.SetNX(ctx, "nx-key", "value2", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)

	// Value should be the first one
	val, err := c.Get(ctx, "nx-key")
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestCache_ZAdd_ZRevRangeWithScores(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.ZAdd(ctx, "zset", 100, "player1"))
	require.NoError(t, c.ZAdd(ctx, "zset", 200, "player2"))

	result, err := c.ZRevRangeWithScores(ctx, "zset", 0, -1)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "player2", result[0].Member) // Higher score first
}

func TestCache_ZRem(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.ZAdd(ctx, "zrem-set", 100, "player1"))

	err := c.ZRem(ctx, "zrem-set", "player1")
	require.NoError(t, err)

	result, err := c.ZRevRangeWithScores(ctx, "zrem-set", 0, -1)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestCache_ZIncrBy(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.ZAdd(ctx, "incr-set", 100, "player1"))
	require.NoError(t, c.ZIncrBy(ctx, "incr-set", 50, "player1"))

	result, err := c.ZRevRangeWithScores(ctx, "incr-set", 0, -1)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, float64(150), result[0].Score)
}
