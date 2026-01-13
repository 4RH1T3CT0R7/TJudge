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

	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1:12345", 100, time.Minute).Return(true, nil)

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

	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1:12345", 100, time.Minute).Return(false, nil)

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

func TestRateLimit_XForwardedFor(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// Should use X-Forwarded-For header for IP
	mockLimiter.On("Allow", mock.Anything, "ratelimit:10.0.0.1", 100, time.Minute).Return(true, nil)

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

func TestRateLimit_XRealIP(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// Should use X-Real-IP header for IP when X-Forwarded-For is not set
	mockLimiter.On("Allow", mock.Anything, "ratelimit:10.0.0.2", 100, time.Minute).Return(true, nil)

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

func TestRateLimit_ErrorFailsOpen(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// When limiter returns error, should fail open (allow request)
	mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1:12345", 100, time.Minute).Return(false, assert.AnError)

	handler := middleware.RateLimit(mockLimiter, 100, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should pass through despite error (fail open)
	assert.Equal(t, http.StatusOK, rr.Code)
	mockLimiter.AssertExpectations(t)
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

			mockLimiter.On("Allow", mock.Anything, "ratelimit:192.168.1.1:12345", tc.limit, tc.window).Return(true, nil)

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
