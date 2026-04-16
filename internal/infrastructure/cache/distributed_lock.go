package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"go.uber.org/zap"
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

// WithLock выполняет функцию с захваченной блокировкой.
// A background goroutine renews the lock at ttl/3 intervals to prevent
// expiry during long-running operations.
func (dl *DistributedLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error {
	// Захватываем блокировку
	token, err := dl.TryLock(ctx, key, ttl, 3, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Start lock renewal goroutine.
	renewCtx, renewCancel := context.WithCancel(ctx)
	renewDone := make(chan struct{})
	// #nosec G118 -- renewCtx derived from caller ctx, не Background. gosec
	// false-positive (видит goroutine, но ctx реально request-scoped).
	go dl.renewLoop(renewCtx, key, token, ttl, renewDone)

	// Гарантируем освобождение блокировки
	defer func() {
		renewCancel()
		<-renewDone // wait for renewal goroutine to exit

		unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Игнорируем ошибку разблокировки, так как основная операция уже выполнена
		_ = dl.Unlock(unlockCtx, key, token)
	}()

	// Выполняем функцию
	return fn(ctx)
}

// renewLoop periodically extends the lock TTL until the context is cancelled.
func (dl *DistributedLock) renewLoop(ctx context.Context, key string, token string, ttl time.Duration, done chan struct{}) {
	defer close(done)

	lockKey := fmt.Sprintf("lock:%s", key)
	interval := ttl / 3
	if interval < 500*time.Millisecond {
		interval = 500 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Lua script: only extend if we still own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	ttlMs := fmt.Sprintf("%d", ttl.Milliseconds())

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			result, err := dl.cache.Eval(renewCtx, script, []string{lockKey}, token, ttlMs)
			cancel()
			if err != nil {
				dl.cache.log.Warn("Failed to renew distributed lock",
					zap.String("key", key),
					zap.Error(err),
				)
			} else if val, ok := result.(int64); ok && val == 0 {
				dl.cache.log.Warn("Distributed lock lost (token mismatch or expired)",
					zap.String("key", key),
				)
				return
			}
		}
	}
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
