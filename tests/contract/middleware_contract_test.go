//go:build contract

package contract

import (
	"compress/gzip"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/internal/domain/tournament"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestContract_Middleware_SecurityHeaders(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/health").Do()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	AssertSecurityHeaders(t, resp)
}

func TestContract_Middleware_RequestID(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/health").Do()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	reqID := resp.Header.Get("X-Request-ID")
	assert.NotEmpty(t, reqID, "X-Request-ID header should be set by RequestID middleware")
}

func TestContract_Middleware_Auth_BearerOnly(t *testing.T) {
	t.Parallel()

	t.Run("bearer_valid", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		token := h.UserToken()
		user := &domain.User{
			ID:        h.TestUserID,
			Username:  "testuser",
			Email:     "test@example.com",
			Role:      domain.RoleUser,
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		}

		h.AuthService.EXPECT().
			GetUserFromToken(mock.Anything, token).
			Return(user, nil).
			Once()

		resp := h.GET("/api/v1/auth/me").
			WithAuth(token).
			Do()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertEnvelope(t, body)
	})

	t.Run("basic_rejected", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		token := h.UserToken()

		// Send "Basic" scheme instead of "Bearer".
		resp := h.GET("/api/v1/auth/me").
			WithHeader("Authorization", "Basic "+token).
			Do()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertErrorResponse(t, body)
	})

	t.Run("missing_header", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		resp := h.GET("/api/v1/auth/me").Do()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertErrorResponse(t, body)
	})
}

func TestContract_Middleware_Auth_BlacklistedToken(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	token := h.UserToken()

	// Override the default IsTokenBlacklisted mock to return true.
	h.MiddlewareAuth.EXPECT().
		IsTokenBlacklisted(mock.Anything, mock.Anything).
		Unset()
	h.MiddlewareAuth.EXPECT().
		IsTokenBlacklisted(mock.Anything, mock.Anything).
		Return(true, nil).
		Maybe()

	resp := h.GET("/api/v1/auth/me").
		WithAuth(token).
		Do()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := ReadBody(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Middleware_RBAC_AdminEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("user_forbidden", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		// POST /api/v1/tournaments requires admin role.
		// A normal user token should result in 403.
		reqBody := map[string]string{
			"name":      "Test Tournament",
			"game_type": "prisoners_dilemma",
		}

		resp := h.POST("/api/v1/tournaments").
			WithAuth(h.UserToken()).
			WithJSON(reqBody).
			Do()

		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertErrorResponse(t, body)
	})

	t.Run("admin_allowed", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		createdTournament := &domain.Tournament{
			ID:        uuid.New(),
			Name:      "Test Tournament",
			GameType:  "prisoners_dilemma",
			Status:    domain.TournamentPending,
			CreatorID: &h.TestAdminID,
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		}

		h.TournamentService.EXPECT().
			Create(mock.Anything, mock.Anything).
			Return(createdTournament, nil).
			Once()

		reqBody := tournament.CreateRequest{
			Name:     "Test Tournament",
			GameType: "prisoners_dilemma",
		}

		resp := h.POST("/api/v1/tournaments").
			WithAuth(h.AdminToken()).
			WithJSON(reqBody).
			Do()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertEnvelope(t, body)
	})
}

func TestContract_Middleware_Compression_Gzip(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	// Mock TournamentService.List to return some data so the response is
	// large enough for the compression middleware to kick in.
	tournaments := make([]*domain.Tournament, 20)
	for i := range tournaments {
		tournaments[i] = &domain.Tournament{
			ID:          uuid.New(),
			Name:        "Tournament with a reasonably long name for compression testing purposes",
			Description: "A description that adds some bulk to the response body so gzip has material to compress effectively.",
			GameType:    "prisoners_dilemma",
			Status:      domain.TournamentPending,
			CreatedAt:   time.Now().UTC().Truncate(time.Second),
			UpdatedAt:   time.Now().UTC().Truncate(time.Second),
		}
	}

	h.TournamentService.EXPECT().
		List(mock.Anything, mock.Anything).
		Return(tournaments, nil).
		Once()

	resp := h.GET("/api/v1/tournaments").
		WithHeader("Accept-Encoding", "gzip").
		Do()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Encoding"), "gzip",
		"response should be gzip-encoded when client sends Accept-Encoding: gzip")

	// Verify the gzip body is actually valid and decodable.
	gr, err := gzip.NewReader(resp.Body)
	require.NoError(t, err, "response body should be valid gzip")
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err, "should be able to decompress gzip body")
	assert.NotEmpty(t, decompressed, "decompressed response should not be empty")
}

