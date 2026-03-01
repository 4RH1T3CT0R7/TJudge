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
