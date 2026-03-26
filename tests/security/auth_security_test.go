//go:build security
// +build security

package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test infrastructure (mirrors tests/e2e pattern)
// ---------------------------------------------------------------------------

var apiURL = getEnv("E2E_API_URL", "http://localhost:8080")

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

type testClient struct {
	client      *http.Client
	baseURL     string
	accessToken string
}

func newTestClient() *testClient {
	return &testClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: apiURL,
	}
}

func (c *testClient) setToken(token string) {
	c.accessToken = token
}

func (c *testClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	return c.client.Do(req)
}

func (c *testClient) doRawRequest(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
	return c.client.Do(req)
}

// registerUser registers a fresh user and returns the access token.
func registerUser(t *testing.T, c *testClient, suffix string) string {
	t.Helper()
	ts := time.Now().UnixNano()
	body := map[string]string{
		"username": fmt.Sprintf("sec_%s_%d", suffix, ts),
		"email":    fmt.Sprintf("sec_%s_%d@test.com", suffix, ts),
		"password": "SecurePass123!",
	}

	resp, err := c.doRequest("POST", "/api/v1/auth/register", body)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Register failed for %s: %d - %s", suffix, resp.StatusCode, string(b))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		Data        *struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &result))

	token := result.AccessToken
	if token == "" && result.Data != nil {
		token = result.Data.AccessToken
	}
	require.NotEmpty(t, token, "registration must return an access token")

	c.setToken(token)
	return token
}

// protectedEndpoints returns a list of endpoints that require authentication.
var protectedEndpoints = []struct {
	method string
	path   string
}{
	{"POST", "/api/v1/teams"},
	{"GET", "/api/v1/programs"},
	{"POST", "/api/v1/programs"},
	{"GET", "/api/v1/auth/me"},
}

// adminEndpoints returns endpoints that require admin role.
var adminEndpoints = []struct {
	method string
	path   string
}{
	{"POST", "/api/v1/tournaments"},
	{"POST", "/api/v1/games"},
}

// =============================================================================
// Test 1: No JWT -- access protected endpoints without token -> 401
// =============================================================================

func TestSecurity_NoJWT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	c := newTestClient()
	// Ensure no token is set.
	c.setToken("")

	for _, ep := range protectedEndpoints {
		t.Run(fmt.Sprintf("%s_%s", ep.method, ep.path), func(t *testing.T) {
			resp, err := c.doRequest(ep.method, ep.path, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"%s %s without JWT should return 401", ep.method, ep.path)
		})
	}
}

// =============================================================================
// Test 2: Invalid JWT -- access with garbage token -> 401
// =============================================================================

func TestSecurity_InvalidJWT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	c := newTestClient()

	garbageTokens := []struct {
		name  string
		token string
	}{
		{"random_string", "this-is-not-a-jwt-token"},
		{"almost_jwt", "eyJhbGciOiJIUzI1NiJ9.garbage.garbage"},
		{"empty", ""},
		{"spaces", "   "},
		{"sql_injection", "' OR 1=1 --"},
		{"base64_garbage", base64.StdEncoding.EncodeToString([]byte("not-a-token"))},
	}

	for _, gt := range garbageTokens {
		t.Run(gt.name, func(t *testing.T) {
			if gt.token == "" {
				// Empty token is the "no JWT" case; just verify 401.
				c.setToken("")
			} else {
				c.setToken(gt.token)
			}

			resp, err := c.doRequest("GET", "/api/v1/auth/me", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"garbage token %q should return 401", gt.name)
		})
	}
}

// =============================================================================
// Test 3: Expired JWT -- craft a token with exp in the past -> 401
// =============================================================================

func TestSecurity_ExpiredJWT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	c := newTestClient()

	// Build a JWT-shaped token with an expired "exp" claim.
	// We sign with a random key, so even the structure won't be valid
	// on the server side -- but the point is to verify the server
	// rejects tokens that look like JWTs but are expired/invalid.
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64URLEncode([]byte(fmt.Sprintf(
		`{"sub":"00000000-0000-0000-0000-000000000000","exp":%d,"role":"user"}`,
		time.Now().Add(-1*time.Hour).Unix(),
	)))
	signature := base64URLEncode(hmacSHA256([]byte("wrong-secret"), header+"."+payload))
	expiredToken := header + "." + payload + "." + signature

	c.setToken(expiredToken)

	resp, err := c.doRequest("GET", "/api/v1/auth/me", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"expired JWT should return 401")
}

// =============================================================================
// Test 4: Admin endpoint without admin role -> 403
// =============================================================================

func TestSecurity_AdminEndpointWithoutAdminRole(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	c := newTestClient()

	// Register a regular (non-admin) user.
	registerUser(t, c, "nonadmin")

	for _, ep := range adminEndpoints {
		t.Run(fmt.Sprintf("%s_%s", ep.method, ep.path), func(t *testing.T) {
			// Send a valid body so we don't get a 400 before the authz check.
			body := map[string]interface{}{
				"name":             fmt.Sprintf("sec_test_%d", time.Now().UnixNano()),
				"display_name":     "Security Test",
				"description":      "Security test entity",
				"rules":            "test rules",
				"game_type":        "prisoners_dilemma",
				"max_participants": 10,
			}

			resp, err := c.doRequest(ep.method, ep.path, body)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusForbidden, resp.StatusCode,
				"regular user on %s %s should return 403", ep.method, ep.path)
		})
	}
}

// =============================================================================
// Test 5: Max body size -- send >1MB body to JSON endpoints -> 413 or 400
// =============================================================================

func TestSecurity_MaxBodySize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	c := newTestClient()

	// Register a user so auth middleware does not reject us with 401
	// before the body-limit middleware kicks in.
	registerUser(t, c, "bodysize")

	// Build a JSON payload that exceeds 1MB (the configured limit).
	// We create a JSON object with a large string field.
	largeValue := strings.Repeat("A", 2*1024*1024) // 2MB of 'A's
	payload := fmt.Sprintf(`{"name":"test","description":"%s"}`, largeValue)
	body := bytes.NewReader([]byte(payload))

	// Target an authenticated JSON endpoint that has the 1MB body limit.
	resp, err := c.doRawRequest("POST", "/api/v1/teams", body, "application/json")
	require.NoError(t, err)
	defer resp.Body.Close()

	// The server should reject with 413 (Request Entity Too Large) or 400.
	// Go's MaxBytesReader triggers a 413 or the handler may return 400
	// when it fails to decode the truncated body.
	assert.Contains(t, []int{
		http.StatusRequestEntityTooLarge,
		http.StatusBadRequest,
	}, resp.StatusCode,
		"body exceeding 1MB limit should return 413 or 400, got %d", resp.StatusCode)
}

// =============================================================================
// Test 5b: Max body size on unauthenticated endpoint
// =============================================================================

func TestSecurity_MaxBodySizeUnauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	c := newTestClient()
	c.setToken("")

	// Build an oversized body (>1MB).
	largeValue := strings.Repeat("B", 2*1024*1024)
	payload := fmt.Sprintf(`{"username":"test","email":"test@test.com","password":"%s"}`, largeValue)
	body := bytes.NewReader([]byte(payload))

	// POST /api/v1/auth/register is public but should still enforce body limits.
	resp, err := c.doRawRequest("POST", "/api/v1/auth/register", body, "application/json")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should not succeed; expect 413 or 400.
	assert.NotEqual(t, http.StatusOK, resp.StatusCode,
		"oversized body on register should not succeed")
	assert.NotEqual(t, http.StatusCreated, resp.StatusCode,
		"oversized body on register should not succeed")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
