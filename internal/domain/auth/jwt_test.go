package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWTManager(t *testing.T) {
	secret := "test-secret-key-123"
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour

	manager := NewJWTManager(secret, accessTTL, refreshTTL)

	assert.NotNil(t, manager)
	assert.Equal(t, []byte(secret), manager.secretKey)
	assert.Equal(t, accessTTL, manager.accessTTL)
	assert.Equal(t, refreshTTL, manager.refreshTTL)
}

func TestJWTManager_GenerateAccessToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()
	username := "testuser"

	token, err := manager.GenerateAccessToken(userID, username)

	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	claims, err := manager.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, userID.String(), claims.Subject)
}

func TestJWTManager_GenerateRefreshToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateRefreshToken(userID)

	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the refresh token
	extractedUserID, err := manager.ValidateRefreshToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, extractedUserID)
}

func TestJWTManager_ValidateToken_Valid(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()
	username := "testuser"

	token, err := manager.GenerateAccessToken(userID, username)
	require.NoError(t, err)

	claims, err := manager.ValidateToken(token)

	require.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, username, claims.Username)
}

func TestJWTManager_ValidateToken_InvalidSignature(t *testing.T) {
	manager1 := NewJWTManager("secret-1", 15*time.Minute, 7*24*time.Hour)
	manager2 := NewJWTManager("secret-2", 15*time.Minute, 7*24*time.Hour)

	token, err := manager1.GenerateAccessToken(uuid.New(), "testuser")
	require.NoError(t, err)

	// Try to validate with different secret
	_, err = manager2.ValidateToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestJWTManager_ValidateToken_Expired(t *testing.T) {
	// Create manager with very short TTL
	manager := NewJWTManager("test-secret", 1*time.Millisecond, 7*24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = manager.ValidateToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestJWTManager_ValidateToken_Malformed(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	_, err := manager.ValidateToken("not-a-valid-jwt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestJWTManager_ValidateToken_Empty(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	_, err := manager.ValidateToken("")
	assert.Error(t, err)
}

func TestJWTManager_ValidateRefreshToken_Valid(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	extractedUserID, err := manager.ValidateRefreshToken(token)

	require.NoError(t, err)
	assert.Equal(t, userID, extractedUserID)
}

func TestJWTManager_ValidateRefreshToken_InvalidSignature(t *testing.T) {
	manager1 := NewJWTManager("secret-1", 15*time.Minute, 7*24*time.Hour)
	manager2 := NewJWTManager("secret-2", 15*time.Minute, 7*24*time.Hour)

	token, err := manager1.GenerateRefreshToken(uuid.New())
	require.NoError(t, err)

	_, err = manager2.ValidateRefreshToken(token)
	assert.Error(t, err)
}

func TestJWTManager_ValidateRefreshToken_Expired(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 1*time.Millisecond)
	userID := uuid.New()

	token, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = manager.ValidateRefreshToken(token)
	assert.Error(t, err)
}

func TestJWTManager_ExtractUserID(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	extractedUserID, err := manager.ExtractUserID(token)

	require.NoError(t, err)
	assert.Equal(t, userID, extractedUserID)
}

func TestJWTManager_ExtractUserID_InvalidToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	_, err := manager.ExtractUserID("invalid-token")
	assert.Error(t, err)
}

func TestJWTManager_ExtractUserID_ExpiredToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 1*time.Millisecond, 7*24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	// ExtractUserID doesn't validate, so it should still work
	extractedUserID, err := manager.ExtractUserID(token)
	require.NoError(t, err)
	assert.Equal(t, userID, extractedUserID)
}

func TestJWTManager_RefreshTokenTTL(t *testing.T) {
	refreshTTL := 7 * 24 * time.Hour
	manager := NewJWTManager("test-secret", 15*time.Minute, refreshTTL)

	assert.Equal(t, refreshTTL, manager.RefreshTokenTTL())
}

func TestJWTManager_AccessTokenTTL(t *testing.T) {
	accessTTL := 15 * time.Minute
	manager := NewJWTManager("test-secret", accessTTL, 7*24*time.Hour)

	assert.Equal(t, accessTTL, manager.AccessTokenTTL())
}

func TestJWTManager_TokensAreUnique_DifferentUsers(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	user1ID := uuid.New()
	user2ID := uuid.New()

	token1, err := manager.GenerateAccessToken(user1ID, "user1")
	require.NoError(t, err)

	token2, err := manager.GenerateAccessToken(user2ID, "user2")
	require.NoError(t, err)

	// Tokens for different users should be different
	assert.NotEqual(t, token1, token2)
}

func TestJWTManager_TokensSameUserSameSecond(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token1, err := manager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	token2, err := manager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	// Tokens generated in the same second with same user data will be identical
	// This is expected behavior for JWT - uniqueness comes from refresh tokens
	// which have unique JTI (JWT ID)
	assert.Equal(t, token1, token2)
}

func TestJWTManager_RefreshTokensHaveUniqueJTI(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token1, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	token2, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	// Refresh tokens should be different due to unique JTI
	assert.NotEqual(t, token1, token2)
}

func TestJWTManager_DifferentUsers(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	user1ID := uuid.New()
	user2ID := uuid.New()

	token1, err := manager.GenerateAccessToken(user1ID, "user1")
	require.NoError(t, err)

	token2, err := manager.GenerateAccessToken(user2ID, "user2")
	require.NoError(t, err)

	claims1, err := manager.ValidateToken(token1)
	require.NoError(t, err)

	claims2, err := manager.ValidateToken(token2)
	require.NoError(t, err)

	assert.NotEqual(t, claims1.UserID, claims2.UserID)
	assert.NotEqual(t, claims1.Username, claims2.Username)
}
