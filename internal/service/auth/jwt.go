package auth

import (
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// тип токена в claim typ. без него access и refresh были взаимозаменяемы:
// утёкший access менялся на бесконечную цепочку refresh
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// iat с миллисекундами: по нему смена пароля отзывает refresh-токены, а с
// точностью до секунды токен, выписанный в ту же секунду до смены, её переживал
func init() {
	jwt.TimePrecision = time.Millisecond
}

// Claims — то что лежит в access токене помимо стандартных полей: id юзера, ник и роль
type Claims struct {
	UserID    uuid.UUID   `json:"user_id"`
	Username  string      `json:"username"`
	Role      models.Role `json:"role"`
	TokenType string      `json:"typ"`
	jwt.RegisteredClaims
}

// refreshClaims — в refresh только тип и стандартные поля, id юзера в sub
type refreshClaims struct {
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// JWTManager подписывает и проверяет токены
type JWTManager struct {
	secretKey  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secretKey string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:  []byte(secretKey),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// GenerateAccessToken делает access токен, внутри роль, живёт accessTTL
func (jm *JWTManager) GenerateAccessToken(userID uuid.UUID, username string, role models.Role) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(jm.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jm.secretKey)
}

// GenerateRefreshToken делает refresh токен, тут только id юзера, живёт дольше access
func (jm *JWTManager) GenerateRefreshToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := &refreshClaims{
		TokenType: tokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(jm.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   userID.String(),
			ID:        uuid.New().String(), // jti, чтобы каждый рефреш был уникальным
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jm.secretKey)
}

// ValidateToken проверяет подпись и тип access, достаёт claims
func (jm *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	if err := jm.parse(tokenString, claims); err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if claims.TokenType != tokenTypeAccess {
		return nil, fmt.Errorf("invalid token type %q", claims.TokenType)
	}
	return claims, nil
}

// ValidateRefreshToken проверяет refresh токен, отдаёт id юзера и время выпуска
// (по нему отсекаются токены, выписанные до смены пароля)
func (jm *JWTManager) ValidateRefreshToken(tokenString string) (uuid.UUID, time.Time, error) {
	claims := &refreshClaims{}
	if err := jm.parse(tokenString, claims); err != nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("invalid refresh token: %w", err)
	}
	if claims.TokenType != tokenTypeRefresh || claims.IssuedAt == nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("invalid refresh token type %q", claims.TokenType)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("invalid user id in token: %w", err)
	}

	return userID, claims.IssuedAt.Time, nil
}

// parse проверяет подпись и сроки
func (jm *JWTManager) parse(tokenString string, claims jwt.Claims) error {
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// обязательно проверяется что алгоритм именно HMAC. если этого не делать,
		// можно подсунуть токен подписанный другим методом (alg confusion) и пролезть
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jm.secretKey, nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("invalid token claims")
	}
	return nil
}

func (jm *JWTManager) RefreshTokenTTL() time.Duration {
	return jm.refreshTTL
}

func (jm *JWTManager) AccessTokenTTL() time.Duration {
	return jm.accessTTL
}
