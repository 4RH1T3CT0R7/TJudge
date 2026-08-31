package auth

import (
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWTManager(t *testing.T) {
	secret := "test-secret-key-123"
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour

	manager := NewJWTManager(secret, accessTTL, refreshTTL)

	assert.Equal(t, []byte(secret), manager.secretKey)
	assert.Equal(t, accessTTL, manager.accessTTL)
	assert.Equal(t, refreshTTL, manager.refreshTTL)
}

// access генерится и тут же валидируется — claims должны вернуться как положили
func TestJWTManager_AccessTokenRoundtrip(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateAccessToken(userID, "testuser", models.RoleAdmin)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := manager.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, models.RoleAdmin, claims.Role)
	assert.Equal(t, userID.String(), claims.Subject)
}

func TestJWTManager_RefreshTokenRoundtrip(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	got, err := manager.ValidateRefreshToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

// alg confusion: токен подписан методом none, менеджер обязан его отбить
func TestJWTManager_ValidateToken_RejectsNoneAlg(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	claims := &Claims{UserID: uuid.New(), Username: "attacker"}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = manager.ValidateToken(signed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

// старые токены выписаны ещё без роли, после выката роль должна становиться user
func TestJWTManager_ValidateToken_EmptyRoleDefaultsToUser(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	claims := &Claims{
		UserID:   uuid.New(),
		Username: "olduser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Subject:   uuid.New().String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(manager.secretKey)
	require.NoError(t, err)

	got, err := manager.ValidateToken(signed)
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, got.Role)
}

func TestJWTManager_ValidateToken_WrongSecret(t *testing.T) {
	manager1 := NewJWTManager("secret-1", 15*time.Minute, 7*24*time.Hour)
	manager2 := NewJWTManager("secret-2", 15*time.Minute, 7*24*time.Hour)

	token, err := manager1.GenerateAccessToken(uuid.New(), "testuser", models.RoleUser)
	require.NoError(t, err)

	// чужим секретом валидировать нельзя
	_, err = manager2.ValidateToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestJWTManager_ValidateToken_Expired(t *testing.T) {
	manager := NewJWTManager("test-secret", 1*time.Millisecond, 7*24*time.Hour)

	token, err := manager.GenerateAccessToken(uuid.New(), "testuser", models.RoleUser)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = manager.ValidateToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestJWTManager_ValidateToken_Garbage(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	_, err := manager.ValidateToken("not-a-valid-jwt")
	assert.Error(t, err)

	_, err = manager.ValidateToken("")
	assert.Error(t, err)
}

func TestJWTManager_ValidateRefreshToken_WrongSecret(t *testing.T) {
	manager1 := NewJWTManager("secret-1", 15*time.Minute, 7*24*time.Hour)
	manager2 := NewJWTManager("secret-2", 15*time.Minute, 7*24*time.Hour)

	token, err := manager1.GenerateRefreshToken(uuid.New())
	require.NoError(t, err)

	_, err = manager2.ValidateRefreshToken(token)
	assert.Error(t, err)
}

func TestJWTManager_ValidateRefreshToken_Expired(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 1*time.Millisecond)

	token, err := manager.GenerateRefreshToken(uuid.New())
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = manager.ValidateRefreshToken(token)
	assert.Error(t, err)
}

// у каждого рефреша свой jti, иначе ротация токенов ломается
func TestJWTManager_RefreshTokensHaveUniqueJTI(t *testing.T) {
	manager := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	token1, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)
	token2, err := manager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	assert.NotEqual(t, token1, token2)
}

func TestJWTManager_TTLGetters(t *testing.T) {
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour
	manager := NewJWTManager("test-secret", accessTTL, refreshTTL)

	assert.Equal(t, accessTTL, manager.AccessTokenTTL())
	assert.Equal(t, refreshTTL, manager.RefreshTokenTTL())
}
