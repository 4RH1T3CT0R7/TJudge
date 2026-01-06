package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const (
	// CSRFTokenHeader имя заголовка для CSRF токена
	CSRFTokenHeader = "X-CSRF-Token"

	// CSRFCookieName имя cookie для CSRF токена
	CSRFCookieName = "csrf_token"

	// CSRFTokenLength длина CSRF токена в байтах
	CSRFTokenLength = 32

	// CSRFTokenTTL время жизни CSRF токена
	CSRFTokenTTL = 24 * time.Hour
)

// CSRFConfig конфигурация CSRF middleware
type CSRFConfig struct {
	// Enabled включает/выключает CSRF protection
	Enabled bool

	// TokenLength длина токена
	TokenLength int

	// CookieName имя cookie
	CookieName string

	// HeaderName имя заголовка
	HeaderName string

	// CookiePath путь для cookie
	CookiePath string

	// CookieDomain домен для cookie
	CookieDomain string

	// CookieSecure требовать HTTPS
	CookieSecure bool

	// CookieHTTPOnly запретить доступ из JavaScript
	CookieHTTPOnly bool

	// CookieSameSite политика SameSite
	CookieSameSite http.SameSite

	// MaxAge время жизни cookie в секундах
	MaxAge int

	// IgnoreMethods методы, для которых CSRF не проверяется
	IgnoreMethods []string

	// ErrorHandler обработчик ошибок
	ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)
}

// DefaultCSRFConfig возвращает конфигурацию по умолчанию
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		Enabled:        true,
		TokenLength:    CSRFTokenLength,
		CookieName:     CSRFCookieName,
		HeaderName:     CSRFTokenHeader,
		CookiePath:     "/",
		CookieSecure:   true,
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		MaxAge:         int(CSRFTokenTTL.Seconds()),
		IgnoreMethods:  []string{"GET", "HEAD", "OPTIONS", "TRACE"},
		ErrorHandler:   defaultCSRFErrorHandler,
	}
}

// csrfTokenStore хранилище CSRF токенов (in-memory, для production использовать Redis)
type csrfTokenStore struct {
	tokens map[string]time.Time
	mu     sync.RWMutex
}

var tokenStore = &csrfTokenStore{
	tokens: make(map[string]time.Time),
}

// generateToken генерирует новый CSRF токен
func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CSRF middleware для защиты от CSRF атак
func CSRF(config CSRFConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем, нужно ли проверять CSRF для этого метода
			if isIgnoredMethod(r.Method, config.IgnoreMethods) {
				// Для safe методов только устанавливаем токен
				ensureCSRFToken(w, r, config)
				next.ServeHTTP(w, r)
				return
			}

			// Для unsafe методов проверяем токен
			if err := validateCSRFToken(r, config); err != nil {
				config.ErrorHandler(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isIgnoredMethod проверяет, игнорируется ли метод
func isIgnoredMethod(method string, ignoredMethods []string) bool {
	for _, m := range ignoredMethods {
		if m == method {
			return true
		}
	}
	return false
}

// ensureCSRFToken устанавливает CSRF токен если его нет
func ensureCSRFToken(w http.ResponseWriter, r *http.Request, config CSRFConfig) {
	cookie, err := r.Cookie(config.CookieName)
	if err != nil || cookie.Value == "" {
		// Генерируем новый токен
		token, err := generateToken(config.TokenLength)
		if err != nil {
			return
		}

		// Сохраняем токен
		tokenStore.mu.Lock()
		tokenStore.tokens[token] = time.Now().Add(time.Duration(config.MaxAge) * time.Second)
		tokenStore.mu.Unlock()

		// Устанавливаем cookie
		http.SetCookie(w, &http.Cookie{
			Name:     config.CookieName,
			Value:    token,
			Path:     config.CookiePath,
			Domain:   config.CookieDomain,
			MaxAge:   config.MaxAge,
			Secure:   config.CookieSecure,
			HttpOnly: config.CookieHTTPOnly,
			SameSite: config.CookieSameSite,
		})

		// Добавляем токен в заголовок ответа для JavaScript
		w.Header().Set(config.HeaderName, token)
	}
}

// validateCSRFToken проверяет CSRF токен
func validateCSRFToken(r *http.Request, config CSRFConfig) error {
	// Получаем токен из cookie
	cookie, err := r.Cookie(config.CookieName)
	if err != nil {
		return &CSRFError{Message: "CSRF cookie not found"}
	}

	// Получаем токен из заголовка
	headerToken := r.Header.Get(config.HeaderName)
	if headerToken == "" {
		// Также проверяем в форме
		headerToken = r.FormValue("csrf_token")
	}

	if headerToken == "" {
		return &CSRFError{Message: "CSRF token not found in request"}
	}

	// Сравниваем токены (constant time comparison)
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
		return &CSRFError{Message: "CSRF token mismatch"}
	}

	// Проверяем, что токен не истёк
	tokenStore.mu.RLock()
	expiry, exists := tokenStore.tokens[cookie.Value]
	tokenStore.mu.RUnlock()

	if !exists {
		return &CSRFError{Message: "CSRF token not found in store"}
	}

	if time.Now().After(expiry) {
		// Удаляем истёкший токен
		tokenStore.mu.Lock()
		delete(tokenStore.tokens, cookie.Value)
		tokenStore.mu.Unlock()
		return &CSRFError{Message: "CSRF token expired"}
	}

	return nil
}

// CSRFError ошибка CSRF
type CSRFError struct {
	Message string
}

func (e *CSRFError) Error() string {
	return e.Message
}

// defaultCSRFErrorHandler обработчик ошибок по умолчанию
func defaultCSRFErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, "Forbidden - CSRF token invalid", http.StatusForbidden)
}

// CleanupExpiredTokens удаляет истёкшие токены (запускать периодически)
func CleanupExpiredTokens() {
	tokenStore.mu.Lock()
	defer tokenStore.mu.Unlock()

	now := time.Now()
	for token, expiry := range tokenStore.tokens {
		if now.After(expiry) {
			delete(tokenStore.tokens, token)
		}
	}
}

// GetCSRFToken возвращает текущий CSRF токен из запроса
func GetCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
