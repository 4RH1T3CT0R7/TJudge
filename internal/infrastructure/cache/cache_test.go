package cache

import (
	"context"
	"fmt"
	"sync"
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

func TestCache_ReplaceList(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "replace-list", "old1", "old2"))

	err := c.ReplaceList(ctx, "replace-list", [][]byte{[]byte("new1"), []byte("new2")})
	require.NoError(t, err)

	result, err := c.LRange(ctx, "replace-list", 0, -1)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Contains(t, result, "new1")
	assert.Contains(t, result, "new2")
}

func TestCache_ReplaceList_Empty(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "replace-empty", "old1", "old2"))

	err := c.ReplaceList(ctx, "replace-empty", [][]byte{})
	require.NoError(t, err)

	length, err := c.LLen(ctx, "replace-empty")
	require.NoError(t, err)
	assert.Equal(t, int64(0), length)
}

func TestCache_Eval_SimpleScript(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	result, err := c.Eval(ctx, "return 1+1", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result)
}

func TestCache_Scan_WithPattern(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "scan:a", "1", time.Minute))
	require.NoError(t, c.Set(ctx, "scan:b", "2", time.Minute))
	require.NoError(t, c.Set(ctx, "other:c", "3", time.Minute))

	var allKeys []string
	var cursor uint64
	for {
		keys, nextCursor, err := c.Scan(ctx, cursor, "scan:*", 100)
		require.NoError(t, err)
		allKeys = append(allKeys, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	assert.Len(t, allKeys, 2)
	assert.Contains(t, allKeys, "scan:a")
	assert.Contains(t, allKeys, "scan:b")
}

func TestCache_Scan_NoResults(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	var allKeys []string
	var cursor uint64
	for {
		keys, nextCursor, err := c.Scan(ctx, cursor, "nonexistent:*", 100)
		require.NoError(t, err)
		allKeys = append(allKeys, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	assert.Empty(t, allKeys)
}

func TestCache_Close(t *testing.T) {
	c := setupTestCache(t)

	err := c.Close()
	assert.NoError(t, err)
}

func TestCache_Del_Multiple(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "k1", "v1", time.Minute))
	require.NoError(t, c.Set(ctx, "k2", "v2", time.Minute))
	require.NoError(t, c.Set(ctx, "k3", "v3", time.Minute))

	err := c.Del(ctx, "k1", "k2", "k3")
	require.NoError(t, err)

	for _, key := range []string{"k1", "k2", "k3"} {
		exists, err := c.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists, "key %s should be deleted", key)
	}
}

func TestCache_Set_TTL(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "ttl-key", "value", 100*time.Millisecond))

	exists, err := c.Exists(ctx, "ttl-key")
	require.NoError(t, err)
	assert.True(t, exists)

	mr.FastForward(200 * time.Millisecond)

	exists, err = c.Exists(ctx, "ttl-key")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCache_ZRevRangeWithScores_Empty(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	result, err := c.ZRevRangeWithScores(ctx, "nonexistent-zset", 0, -1)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestCache_LPush_LLen_Multiple(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "multi-list", "item1"))
	length, err := c.LLen(ctx, "multi-list")
	require.NoError(t, err)
	assert.Equal(t, int64(1), length)

	require.NoError(t, c.LPush(ctx, "multi-list", "item2"))
	length, err = c.LLen(ctx, "multi-list")
	require.NoError(t, err)
	assert.Equal(t, int64(2), length)

	require.NoError(t, c.LPush(ctx, "multi-list", "item3"))
	length, err = c.LLen(ctx, "multi-list")
	require.NoError(t, err)
	assert.Equal(t, int64(3), length)
}

func TestCache_BRPop_WithItem(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "brpop-list", "hello"))

	result, err := c.BRPop(ctx, 1*time.Second, "brpop-list")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []string{"brpop-list", "hello"}, result)
}

// --- Concurrency / Race Detection Tests ---

func TestCache_ConcurrentReadWrite(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("concurrent-key-%d-%d", id, i)
				value := fmt.Sprintf("value-%d-%d", id, i)

				require.NotPanics(t, func() {
					err := c.Set(ctx, key, value, 5*time.Minute)
					assert.NoError(t, err)

					got, err := c.Get(ctx, key)
					assert.NoError(t, err)
					assert.Equal(t, value, got)

					err = c.Del(ctx, key)
					assert.NoError(t, err)
				})
			}
		}(g)
	}

	wg.Wait()
}

func TestCache_ConcurrentSortedSet(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	const goroutines = 5
	const iterations = 100
	const zsetKey = "concurrent-zset"

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				member := fmt.Sprintf("member-%d-%d", id, i)
				score := float64(id*iterations + i)

				require.NotPanics(t, func() {
					err := c.ZAdd(ctx, zsetKey, score, member)
					assert.NoError(t, err)

					results, err := c.ZRevRangeWithScores(ctx, zsetKey, 0, 9)
					assert.NoError(t, err)
					assert.NotNil(t, results)
				})
			}
		}(g)
	}

	wg.Wait()

	// After all goroutines finish, the sorted set should contain all members.
	results, err := c.ZRevRangeWithScores(ctx, zsetKey, 0, -1)
	require.NoError(t, err)
	assert.Equal(t, goroutines*iterations, len(results))
}
