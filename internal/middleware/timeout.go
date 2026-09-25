package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type TimeoutConfig struct {
	Default   time.Duration
	Database  time.Duration
	Cache     time.Duration
	Heavy     time.Duration
	WebSocket time.Duration
}

func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Default:   10 * time.Second,
		Database:  15 * time.Second,
		Cache:     5 * time.Second,
		Heavy:     30 * time.Second,
		WebSocket: 0, // 0 = без таймаута, иначе долгое ws-соединение просто оборвётся
	}
}

// SmartTimeout выбирает таймаут по типу запроса: тяжёлым выборкам достаётся больше,
// записи — меньше. тип операции угадывается по кускам пути, приём грубоватый,
// но на текущем наборе роутов работает нормально
func SmartTimeout(config TimeoutConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timeout := getTimeoutForRequest(r, config)

			// 0 — это ws, он не ограничивается
			if timeout == 0 {
				next.ServeHTTP(w, r)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func getTimeoutForRequest(r *http.Request, config TimeoutConfig) time.Duration {
	path := r.URL.Path
	method := r.Method

	if strings.Contains(path, "/ws/") {
		return config.WebSocket
	}

	// тяжёлые агрегаты и админское планирование раундов: сброс прошлого раунда
	// и вставка десятков тысяч матчей в 5 секунд записи не укладываются
	if strings.Contains(path, "/leaderboard") ||
		strings.Contains(path, "/statistics") ||
		strings.Contains(path, "/stats") ||
		strings.HasSuffix(path, "/run-matches") ||
		strings.HasSuffix(path, "/run-game-matches") ||
		strings.HasSuffix(path, "/retry-matches") ||
		strings.HasSuffix(path, "/reset-round") {
		return config.Heavy
	}

	// выборки из бд (списки, поиск)
	if (strings.Contains(path, "/tournaments") && method == "GET") ||
		(strings.Contains(path, "/matches") && method == "GET") ||
		strings.Contains(path, "/programs") {
		return config.Database
	}

	if method == "POST" || method == "PUT" || method == "DELETE" {
		return config.Cache
	}

	return config.Default
}