func TestContract_Middleware_ErrorFormat_Consistent(t *testing.T) {
	t.Parallel()

	t.Run("401_unauthorized", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		resp := h.GET("/api/v1/auth/me").Do()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		AssertJSON(t, resp)
		body := ReadBody(t, resp)
		AssertErrorResponse(t, body)
	})

	t.Run("403_forbidden", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		reqBody := map[string]string{"name": "Test"}

		resp := h.POST("/api/v1/tournaments").
			WithAuth(h.UserToken()).
			WithJSON(reqBody).
			Do()

		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		AssertJSON(t, resp)
		body := ReadBody(t, resp)
		AssertErrorResponse(t, body)
	})

	t.Run("404_not_found", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		fakeID := uuid.New()
		h.TournamentService.EXPECT().
			GetByID(mock.Anything, fakeID).
			Return(nil, errors.ErrNotFound).
			Once()

		resp := h.GET("/api/v1/tournaments/" + fakeID.String()).Do()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		AssertJSON(t, resp)
		body := ReadBody(t, resp)
		AssertErrorResponse(t, body)
	})
}

func TestContract_Middleware_Envelope_Consistent(t *testing.T) {
	t.Parallel()

	t.Run("login_response", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		user := &domain.User{
			ID:        uuid.New(),
			Username:  "envelopeuser",
			Email:     "env@example.com",
			Role:      domain.RoleUser,
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		}
		authResp := &auth.AuthResponse{
			AccessToken:  "at",
			RefreshToken: "rt",
			User:         user,
		}

		h.AuthService.EXPECT().
			Login(mock.Anything, mock.Anything).
			Return(authResp, nil).
			Once()

		resp := h.POST("/api/v1/auth/login").
			WithJSON(auth.LoginRequest{Username: "envelopeuser", Password: "pass"}).
			Do()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertEnvelope(t, body)
	})

	t.Run("tournament_list_response", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		h.TournamentService.EXPECT().
			List(mock.Anything, mock.Anything).
			Return([]*domain.Tournament{}, nil).
			Once()

		resp := h.GET("/api/v1/tournaments").Do()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertEnvelope(t, body)
	})

	t.Run("me_response", func(t *testing.T) {
		t.Parallel()
		h := NewTestHarness(t)

		token := h.UserToken()
		user := &domain.User{
			ID:        h.TestUserID,
			Username:  "meuser",
			Email:     "me@example.com",
			Role:      domain.RoleUser,
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		}

		h.AuthService.EXPECT().
			GetUserFromToken(mock.Anything, token).
			Return(user, nil).
			Once()

		resp := h.GET("/api/v1/auth/me").
			WithAuth(token).
			Do()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		body := ReadBody(t, resp)
		AssertEnvelope(t, body)
	})
}

func TestContract_Middleware_HealthCheck(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/health").Do()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := ReadBody(t, resp)
	assert.Equal(t, "OK", string(body), "health endpoint should return plain text OK")

	// Health endpoint should NOT return JSON envelope.
	ct := resp.Header.Get("Content-Type")
	assert.NotContains(t, ct, "application/json",
		"health endpoint should return plain text, not JSON")
}

func TestContract_Middleware_CORS_Headers(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	// Send a CORS preflight request.
	req, err := http.NewRequest(http.MethodOptions, h.URL+"/api/v1/auth/login", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// CORS preflight should succeed (200 or 204).
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"CORS preflight should return 200 or 204, got %d", resp.StatusCode)

	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"),
		"Access-Control-Allow-Origin header should be present")
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Methods"),
		"Access-Control-Allow-Methods header should be present")
}
