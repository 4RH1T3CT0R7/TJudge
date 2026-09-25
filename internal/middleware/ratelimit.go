package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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

// запасной лимит строже основного (0.5 = вдвое). если бы fallback был мягче,
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

// во сколько раз лимит на чтение (GET/HEAD) выше общего: живые обновления
// турнира перечитывают лидерборды несколько раз в секунду
const readLimitMultiplier = 10

// RateLimit ограничивает число запросов одного клиента за window. клиент - юзер
// из валидного bearer-токена (tokens может быть nil), иначе ip: так аудитория за
// одним NAT не делит один счётчик. чтение считается отдельно и с лимитом в
// readLimitMultiplier раз выше, чтобы частые GET не выбивали логин и загрузки.
// name разводит счётчики разных лимитов. при недоступном редисе лимитер не
// открывается нараспашку, а падает на in-memory fallback (вдвое строже)
func RateLimit(limiter RateLimiter, name string, limit int, window time.Duration, tokens AuthService, log *logger.Logger, stopCh ...chan struct{}) func(http.Handler) http.Handler {
	writeFallback := newFallbackLimiter(limit, window)
	readFallback := newFallbackLimiter(limit*readLimitMultiplier, window)

	// горутина периодически чистит fallback-мапы. stopCh нужен для graceful
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
				writeFallback.cleanup(10 * time.Minute)
				readFallback.cleanup(10 * time.Minute)
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			// локалхост не лимитируется, но только вне прода (удобно для разработки)
			if os.Getenv("ENVIRONMENT") != "production" && isLocalhost(ip) {
				next.ServeHTTP(w, r)
				return
			}

			l, bucket, fallback := limit, name, writeFallback
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				l, bucket, fallback = limit*readLimitMultiplier, name+":read", readFallback
			}
			key := fmt.Sprintf("ratelimit:%s:%s", bucket, rateLimitSubject(r, tokens, ip))

			allowed, err := limiter.Allow(r.Context(), key, l, window)
			if err != nil {
				// редис лежит - в ход идёт запасной лимитер
				log.Warn("Rate limit check failed, falling back to in-memory limiter",
					zap.String("key", key),
					zap.Error(err),
				)
				allowed = fallback.allow(key)
			}

			if !allowed {
				log.Info("Rate limit exceeded",
					zap.String("key", key),
					zap.String("path", r.URL.Path),
				)

				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l))
				w.Header().Set("X-RateLimit-Window", window.String())
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))

				writeError(w, errors.ErrRateLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitSubject - за кем считаются запросы. блэклист токена тут не
// проверяется: отозванный токен всё равно отобьёт Auth, а тратит он только
// счётчик своего владельца
func rateLimitSubject(r *http.Request, tokens AuthService, ip string) string {
	if tokens != nil {
		if token := ExtractToken(r); token != "" {
			if claims, err := tokens.ValidateToken(token); err == nil {
				return "user:" + claims.UserID.String()
			}
		}
	}
	return "ip:" + ip
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

// RealIP подменяет RemoteAddr на адрес клиента из X-Forwarded-For / X-Real-IP,
// но только если запрос пришёл от доверенного прокси. trustedCIDRs пустой -
// доверяются loopback и приватные сети (nginx в docker-сети). True-Client-IP
// не читается вовсе: chi RealIP верил ему от кого угодно, и рейтлимит
// обходился подменой заголовка
func RealIP(trustedCIDRs []string) func(http.Handler) http.Handler {
	var nets []*net.IPNet
	for _, c := range trustedCIDRs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	trusted := func(ip net.IP) bool {
		if len(nets) == 0 {
			return ip.IsLoopback() || ip.IsPrivate()
		}
		for _, n := range nets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = realClientIP(r, trusted)
			next.ServeHTTP(w, r)
		})
	}
}

func realClientIP(r *http.Request, trusted func(net.IP) bool) string {
	peer := getClientIP(r)
	if ip := net.ParseIP(peer); ip == nil || !trusted(ip) {
		return r.RemoteAddr
	}

	// каждый прокси дописывает адрес своего соседа справа, левые записи мог
	// прислать сам клиент. поэтому берётся первый справа адрес не из доверенных
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(hops[i]))
		if ip == nil {
			break
		}
		if !trusted(ip) {
			return ip.String()
		}
	}

	// nginx пишет сюда $remote_addr, клиентское значение он перетирает
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}
	return r.RemoteAddr
}

// getClientIP берёт ip из RemoteAddr - его уже выставил RealIP выше
func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
