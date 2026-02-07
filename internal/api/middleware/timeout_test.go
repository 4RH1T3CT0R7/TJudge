package middleware

import (
	"context"
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

func TestSmartTimeout_GetTournaments_DatabaseTimeout(t *testing.T) {
	config := DefaultTimeoutConfig()

	handler := SmartTimeout(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		assert.True(t, ok)
		remaining := time.Until(deadline)
		assert.InDelta(t, config.Database.Seconds(), remaining.Seconds(), 1.0)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/tournaments", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSmartTimeout_POSTRequest_CacheTimeout(t *testing.T) {
	config := DefaultTimeoutConfig()

	handler := SmartTimeout(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		assert.True(t, ok)
		remaining := time.Until(deadline)
		assert.InDelta(t, config.Cache.Seconds(), remaining.Seconds(), 1.0)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
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

func TestWithOperationTimeout_Database(t *testing.T) {
	config := DefaultTimeoutConfig()
	ctx := context.Background()

	newCtx, cancel := WithOperationTimeout(ctx, OperationDatabase, config)
	defer cancel()

	deadline, ok := newCtx.Deadline()
	assert.True(t, ok)
	remaining := time.Until(deadline)
	assert.InDelta(t, config.Database.Seconds(), remaining.Seconds(), 1.0)
}

func TestWithOperationTimeout_Cache(t *testing.T) {
	config := DefaultTimeoutConfig()
	ctx := context.Background()

	newCtx, cancel := WithOperationTimeout(ctx, OperationCache, config)
	defer cancel()

	deadline, ok := newCtx.Deadline()
	assert.True(t, ok)
	remaining := time.Until(deadline)
	assert.InDelta(t, config.Cache.Seconds(), remaining.Seconds(), 1.0)
}

func TestWithOperationTimeout_Heavy(t *testing.T) {
	config := DefaultTimeoutConfig()
	ctx := context.Background()

	newCtx, cancel := WithOperationTimeout(ctx, OperationHeavy, config)
	defer cancel()

	deadline, ok := newCtx.Deadline()
	assert.True(t, ok)
	remaining := time.Until(deadline)
	assert.InDelta(t, config.Heavy.Seconds(), remaining.Seconds(), 1.0)
}

func TestWithOperationTimeout_WebSocket_NoTimeout(t *testing.T) {
	config := DefaultTimeoutConfig()
	ctx := context.Background()

	newCtx, cancel := WithOperationTimeout(ctx, OperationWebSocket, config)
	defer cancel()

	_, ok := newCtx.Deadline()
	assert.False(t, ok, "WebSocket should not have deadline")
}

func TestWithOperationTimeout_Default(t *testing.T) {
	config := DefaultTimeoutConfig()
	ctx := context.Background()

	newCtx, cancel := WithOperationTimeout(ctx, OperationDefault, config)
	defer cancel()

	deadline, ok := newCtx.Deadline()
	assert.True(t, ok)
	remaining := time.Until(deadline)
	assert.InDelta(t, config.Default.Seconds(), remaining.Seconds(), 1.0)
}

func TestDefaultTimeoutConfig_Values(t *testing.T) {
	config := DefaultTimeoutConfig()

	assert.Equal(t, 10*time.Second, config.Default)
	assert.Equal(t, 15*time.Second, config.Database)
	assert.Equal(t, 5*time.Second, config.Cache)
	assert.Equal(t, 30*time.Second, config.Heavy)
	assert.Equal(t, time.Duration(0), config.WebSocket)
}
