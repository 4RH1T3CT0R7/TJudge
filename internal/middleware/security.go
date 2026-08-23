package middleware

import (
	"net/http"
)

// SecurityConfig - настройки security-заголовков
type SecurityConfig struct {
	// включает X-XSS-Protection, хотя новые браузеры на него давно забили
	// и просто игнорят - оставил ради старых
	XSSProtection bool

	// X-Content-Type-Options: nosniff
	ContentTypeNosniff bool

	// значение X-Frame-Options: DENY, SAMEORIGIN, ALLOW-FROM uri
	XFrameOptions string

	// значение CSP
	ContentSecurityPolicy string

	ReferrerPolicy string

	// значение HSTS
	StrictTransportSecurity string

	PermissionsPolicy string
}

// DefaultSecurityConfig - конфиг по умолчанию
//
// в CSP прикрыто лишнее: object-src 'none' (Flash/апплеты), base-uri 'self'
// (base-tag injection), form-action 'self', frame-ancestors 'none' (кликджекинг)
// 'unsafe-inline' в script-src пока держим из-за inline-скрипта в index.html,
// потом надо уйти на nonce; в style-src он нужен Tailwind, риск меньше
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		XSSProtection:           true,
		ContentTypeNosniff:      true,
		XFrameOptions:           "DENY",
		ContentSecurityPolicy:   "default-src 'self'; script-src 'self' 'unsafe-inline' https://static.cloudflareinsights.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: blob:; font-src 'self' https://fonts.gstatic.com; connect-src 'self' ws: wss: https://cloudflareinsights.com; frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self'",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		PermissionsPolicy:       "camera=(), microphone=(), geolocation=()",
	}
}

// SecurityHeaders добавляет security-заголовки в ответы
func SecurityHeaders(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.XSSProtection {
				w.Header().Set("X-XSS-Protection", "1; mode=block")
			}

			if config.ContentTypeNosniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			if config.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", config.XFrameOptions)
			}

			if config.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			}

			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}

			// HSTS ставим только когда соединение реально по TLS: напрямую это
			// r.TLS, а за реверс-прокси TLS рвётся на нём, поэтому смотрим X-Forwarded-Proto
			if config.StrictTransportSecurity != "" && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
				w.Header().Set("Strict-Transport-Security", config.StrictTransportSecurity)
			}

			if config.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}

			w.Header().Set("X-Download-Options", "noopen")
			w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")

			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders - security-заголовки с дефолтным конфигом
func SecureHeaders() func(http.Handler) http.Handler {
	return SecurityHeaders(DefaultSecurityConfig())
}
