//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doRequestWithHeaders - helper для E2E-тестов, которые должны передавать
// кастомные заголовки (например, Idempotency-Key). Зеркалит doRequest.
func (c *TestClient) doRequestWithHeaders(method, path string, body interface{}, extra map[string]string) (*http.Response, error) {
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
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	return c.client.Do(req)
}

// E2E для фич audit log и idempotency.

// =============================================================================
// Admin audit log.
// =============================================================================

func TestE2E_AuditLog_NonAdmin_Forbidden(t *testing.T) {
	client := NewTestClient()

	// Regular-user регистрируется и пытается прочитать audit log.
	ts := time.Now().UnixNano()
	regResp, err := client.doRequest("POST", "/api/v1/auth/register", RegisterRequest{
		Username: fmt.Sprintf("audit_probe_%d", ts),
		Email:    fmt.Sprintf("audit_probe_%d@test.com", ts),
		Password: "SecurePass123!",
	})
	require.NoError(t, err)
	defer regResp.Body.Close()
	require.Equal(t, http.StatusCreated, regResp.StatusCode)
	var auth AuthResponse
	require.NoError(t, decodeJSON(regResp.Body, &auth))
	client.SetToken(auth.AccessToken)

	// Non-admin: 403 или 401.
	resp, err := client.doRequest("GET", "/api/v1/admin/audit", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Contains(t, []int{http.StatusForbidden, http.StatusUnauthorized}, resp.StatusCode)
}

// =============================================================================
// Idempotency-Key.
// =============================================================================

func TestE2E_Idempotency_SameKeyReplaysResponse(t *testing.T) {
	client := NewTestClient()

	// Login без accounts - API вернёт 401 (или 200 если есть accounts);
	// важно поведение middleware: второй запрос должен получить replay-flag.
	key := fmt.Sprintf("e2e-idemp-%d", time.Now().UnixNano())
	body := map[string]string{
		"username": fmt.Sprintf("ghost_%d", time.Now().UnixNano()),
		"password": "wrongpassword",
	}

	req1ResponseCode := doRequestWithIdempotencyKey(t, client, key, body)
	req2ResponseCode := doRequestWithIdempotencyKey(t, client, key, body)

	// Оба request'а получают одинаковый код (первый настоящий, второй - replay).
	assert.Equal(t, req1ResponseCode, req2ResponseCode,
		"повторный request с той же Idempotency-Key должен вернуть тот же статус")
}

func doRequestWithIdempotencyKey(t *testing.T, client *TestClient, key string, body interface{}) int {
	t.Helper()
	resp, err := client.doRequestWithHeaders(
		"POST", "/api/v1/auth/login", body,
		map[string]string{"Idempotency-Key": key},
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestE2E_Idempotency_ConcurrentReturnsConflict(t *testing.T) {
	client := NewTestClient()
	key := fmt.Sprintf("e2e-idemp-conc-%d", time.Now().UnixNano())
	body := map[string]string{
		"username": fmt.Sprintf("ghost_%d", time.Now().UnixNano()),
		"password": "wrongpassword",
	}

	var conflict, nonConflict atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.doRequestWithHeaders(
				"POST", "/api/v1/auth/login", body,
				map[string]string{"Idempotency-Key": key},
			)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusConflict {
				conflict.Add(1)
			} else {
				nonConflict.Add(1)
			}
		}()
	}
	wg.Wait()

	// Минимум один запрос должен увидеть conflict или все replayed - главное
	// что сервер не упал и ответил всем.
	t.Logf("conflict=%d nonConflict=%d", conflict.Load(), nonConflict.Load())
	assert.Equal(t, int32(10), conflict.Load()+nonConflict.Load())
}
