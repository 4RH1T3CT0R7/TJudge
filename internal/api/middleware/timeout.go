package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// OperationType определяет тип операции для настройки таймаута
type OperationType string

const (
	OperationDefault   OperationType = "default"
	OperationDatabase  OperationType = "database"
	OperationCache     OperationType = "cache"
	OperationHeavy     OperationType = "heavy" // Тяжёлые операции (leaderboard, stats)
	OperationWebSocket OperationType = "websocket"
)

// TimeoutConfig конфигурация таймаутов для разных типов операций
type TimeoutConfig struct {
	Default   time.Duration
	Database  time.Duration
	Cache     time.Duration
	Heavy     time.Duration
	WebSocket time.Duration
}

// DefaultTimeoutConfig возвращает конфигурацию таймаутов по умолчанию
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Default:   10 * time.Second,
		Database:  15 * time.Second,
		Cache:     5 * time.Second,
		Heavy:     30 * time.Second,
		WebSocket: 0, // Без таймаута
	}
}

// SmartTimeout создаёт middleware с умными таймаутами
// Определяет тип операции по URL и применяет соответствующий таймаут
func SmartTimeout(config TimeoutConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timeout := getTimeoutForRequest(r, config)

			// Для WebSocket соединений не устанавливаем таймаут
			if timeout == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Создаём контекст с таймаутом
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Продолжаем обработку с новым контекстом
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// getTimeoutForRequest определяет таймаут на основе запроса
func getTimeoutForRequest(r *http.Request, config TimeoutConfig) time.Duration {
	path := r.URL.Path
	method := r.Method

	// WebSocket соединения
	if strings.Contains(path, "/ws/") {
		return config.WebSocket
	}

	// Тяжёлые операции
	if strings.Contains(path, "/leaderboard") ||
		strings.Contains(path, "/statistics") ||
		strings.Contains(path, "/stats") {
		return config.Heavy
	}

	// База данных (списки, поиск)
	if (strings.Contains(path, "/tournaments") && method == "GET") ||
		(strings.Contains(path, "/matches") && method == "GET") ||
		strings.Contains(path, "/programs") {
		return config.Database
	}

	// Операции записи (быстрые)
	if method == "POST" || method == "PUT" || method == "DELETE" {
		return config.Cache
	}

	// По умолчанию
	return config.Default
}

// WithOperationTimeout создаёт контекст с таймаутом для конкретного типа операции
// Используется в сервисах для ручного управления таймаутами
func WithOperationTimeout(ctx context.Context, op OperationType, config TimeoutConfig) (context.Context, context.CancelFunc) {
	var timeout time.Duration

	switch op {
	case OperationDatabase:
		timeout = config.Database
	case OperationCache:
		timeout = config.Cache
	case OperationHeavy:
		timeout = config.Heavy
	case OperationWebSocket:
		return ctx, func() {} // Без таймаута
	default:
		timeout = config.Default
	}

	return context.WithTimeout(ctx, timeout)
}
