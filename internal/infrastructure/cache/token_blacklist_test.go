package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenBlacklistCache_AddAndCheck(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	// неизвестный токен не в блеклисте
	blacklisted, err := tbc.IsBlacklisted(ctx, "unknown")
	require.NoError(t, err)
	assert.False(t, blacklisted)

	require.NoError(t, tbc.Add(ctx, "test-token-123", 5*time.Minute))

	blacklisted, err = tbc.IsBlacklisted(ctx, "test-token-123")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestTokenBlacklistCache_Remove(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	require.NoError(t, tbc.Add(ctx, "token-to-remove", 5*time.Minute))
	require.NoError(t, tbc.Remove(ctx, "token-to-remove"))

	blacklisted, err := tbc.IsBlacklisted(ctx, "token-to-remove")
	require.NoError(t, err)
	assert.False(t, blacklisted)

	// снятие несуществующего токена не ошибка
	require.NoError(t, tbc.Remove(ctx, "never-added"))
}

func TestTokenBlacklistCache_TTLExpiry(t *testing.T) {
	cache, mr := setupTestCacheWithMR(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	require.NoError(t, tbc.Add(ctx, "expiring-token", 100*time.Millisecond))

	blacklisted, err := tbc.IsBlacklisted(ctx, "expiring-token")
	require.NoError(t, err)
	assert.True(t, blacklisted)

	mr.FastForward(200 * time.Millisecond)

	blacklisted, err = tbc.IsBlacklisted(ctx, "expiring-token")
	require.NoError(t, err)
	assert.False(t, blacklisted)
}

// AddIfNotExists атомарный через setnx: первый раз кладёт (true), второй раз
// токен уже есть (false). нужно для ротации токенов, иначе TOCTOU-гонка
func TestTokenBlacklistCache_AddIfNotExists(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	added, err := tbc.AddIfNotExists(ctx, "rotate-token", 5*time.Minute)
	require.NoError(t, err)
	assert.True(t, added)

	added, err = tbc.AddIfNotExists(ctx, "rotate-token", 5*time.Minute)
	require.NoError(t, err)
	assert.False(t, added)

	blacklisted, err := tbc.IsBlacklisted(ctx, "rotate-token")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestTokenBlacklistCache_AddOverwritesTTL(t *testing.T) {
	cache, mr := setupTestCacheWithMR(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	require.NoError(t, tbc.Add(ctx, "token-ttl", 100*time.Millisecond))
	// повторный Add с длинным ttl перетирает короткий
	require.NoError(t, tbc.Add(ctx, "token-ttl", 5*time.Minute))

	mr.FastForward(200 * time.Millisecond)

	blacklisted, err := tbc.IsBlacklisted(ctx, "token-ttl")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}
