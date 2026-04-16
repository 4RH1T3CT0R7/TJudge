package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func newTestWebSocketHandler(t *testing.T) *WebSocketHandler {
	t.Helper()
	log, _ := logger.New("error", "json")
	hub := websocket.NewHub(log)
	return NewWebSocketHandler(hub, log)
}

func TestWebSocketHandler_GetStats(t *testing.T) {
	handler := newTestWebSocketHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/ws/stats", nil)
	rr := httptest.NewRecorder()

	handler.GetStats(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var stats map[string]interface{}
	decodeJSONData(t, rr.Body, &stats)
	assert.Contains(t, stats, "tournaments")
	assert.Contains(t, stats, "total_clients")
}

func TestWebSocketHandler_HandleTournament_InvalidUUID(t *testing.T) {
	handler := newTestWebSocketHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/ws/tournaments/not-uuid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.HandleTournament(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWebSocketHandler_HandleTournament_MissingAuth(t *testing.T) {
	handler := newTestWebSocketHandler(t)
	tournamentID := uuid.New()

	req := httptest.NewRequest("GET", "/api/v1/ws/tournaments/"+tournamentID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	// No middleware.UserIDKey in context
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.HandleTournament(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// Note: HandleTournament with valid auth + WebSocket upgrade cannot be tested
// with httptest.NewRecorder since gorilla/websocket requires a real HTTP connection.
// The test would need httptest.NewServer + a real WebSocket dialer.

// TestCheckWebSocketOrigin_ProdFailClosed защищает от CSWSH: в prod
// wildcard и пустой origin-list ДОЛЖНЫ отклоняться (P0.2).
func TestCheckWebSocketOrigin_ProdFailClosed(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://evil.example")
	assert.False(t, checkWebSocketOrigin(req), "empty allowlist in prod must reject")
}

func TestCheckWebSocketOrigin_ProdWildcardReject(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "*")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://evil.example")
	assert.False(t, checkWebSocketOrigin(req), "wildcard in prod must be rejected")
}

func TestCheckWebSocketOrigin_ProdExplicitAllow(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "https://tjudge.example,https://admin.tjudge.example")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://tjudge.example")
	assert.True(t, checkWebSocketOrigin(req))
}

func TestCheckWebSocketOrigin_ProdExplicitReject(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "https://tjudge.example")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://evil.example")
	assert.False(t, checkWebSocketOrigin(req))
}

func TestCheckWebSocketOrigin_ProdEmptyOriginBrowser(t *testing.T) {
	// Browser-initiated request всегда шлёт Sec-Fetch-Site → rejected при пустом Origin.
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "https://tjudge.example")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	// Нет Origin-заголовка
	assert.False(t, checkWebSocketOrigin(req), "browser client with empty Origin must be rejected in prod")
}

func TestCheckWebSocketOrigin_ProdEmptyOriginNonBrowser(t *testing.T) {
	// curl/bot не шлёт Sec-Fetch-Site → разрешаем.
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "https://tjudge.example")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	// Ни Origin, ни Sec-Fetch-Site
	assert.True(t, checkWebSocketOrigin(req), "non-browser client in prod should be allowed")
}

func TestCheckWebSocketOrigin_DevAllowAll(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	assert.True(t, checkWebSocketOrigin(req))
}

func TestCheckWebSocketOrigin_DevWildcardAllow(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "*")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "http://some.other")
	assert.True(t, checkWebSocketOrigin(req))
}
