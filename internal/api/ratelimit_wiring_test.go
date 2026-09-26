package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/handlers"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/auth"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTokens struct{ userID uuid.UUID }

func (s stubTokens) ValidateToken(string) (*auth.Claims, error) {
	return &auth.Claims{UserID: s.userID}, nil
}

func (s stubTokens) GetUserFromToken(context.Context, string) (*models.User, error) {
	return &models.User{ID: s.userID}, nil
}

func (stubTokens) IsTokenBlacklisted(context.Context, string) (bool, error) { return false, nil }

// запоминает ключи и отказывает счётчику по ip, чтобы до хендлера не дойти
type keyRecorder struct {
	mu   sync.Mutex
	keys []string
}

func (k *keyRecorder) Allow(_ context.Context, key string, _ int, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys = append(k.keys, key)
	return !strings.Contains(key, ":ip:"), nil
}

// логин с чужим bearer-токеном всё равно считается по ip: иначе каждый
// зарегистрированный аккаунт давал бы перебору паролей свой счётчик
// шлёт POST с bearer-токеном и возвращает код ответа и ключи лимитера
func postWithToken(t *testing.T, path string) (int, []string, uuid.UUID) {
	t.Helper()
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	userID := uuid.New()
	limiter := &keyRecorder{}
	s := NewServer(ServerDeps{
		// хендлеры не вызываются, но у GameHandler встроенные указатели
		GameHandler: &handlers.GameHandler{
			GameCRUDHandler:       &handlers.GameCRUDHandler{},
			TournamentGameHandler: &handlers.TournamentGameHandler{},
			GameRoundHandler:      &handlers.GameRoundHandler{},
		},
		AuthService: stubTokens{userID: userID},
		RateLimiter: limiter,
		RateLimit:   config.RateLimitConfig{Enabled: true, RequestsPerMinute: 100},
		Log:         log,
	})
	defer s.Close()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
	req.RemoteAddr = "203.0.113.9:5000"
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec.Code, limiter.keys, userID
}

func TestAuthRateLimit_KeyedByIP(t *testing.T) {
	code, keys, userID := postWithToken(t, "/api/v1/auth/login")

	assert.Equal(t, http.StatusTooManyRequests, code)
	assert.Equal(t, []string{
		"ratelimit:api:user:" + userID.String(),
		"ratelimit:auth:ip:203.0.113.9",
	}, keys)
}

// вступление по коду тоже по ip: иначе пачка аккаунтов с одного адреса
// перебирала бы инвайт-коды каждый в своём счётчике
func TestJoinRateLimit_KeyedByIP(t *testing.T) {
	code, keys, userID := postWithToken(t, "/api/v1/teams/join")

	assert.Equal(t, http.StatusTooManyRequests, code)
	assert.Equal(t, []string{
		"ratelimit:api:user:" + userID.String(),
		"ratelimit:join:ip:203.0.113.9",
	}, keys)
}
