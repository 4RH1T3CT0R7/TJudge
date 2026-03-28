//go:build contract

package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ReadBody reads and returns the full response body bytes, then closes it.
func ReadBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")
	return data
}

// AssertEnvelope checks that the response body follows the standard
// {"data": ...} envelope and returns the parsed data field as a
// map[string]interface{}.
func AssertEnvelope(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()

	var envelope map[string]interface{}
	err := json.Unmarshal(body, &envelope)
	require.NoError(t, err, "response body is not valid JSON: %s", string(body))

	_, hasData := envelope["data"]
	require.True(t, hasData, "response envelope missing 'data' field: %s", string(body))

	// The data field may be null, a primitive, an array, or an object.
	// When it is an object, return it as map[string]interface{}.
	if envelope["data"] == nil {
		return nil
	}

	dataMap, ok := envelope["data"].(map[string]interface{})
	if !ok {
		// data might be an array or scalar; return nil to signal non-object data.
		return nil
	}
	return dataMap
}

// AssertErrorResponse checks that the response body follows the standard
// {"error": "..."} error envelope and returns the error message string.
func AssertErrorResponse(t *testing.T, body []byte) string {
	t.Helper()

	var errResp map[string]interface{}
	err := json.Unmarshal(body, &errResp)
	require.NoError(t, err, "error response body is not valid JSON: %s", string(body))

	errVal, hasError := errResp["error"]
	require.True(t, hasError, "error response missing 'error' field: %s", string(body))

	errMsg, ok := errVal.(string)
	require.True(t, ok, "error field is not a string: %v", errVal)
	require.NotEmpty(t, errMsg, "error message should not be empty")

	return errMsg
}

// AssertSecurityHeaders checks that the middleware set proper security
// headers on the response.
func AssertSecurityHeaders(t *testing.T, resp *http.Response) {
	t.Helper()

	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"),
		"X-XSS-Protection header mismatch")
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"),
		"X-Content-Type-Options header mismatch")
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"),
		"X-Frame-Options header mismatch")
	assert.NotEmpty(t, resp.Header.Get("Content-Security-Policy"),
		"Content-Security-Policy header should be set")
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"),
		"Referrer-Policy header mismatch")
	assert.Equal(t, "camera=(), microphone=(), geolocation=()", resp.Header.Get("Permissions-Policy"),
		"Permissions-Policy header mismatch")
	assert.Equal(t, "noopen", resp.Header.Get("X-Download-Options"),
		"X-Download-Options header mismatch")
	assert.Equal(t, "none", resp.Header.Get("X-Permitted-Cross-Domain-Policies"),
		"X-Permitted-Cross-Domain-Policies header mismatch")
}

// AssertJSON checks that the response Content-Type is application/json.
func AssertJSON(t *testing.T, resp *http.Response) {
	t.Helper()

	ct := resp.Header.Get("Content-Type")
	assert.Contains(t, ct, "application/json",
		"Content-Type should be application/json, got %q", ct)
}
