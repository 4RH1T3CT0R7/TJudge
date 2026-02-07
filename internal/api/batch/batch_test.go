package batch

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 10, cfg.MaxRequests)
	assert.Equal(t, int64(1<<20), cfg.MaxBodySize)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 10*time.Second, cfg.RequestTimeout)
	assert.Contains(t, cfg.AllowedMethods, "GET")
	assert.Contains(t, cfg.AllowedMethods, "POST")
	assert.Contains(t, cfg.AllowedMethods, "PUT")
	assert.Contains(t, cfg.AllowedMethods, "DELETE")
	assert.Contains(t, cfg.AllowedMethods, "PATCH")
	assert.Contains(t, cfg.AllowedPaths, "/api/v1/")
}

func newTestBatchHandler() *Handler {
	// Simple echo handler that returns the request method and path
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]string{"method": r.Method, "path": r.URL.Path}
		_ = json.NewEncoder(w).Encode(resp)
	})

	return NewHandler(inner, DefaultConfig())
}

func TestServeHTTP_NonPostMethod(t *testing.T) {
	handler := newTestBatchHandler()

	req := httptest.NewRequest("GET", "/batch", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestServeHTTP_InvalidJSON(t *testing.T) {
	handler := newTestBatchHandler()

	req := httptest.NewRequest("POST", "/batch", bytes.NewReader([]byte("not-json")))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid batch request")
}

func TestServeHTTP_EmptyBatch(t *testing.T) {
	handler := newTestBatchHandler()

	body, _ := json.Marshal(BatchRequest{Requests: []Request{}})
	req := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Empty batch request")
}

func TestServeHTTP_ExceedsMaxRequests(t *testing.T) {
	handler := newTestBatchHandler()

	requests := make([]Request, 11)
	for i := range requests {
		requests[i] = Request{ID: "r", Method: "GET", Path: "/api/v1/test"}
	}

	body, _ := json.Marshal(BatchRequest{Requests: requests})
	req := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Too many requests")
}

func TestValidateRequest_MissingID(t *testing.T) {
	handler := newTestBatchHandler()

	err := handler.validateRequest(&Request{Method: "GET", Path: "/api/v1/test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestValidateRequest_MissingMethod(t *testing.T) {
	handler := newTestBatchHandler()

	err := handler.validateRequest(&Request{ID: "1", Path: "/api/v1/test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "method is required")
}

func TestValidateRequest_MissingPath(t *testing.T) {
	handler := newTestBatchHandler()

	err := handler.validateRequest(&Request{ID: "1", Method: "GET"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path is required")
}

func TestValidateRequest_ForbiddenMethod(t *testing.T) {
	handler := newTestBatchHandler()

	err := handler.validateRequest(&Request{ID: "1", Method: "OPTIONS", Path: "/api/v1/test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestValidateRequest_ForbiddenPath(t *testing.T) {
	handler := newTestBatchHandler()

	err := handler.validateRequest(&Request{ID: "1", Method: "GET", Path: "/admin/secrets"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestValidateRequest_Valid(t *testing.T) {
	handler := newTestBatchHandler()

	err := handler.validateRequest(&Request{ID: "1", Method: "GET", Path: "/api/v1/tournaments"})
	assert.NoError(t, err)
}

func TestServeHTTP_SingleRequest(t *testing.T) {
	handler := newTestBatchHandler()

	batchReq := BatchRequest{
		Requests: []Request{
			{ID: "req-1", Method: "GET", Path: "/api/v1/tournaments"},
		},
	}

	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var batchResp BatchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &batchResp))
	assert.Len(t, batchResp.Responses, 1)
	assert.Equal(t, "req-1", batchResp.Responses[0].ID)
	assert.Equal(t, http.StatusOK, batchResp.Responses[0].StatusCode)
}

func TestServeHTTP_MultipleRequests(t *testing.T) {
	handler := newTestBatchHandler()

	batchReq := BatchRequest{
		Requests: []Request{
			{ID: "req-1", Method: "GET", Path: "/api/v1/tournaments"},
			{ID: "req-2", Method: "GET", Path: "/api/v1/games"},
			{ID: "req-3", Method: "POST", Path: "/api/v1/teams"},
		},
	}

	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var batchResp BatchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &batchResp))
	assert.Len(t, batchResp.Responses, 3)

	// All responses should be OK
	for _, resp := range batchResp.Responses {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func TestServeHTTP_HeaderPropagation(t *testing.T) {
	// Handler that checks for a specific header
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]string{"auth": auth}
		_ = json.NewEncoder(w).Encode(resp)
	})
	handler := NewHandler(inner, DefaultConfig())

	batchReq := BatchRequest{
		Requests: []Request{
			{ID: "req-1", Method: "GET", Path: "/api/v1/test"},
		},
	}

	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var batchResp BatchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &batchResp))
	assert.Len(t, batchResp.Responses, 1)

	// Verify the header was propagated
	var respBody map[string]string
	require.NoError(t, json.Unmarshal(batchResp.Responses[0].Body, &respBody))
	assert.Equal(t, "Bearer token123", respBody["auth"])
}

func TestServeHTTP_RequestWithBody(t *testing.T) {
	// Handler that reads and echoes the body
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bodyData map[string]string
		if err := json.NewDecoder(r.Body).Decode(&bodyData); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(bodyData)
	})
	handler := NewHandler(inner, DefaultConfig())

	reqBody, _ := json.Marshal(map[string]string{"name": "test-team"})
	batchReq := BatchRequest{
		Requests: []Request{
			{ID: "req-1", Method: "POST", Path: "/api/v1/teams", Body: reqBody},
		},
	}

	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var batchResp BatchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &batchResp))
	assert.Equal(t, http.StatusOK, batchResp.Responses[0].StatusCode)

	var respBody map[string]string
	require.NoError(t, json.Unmarshal(batchResp.Responses[0].Body, &respBody))
	assert.Equal(t, "test-team", respBody["name"])
}

func TestServeHTTP_ValidationErrorInBatch(t *testing.T) {
	handler := newTestBatchHandler()

	batchReq := BatchRequest{
		Requests: []Request{
			{ID: "req-1", Method: "GET", Path: "/api/v1/tournaments"},
			{ID: "", Method: "GET", Path: "/api/v1/games"}, // Missing ID
		},
	}

	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid request 1")
}
