package api

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/handlers"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/auth"
	"github.com/bmstu-itstech/tjudge/internal/ws"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuth пропускает любой токен как обычного юзера
type stubAuth struct{ userID uuid.UUID }

func (a stubAuth) ValidateToken(string) (*auth.Claims, error) {
	return &auth.Claims{UserID: a.userID, Role: models.RoleUser}, nil
}

func (a stubAuth) GetUserFromToken(context.Context, string) (*models.User, error) {
	return &models.User{ID: a.userID}, nil
}

func (stubAuth) IsTokenBlacklisted(context.Context, string) (bool, error) { return false, nil }

// хендшейк через полную цепочку мидлварей с браузерным Accept-Encoding:
// gzip-обёртка без Hijack отдавала на него 500
func TestServer_WebSocketHandshakeWithGzip(t *testing.T) {
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	hub := ws.NewHub(log)
	go hub.Run(t.Context())

	// остальные хендлеры пустые: роуты только регистрируются, не вызываются
	srv := NewServer(ServerDeps{
		AuthHandler:       &handlers.AuthHandler{},
		TournamentHandler: &handlers.TournamentHandler{},
		ProgramHandler:    &handlers.ProgramHandler{},
		MatchHandler:      &handlers.MatchHandler{},
		GameHandler:       &handlers.GameHandler{},
		TeamHandler:       &handlers.TeamHandler{},
		SystemHandler:     &handlers.SystemHandler{},
		WSHandler:         handlers.NewWebSocketHandler(hub, log),
		AuthService:       stubAuth{userID: uuid.New()},
		Log:               log,
	})
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws/tournaments/" + uuid.NewString()
	dialer := websocket.Dialer{Subprotocols: []string{"access_token.test"}}
	conn, resp, err := dialer.Dial(url, http.Header{"Accept-Encoding": {"gzip, deflate, br"}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Content-Encoding"))
}

// отклоняет любой токен, как протухший
type expiredTokens struct{}

func (expiredTokens) ValidateToken(string) (*auth.Claims, error) {
	return nil, stderrors.New("token is expired")
}

func (expiredTokens) GetUserFromToken(context.Context, string) (*models.User, error) {
	return nil, stderrors.New("token is expired")
}

func (expiredTokens) IsTokenBlacklisted(context.Context, string) (bool, error) { return false, nil }

type logoutRecorder struct {
	handlers.AuthService
	refresh string
}

func (l *logoutRecorder) Logout(_ context.Context, _, refreshToken string) error {
	l.refresh = refreshToken
	return nil
}

// logout с протухшим access доходит до хендлера: иначе refresh из тела не
// отзывался и жил на сервере до конца срока
func TestServer_LogoutWithExpiredAccess(t *testing.T) {
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	svc := &logoutRecorder{}
	srv := NewServer(ServerDeps{
		AuthHandler: handlers.NewAuthHandler(svc, log),
		GameHandler: &handlers.GameHandler{
			GameCRUDHandler:       &handlers.GameCRUDHandler{},
			TournamentGameHandler: &handlers.TournamentGameHandler{},
			GameRoundHandler:      &handlers.GameRoundHandler{},
		},
		AuthService: expiredTokens{},
		Log:         log,
	})
	t.Cleanup(srv.Close)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{"refresh_token":"r1"}`))
	req.Header.Set("Authorization", "Bearer expired")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "r1", svc.refresh)
}

// TestServer_Close_Idempotent - регрессия на идемпотентность Close().
// Close() безопасен для повторного вызова и закрывает stopCh для cleanup-
// горутины fallback-лимитера. Тест работает на минимальной структуре без
// поднятия полного NewServer (handlers nil).
func TestServer_Close_Idempotent(t *testing.T) {
	s := &Server{
		rateLimitStopCh: make(chan struct{}),
	}

	assert.NotPanics(t, func() { s.Close() }, "первый Close не должен паниковать")
	assert.NotPanics(t, func() { s.Close() }, "повторный Close не должен паниковать")
	assert.NotPanics(t, func() { s.Close() }, "третий Close тоже OK")

	// После Close канал должен быть закрыт - чтение возвращает zero-value без блокировки.
	select {
	case <-s.rateLimitStopCh:
		// ok - канал закрыт
	default:
		t.Fatal("rateLimitStopCh должен быть закрыт после Close")
	}
}

// TestServer_Close_NilStopCh - Close на пустом Server не должен паниковать,
// даже если rateLimitStopCh не инициализирован
func TestServer_Close_NilStopCh(t *testing.T) {
	s := &Server{}
	assert.NotPanics(t, func() { s.Close() })
}
