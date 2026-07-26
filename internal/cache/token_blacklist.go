package cache

import (
	"context"
	"fmt"
	"time"
)

type TokenBlacklistCache struct {
	cache *Cache
}

func NewTokenBlacklistCache(cache *Cache) *TokenBlacklistCache {
	return &TokenBlacklistCache{
		cache: cache,
	}
}

func (tbc *TokenBlacklistCache) Add(ctx context.Context, token string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:token:%s", token)

	err := tbc.cache.Set(ctx, key, "1", ttl)
	if err != nil {
		return fmt.Errorf("failed to add token to blacklist: %w", err)
	}

	return nil
}

func (tbc *TokenBlacklistCache) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", token)

	exists, err := tbc.cache.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check token blacklist: %w", err)
	}

	return exists, nil
}

// AddIfNotExists кладёт токен только если его ещё нет, атомарно через setnx.
// true = добавили. нужно для ротации токенов, иначе TOCTOU-гонка
func (tbc *TokenBlacklistCache) AddIfNotExists(ctx context.Context, token string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", token)

	wasSet, err := tbc.cache.SetNX(ctx, key, "1", ttl)
	if err != nil {
		return false, fmt.Errorf("failed to atomically blacklist token: %w", err)
	}

	return wasSet, nil
}

// Remove - для тестов/админки
func (tbc *TokenBlacklistCache) Remove(ctx context.Context, token string) error {
	key := fmt.Sprintf("blacklist:token:%s", token)

	err := tbc.cache.Del(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to remove token from blacklist: %w", err)
	}

	return nil
}
