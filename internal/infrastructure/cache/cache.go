package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
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

	// Проверяем соединение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Info("Redis connected successfully",
		zap.String("addr", cfg.Address()),
		zap.Int("db", cfg.DB),
	)

	return &Cache{
		client:  client,
		log:     log,
		metrics: m,
	}, nil
}

// Get получает значение по ключу
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()

	if err == redis.Nil {
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
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
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

// ZRevRangeWithScores получает элементы из sorted set в обратном порядке
func (c *Cache) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	result, err := c.client.ZRevRangeWithScores(ctx, key, start, stop).Result()

	if err == redis.Nil {
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
func (c *Cache) LPush(ctx context.Context, key string, values ...interface{}) error {
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
	if err == redis.Nil {
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
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
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

// SetNX устанавливает значение только если ключа не существует (для distributed locks)
func (c *Cache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	result, err := c.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		c.log.LogError("Redis SETNX failed", err, zap.String("key", key))
		return false, err
	}
	return result, nil
}

// Publish публикует сообщение в канал
func (c *Cache) Publish(ctx context.Context, channel string, message interface{}) error {
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

// Health проверяет здоровье Redis
func (c *Cache) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return c.client.Ping(ctx).Err()
}

// Close закрывает соединение с Redis
func (c *Cache) Close() error {
	c.log.Info("Closing Redis connection")
	return c.client.Close()
}
