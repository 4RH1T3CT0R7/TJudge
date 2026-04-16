package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSecurityConfig_Headers(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "1; mode=block", rr.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
	csp := rr.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	// P0.3 этап 1: обязательные защитные директивы CSP.
	assert.Contains(t, csp, "object-src 'none'", "object-src blocks Flash/Java XSS vectors")
	assert.Contains(t, csp, "base-uri 'self'", "base-uri защищает от base-tag injection")
	assert.Contains(t, csp, "form-action 'self'", "form-action предотвращает submit на evil origin")
	assert.Contains(t, csp, "frame-ancestors 'none'", "clickjacking protection")
	assert.Equal(t, "strict-origin-when-cross-origin", rr.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=(), microphone=(), geolocation=()", rr.Header().Get("Permissions-Policy"))
	assert.Equal(t, "noopen", rr.Header().Get("X-Download-Options"))
	assert.Equal(t, "none", rr.Header().Get("X-Permitted-Cross-Domain-Policies"))
}

func TestSecurityHeaders_CustomConfig(t *testing.T) {
	config := SecurityConfig{
		XSSProtection:         false, // disabled
		ContentTypeNosniff:    true,
		XFrameOptions:         "SAMEORIGIN",
		ContentSecurityPolicy: "",
	}

	handler := SecurityHeaders(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Empty(t, rr.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "SAMEORIGIN", rr.Header().Get("X-Frame-Options"))
	assert.Empty(t, rr.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeaders_HSTS_WithTLS(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{} // Simulate TLS
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "max-age=31536000; includeSubDomains", rr.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_HSTS_WithoutTLS(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	// No TLS
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
}

func TestSecureHeaders_UsesDefaultConfig(t *testing.T) {
	handler := SecureHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should have default headers
	assert.Equal(t, "1; mode=block", rr.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
}

func TestSecurityHeaders_NextHandlerCalled(t *testing.T) {
	called := false
	handler := SecurityHeaders(DefaultSecurityConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSecurityHeaders_EmptyPermissionsPolicy(t *testing.T) {
	config := SecurityConfig{
		PermissionsPolicy: "",
	}

	handler := SecurityHeaders(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Empty(t, rr.Header().Get("Permissions-Policy"))
}

func TestSecurityHeaders_EmptyReferrerPolicy(t *testing.T) {
	config := SecurityConfig{
		ReferrerPolicy: "",
	}

	handler := SecurityHeaders(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Empty(t, rr.Header().Get("Referrer-Policy"))
}
