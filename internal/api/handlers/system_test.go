package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSystemHandler(t *testing.T) *SystemHandler {
	t.Helper()
	log, _ := logger.New("error", "json")
	return NewSystemHandler(log)
}

func TestSystemHandler_GetMetrics_Success(t *testing.T) {
	handler := newTestSystemHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/system/metrics", nil)
	rr := httptest.NewRecorder()

	handler.GetMetrics(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var result map[string]interface{}
	decodeJSONData(t, rr.Body, &result)

	// Проверяем, что top-level ключи присутствуют
	assert.Contains(t, result, "cpu")
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "go")
	assert.Contains(t, result, "host")
	assert.Contains(t, result, "disk")
}

func TestSystemHandler_GetMetrics_GoRuntime(t *testing.T) {
	handler := newTestSystemHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/system/metrics", nil)
	rr := httptest.NewRecorder()

	handler.GetMetrics(rr, req)

	var result SystemMetrics
	decodeJSONData(t, rr.Body, &result)

	assert.NotEmpty(t, result.Go.Version)
	assert.Greater(t, result.Go.Goroutines, 0)
	assert.Greater(t, result.Go.GOMAXPROCS, 0)
	assert.Greater(t, result.CPU.Cores, 0)
}

func TestSystemHandler_GetHealth_Success(t *testing.T) {
	handler := newTestSystemHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/system/health", nil)
	rr := httptest.NewRecorder()

	handler.GetHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]interface{}
	decodeJSONData(t, rr.Body, &result)

	assert.Contains(t, result, "status")
	assert.Contains(t, result, "timestamp")
	assert.Contains(t, result, "pid")
	assert.NotEmpty(t, result["timestamp"])
}

func TestSystemHandler_GetHealth_StatusHealthy(t *testing.T) {
	handler := newTestSystemHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/system/health", nil)
	rr := httptest.NewRecorder()

	handler.GetHealth(rr, req)

	var result map[string]interface{}
	decodeJSONData(t, rr.Body, &result)

	// Статус должен быть "healthy" или "warning" в зависимости от состояния системы
	status, ok := result["status"].(string)
	require.True(t, ok)
	assert.Contains(t, []string{"healthy", "warning"}, status)
}
