package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_GetMiss(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	val, err := c.Get(ctx, "nonexistent-key")

	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestCache_SetGet(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "test-key", "test-value", 5*time.Minute)
	require.NoError(t, err)

	val, err := c.Get(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "test-value", val)
}

func TestCache_Del(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "k1", "v1", time.Minute))
	require.NoError(t, c.Set(ctx, "k2", "v2", time.Minute))

	err := c.Del(ctx, "k1", "k2")
	require.NoError(t, err)

	for _, key := range []string{"k1", "k2"} {
		exists, err := c.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists)
	}
}

func TestCache_Exists(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "exists-key", "value", time.Minute))

	exists, err := c.Exists(ctx, "exists-key")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = c.Exists(ctx, "nope")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCache_Expire(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "expire-key", "value", 0)) // без ttl
	require.NoError(t, c.Expire(ctx, "expire-key", 2*time.Second))

	exists, err := c.Exists(ctx, "expire-key")
	require.NoError(t, err)
	assert.True(t, exists)

	mr.FastForward(3 * time.Second)

	exists, err = c.Exists(ctx, "expire-key")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCache_SetNX(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	// первый setnx проходит, второй нет
	ok, err := c.SetNX(ctx, "nx-key", "value", time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.SetNX(ctx, "nx-key", "value2", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)

	val, err := c.Get(ctx, "nx-key")
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestCache_Incr(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	val, err := c.Incr(ctx, "incr-key")
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = c.Incr(ctx, "incr-key")
	require.NoError(t, err)
	assert.Equal(t, int64(2), val)
}

// --- списки ---

func TestCache_LPushRPop(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "list-key", "item1", "item2"))

	val, err := c.RPop(ctx, "list-key")
	require.NoError(t, err)
	assert.Equal(t, "item1", val)

	val, err = c.RPop(ctx, "list-key")
	require.NoError(t, err)
	assert.Equal(t, "item2", val)

	// пустой список отдаёт ""
	val, err = c.RPop(ctx, "empty-list")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestCache_LRangeLLenLTrim(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "range-list", "a", "b", "c", "d"))

	items, err := c.LRange(ctx, "range-list", 0, -1)
	require.NoError(t, err)
	assert.Len(t, items, 4)

	length, err := c.LLen(ctx, "range-list")
	require.NoError(t, err)
	assert.Equal(t, int64(4), length)

	require.NoError(t, c.LTrim(ctx, "range-list", 0, 1))
	length, err = c.LLen(ctx, "range-list")
	require.NoError(t, err)
	assert.Equal(t, int64(2), length)
}

func TestCache_BRPop(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "brpop-list", "hello"))

	res, err := c.BRPop(ctx, 1*time.Second, "brpop-list")
	require.NoError(t, err)
	assert.Equal(t, []string{"brpop-list", "hello"}, res)
}

func TestCache_ReplaceList(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, "replace-list", "old1", "old2"))

	err := c.ReplaceList(ctx, "replace-list", [][]byte{[]byte("new1"), []byte("new2")})
	require.NoError(t, err)

	res, err := c.LRange(ctx, "replace-list", 0, -1)
	require.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Contains(t, res, "new1")
	assert.Contains(t, res, "new2")
}

func TestCache_BatchLPush(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	items := map[string][]any{
		"batch-list-a": {[]byte("a1"), []byte("a2")},
		"batch-list-b": {[]byte("b1")},
	}

	err := c.BatchLPush(ctx, items)
	require.NoError(t, err)

	lenA, err := c.LLen(ctx, "batch-list-a")
	require.NoError(t, err)
	assert.Equal(t, int64(2), lenA)

	lenB, err := c.LLen(ctx, "batch-list-b")
	require.NoError(t, err)
	assert.Equal(t, int64(1), lenB)

	// пустой вход не должен падать
	require.NoError(t, c.BatchLPush(ctx, nil))
}

// --- sorted set ---

