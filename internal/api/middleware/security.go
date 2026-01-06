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

// DefaultSecurityConfig возвращает конфигурацию по умолчанию
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		XSSProtection:           true,
		ContentTypeNosniff:      true,
		XFrameOptions:           "DENY",
		ContentSecurityPolicy:   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		PermissionsPolicy:       "camera=(), microphone=(), geolocation=()",
	}
}

// APISecurityConfig возвращает конфигурацию для API (менее строгая CSP)
func APISecurityConfig() SecurityConfig {
	return SecurityConfig{
		XSSProtection:           true,
		ContentTypeNosniff:      true,
		XFrameOptions:           "DENY",
		ContentSecurityPolicy:   "default-src 'none'; frame-ancestors 'none'",
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
			if config.StrictTransportSecurity != "" && r.TLS != nil {
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
	return SecurityHeaders(APISecurityConfig())
}
