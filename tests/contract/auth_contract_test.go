//go:build contract

package contract

import (
	"net/http"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testUser returns a domain.User suitable for auth contract tests.
func testUser() *domain.User {
	return &domain.User{
		ID:        uuid.New(),
		Username:  "testuser",
		Email:     "test@example.com",
		Role:      domain.RoleUser,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

// testAuthResponse returns an AuthResponse with the given user.
func testAuthResponse(user *domain.User) *auth.AuthResponse {
	return &auth.AuthResponse{
		AccessToken:  "access-token-value",
		RefreshToken: "refresh-token-value",
		User:         user,
	}
}

func TestContract_Auth_Register_201(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	user := testUser()
	resp := testAuthResponse(user)

	h.AuthService.EXPECT().
		Register(mock.Anything, mock.Anything).
		Return(resp, nil).
		Once()

	reqBody := auth.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePass123!",
	}

	httpResp := h.POST("/api/v1/auth/register").
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusCreated, httpResp.StatusCode)
	AssertJSON(t, httpResp)
	AssertSecurityHeaders(t, httpResp)

	body := ReadBody(t, httpResp)
	data := AssertEnvelope(t, body)
	require.NotNil(t, data)
	assert.Equal(t, "access-token-value", data["access_token"])
	assert.Equal(t, "refresh-token-value", data["refresh_token"])

	userData, ok := data["user"].(map[string]interface{})
	require.True(t, ok, "user field should be an object")
	assert.Equal(t, user.Username, userData["username"])
	assert.Equal(t, user.Email, userData["email"])
}

func TestContract_Auth_Register_400_InvalidBody(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	httpResp := h.POST("/api/v1/auth/register").
		WithBody([]byte(`{invalid json`)).
		WithHeader("Content-Type", "application/json").
		Do()

	require.Equal(t, http.StatusBadRequest, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	AssertErrorResponse(t, body)
}

func TestContract_Auth_Register_409_AlreadyExists(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	h.AuthService.EXPECT().
		Register(mock.Anything, mock.Anything).
		Return(nil, errors.ErrAlreadyExists.WithMessage("username already taken")).
		Once()

	reqBody := auth.RegisterRequest{
		Username: "existinguser",
		Email:    "existing@example.com",
		Password: "SecurePass123!",
	}

	httpResp := h.POST("/api/v1/auth/register").
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusConflict, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	errMsg := AssertErrorResponse(t, body)
	assert.Contains(t, errMsg, "username already taken")
}

func TestContract_Auth_Login_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	user := testUser()
	resp := testAuthResponse(user)

	h.AuthService.EXPECT().
		Login(mock.Anything, mock.Anything).
		Return(resp, nil).
		Once()

	reqBody := auth.LoginRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	}

	httpResp := h.POST("/api/v1/auth/login").
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	AssertJSON(t, httpResp)
	AssertSecurityHeaders(t, httpResp)

	body := ReadBody(t, httpResp)
	data := AssertEnvelope(t, body)
	require.NotNil(t, data)
	assert.Equal(t, "access-token-value", data["access_token"])
	assert.Equal(t, "refresh-token-value", data["refresh_token"])
	assert.NotNil(t, data["user"])
}

func TestContract_Auth_Login_401_InvalidCredentials(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	h.AuthService.EXPECT().
		Login(mock.Anything, mock.Anything).
		Return(nil, errors.ErrInvalidCredentials).
		Once()

	reqBody := auth.LoginRequest{
		Username: "wronguser",
		Password: "wrongpass",
	}

	httpResp := h.POST("/api/v1/auth/login").
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusUnauthorized, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	AssertErrorResponse(t, body)
}

func TestContract_Auth_Refresh_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	user := testUser()
	resp := testAuthResponse(user)

	h.AuthService.EXPECT().
		RefreshTokens(mock.Anything, "old-refresh-token").
		Return(resp, nil).
		Once()

	reqBody := map[string]string{
		"refresh_token": "old-refresh-token",
	}

	httpResp := h.POST("/api/v1/auth/refresh").
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	data := AssertEnvelope(t, body)
	require.NotNil(t, data)
	assert.Equal(t, "access-token-value", data["access_token"])
	assert.Equal(t, "refresh-token-value", data["refresh_token"])
}

func TestContract_Auth_Logout_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	token := h.UserToken()

	// The handler extracts the access token from Authorization header
	// and the refresh token from the body, then calls Logout(ctx, access, refresh).
	h.AuthService.EXPECT().
		Logout(mock.Anything, token, "the-refresh-token").
		Return(nil).
		Once()

	reqBody := map[string]string{
		"refresh_token": "the-refresh-token",
	}

	httpResp := h.POST("/api/v1/auth/logout").
		WithAuth(token).
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusNoContent, httpResp.StatusCode)
}

func TestContract_Auth_Logout_401_NoAuth(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	// No auth token provided - the auth middleware should reject the request.
	httpResp := h.POST("/api/v1/auth/logout").
		WithJSON(map[string]string{"refresh_token": "some-token"}).
		Do()

	require.Equal(t, http.StatusUnauthorized, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	AssertErrorResponse(t, body)
}

func TestContract_Auth_Me_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	token := h.UserToken()
	user := testUser()

	// The Me handler calls authService.GetUserFromToken(ctx, token)
	// where token is extracted from the Authorization header.
	h.AuthService.EXPECT().
		GetUserFromToken(mock.Anything, token).
		Return(user, nil).
		Once()

	httpResp := h.GET("/api/v1/auth/me").
		WithAuth(token).
		Do()

	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	AssertJSON(t, httpResp)
	AssertSecurityHeaders(t, httpResp)

	body := ReadBody(t, httpResp)
	data := AssertEnvelope(t, body)
	require.NotNil(t, data)
	assert.Equal(t, user.Username, data["username"])
	assert.Equal(t, user.Email, data["email"])

	// PasswordHash must not be exposed (json:"-").
	_, hasHash := data["password_hash"]
	assert.False(t, hasHash, "password_hash must not be present in response")
}

func TestContract_Auth_Me_401_NoAuth(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	httpResp := h.GET("/api/v1/auth/me").Do()

	require.Equal(t, http.StatusUnauthorized, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	AssertErrorResponse(t, body)
}

func TestContract_Auth_UpdateProfile_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	token := h.UserToken()

	updatedUser := &domain.User{
		ID:        h.TestUserID,
		Username:  "testuser",
		Email:     "newemail@example.com",
		Role:      domain.RoleUser,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}

	// The handler calls authService.UpdateProfile(ctx, userID.String(), req).
	// userID comes from context (set by auth middleware from JWT claims).
	h.AuthService.EXPECT().
		UpdateProfile(mock.Anything, h.TestUserID.String(), mock.Anything).
		Return(updatedUser, nil).
		Once()

	reqBody := auth.UpdateProfileRequest{
		Email:           "newemail@example.com",
		CurrentPassword: "OldPass123!",
	}

	httpResp := h.PUT("/api/v1/auth/profile").
		WithAuth(token).
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	data := AssertEnvelope(t, body)
	require.NotNil(t, data)
	assert.Equal(t, "newemail@example.com", data["email"])
}

func TestContract_Auth_UpdateProfile_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	reqBody := auth.UpdateProfileRequest{
		Email: "new@example.com",
	}

	httpResp := h.PUT("/api/v1/auth/profile").
		WithJSON(reqBody).
		Do()

	require.Equal(t, http.StatusUnauthorized, httpResp.StatusCode)
	AssertJSON(t, httpResp)

	body := ReadBody(t, httpResp)
	AssertErrorResponse(t, body)
}