func TestCache_ZAddZRevRange(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.ZAdd(ctx, "zset", 100, "player1"))
	require.NoError(t, c.ZAdd(ctx, "zset", 200, "player2"))

	res, err := c.ZRevRangeWithScores(ctx, "zset", 0, -1)
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, "player2", res[0].Member) // больший скор первым

	// убрали одного - остался второй
	require.NoError(t, c.ZRem(ctx, "zset", "player1"))
	res, err = c.ZRevRangeWithScores(ctx, "zset", 0, -1)
	require.NoError(t, err)
	require.Len(t, res, 1)

	// пустой zset -> пустая выдача
	res, err = c.ZRevRangeWithScores(ctx, "nope-zset", 0, -1)
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestCache_ZIncrBy(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.ZAdd(ctx, "incr-set", 100, "player1"))
	require.NoError(t, c.ZIncrBy(ctx, "incr-set", 50, "player1"))

	res, err := c.ZRevRangeWithScores(ctx, "incr-set", 0, -1)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, float64(150), res[0].Score)
}

func TestCache_SAddSRem(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	count, err := c.SAdd(ctx, "test-set", "member1", "member2")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// повторный member даёт 0
	count, err = c.SAdd(ctx, "test-set", "member1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.NoError(t, c.SRem(ctx, "test-set", "member1"))
}

// --- eval / scan / pubsub / служебное ---

func TestCache_Eval(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	res, err := c.Eval(ctx, "return 1+1", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), res)

	require.NoError(t, c.Set(ctx, "eval-key", "hello", time.Minute))
	res, err = c.Eval(ctx, `return redis.call("GET", KEYS[1])`, []string{"eval-key"})
	require.NoError(t, err)
	assert.Equal(t, "hello", res)
}

func TestCache_Scan(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "scan:a", "1", time.Minute))
	require.NoError(t, c.Set(ctx, "scan:b", "2", time.Minute))
	require.NoError(t, c.Set(ctx, "other:c", "3", time.Minute))

	keys := scanAll(t, c, "scan:*")
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "scan:a")
	assert.Contains(t, keys, "scan:b")

	// паттерн без совпадений
	assert.Empty(t, scanAll(t, c, "nonexistent:*"))
}

// scanAll гоняет курсор до конца и собирает ключи
func scanAll(t *testing.T, c *Cache, pattern string) []string {
	t.Helper()
	var all []string
	var cursor uint64
	for {
		keys, next, err := c.Scan(context.Background(), cursor, pattern, 100)
		require.NoError(t, err)
		all = append(all, keys...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return all
}

func TestCache_HealthPublishSubscribe(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	assert.NoError(t, c.Health(ctx))
	assert.NoError(t, c.Publish(ctx, "test-channel", "test-message"))

	sub := c.Subscribe(ctx, "test-chan")
	require.NotNil(t, sub)
	_ = sub.Close()
}

func TestCache_Close(t *testing.T) {
	c := setupTestCache(t)

	assert.NoError(t, c.Close())
}

// гоняем set/get/del параллельно, интересует детектор гонок
func TestCache_ConcurrentReadWrite(t *testing.T) {
	c := setupTestCache(t)
	ctx := context.Background()

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range iterations {
				key := fmt.Sprintf("ck-%d-%d", id, i)
				val := fmt.Sprintf("v-%d-%d", id, i)

				assert.NoError(t, c.Set(ctx, key, val, 5*time.Minute))

				got, err := c.Get(ctx, key)
				assert.NoError(t, err)
				assert.Equal(t, val, got)

				assert.NoError(t, c.Del(ctx, key))
			}
		}(g)
	}

	wg.Wait()
}

// когда редис недоступен, операции должны возвращать ошибку а не паниковать
func TestCache_ErrorPaths(t *testing.T) {
	mr := miniredis.RunT(t)

	// MaxRetries -1 и короткий диалтаймаут чтобы упавшее соединение
	// сразу отдавало ошибку, а не ретраилось
	client := redis.NewClient(&redis.Options{
		Addr:        mr.Addr(),
		MaxRetries:  -1,
		DialTimeout: 10 * time.Millisecond,
		PoolSize:    1,
	})
	log, _ := logger.New("error", "json")
	c := &Cache{client: client, log: log, metrics: metrics.New()}
	ctx := context.Background()

	mr.Close() // роняем редис

	_, err := c.Get(ctx, "key")
	assert.Error(t, err)

	assert.Error(t, c.Set(ctx, "key", "value", time.Minute))
	assert.Error(t, c.Del(ctx, "key"))
	assert.Error(t, c.ZAdd(ctx, "key", 1.0, "member"))
	assert.Error(t, c.LPush(ctx, "key", "value"))
	assert.Error(t, c.Health(ctx))

	_, err = c.Eval(ctx, "return 1", nil)
	assert.Error(t, err)
}
