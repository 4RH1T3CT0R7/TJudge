package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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
			key := fmt.Sprintf("ratelimit:%s", ip)

			// Проверяем лимит
			allowed, err := limiter.Allow(r.Context(), key, limit, window)
			if err != nil {
				log.LogError("Rate limit check failed", err,
					zap.String("ip", ip),
				)
				// В случае ошибки пропускаем запрос (fail open)
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

// getClientIP извлекает IP адрес клиента из запроса
func getClientIP(r *http.Request) string {
	// Проверяем заголовки прокси
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Используем RemoteAddr
	return r.RemoteAddr
}
