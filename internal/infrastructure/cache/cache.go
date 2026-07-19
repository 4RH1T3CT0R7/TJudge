package cache

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Cache struct {
	client  *redis.Client
	log     *logger.Logger
	metrics *metrics.Metrics
}

// New - подключение к редису с ретраями (см. ниже)
func New(cfg *config.RedisConfig, log *logger.Logger, m *metrics.Metrics) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// после рестарта редис отвечает LOADING пока грузит aof, или ещё не поднялся.
	// это на пару секунд, поэтому ждём а не падаем сразу (иначе роняло api/worker
	// каскадом на каждом рестарте редиса)
	const (
		connectTimeout = 60 * time.Second
		retryInterval  = 2 * time.Second
	)
	deadline := time.Now().Add(connectTimeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.Ping(ctx).Err()
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("failed to connect to redis after %s: %w", connectTimeout, err)
		}
		log.Warn("Redis недоступен, повтор подключения",
			zap.Error(err),
			zap.Duration("retry_in", retryInterval),
		)
		time.Sleep(retryInterval)
	}

	log.Info("Redis connected successfully",
		zap.String("addr", cfg.Address()),
		zap.Int("db", cfg.DB),
	)

	if m != nil {
		m.PrimeCacheType("get", "zrevrange")
	}

	return &Cache{
		client:  client,
		log:     log,
		metrics: m,
	}, nil
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()

	if stderrors.Is(err, redis.Nil) {
		c.metrics.RecordCacheMiss("get")
		return "", nil
	}

	if err != nil {
		c.log.LogError("Redis GET failed", err, zap.String("key", key))
		return "", err
	}

	c.metrics.RecordCacheHit("get")
	return val, nil
}

func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	err := c.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		c.log.LogError("Redis SET failed", err, zap.String("key", key))
		return err
	}
	return nil
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	err := c.client.Del(ctx, keys...).Err()
	if err != nil {
		c.log.LogError("Redis DEL failed", err)
		return err
	}
	return nil
}

func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.log.LogError("Redis EXISTS failed", err, zap.String("key", key))
		return false, err
	}
	return count > 0, nil
}

func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	err := c.client.Expire(ctx, key, ttl).Err()
	if err != nil {
		c.log.LogError("Redis EXPIRE failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// Incr - атомарный +1, создаёт ключ если не было
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		c.log.LogError("Redis INCR failed", err, zap.String("key", key))
		return 0, err
	}
	return val, nil
}

func (c *Cache) ZAdd(ctx context.Context, key string, score float64, member string) error {
	err := c.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: member,
	}).Err()

	if err != nil {
		c.log.LogError("Redis ZADD failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// ZAddBatchMember - элемент для BatchZAdd
type ZAddBatchMember struct {
	Key    string
	Score  float64
	Member string
}

// BatchZAdd - N элементов одним пайплайном (рейтинг после матча кладёт две ZADD за раз)
func (c *Cache) BatchZAdd(ctx context.Context, members []ZAddBatchMember) error {
	if len(members) == 0 {
		return nil
	}
	pipe := c.client.Pipeline()
	for _, item := range members {
		pipe.ZAdd(ctx, item.Key, redis.Z{Score: item.Score, Member: item.Member})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		c.log.LogError("Redis pipelined ZADD failed", err, zap.Int("batch_size", len(members)))
		return err
	}
	return nil
}

func (c *Cache) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	result, err := c.client.ZRevRangeWithScores(ctx, key, start, stop).Result()

	if stderrors.Is(err, redis.Nil) {
		c.metrics.RecordCacheMiss("zrevrange")
		return []redis.Z{}, nil
	}

	if err != nil {
		c.log.LogError("Redis ZREVRANGE failed", err, zap.String("key", key))
		return nil, err
	}

	c.metrics.RecordCacheHit("zrevrange")
	return result, nil
}

func (c *Cache) ZIncrBy(ctx context.Context, key string, increment float64, member string) error {
	err := c.client.ZIncrBy(ctx, key, increment, member).Err()
	if err != nil {
		c.log.LogError("Redis ZINCRBY failed", err, zap.String("key", key))
		return err
	}
	return nil
}

func (c *Cache) ZRem(ctx context.Context, key string, members ...string) error {
	err := c.client.ZRem(ctx, key, members).Err()
	if err != nil {
		c.log.LogError("Redis ZREM failed", err, zap.String("key", key))
		return err
	}
	return nil
}

func (c *Cache) LPush(ctx context.Context, key string, values ...any) error {
	err := c.client.LPush(ctx, key, values...).Err()
	if err != nil {
		c.log.LogError("Redis LPUSH failed", err, zap.String("key", key))
		return err
	}
	return nil
}

func (c *Cache) RPop(ctx context.Context, key string) (string, error) {
	val, err := c.client.RPop(ctx, key).Result()
	if stderrors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		c.log.LogError("Redis RPOP failed", err, zap.String("key", key))
		return "", err
	}
	return val, nil
}

// BRPop блокирующее удаление последнего элемента из списка
func (c *Cache) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	result, err := c.client.BRPop(ctx, timeout, keys...).Result()
	if stderrors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		// отмена контекста - это штатный graceful shutdown (заблокированным
		// горутинам пула прилетает cancel), не логируем чтобы не пугать доктора
		if stderrors.Is(err, context.Canceled) {
			return nil, err
		}
		c.log.LogError("Redis BRPOP failed", err)
		return nil, err
	}
	return result, nil
}

