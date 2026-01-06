package cache

import (
	"context"
	"fmt"
	"time"
)

// RateLimiter реализует rate limiting используя Redis
type RateLimiter struct {
	cache *Cache
}

// NewRateLimiter создаёт новый rate limiter
func NewRateLimiter(cache *Cache) *RateLimiter {
	return &RateLimiter{
		cache: cache,
	}
}

// Allow проверяет, разрешён ли запрос для данного ключа
// Использует алгоритм fixed window counter
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	// Получаем текущее количество запросов
	current, err := rl.cache.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get rate limit counter: %w", err)
	}

	// Если ключа нет, это первый запрос
	if current == "" {
		// Устанавливаем счётчик в 1 с TTL равным окну
		if err := rl.cache.Set(ctx, key, 1, window); err != nil {
			return false, fmt.Errorf("failed to set rate limit counter: %w", err)
		}
		return true, nil
	}

	// Парсим текущее значение
	count := 0
	if current != "" {
		_, _ = fmt.Sscanf(current, "%d", &count)
	}

	// Проверяем лимит
	if count >= limit {
		return false, nil
	}

	// Увеличиваем счётчик
	newCount := count + 1
	if err := rl.cache.Set(ctx, key, newCount, window); err != nil {
		return false, fmt.Errorf("failed to increment rate limit counter: %w", err)
	}

	return true, nil
}

// AllowWithIncr проверяет лимит используя Redis INCR (более эффективно)
func (rl *RateLimiter) AllowWithIncr(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	// Используем SetNX для установки начального значения
	set, err := rl.cache.SetNX(ctx, key, 0, window)
	if err != nil {
		return false, fmt.Errorf("failed to init rate limit counter: %w", err)
	}

	// Если ключ был установлен, устанавливаем TTL
	if set {
		if err := rl.cache.Expire(ctx, key, window); err != nil {
			return false, fmt.Errorf("failed to set rate limit TTL: %w", err)
		}
	}

	// Получаем текущее значение
	current, err := rl.cache.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get rate limit counter: %w", err)
	}

	count := 0
	if current != "" {
		_, _ = fmt.Sscanf(current, "%d", &count)
	}

	// Проверяем лимит
	if count >= limit {
		return false, nil
	}

	// Инкрементируем
	newCount := count + 1
	if err := rl.cache.Set(ctx, key, newCount, window); err != nil {
		return false, fmt.Errorf("failed to increment rate limit counter: %w", err)
	}

	return true, nil
}

// Reset сбрасывает счётчик для ключа
func (rl *RateLimiter) Reset(ctx context.Context, key string) error {
	return rl.cache.Del(ctx, key)
}
