package middleware

import (
	"net/http"
)

// SecurityConfig конфигурация security middleware
type SecurityConfig struct {
	// XSSProtection включает X-XSS-Protection
	XSSProtection bool

	// ContentTypeNosniff включает X-Content-Type-Options: nosniff
	ContentTypeNosniff bool

	// XFrameOptions значение заголовка X-Frame-Options
	// Возможные значения: DENY, SAMEORIGIN, ALLOW-FROM uri
	XFrameOptions string

	// ContentSecurityPolicy значение заголовка CSP
	ContentSecurityPolicy string

	// ReferrerPolicy значение заголовка Referrer-Policy
	ReferrerPolicy string

	// StrictTransportSecurity значение заголовка HSTS
	StrictTransportSecurity string

	// PermissionsPolicy значение заголовка Permissions-Policy
	PermissionsPolicy string
}

// DefaultSecurityConfig возвращает конфигурацию по умолчанию.
//
// CSP ужесточён (P0.3 этап 1):
//   - object-src 'none'              — блокирует Flash/Java-аплеты (XSS vector)
//   - base-uri 'self'                — защита от base-tag injection
//   - form-action 'self'             — отправка форм только на свой origin
//   - frame-ancestors 'none'         — clickjacking защита
//
// 'unsafe-inline' временно остаётся в script-src до выноса inline-скрипта
// из web/index.html (P0.3 этап 2 — перейти на nonce или hash-based CSP).
// 'unsafe-inline' в style-src нужен для Tailwind style-injection; риск ниже.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		XSSProtection:           true,
		ContentTypeNosniff:      true,
		XFrameOptions:           "DENY",
		ContentSecurityPolicy:   "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: blob:; font-src 'self' https://fonts.gstatic.com; connect-src 'self' ws: wss:; frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self'",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		PermissionsPolicy:       "camera=(), microphone=(), geolocation=()",
	}
}

// SecurityHeaders добавляет security headers к ответам
func SecurityHeaders(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// X-XSS-Protection
			if config.XSSProtection {
				w.Header().Set("X-XSS-Protection", "1; mode=block")
			}

			// X-Content-Type-Options
			if config.ContentTypeNosniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// X-Frame-Options
			if config.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", config.XFrameOptions)
			}

			// Content-Security-Policy
			if config.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			}

			// Referrer-Policy
			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}

			// Strict-Transport-Security (только для HTTPS)
			// Also set HSTS when behind a reverse proxy that terminates TLS and
			// forwards the protocol via X-Forwarded-Proto.
			if config.StrictTransportSecurity != "" && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
				w.Header().Set("Strict-Transport-Security", config.StrictTransportSecurity)
			}

			// Permissions-Policy
			if config.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}

			// Дополнительные заголовки безопасности
			w.Header().Set("X-Download-Options", "noopen")
			w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")

			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders применяет security headers с конфигурацией по умолчанию
func SecureHeaders() func(http.Handler) http.Handler {
	return SecurityHeaders(DefaultSecurityConfig())
}
