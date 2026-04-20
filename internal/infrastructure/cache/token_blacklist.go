package cache

import (
	"context"
	"fmt"
	"time"
)

// TokenBlacklistCache управляет чёрным списком токенов
type TokenBlacklistCache struct {
	cache *Cache
}

// NewTokenBlacklistCache создаёт новый кэш чёрного списка токенов
func NewTokenBlacklistCache(cache *Cache) *TokenBlacklistCache {
	return &TokenBlacklistCache{
		cache: cache,
	}
}

// Add добавляет токен в чёрный список
func (tbc *TokenBlacklistCache) Add(ctx context.Context, token string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:token:%s", token)

	err := tbc.cache.Set(ctx, key, "1", ttl)
	if err != nil {
		return fmt.Errorf("failed to add token to blacklist: %w", err)
	}

	return nil
}

// IsBlacklisted проверяет, находится ли токен в чёрном списке
func (tbc *TokenBlacklistCache) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", token)

	exists, err := tbc.cache.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check token blacklist: %w", err)
	}

	return exists, nil
}

// AddIfNotExists атомарно добавляет токен в чёрный список только если его там нет.
// Возвращает true если токен был добавлен (новый), false если уже существовал.
// Используется для token rotation: предотвращает TOCTOU race condition.
func (tbc *TokenBlacklistCache) AddIfNotExists(ctx context.Context, token string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", token)

	wasSet, err := tbc.cache.SetNX(ctx, key, "1", ttl)
	if err != nil {
		return false, fmt.Errorf("failed to atomically blacklist token: %w", err)
	}

	return wasSet, nil
}

// Remove удаляет токен из чёрного списка (для тестов или администрирования)
func (tbc *TokenBlacklistCache) Remove(ctx context.Context, token string) error {
	key := fmt.Sprintf("blacklist:token:%s", token)

	err := tbc.cache.Del(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to remove token from blacklist: %w", err)
	}

	return nil
}
