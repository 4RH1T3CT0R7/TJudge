package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenBlacklistCache_Add_And_IsBlacklisted(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	err := tbc.Add(ctx, "test-token-123", 5*time.Minute)
	require.NoError(t, err)

	blacklisted, err := tbc.IsBlacklisted(ctx, "test-token-123")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestTokenBlacklistCache_IsBlacklisted_NonExistent(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	blacklisted, err := tbc.IsBlacklisted(ctx, "non-existent-token")
	require.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestTokenBlacklistCache_Remove(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	// Add then remove
	err := tbc.Add(ctx, "token-to-remove", 5*time.Minute)
	require.NoError(t, err)

	err = tbc.Remove(ctx, "token-to-remove")
	require.NoError(t, err)

	blacklisted, err := tbc.IsBlacklisted(ctx, "token-to-remove")
	require.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestTokenBlacklistCache_TTLExpiry(t *testing.T) {
	cache, mr := setupTestCacheWithMR(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	err := tbc.Add(ctx, "expiring-token", 100*time.Millisecond)
	require.NoError(t, err)

	// Should be blacklisted before expiry
	blacklisted, err := tbc.IsBlacklisted(ctx, "expiring-token")
	require.NoError(t, err)
	assert.True(t, blacklisted)

	// Fast-forward past TTL
	mr.FastForward(200 * time.Millisecond)

	// Should no longer be blacklisted
	blacklisted, err = tbc.IsBlacklisted(ctx, "expiring-token")
	require.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestTokenBlacklistCache_MultipleTokens(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	tokens := []string{"token-a", "token-b", "token-c"}
	for _, token := range tokens {
		err := tbc.Add(ctx, token, 5*time.Minute)
		require.NoError(t, err)
	}

	// All should be blacklisted
	for _, token := range tokens {
		blacklisted, err := tbc.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.True(t, blacklisted, "expected %s to be blacklisted", token)
	}

	// Remove one
	err := tbc.Remove(ctx, "token-b")
	require.NoError(t, err)

	// Only token-b should be gone
	blacklisted, err := tbc.IsBlacklisted(ctx, "token-a")
	require.NoError(t, err)
	assert.True(t, blacklisted)

	blacklisted, err = tbc.IsBlacklisted(ctx, "token-b")
	require.NoError(t, err)
	assert.False(t, blacklisted)

	blacklisted, err = tbc.IsBlacklisted(ctx, "token-c")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestTokenBlacklistCache_FullCycle(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	token := "lifecycle-token"

	// 1. Not blacklisted initially
	blacklisted, err := tbc.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.False(t, blacklisted)

	// 2. Add to blacklist
	err = tbc.Add(ctx, token, 5*time.Minute)
	require.NoError(t, err)

	// 3. Now blacklisted
	blacklisted, err = tbc.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.True(t, blacklisted)

	// 4. Remove
	err = tbc.Remove(ctx, token)
	require.NoError(t, err)

	// 5. Not blacklisted again
	blacklisted, err = tbc.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.False(t, blacklisted)

	// 6. Re-add
	err = tbc.Add(ctx, token, 5*time.Minute)
	require.NoError(t, err)

	// 7. Blacklisted again
	blacklisted, err = tbc.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestTokenBlacklistCache_Remove_NonExistent(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	// Removing a non-existent token should not error
	err := tbc.Remove(ctx, "never-added")
	assert.NoError(t, err)
}

func TestTokenBlacklistCache_Add_OverwritesTTL(t *testing.T) {
	cache, mr := setupTestCacheWithMR(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	// Add with short TTL
	err := tbc.Add(ctx, "token-ttl", 100*time.Millisecond)
	require.NoError(t, err)

	// Re-add with longer TTL
	err = tbc.Add(ctx, "token-ttl", 5*time.Minute)
	require.NoError(t, err)

	// Fast-forward past the original TTL
	mr.FastForward(200 * time.Millisecond)

	// Should still be blacklisted (longer TTL applies)
	blacklisted, err := tbc.IsBlacklisted(ctx, "token-ttl")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestTokenBlacklistCache_EmptyToken(t *testing.T) {
	cache := setupTestCache(t)
	defer cache.Close()

	tbc := NewTokenBlacklistCache(cache)
	ctx := context.Background()

	// Empty string token should work (no crash)
	err := tbc.Add(ctx, "", 5*time.Minute)
	require.NoError(t, err)

	blacklisted, err := tbc.IsBlacklisted(ctx, "")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}
