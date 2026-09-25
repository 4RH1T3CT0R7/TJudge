package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/service/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRateLimiter - мок RateLimiter
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

	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:ip:192.168.1.1", 100, time.Minute).Return(true, nil)

	handler := middleware.RateLimit(mockLimiter, "api", 100, time.Minute, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockLimiter.AssertExpectations(t)
}

func TestRateLimit_ExceededLimit(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:ip:192.168.1.1", 100, time.Minute).Return(false, nil)

	handler := middleware.RateLimit(mockLimiter, "api", 100, time.Minute, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when rate limit exceeded")
	}))

	req := httptest.NewRequest("POST", "/", nil)
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

	// localhost обходит rate limit - мок вызываться не должен
	handler := middleware.RateLimit(mockLimiter, "api", 100, time.Minute, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// limiter не должен вызываться (localhost bypass)
	mockLimiter.AssertNotCalled(t, "Allow", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRateLimit_XForwardedFor_Ignored(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// X-Forwarded-For напрямую брать нельзя; getClientIP смотрит только на
	// r.RemoteAddr (его ставит RealIP из доверенных прокси)
	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:ip:192.168.1.1", 100, time.Minute).Return(true, nil)

	handler := middleware.RateLimit(mockLimiter, "api", 100, time.Minute, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
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

	// то же для X-Real-IP - он тоже игнорируется, берётся только r.RemoteAddr
	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:ip:192.168.1.1", 100, time.Minute).Return(true, nil)

	handler := middleware.RateLimit(mockLimiter, "api", 100, time.Minute, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
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

	// при ошибке Redis fallback строже основного (0.5x), а не 2x -
	// иначе rate limit можно обойти, положив Redis
	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:ip:192.168.1.1", 10, time.Minute).Return(false, assert.AnError)

	handlerCalled := 0
	handler := middleware.RateLimit(mockLimiter, "api", 10, time.Minute, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled++
		w.WriteHeader(http.StatusOK)
	}))

	// fallback burst = max(1, int(10*0.5)) = 5, первые 5 запросов проходят
	for i := range 5 {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should pass", i+1)
	}
	assert.Equal(t, 5, handlerCalled)

	// следующий уже режется fallback'ом (строже основного)
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimit_ErrorFallbackPerIP(t *testing.T) {
	mockLimiter := new(MockRateLimiter)
	log := newTestLogger()

	// оба IP получат ошибку Redis
	mockLimiter.On("Allow", mock.Anything, mock.Anything, 1, time.Minute).Return(false, assert.AnError)

	handler := middleware.RateLimit(mockLimiter, "api", 1, time.Minute, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// выжимается fallback для IP1: limit=1, множитель 0.5,
	// int(1*0.5)=0, но зажимается к минимуму 1, значит burst=1
	for range 1 {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// IP1 теперь заблокирован
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// IP2 по-прежнему проходит - у него свой bucket
	req = httptest.NewRequest("POST", "/", nil)
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

			mockLimiter.On("Allow", mock.Anything, "ratelimit:api:ip:192.168.1.1", tc.limit, tc.window).Return(true, nil)

			handler := middleware.RateLimit(mockLimiter, "api", tc.limit, tc.window, nil, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("POST", "/", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			mockLimiter.AssertExpectations(t)
		})
	}
}

// заголовкам с адресом клиента верится только от доверенного прокси,
// True-Client-IP игнорируется всегда
func TestRealIP(t *testing.T) {
	cases := []struct {
		name    string
		trusted []string
		remote  string
		headers map[string]string
		want    string
	}{
		{"прямой клиент подделывает заголовки", nil, "203.0.113.9:5000",
			map[string]string{"X-Forwarded-For": "1.1.1.1", "X-Real-IP": "2.2.2.2", "True-Client-IP": "3.3.3.3"}, "203.0.113.9:5000"},
		{"True-Client-IP от nginx не читается", nil, "172.28.0.5:5000",
			map[string]string{"True-Client-IP": "3.3.3.3", "X-Real-IP": "198.51.100.7"}, "198.51.100.7"},
		{"без списка XFF не читается: клиент из приватной сети", nil, "172.28.0.5:5000",
			map[string]string{"X-Forwarded-For": "6.6.6.6, 10.0.0.42", "X-Real-IP": "10.0.0.42"}, "10.0.0.42"},
		{"прокси перед nginx", []string{"172.28.0.0/16", "203.0.113.0/24"}, "172.28.0.5:5000",
			map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.7, 203.0.113.1", "X-Real-IP": "203.0.113.1"}, "198.51.100.7"},
		{"приватный адрес вне списка не пропускается", []string{"172.28.0.0/16"}, "172.28.0.5:5000",
			map[string]string{"X-Forwarded-For": "6.6.6.6, 10.0.0.42", "X-Real-IP": "10.0.0.42"}, "10.0.0.42"},
		{"свой список заменяет приватные сети", []string{"203.0.113.0/24"}, "172.28.0.5:5000",
			map[string]string{"X-Real-IP": "198.51.100.7"}, "172.28.0.5:5000"},
		{"адрес из своего списка", []string{"203.0.113.0/24"}, "203.0.113.10:5000",
			map[string]string{"X-Real-IP": "198.51.100.7"}, "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := middleware.RealIP(tc.trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remote
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			assert.Equal(t, tc.want, got)
		})
	}
}

// залогиненный считается по user id, а не по общему ip за NAT;
// чтение идёт в свой счётчик с лимитом в 10 раз выше
func TestRateLimit_SubjectAndReadBucket(t *testing.T) {
	userID := uuid.New()
	tokens := new(MockAuthService)
	tokens.On("ValidateToken", "good").Return(&auth.Claims{UserID: userID}, nil)
	tokens.On("ValidateToken", "bad").Return(nil, assert.AnError)

	mockLimiter := new(MockRateLimiter)
	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:user:"+userID.String(), 100, time.Minute).Return(true, nil).Once()
	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:ip:192.168.1.1", 100, time.Minute).Return(true, nil).Once()
	mockLimiter.On("Allow", mock.Anything, "ratelimit:api:read:user:"+userID.String(), 1000, time.Minute).Return(true, nil).Once()

	handler := middleware.RateLimit(mockLimiter, "api", 100, time.Minute, tokens, newTestLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for _, tc := range []struct{ method, token string }{
		{"POST", "good"},
		{"POST", "bad"},
		{"GET", "good"},
	} {
		req := httptest.NewRequest(tc.method, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		req.Header.Set("Authorization", "Bearer "+tc.token)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	mockLimiter.AssertExpectations(t)
}
