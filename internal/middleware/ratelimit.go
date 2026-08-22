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

// RateLimiter - основной лимитер (живёт в редисе)
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// fallbackLimiter - запасной in-memory лимитер на случай падения редиса.
// token bucket на каждый ip
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

// запасной лимит СТРОЖЕ основного (0.5 = вдвое). если бы fallback был мягче,
// уронить редис = способ обойти основной лимит
const fallbackLimitMultiplier = 0.5

func newFallbackLimiter(limit int, window time.Duration) *fallbackLimiter {
	// минимум 1, чтобы limit=1 не превратился в ноль
	fallbackLimit := max(int(float64(limit)*fallbackLimitMultiplier), 1)
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

// cleanup выкидывает ip которых не видели дольше maxAge
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

// RateLimit ограничивает число запросов с одного ip. при недоступном редисе
// не открываемся нараспашку, а падаем на in-memory fallback (вдвое строже)
func RateLimit(limiter RateLimiter, limit int, window time.Duration, log *logger.Logger, stopCh ...chan struct{}) func(http.Handler) http.Handler {
	fallback := newFallbackLimiter(limit, window)

	// горутина периодически чистит fallback-мапу. stopCh нужен для graceful
	// shutdown - без него горутина живёт вечно
	var stop <-chan struct{}
	if len(stopCh) > 0 && stopCh[0] != nil {
		stop = stopCh[0]
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stop: // чтение из nil-канала висит вечно, так что без stopCh чистка просто крутится всегда
				return
			case <-ticker.C:
				fallback.cleanup(10 * time.Minute)
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			// локалхост не лимитируем, но только вне прода (удобно для разработки)
			if os.Getenv("ENVIRONMENT") != "production" && isLocalhost(ip) {
				next.ServeHTTP(w, r)
				return
			}

			key := fmt.Sprintf("ratelimit:%s", ip)

			allowed, err := limiter.Allow(r.Context(), key, limit, window)
			if err != nil {
				log.Warn("Rate limit check failed, falling back to in-memory limiter",
					zap.String("ip", ip),
					zap.Error(err),
				)

				// редис лежит - работаем через запасной лимитер
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

func isLocalhost(ip string) bool {
	if ip == "127.0.0.1" || ip == "localhost" {
		return true
	}
	if ip == "::1" || ip == "[::1]" {
		return true
	}
	// варианты с портом типа "127.0.0.1:xxxxx" и "[::1]:xxxxx"
	if len(ip) > 9 && ip[:9] == "127.0.0.1" {
		return true
	}
	if len(ip) > 4 && (ip[:4] == "::1:" || ip[:5] == "[::1]") {
		return true
	}
	return false
}

// getClientIP берёт ip из RemoteAddr - его уже выставил RealIP из chi.
// сырые заголовки типа X-Forwarded-For тут НЕ читаем, иначе их можно подделать
// и обойти лимит
func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
