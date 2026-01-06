package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAuthService mocks the auth service
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *MockAuthService) Login(ctx context.Context, req *auth.LoginRequest) (*auth.AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *MockAuthService) RefreshTokens(ctx context.Context, refreshToken string) (*auth.AuthResponse, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *MockAuthService) Logout(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockAuthService) GetUserFromToken(ctx context.Context, token string) (*domain.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockAuthService) ValidateToken(token string) (*auth.Claims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Claims), args.Error(1)
}

func TestAuthHandler_Register(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successful registration", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "password123",
		}

		testUser := &domain.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
			Role:     domain.RoleUser,
		}

		expectedResponse := &auth.AuthResponse{
			AccessToken:  "access_token",
			RefreshToken: "refresh_token",
			User:         testUser,
		}

		mockService.On("Register", mock.Anything, &reqBody).Return(expectedResponse, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response auth.AuthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)
		assert.Equal(t, expectedResponse.RefreshToken, response.RefreshToken)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid request body", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("validation error", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.RegisterRequest{
			Username: "", // Invalid - empty username
			Email:    "test@example.com",
			Password: "password123",
		}

		mockService.On("Register", mock.Anything, &reqBody).Return(nil, errors.ErrValidation.WithMessage("username is required"))

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("user already exists", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.RegisterRequest{
			Username: "existinguser",
			Email:    "existing@example.com",
			Password: "password123",
		}

		mockService.On("Register", mock.Anything, &reqBody).Return(nil, errors.ErrConflict.WithMessage("user already exists"))

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestAuthHandler_Login(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successful login", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.LoginRequest{
			Username: "testuser",
			Password: "password123",
		}

		testUser := &domain.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
			Role:     domain.RoleUser,
		}

		expectedResponse := &auth.AuthResponse{
			AccessToken:  "access_token",
			RefreshToken: "refresh_token",
			User:         testUser,
		}

		mockService.On("Login", mock.Anything, &reqBody).Return(expectedResponse, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Login(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response auth.AuthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.LoginRequest{
			Username: "testuser",
			Password: "wrongpassword",
		}

		mockService.On("Login", mock.Anything, &reqBody).Return(nil, errors.ErrUnauthorized.WithMessage("invalid credentials"))

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Login(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.LoginRequest{
			Username: "nonexistent",
			Password: "password123",
		}

		mockService.On("Login", mock.Anything, &reqBody).Return(nil, errors.ErrNotFound.WithMessage("user not found"))

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Login(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successful token refresh", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := map[string]string{
			"refresh_token": "valid_refresh_token",
		}

		testUser := &domain.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
			Role:     domain.RoleUser,
		}

		expectedResponse := &auth.AuthResponse{
			AccessToken:  "new_access_token",
			RefreshToken: "new_refresh_token",
			User:         testUser,
		}

		mockService.On("RefreshTokens", mock.Anything, "valid_refresh_token").Return(expectedResponse, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Refresh(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response auth.AuthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := map[string]string{
			"refresh_token": "invalid_token",
		}

		mockService.On("RefreshTokens", mock.Anything, "invalid_token").Return(nil, errors.ErrInvalidToken.WithMessage("invalid refresh token"))

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Refresh(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("expired refresh token", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := map[string]string{
			"refresh_token": "expired_token",
		}

		mockService.On("RefreshTokens", mock.Anything, "expired_token").Return(nil, errors.ErrTokenExpired.WithMessage("refresh token expired"))

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Refresh(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successful logout", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		token := "valid_access_token"
		mockService.On("Logout", mock.Anything, token).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token format", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("already logged out token", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		token := "already_blacklisted_token"
		mockService.On("Logout", mock.Anything, token).Return(errors.ErrInvalidToken.WithMessage("token already invalidated"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		// Should still return success for idempotency
		assert.Equal(t, http.StatusNoContent, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestAuthHandler_Me(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get current user", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		userID := uuid.New()
		expectedUser := &domain.User{
			ID:       userID,
			Username: "testuser",
			Email:    "test@example.com",
			Role:     domain.RoleUser,
		}

		token := "valid_access_token"
		mockService.On("GetUserFromToken", mock.Anything, token).Return(expectedUser, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.Me(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.User
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, expectedUser.ID, response.ID)
		assert.Equal(t, expectedUser.Username, response.Username)
		assert.Equal(t, expectedUser.Email, response.Email)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid token", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		token := "invalid_token"
		mockService.On("GetUserFromToken", mock.Anything, token).Return(nil, errors.ErrInvalidToken.WithMessage("invalid token"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.Me(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()

		handler.Me(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
