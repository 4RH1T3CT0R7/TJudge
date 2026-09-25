package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSmartTimeout_WebSocket_NoTimeout(t *testing.T) {
	config := DefaultTimeoutConfig()

	handler := SmartTimeout(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := r.Context().Deadline()
		assert.False(t, ok, "WebSocket request should not have a deadline")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/ws/tournaments/123", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSmartTimeout_Leaderboard_HeavyTimeout(t *testing.T) {
	config := DefaultTimeoutConfig()

	handler := SmartTimeout(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		assert.True(t, ok)
		remaining := time.Until(deadline)
		assert.InDelta(t, config.Heavy.Seconds(), remaining.Seconds(), 1.0)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/tournaments/123/leaderboard", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSmartTimeout_Default(t *testing.T) {
	config := DefaultTimeoutConfig()

	handler := SmartTimeout(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		assert.True(t, ok)
		remaining := time.Until(deadline)
		assert.InDelta(t, config.Default.Seconds(), remaining.Seconds(), 1.0)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDefaultTimeoutConfig_Values(t *testing.T) {
	config := DefaultTimeoutConfig()

	assert.Equal(t, 10*time.Second, config.Default)
	assert.Equal(t, 15*time.Second, config.Database)
	assert.Equal(t, 5*time.Second, config.Cache)
	assert.Equal(t, 30*time.Second, config.Heavy)
	assert.Equal(t, time.Duration(0), config.WebSocket)
}

// админские ручки планирования не должны получать 5-секундный таймаут записи
func TestGetTimeoutForRequest_SchedulingIsHeavy(t *testing.T) {
	config := DefaultTimeoutConfig()
	for _, path := range []string{
		"/api/v1/tournaments/x/run-matches",
		"/api/v1/tournaments/x/run-game-matches",
		"/api/v1/tournaments/x/retry-matches",
		"/api/v1/tournaments/x/games/y/reset-round",
		"/api/v1/tournaments/x/complete",
	} {
		req := httptest.NewRequest("POST", path, nil)
		assert.Equal(t, config.Heavy, getTimeoutForRequest(req, config), path)
	}
	// обычная запись остаётся короткой
	for _, path := range []string{"/api/v1/teams", "/api/v1/tournaments/x/games/y/complete-round"} {
		req := httptest.NewRequest("POST", path, nil)
		assert.Equal(t, config.Cache, getTimeoutForRequest(req, config), path)
	}
}
