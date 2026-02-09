package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// RateLimiter интерфейс для rate limiting
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// RateLimit middleware для ограничения количества запросов
func RateLimit(limiter RateLimiter, limit int, window time.Duration, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем IP адрес клиента
			ip := getClientIP(r)

			// Bypass rate limiting for localhost only in non-production environments
			// (useful for local development and tests).
			if os.Getenv("ENVIRONMENT") != "production" && isLocalhost(ip) {
				next.ServeHTTP(w, r)
				return
			}

			key := fmt.Sprintf("ratelimit:%s", ip)

			// Проверяем лимит
			allowed, err := limiter.Allow(r.Context(), key, limit, window)
			if err != nil {
				log.LogError("Rate limit check failed", err,
					zap.String("ip", ip),
				)
				// Intentional fail-open: when the rate limiter backend (Redis) is
				// unavailable, we allow the request through to preserve service
				// availability. Rate limiting is a best-effort protection; dropping
				// legitimate traffic due to an infrastructure failure is worse than
				// temporarily losing rate limit enforcement.
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				log.Info("Rate limit exceeded",
					zap.String("ip", ip),
					zap.String("path", r.URL.Path),
				)

				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
				w.Header().Set("X-RateLimit-Window", window.String())
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))

				writeError(w, errors.ErrRateLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isLocalhost checks if the IP is localhost
func isLocalhost(ip string) bool {
	// Handle IPv4 localhost
	if ip == "127.0.0.1" || ip == "localhost" {
		return true
	}
	// Handle IPv6 localhost
	if ip == "::1" || ip == "[::1]" {
		return true
	}
	// Handle localhost with port (e.g., "127.0.0.1:xxxxx" or "[::1]:xxxxx")
	if len(ip) > 9 && ip[:9] == "127.0.0.1" {
		return true
	}
	if len(ip) > 4 && (ip[:4] == "::1:" || ip[:5] == "[::1]") {
		return true
	}
	return false
}

// getClientIP извлекает IP адрес клиента из запроса.
// X-Forwarded-For может contain a comma-separated list of IPs when multiple
// proxies are involved: "client, proxy1, proxy2". The leftmost (first) entry
// is the original client IP.
func getClientIP(r *http.Request) string {
	// Проверяем заголовки прокси
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) IP — the original client address.
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Используем RemoteAddr
	return r.RemoteAddr
}