func (c *Cache) LLen(ctx context.Context, key string) (int64, error) {
	length, err := c.client.LLen(ctx, key).Result()
	if err != nil {
		c.log.LogError("Redis LLEN failed", err, zap.String("key", key))
		return 0, err
	}
	return length, nil
}

func (c *Cache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	result, err := c.client.LRange(ctx, key, start, stop).Result()
	if err != nil {
		c.log.LogError("Redis LRANGE failed", err, zap.String("key", key))
		return nil, err
	}
	return result, nil
}

func (c *Cache) LTrim(ctx context.Context, key string, start, stop int64) error {
	err := c.client.LTrim(ctx, key, start, stop).Err()
	if err != nil {
		c.log.LogError("Redis LTRIM failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// SAdd - добавить в set, возвращает сколько новых
func (c *Cache) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	count, err := c.client.SAdd(ctx, key, members...).Result()
	if err != nil {
		c.log.LogError("Redis SADD failed", err, zap.String("key", key))
		return 0, err
	}
	return count, nil
}

// SAddWithExpire - добавить в set и выставить ttl одним lua, чтобы атомарно. ttl >= 1с
func (c *Cache) SAddWithExpire(ctx context.Context, key string, ttl time.Duration, members ...any) (int64, error) {
	if len(members) == 0 {
		return 0, nil
	}

	script := `
local added = redis.call("SADD", KEYS[1], unpack(ARGV, 2))
redis.call("EXPIRE", KEYS[1], ARGV[1])
return added
`
	ttlSec := int(ttl.Seconds())
	if ttlSec <= 0 {
		return 0, fmt.Errorf("SAddWithExpire requires TTL >= 1 second, got %v", ttl)
	}

	args := make([]any, 0, 1+len(members))
	args = append(args, ttlSec)
	args = append(args, members...)

	result, err := c.client.Eval(ctx, script, []string{key}, args...).Result()
	if err != nil {
		c.log.LogError("Redis SAddWithExpire failed", err, zap.String("key", key))
		return 0, err
	}
	count, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected result type from SAddWithExpire: %T", result)
		c.log.LogError("Redis SAddWithExpire unexpected result type", err, zap.String("key", key))
		return 0, err
	}
	return count, nil
}

func (c *Cache) SRem(ctx context.Context, key string, members ...any) error {
	err := c.client.SRem(ctx, key, members...).Err()
	if err != nil {
		c.log.LogError("Redis SREM failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// SetNX - выставить только если ключа нет (нужно для локов)
func (c *Cache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	result, err := c.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		c.log.LogError("Redis SETNX failed", err, zap.String("key", key))
		return false, err
	}
	return result, nil
}

// BatchSetNX - пачка SetNX пайплайном, true = ключ новый
func (c *Cache) BatchSetNX(ctx context.Context, keys map[string]any, ttl time.Duration) (map[string]bool, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	pipe := c.client.Pipeline()
	cmds := make(map[string]*redis.BoolCmd, len(keys))
	for key, value := range keys {
		cmds[key] = pipe.SetNX(ctx, key, value, ttl)
	}

	_, pipeErr := pipe.Exec(ctx)
	// даже если pipeline ошибся, собираем что успело выполниться -
	// на этом держится откат dedup в очереди
	results := make(map[string]bool, len(cmds))
	for key, cmd := range cmds {
		val, cmdErr := cmd.Result()
		if cmdErr != nil {
			continue
		}
		results[key] = val
	}

	if pipeErr != nil && len(results) == 0 {
		c.log.LogError("Redis BatchSetNX pipeline failed completely", pipeErr)
		return nil, pipeErr
	}

	return results, pipeErr
}

func (c *Cache) Publish(ctx context.Context, channel string, message any) error {
	err := c.client.Publish(ctx, channel, message).Err()
	if err != nil {
		c.log.LogError("Redis PUBLISH failed", err, zap.String("channel", channel))
		return err
	}
	return nil
}

func (c *Cache) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// ReplaceList - целиком заменить список в одной транзакции (del + lpush в MULTI/EXEC)
func (c *Cache) ReplaceList(ctx context.Context, key string, values [][]byte) error {
	pipe := c.client.TxPipeline()
	pipe.Del(ctx, key)
	if len(values) > 0 {
		args := make([]any, len(values))
		for i, v := range values {
			args[i] = v
		}
		pipe.LPush(ctx, key, args...)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.log.LogError("Redis ReplaceList failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// BatchLPush - разложить значения по спискам одним пайплайном
func (c *Cache) BatchLPush(ctx context.Context, items map[string][]any) error {
	if len(items) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	for key, values := range items {
		for _, v := range values {
			pipe.LPush(ctx, key, v)
		}
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.log.LogError("Redis BatchLPush failed", err)
		return err
	}
	return nil
}

func (c *Cache) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return c.client.Ping(ctx).Err()
}

func (c *Cache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	result, err := c.client.Eval(ctx, script, keys, args...).Result()
	if err != nil {
		c.log.LogError("Redis EVAL failed", err)
		return nil, err
	}
	return result, nil
}

func (c *Cache) Scan(ctx context.Context, cursor uint64, pattern string, count int64) ([]string, uint64, error) {
	keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, count).Result()
	if err != nil {
		c.log.LogError("Redis SCAN failed", err, zap.String("pattern", pattern))
		return nil, 0, err
	}
	return keys, nextCursor, nil
}

func (c *Cache) Close() error {
	c.log.Info("Closing Redis connection")
	return c.client.Close()
}
