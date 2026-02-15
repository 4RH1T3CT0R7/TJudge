package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// DistributedLock реализует distributed lock на Redis
type DistributedLock struct {
	cache *Cache
}

// NewDistributedLock создаёт новый distributed lock
func NewDistributedLock(cache *Cache) *DistributedLock {
	return &DistributedLock{
		cache: cache,
	}
}

// Lock пытается захватить блокировку
func (dl *DistributedLock) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	// Генерируем уникальный token для этой блокировки
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate lock token: %w", err)
	}

	lockKey := fmt.Sprintf("lock:%s", key)

	// Пытаемся установить блокировку с помощью SETNX
	acquired, err := dl.cache.SetNX(ctx, lockKey, token, ttl)
	if err != nil {
		return "", fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		return "", errors.ErrConflict.WithMessage("lock already held")
	}

	return token, nil
}

// TryLock пытается захватить блокировку с повторными попытками
func (dl *DistributedLock) TryLock(ctx context.Context, key string, ttl time.Duration, maxAttempts int, retryDelay time.Duration) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		token, err := dl.Lock(ctx, key, ttl)
		if err == nil {
			return token, nil
		}

		lastErr = err

		// Если это не последняя попытка, ждём перед повторной попыткой
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(retryDelay):
				// Продолжаем
			}
		}
	}

	return "", fmt.Errorf("failed to acquire lock after %d attempts: %w", maxAttempts, lastErr)
}

// Unlock освобождает блокировку атомарно с помощью Lua скрипта
func (dl *DistributedLock) Unlock(ctx context.Context, key string, token string) error {
	lockKey := fmt.Sprintf("lock:%s", key)

	// Lua script atomically checks token and deletes if matching
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := dl.cache.Eval(ctx, script, []string{lockKey}, token)
	if err != nil {
		return fmt.Errorf("failed to unlock: %w", err)
	}

	// result is 0 if token didn't match (or key already gone), 1 if deleted
	if val, ok := result.(int64); ok && val == 0 {
		// Key was already gone or token mismatch -- not necessarily an error
		// If key doesn't exist, it was already unlocked (TTL expired)
		exists, err := dl.cache.Exists(ctx, lockKey)
		if err != nil {
			return fmt.Errorf("failed to check lock existence: %w", err)
		}
		if exists {
			return errors.ErrConflict.WithMessage("lock token mismatch")
		}
		// Already unlocked -- safe no-op
		return nil
	}

	return nil
}

// WithLock выполняет функцию с захваченной блокировкой
func (dl *DistributedLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error {
	// Захватываем блокировку
	token, err := dl.TryLock(ctx, key, ttl, 3, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Гарантируем освобождение блокировки
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Игнорируем ошибку разблокировки, так как основная операция уже выполнена
		_ = dl.Unlock(unlockCtx, key, token)
	}()

	// Выполняем функцию
	return fn(ctx)
}

// generateToken генерирует случайный токен для блокировки
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// IsLocked проверяет, захвачена ли блокировка
func (dl *DistributedLock) IsLocked(ctx context.Context, key string) (bool, error) {
	lockKey := fmt.Sprintf("lock:%s", key)
	return dl.cache.Exists(ctx, lockKey)
}
