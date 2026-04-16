package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheControl_SetsHeadersOnGET(t *testing.T) {
	handler := CacheControl(60)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/games", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=60")
	assert.Contains(t, rec.Header().Get("Cache-Control"), "public")
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	body, _ := io.ReadAll(rec.Body)
	assert.Equal(t, `{"ok":true}`, string(body))
}

func TestCacheControl_NoStoreWhenZero(t *testing.T) {
	handler := CacheControl(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestCacheControl_ReturnsNotModifiedOnETagMatch(t *testing.T) {
	handler := CacheControl(60)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`stable-content`))
	}))

	// Первый запрос — получаем ETag.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	etag := rec.Header().Get("ETag")
	assert.NotEmpty(t, etag)

	// Повторный запрос с If-None-Match → 304.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusNotModified, rec2.Code)
	body, _ := io.ReadAll(rec2.Body)
	assert.Empty(t, string(body), "304 должен быть без body")
}

func TestCacheControl_DifferentBodyDifferentETag(t *testing.T) {
	counter := 0
	handler := CacheControl(60)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter++
		_, _ = w.Write([]byte(strings.Repeat("x", counter)))
	}))

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.NotEqual(t, rec1.Header().Get("ETag"), rec2.Header().Get("ETag"),
		"разные body должны давать разные ETag")
}

func TestCacheControl_IgnoresPOST(t *testing.T) {
	handler := CacheControl(60)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Empty(t, rec.Header().Get("Cache-Control"), "POST не должен кэшироваться middleware'ом")
	assert.Empty(t, rec.Header().Get("ETag"))
}

func TestCacheControl_SkipsErrorResponses(t *testing.T) {
	handler := CacheControl(60)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`error`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	// 5xx не должен получать ETag (чтобы клиент не кешировал ошибку).
	assert.Empty(t, rec.Header().Get("ETag"))
}
