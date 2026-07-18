package cache

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Cache оборачивает Redis клиент и добавляет метрики
type Cache struct {
	client  *redis.Client
	log     *logger.Logger
	metrics *metrics.Metrics
}

// New создаёт новое подключение к Redis
func New(cfg *config.RedisConfig, log *logger.Logger, m *metrics.Metrics) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// Проверяем соединение с ретраями: после рестарта Redis отвечает
	// LOADING (грузит AOF в память), при старте хоста может быть ещё
	// недоступен. Это транзиентные состояния на секунды - ждать правильнее,
	// чем мгновенно падать fatal'ом (роняло api/worker каскадом при
	// каждом рестарте Redis).
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

// Get получает значение по ключу
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

// Set устанавливает значение с TTL
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	err := c.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		c.log.LogError("Redis SET failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// Del удаляет ключ
func (c *Cache) Del(ctx context.Context, keys ...string) error {
	err := c.client.Del(ctx, keys...).Err()
	if err != nil {
		c.log.LogError("Redis DEL failed", err)
		return err
	}
	return nil
}

// Exists проверяет существование ключа
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.log.LogError("Redis EXISTS failed", err, zap.String("key", key))
		return false, err
	}
	return count > 0, nil
}

// Expire устанавливает TTL для ключа
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	err := c.client.Expire(ctx, key, ttl).Err()
	if err != nil {
		c.log.LogError("Redis EXPIRE failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// Incr атомарно увеличивает значение ключа на 1. Создаёт ключ со значением 1 если не существует.
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		c.log.LogError("Redis INCR failed", err, zap.String("key", key))
		return 0, err
	}
	return val, nil
}

// ZAdd добавляет элемент в sorted set
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

// ZAddBatchMember - один элемент для BatchZAdd.
type ZAddBatchMember struct {
	Key    string
	Score  float64
	Member string
}

// BatchZAdd добавляет N элементов через Redis-пайплайн за один RTT.
// Используется для rating-апдейтов после матча (две ZADD за раз),
// а также для массовых инвалидаций/перекачек.
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

// ZRevRangeWithScores получает элементы из sorted set в обратном порядке
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

// ZIncrBy увеличивает score элемента в sorted set
func (c *Cache) ZIncrBy(ctx context.Context, key string, increment float64, member string) error {
	err := c.client.ZIncrBy(ctx, key, increment, member).Err()
	if err != nil {
		c.log.LogError("Redis ZINCRBY failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// ZRem удаляет элемент из sorted set
func (c *Cache) ZRem(ctx context.Context, key string, members ...string) error {
	err := c.client.ZRem(ctx, key, members).Err()
	if err != nil {
		c.log.LogError("Redis ZREM failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// LPush добавляет элемент в начало списка
func (c *Cache) LPush(ctx context.Context, key string, values ...any) error {
	err := c.client.LPush(ctx, key, values...).Err()
	if err != nil {
		c.log.LogError("Redis LPUSH failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// RPop удаляет и возвращает последний элемент списка
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
		// Отмена контекста - штатный graceful shutdown (каждая заблокированная
		// горутина пула получает cancel); не считаем ошибкой, чтобы не
		// засыпать логи и доктора ложными "Redis BRPOP failed" на рестартах.
		if stderrors.Is(err, context.Canceled) {
			return nil, err
		}
		c.log.LogError("Redis BRPOP failed", err)
		return nil, err
	}
	return result, nil
}

// LLen возвращает длину списка
func (c *Cache) LLen(ctx context.Context, key string) (int64, error) {
	length, err := c.client.LLen(ctx, key).Result()
	if err != nil {
		c.log.LogError("Redis LLEN failed", err, zap.String("key", key))
		return 0, err
	}
	return length, nil
}

// LRange возвращает элементы списка в диапазоне [start, stop]
func (c *Cache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	result, err := c.client.LRange(ctx, key, start, stop).Result()
	if err != nil {
		c.log.LogError("Redis LRANGE failed", err, zap.String("key", key))
		return nil, err
	}
	return result, nil
}

// LTrim обрезает список, оставляя только элементы в диапазоне [start, stop]
func (c *Cache) LTrim(ctx context.Context, key string, start, stop int64) error {
	err := c.client.LTrim(ctx, key, start, stop).Err()
	if err != nil {
		c.log.LogError("Redis LTRIM failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// SAdd добавляет элементы в множество (SET). Возвращает количество добавленных элементов.
func (c *Cache) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	count, err := c.client.SAdd(ctx, key, members...).Result()
	if err != nil {
		c.log.LogError("Redis SADD failed", err, zap.String("key", key))
		return 0, err
	}
	return count, nil
}

// SAddWithExpire атомарно добавляет элементы в множество и устанавливает TTL через Lua скрипт.
// Возвращает количество новых добавленных элементов. TTL должен быть >= 1 секунды.
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

// SRem удаляет элементы из множества (SET)
func (c *Cache) SRem(ctx context.Context, key string, members ...any) error {
	err := c.client.SRem(ctx, key, members...).Err()
	if err != nil {
		c.log.LogError("Redis SREM failed", err, zap.String("key", key))
		return err
	}
	return nil
}

// SetNX устанавливает значение только если ключа не существует (для distributed locks)
func (c *Cache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	result, err := c.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		c.log.LogError("Redis SETNX failed", err, zap.String("key", key))
		return false, err
	}
	return result, nil
}

// BatchSetNX выполняет несколько SetNX операций через pipeline.
// Возвращает map[key]bool, где true означает что ключ был создан (новый).
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
	// Даже при ошибке pipeline собираем индивидуальные результаты:
	// часть команд могла успешно выполниться.
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

// Publish публикует сообщение в канал
func (c *Cache) Publish(ctx context.Context, channel string, message any) error {
	err := c.client.Publish(ctx, channel, message).Err()
	if err != nil {
		c.log.LogError("Redis PUBLISH failed", err, zap.String("channel", channel))
		return err
	}
	return nil
}

// Subscribe подписывается на канал
func (c *Cache) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// ReplaceList atomically replaces a list with the given values using MULTI/EXEC pipeline.
// It deletes the key and then pushes all values back in a single transaction.
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

// BatchLPush добавляет несколько элементов в разные списки одним pipeline-запросом.
// items - map[key][]value, каждый value добавляется в список с ключом key.
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

// Health проверяет здоровье Redis
func (c *Cache) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return c.client.Ping(ctx).Err()
}

// Eval выполняет Lua скрипт на Redis
func (c *Cache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	result, err := c.client.Eval(ctx, script, keys, args...).Result()
	if err != nil {
		c.log.LogError("Redis EVAL failed", err)
		return nil, err
	}
	return result, nil
}

// Scan итеративно сканирует ключи Redis по паттерну
func (c *Cache) Scan(ctx context.Context, cursor uint64, pattern string, count int64) ([]string, uint64, error) {
	keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, count).Result()
	if err != nil {
		c.log.LogError("Redis SCAN failed", err, zap.String("pattern", pattern))
		return nil, 0, err
	}
	return keys, nextCursor, nil
}

// Close закрывает соединение с Redis
func (c *Cache) Close() error {
	c.log.Info("Closing Redis connection")
	return c.client.Close()
}
