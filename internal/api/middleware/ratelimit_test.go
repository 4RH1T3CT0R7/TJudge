package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRateLimiter implements middleware.RateLimiter for testing
type MockRateLimiter struct {
	mock.Mock
}

func (m *MockRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	args := m.Called(ctx, key, limit, window)
	return args.Bool(0), args.Error(1)
}

func TestRateLimit_AllowedRequest(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1", 100, time.Minute).Return(true, nil)

	handler := middleware.RateLimit(mockLimiter, 100, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockLimiter.AssertExpectations(t)
}

func TestRateLimit_ExceededLimit(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1", 100, time.Minute).Return(false, nil)

	handler := middleware.RateLimit(mockLimiter, 100, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when rate limit exceeded")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "100", rr.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "1m0s", rr.Header().Get("X-RateLimit-Window"))
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
	mockLimiter.AssertExpectations(t)
}

func TestRateLimit_LocalhostBypass(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// Localhost should bypass rate limiting - no mock calls expected
	handler := middleware.RateLimit(mockLimiter, 100, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testCases := []struct {
		name       string
		remoteAddr string
	}{
		{"IPv4 localhost", "127.0.0.1:12345"},
		{"IPv6 localhost", "::1"},
		{"IPv6 localhost with brackets", "[::1]:12345"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remoteAddr
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}

	// Verify limiter was never called (localhost bypass)
	mockLimiter.AssertNotCalled(t, "Allow", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRateLimit_XForwardedFor_Ignored(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// X-Forwarded-For should NOT be used directly; getClientIP uses only
	// r.RemoteAddr (which chi's RealIP middleware sets from trusted proxies).
	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1", 100, time.Minute).Return(true, nil)

	handler := middleware.RateLimit(mockLimiter, 100, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockLimiter.AssertExpectations(t)
}

func TestRateLimit_XRealIP_Ignored(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// X-Real-IP should NOT be used directly; getClientIP uses only
	// r.RemoteAddr (which chi's RealIP middleware sets from trusted proxies).
	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1", 100, time.Minute).Return(true, nil)

	handler := middleware.RateLimit(mockLimiter, 100, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Real-IP", "10.0.0.2")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockLimiter.AssertExpectations(t)
}

func TestRateLimit_ErrorFallsBackToInMemory(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// When limiter returns error, should fall back to in-memory limiter (2x limit)
	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1", 5, time.Minute).Return(false, assert.AnError)

	handlerCalled := 0
	handler := middleware.RateLimit(mockLimiter, 5, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled++
		w.WriteHeader(http.StatusOK)
	}))

	// Fallback limit = 5 * 2 = 10 (burst). First 10 requests should pass.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should pass", i+1)
	}
	assert.Equal(t, 10, handlerCalled)

	// Next request should be rate-limited by fallback
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimit_ErrorFallbackPerIP(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// Both IPs will fail Redis
	mockLimiter.On("Allow", mock.Anything, mock.Anything, 1, time.Minute).Return(false, assert.AnError)

	handler := middleware.RateLimit(mockLimiter, 1, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust fallback for IP1 (burst = 2)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// IP1 should now be blocked
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// IP2 should still pass (separate bucket)
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimit_DifferentWindows(t *testing.T) {
	testCases := []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{"Short window", 10, 10 * time.Second},
		{"Medium window", 100, time.Minute},
		{"Long window", 1000, time.Hour},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockLimiter := new(MockRateLimiter)
			log := newTestLogger()

			mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1", tc.limit, tc.window).Return(true, nil)

			handler := middleware.RateLimit(mockLimiter, tc.limit, tc.window, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			mockLimiter.AssertExpectations(t)
		})
	}
}
