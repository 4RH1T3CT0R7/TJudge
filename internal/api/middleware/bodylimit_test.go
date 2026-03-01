package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/stretchr/testify/assert"
)

func TestMaxBodySize_BelowLimit(t *testing.T) {
	body := "hello" // 5 bytes
	limit := int64(10)

	var readBody string
	var readErr error
	handler := middleware.MaxBodySize(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		readBody = string(data)
		readErr = err
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, readErr)
	assert.Equal(t, "hello", readBody)
}

func TestMaxBodySize_ExactlyAtLimit(t *testing.T) {
	body := "1234567890" // 10 bytes
	limit := int64(10)

	var readBody string
	var readErr error
	handler := middleware.MaxBodySize(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		readBody = string(data)
		readErr = err
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, readErr)
	assert.Equal(t, "1234567890", readBody)
}

func TestMaxBodySize_ExceedsLimit(t *testing.T) {
	body := "this body exceeds the limit" // 27 bytes
	limit := int64(10)

	var readErr error
	handler := middleware.MaxBodySize(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		readErr = err
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Error(t, readErr, "reading body beyond limit should return an error")
}
