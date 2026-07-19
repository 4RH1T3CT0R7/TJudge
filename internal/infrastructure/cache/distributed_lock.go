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

// DistributedLock - лок на редисе (redlock по мотивам redis.io)
type DistributedLock struct {
	cache *Cache
}

func NewDistributedLock(cache *Cache) *DistributedLock {
	return &DistributedLock{
		cache: cache,
	}
}

// Lock захватывает лок через SETNX со своим токеном
func (dl *DistributedLock) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate lock token: %w", err)
	}

	lockKey := fmt.Sprintf("lock:%s", key)

	acquired, err := dl.cache.SetNX(ctx, lockKey, token, ttl)
	if err != nil {
		return "", fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		return "", errors.ErrConflict.WithMessage("lock already held")
	}

	return token, nil
}

// TryLock - Lock с ретраями
func (dl *DistributedLock) TryLock(ctx context.Context, key string, ttl time.Duration, maxAttempts int, retryDelay time.Duration) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		token, err := dl.Lock(ctx, key, ttl)
		if err == nil {
			return token, nil
		}

		lastErr = err

		// после последней попытки не спим
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(retryDelay):
			}
		}
	}

	return "", fmt.Errorf("failed to acquire lock after %d attempts: %w", maxAttempts, lastErr)
}

// Unlock снимает лок атомарно через lua
func (dl *DistributedLock) Unlock(ctx context.Context, key string, token string) error {
	lockKey := fmt.Sprintf("lock:%s", key)

	// скрипт с redis.io, удаляет лок только если токен наш
	// https://redis.io/docs/latest/develop/use/patterns/distributed-locks/
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

	// 0 = токен не наш или ключа уже нет, 1 = удалили
	if val, ok := result.(int64); ok && val == 0 {
		// если ключа нет - лок уже сняли (протух ttl), это ок.
		// если ключ есть но токен не наш - кто-то другой держит лок
		exists, err := dl.cache.Exists(ctx, lockKey)
		if err != nil {
			return fmt.Errorf("failed to check lock existence: %w", err)
		}
		if exists {
			return errors.ErrConflict.WithMessage("lock token mismatch")
		}
		return nil
	}

	return nil
}

// WithLock выполняет fn под локом. пока fn работает, фоновая горутина
// продлевает лок каждые ttl/3 чтобы он не протух на долгой операции
func (dl *DistributedLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error {
	token, err := dl.TryLock(ctx, key, ttl, 3, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	renewCtx, renewCancel := context.WithCancel(ctx)
	renewDone := make(chan struct{})
	go dl.renewLoop(renewCtx, key, token, ttl, renewDone)

	defer func() {
		// сначала гасим продление и ЖДЁМ горутину, только потом снимаем лок,
		// иначе renew может продлить лок уже после снятия
		renewCancel()
		<-renewDone

		// свой контекст, родительский к этому моменту может быть отменён
		unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// ошибку тут глотаем, основная работа уже сделана
		_ = dl.Unlock(unlockCtx, key, token)
	}()

	return fn(ctx)
}

// renewLoop продлевает ttl лока пока не отменят контекст
func (dl *DistributedLock) renewLoop(ctx context.Context, key string, token string, ttl time.Duration, done chan struct{}) {
	defer close(done)

	lockKey := fmt.Sprintf("lock:%s", key)
	// 500мс - пол из экспериментов, чтобы на мелких ttl не молотить редис
	interval := max(ttl/3, 500*time.Millisecond)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// продлеваем только если лок всё ещё наш
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
				// лок увели или он протух, продлевать больше нечего
				dl.cache.log.Warn("Distributed lock lost (token mismatch or expired)",
					zap.String("key", key),
				)
				return
			}
		}
	}
}

// generateToken - 16 случайных байт в hex
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (dl *DistributedLock) IsLocked(ctx context.Context, key string) (bool, error) {
	lockKey := fmt.Sprintf("lock:%s", key)
	return dl.cache.Exists(ctx, lockKey)
}
