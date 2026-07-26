//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// AuthFlowSuite tests the auth token blacklist flow using real Redis.
type AuthFlowSuite struct {
	suite.Suite
	cache          *cache.Cache
	tokenBlacklist *cache.TokenBlacklistCache
	ctx            context.Context
}

func (s *AuthFlowSuite) SetupSuite() {
	if os.Getenv("RUN_INTEGRATION") != "true" {
		s.T().Skip("Skipping integration tests (set RUN_INTEGRATION=true)")
	}

	s.ctx = context.Background()

	host := getEnv("REDIS_HOST", "localhost")
	port := getEnvInt("REDIS_PORT", 6379)
	password := getEnv("REDIS_PASSWORD", "")

	log, _ := logger.New("debug", "json")
	m := metrics.New()

	var err error
	s.cache, err = cache.New(&config.RedisConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       1, // Use DB 1 for tests
		PoolSize: 10,
	}, log, m)
	require.NoError(s.T(), err)

	s.tokenBlacklist = cache.NewTokenBlacklistCache(s.cache)
}

func (s *AuthFlowSuite) TearDownSuite() {
	if s.cache != nil {
		s.cache.Close()
	}
}

func (s *AuthFlowSuite) SetupTest() {
	// Clean up blacklist keys before each test
	s.cache.Del(s.ctx, "blacklist:token:*")
}

// =============================================================================
// Token Blacklist Tests
// =============================================================================

func (s *AuthFlowSuite) TestAuthFlow_TokenBlacklist_AddAndCheck() {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test-token-add-check"

	// Add token to blacklist with 1 minute TTL
	err := s.tokenBlacklist.Add(s.ctx, token, time.Minute)
	require.NoError(s.T(), err)

	// Verify the token is blacklisted
	blacklisted, err := s.tokenBlacklist.IsBlacklisted(s.ctx, token)
	require.NoError(s.T(), err)
	assert.True(s.T(), blacklisted, "token should be blacklisted after Add")

	// Verify a non-existent token is NOT blacklisted
	nonExistent := "non-existent-token-that-was-never-added"
	blacklisted, err = s.tokenBlacklist.IsBlacklisted(s.ctx, nonExistent)
	require.NoError(s.T(), err)
	assert.False(s.T(), blacklisted, "non-existent token should not be blacklisted")
}

func (s *AuthFlowSuite) TestAuthFlow_TokenBlacklist_TTLExpiry() {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test-token-ttl-expiry"

	// Add token with a very short TTL
	err := s.tokenBlacklist.Add(s.ctx, token, 200*time.Millisecond)
	require.NoError(s.T(), err)

	// Verify it is blacklisted immediately
	blacklisted, err := s.tokenBlacklist.IsBlacklisted(s.ctx, token)
	require.NoError(s.T(), err)
	assert.True(s.T(), blacklisted, "token should be blacklisted immediately after Add")

	// Wait for TTL to expire
	time.Sleep(400 * time.Millisecond)

	// Verify the token is no longer blacklisted
	blacklisted, err = s.tokenBlacklist.IsBlacklisted(s.ctx, token)
	require.NoError(s.T(), err)
	assert.False(s.T(), blacklisted, "token should no longer be blacklisted after TTL expiry")
}

func (s *AuthFlowSuite) TestAuthFlow_TokenBlacklist_MultipleTokens() {
	tokens := []string{
		"token-multi-1-aaa111",
		"token-multi-2-bbb222",
		"token-multi-3-ccc333",
		"token-multi-4-ddd444",
		"token-multi-5-eee555",
	}

	// Blacklist all tokens
	for _, token := range tokens {
		err := s.tokenBlacklist.Add(s.ctx, token, time.Minute)
		require.NoError(s.T(), err)
	}

	// Verify all are blacklisted
	for _, token := range tokens {
		blacklisted, err := s.tokenBlacklist.IsBlacklisted(s.ctx, token)
		require.NoError(s.T(), err)
		assert.True(s.T(), blacklisted, "token %s should be blacklisted", token)
	}

	// Remove one token and verify only that one is gone
	err := s.tokenBlacklist.Remove(s.ctx, tokens[2])
	require.NoError(s.T(), err)

	blacklisted, err := s.tokenBlacklist.IsBlacklisted(s.ctx, tokens[2])
	require.NoError(s.T(), err)
	assert.False(s.T(), blacklisted, "removed token should no longer be blacklisted")

	// Other tokens should still be blacklisted
	for i, token := range tokens {
		if i == 2 {
			continue
		}
		blacklisted, err := s.tokenBlacklist.IsBlacklisted(s.ctx, token)
		require.NoError(s.T(), err)
		assert.True(s.T(), blacklisted, "token %s should still be blacklisted", token)
	}
}

func (s *AuthFlowSuite) TestAuthFlow_TokenBlacklist_ConcurrentAccess() {
	const numGoroutines = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Each goroutine adds and then checks

	errors := make(chan error, numGoroutines*2)

	for i := 0; i < numGoroutines; i++ {
		token := fmt.Sprintf("concurrent-token-%d", i)

		// Goroutine to add a token
		go func(t string) {
			defer wg.Done()
			if err := s.tokenBlacklist.Add(s.ctx, t, time.Minute); err != nil {
				errors <- fmt.Errorf("failed to add token %s: %w", t, err)
			}
		}(token)

		// Goroutine to check a token (may or may not be added yet)
		go func(t string) {
			defer wg.Done()
			_, err := s.tokenBlacklist.IsBlacklisted(s.ctx, t)
			if err != nil {
				errors <- fmt.Errorf("failed to check token %s: %w", t, err)
			}
		}(token)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		s.T().Errorf("concurrent access error: %v", err)
	}

	// After all goroutines complete, all tokens should be blacklisted
	for i := 0; i < numGoroutines; i++ {
		token := fmt.Sprintf("concurrent-token-%d", i)
		blacklisted, err := s.tokenBlacklist.IsBlacklisted(s.ctx, token)
		require.NoError(s.T(), err)
		assert.True(s.T(), blacklisted, "token %s should be blacklisted after concurrent Add", token)
	}
}

func TestAuthFlowSuite(t *testing.T) {
	suite.Run(t, new(AuthFlowSuite))
}
