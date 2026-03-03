package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newCSRFHandler(config CSRFConfig) http.Handler {
	return CSRF(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
}

func TestCSRF_SafeMethods_Pass(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	safeMethods := []string{"GET", "HEAD", "OPTIONS"}
	for _, method := range safeMethods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestCSRF_POST_WithoutCookie_Forbidden(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	req := httptest.NewRequest("POST", "/api/v1/data", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCSRF_POST_WithoutHeader_Forbidden(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	// Add cookie but no header
	token := "some-token"
	req := httptest.NewRequest("POST", "/api/v1/data", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCSRF_POST_MismatchedTokens_Forbidden(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	req := httptest.NewRequest("POST", "/api/v1/data", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: "token-a"})
	req.Header.Set(config.HeaderName, "token-b") // Different token
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCSRF_POST_MatchingTokens_OK(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	// Generate a token and store it
	token, err := generateToken(config.TokenLength)
	assert.NoError(t, err)

	// Store the token in the token store
	tokenStore.mu.Lock()
	tokenStore.tokens[token] = time.Now().Add(time.Duration(config.MaxAge) * time.Second)
	tokenStore.mu.Unlock()
	defer func() {
		tokenStore.mu.Lock()
		delete(tokenStore.tokens, token)
		tokenStore.mu.Unlock()
	}()

	req := httptest.NewRequest("POST", "/api/v1/data", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})
	req.Header.Set(config.HeaderName, token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCSRF_Disabled_PassesAll(t *testing.T) {
	config := DefaultCSRFConfig()
	config.Enabled = false
	handler := newCSRFHandler(config)

	req := httptest.NewRequest("POST", "/api/v1/data", nil)
	// No CSRF tokens at all
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCSRF_GET_SetsCookie(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should set CSRF cookie
	cookies := rr.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == config.CookieName {
			csrfCookie = c
			break
		}
	}
	assert.NotNil(t, csrfCookie)
	assert.NotEmpty(t, csrfCookie.Value)
}

func TestCSRFError_Error(t *testing.T) {
	err := &CSRFError{Message: "test error"}
	assert.Equal(t, "test error", err.Error())
}

func TestCleanupExpiredTokens(t *testing.T) {
	// Add an expired token
	tokenStore.mu.Lock()
	tokenStore.tokens["expired-token"] = time.Now().Add(-1 * time.Hour)
	tokenStore.tokens["valid-token"] = time.Now().Add(1 * time.Hour)
	tokenStore.mu.Unlock()

	CleanupExpiredTokens()

	tokenStore.mu.RLock()
	_, expiredExists := tokenStore.tokens["expired-token"]
	_, validExists := tokenStore.tokens["valid-token"]
	tokenStore.mu.RUnlock()

	assert.False(t, expiredExists, "Expired token should be cleaned up")
	assert.True(t, validExists, "Valid token should remain")

	// Cleanup
	tokenStore.mu.Lock()
	delete(tokenStore.tokens, "valid-token")
	tokenStore.mu.Unlock()
}

func TestGetCSRFToken_FromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-token"})

	token := GetCSRFToken(req)
	assert.Equal(t, "test-token", token)
}

func TestGetCSRFToken_NoCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	token := GetCSRFToken(req)
	assert.Empty(t, token)
}

func TestCSRF_POST_FormValueToken_OK(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	// Generate a token and store it
	token, err := generateToken(config.TokenLength)
	assert.NoError(t, err)

	tokenStore.mu.Lock()
	tokenStore.tokens[token] = time.Now().Add(time.Duration(config.MaxAge) * time.Second)
	tokenStore.mu.Unlock()
	defer func() {
		tokenStore.mu.Lock()
		delete(tokenStore.tokens, token)
		tokenStore.mu.Unlock()
	}()

	// POST with cookie and form value (no header) — form value is the fallback path
	req := httptest.NewRequest("POST", "/api/v1/data?csrf_token="+token, nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCSRF_POST_ExpiredToken_Forbidden(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	// Generate a token and store it with an expiry in the past
	token, err := generateToken(config.TokenLength)
	assert.NoError(t, err)

	tokenStore.mu.Lock()
	tokenStore.tokens[token] = time.Now().Add(-1 * time.Hour)
	tokenStore.mu.Unlock()
	defer func() {
		tokenStore.mu.Lock()
		delete(tokenStore.tokens, token)
		tokenStore.mu.Unlock()
	}()

	req := httptest.NewRequest("POST", "/api/v1/data", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})
	req.Header.Set(config.HeaderName, token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCSRF_POST_TokenNotInStore_Forbidden(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	// Generate a valid token but do NOT store it in tokenStore
	token, err := generateToken(config.TokenLength)
	assert.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/data", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})
	req.Header.Set(config.HeaderName, token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCSRF_GET_ExistingCookiePreserved(t *testing.T) {
	config := DefaultCSRFConfig()
	handler := newCSRFHandler(config)

	// Send a GET request with an existing CSRF cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: "my-token-value"})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// ensureCSRFToken should NOT overwrite the existing cookie.
	// Verify that no Set-Cookie header was added to the response.
	cookies := rr.Result().Cookies()
	for _, c := range cookies {
		assert.NotEqual(t, config.CookieName, c.Name,
			"Set-Cookie should not be sent when CSRF cookie already exists")
	}
}

func TestStartCSRFCleanup(t *testing.T) {
	// Add an expired token to verify cleanup works
	expiredToken, err := generateToken(CSRFTokenLength)
	assert.NoError(t, err)

	tokenStore.mu.Lock()
	tokenStore.tokens[expiredToken] = time.Now().Add(-1 * time.Hour)
	tokenStore.mu.Unlock()
	defer func() {
		tokenStore.mu.Lock()
		delete(tokenStore.tokens, expiredToken)
		tokenStore.mu.Unlock()
	}()

	// Start the cleanup goroutine — it uses a 10-minute ticker internally,
	// so we just verify it starts without panic and respects cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	StartCSRFCleanup(ctx)

	// Manually trigger cleanup (the goroutine calls CleanupExpiredTokens on tick)
	CleanupExpiredTokens()

	// Verify the expired token was cleaned up
	tokenStore.mu.RLock()
	_, exists := tokenStore.tokens[expiredToken]
	tokenStore.mu.RUnlock()
	assert.False(t, exists, "Expired token should be cleaned up")

	// Cancel context to stop the goroutine
	cancel()

	// Brief wait to let the goroutine exit cleanly
	time.Sleep(50 * time.Millisecond)
}
