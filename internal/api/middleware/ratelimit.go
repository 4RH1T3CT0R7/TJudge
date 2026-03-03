package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiter интерфейс для rate limiting
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// fallbackLimiter обеспечивает in-memory rate limiting при недоступности Redis.
// Использует token bucket per-IP с более мягкими порогами (2x от основного лимита).
type fallbackLimiter struct {
	mu       sync.Mutex
	limiters map[string]*fallbackEntry
	rate     rate.Limit
	burst    int
}

type fallbackEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newFallbackLimiter(limit int, window time.Duration) *fallbackLimiter {
	// Fallback использует 2x лимит — более мягкий порог для легитимного трафика
	fallbackLimit := limit * 2
	r := rate.Limit(float64(fallbackLimit) / window.Seconds())

	return &fallbackLimiter{
		limiters: make(map[string]*fallbackEntry),
		rate:     r,
		burst:    fallbackLimit,
	}
}

func (f *fallbackLimiter) allow(ip string) bool {
	f.mu.Lock()
	entry, ok := f.limiters[ip]
	if !ok {
		entry = &fallbackEntry{
			limiter: rate.NewLimiter(f.rate, f.burst),
		}
		f.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	f.mu.Unlock()

	return entry.limiter.Allow()
}

// cleanup удаляет записи, которые не обновлялись дольше maxAge.
func (f *fallbackLimiter) cleanup(maxAge time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	for ip, entry := range f.limiters {
		if now.Sub(entry.lastSeen) > maxAge {
			delete(f.limiters, ip)
		}
	}
}

// RateLimit middleware для ограничения количества запросов.
// При ошибке основного лимитера (Redis) переключается на in-memory fallback
// с более мягкими порогами (2x), вместо полного fail-open.
func RateLimit(limiter RateLimiter, limit int, window time.Duration, log *logger.Logger, stopCh ...chan struct{}) func(http.Handler) http.Handler {
	fallback := newFallbackLimiter(limit, window)

	// Периодическая очистка устаревших записей fallback лимитера.
	// Передайте stopCh для graceful shutdown (горутина завершится при закрытии канала).
	var stop <-chan struct{}
	if len(stopCh) > 0 && stopCh[0] != nil {
		stop = stopCh[0]
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stop: // receiving from nil channel blocks forever, so cleanup runs indefinitely when no stop channel is provided
				return
			case <-ticker.C:
				fallback.cleanup(10 * time.Minute)
			}
		}
	}()

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
				log.Warn("Rate limit check failed, falling back to in-memory limiter",
					zap.String("ip", ip),
					zap.Error(err),
				)

				// Fallback: in-memory rate limiter с более мягкими порогами (2x).
				// Защищает от DDoS при падении Redis, не блокируя легитимный трафик.
				if !fallback.allow(ip) {
					log.Info("Rate limit exceeded (fallback)",
						zap.String("ip", ip),
						zap.String("path", r.URL.Path),
					)

					w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
					w.Header().Set("X-RateLimit-Window", window.String())
					w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))

					httputil.WriteError(w, errors.ErrRateLimitExceeded)
					return
				}

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

				httputil.WriteError(w, errors.ErrRateLimitExceeded)
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
// Uses RemoteAddr which is already set by chi's RealIP middleware
// from trusted proxy headers. Don't re-read raw headers to avoid
// spoofing bypass.
func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
