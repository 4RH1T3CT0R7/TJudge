//go:build contract

package contract

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContract_System_Metrics_200 verifies that an admin can access the system
// metrics endpoint. The handler reads from Go runtime and gopsutil directly,
// so no mocks are required.
func TestContract_System_Metrics_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/api/v1/system/metrics").
		WithAuth(h.AdminToken()).
		Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	data := AssertEnvelope(t, body)
	require.NotNil(t, data, "metrics data should be a JSON object")

	// Verify key metric sections are present.
	assert.Contains(t, data, "cpu", "metrics should include cpu section")
	assert.Contains(t, data, "memory", "metrics should include memory section")
	assert.Contains(t, data, "go", "metrics should include go runtime section")
	assert.Contains(t, data, "host", "metrics should include host section")
}

// TestContract_System_Metrics_403 verifies that a regular user is rejected
// from the admin-only system metrics endpoint.
func TestContract_System_Metrics_403(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/api/v1/system/metrics").
		WithAuth(h.UserToken()).
		Do()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertErrorResponse(t, body)
}

// TestContract_System_Metrics_401 verifies that an unauthenticated request to
// system metrics is rejected with 401.
func TestContract_System_Metrics_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/api/v1/system/metrics").Do()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertErrorResponse(t, body)
}

// TestContract_System_Health_200 verifies that an admin can access the system
// health endpoint under /api/v1/system/health. The handler reads runtime
// information directly without external dependencies.
func TestContract_System_Health_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/api/v1/system/health").
		WithAuth(h.AdminToken()).
		Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	data := AssertEnvelope(t, body)
	require.NotNil(t, data, "health data should be a JSON object")

	assert.Contains(t, data, "status", "health response should include status")
	assert.Contains(t, data, "timestamp", "health response should include timestamp")
	assert.Contains(t, data, "pid", "health response should include pid")
}

// TestContract_System_HealthPublic_200 verifies the public /health endpoint
// registered directly in routes.go. It returns plain text "OK" and requires
// no authentication.
func TestContract_System_HealthPublic_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/health").Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := ReadBody(t, resp)
	assert.Equal(t, "OK", string(body))
}
