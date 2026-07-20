package cache

import (
	"context"
	"fmt"
	"time"
)

// RateLimiter - rate limit на редисе
type RateLimiter struct {
	cache *Cache
}

func NewRateLimiter(cache *Cache) *RateLimiter {
	return &RateLimiter{
		cache: cache,
	}
}

// Allow - fixed window: инкрементим счётчик, на первом запросе вешаем ttl окна
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	script := `
		local current = redis.call("INCR", KEYS[1])
		if current == 1 then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return current
	`
	windowSeconds := int(window.Seconds())
	result, err := rl.cache.Eval(ctx, script, []string{key}, windowSeconds)
	if err != nil {
		return false, fmt.Errorf("rate limit check failed: %w", err)
	}

	count, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected rate limit result type")
	}

	return count <= int64(limit), nil
}

// AllowWithIncr - то же но через INCRBY (сейчас инкремент всегда 1)
func (rl *RateLimiter) AllowWithIncr(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	script := `
		local current = redis.call("INCRBY", KEYS[1], ARGV[2])
		if current == tonumber(ARGV[2]) then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return current
	`
	windowSeconds := int(window.Seconds())
	increment := 1
	result, err := rl.cache.Eval(ctx, script, []string{key}, windowSeconds, increment)
	if err != nil {
		return false, fmt.Errorf("rate limit check failed: %w", err)
	}

	count, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected rate limit result type")
	}

	return count <= int64(limit), nil
}

func (rl *RateLimiter) Reset(ctx context.Context, key string) error {
	return rl.cache.Del(ctx, key)
}
