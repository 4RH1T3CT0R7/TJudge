package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService — мок сервиса аутентификации
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

func (m *MockAuthService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	args := m.Called(ctx, accessToken, refreshToken)
	return args.Error(0)
}

func (m *MockAuthService) UpdateProfile(ctx context.Context, userID string, req *auth.UpdateProfileRequest) (*models.User, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockAuthService) GetUserFromToken(ctx context.Context, token string) (*models.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
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

	t.Run("успешная регистрация", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "password123",
		}

		expectedResponse := &auth.AuthResponse{
			AccessToken:  "access_token",
			RefreshToken: "refresh_token",
			User:         &models.User{ID: uuid.New(), Username: "testuser"},
		}

		mockService.On("Register", mock.Anything, &reqBody).Return(expectedResponse, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response auth.AuthResponse
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)
		assert.Equal(t, expectedResponse.RefreshToken, response.RefreshToken)

		mockService.AssertExpectations(t)
	})

	t.Run("битый JSON в теле", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("пользователь уже существует", func(t *testing.T) {
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

	t.Run("успешный вход", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := auth.LoginRequest{
			Username: "testuser",
			Password: "password123",
		}

		expectedResponse := &auth.AuthResponse{
			AccessToken:  "access_token",
			RefreshToken: "refresh_token",
			User:         &models.User{ID: uuid.New(), Username: "testuser"},
		}

		mockService.On("Login", mock.Anything, &reqBody).Return(expectedResponse, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Login(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response auth.AuthResponse
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)

		mockService.AssertExpectations(t)
	})

	// неверный пароль сервис отдаёт как 401
	t.Run("неверный пароль", func(t *testing.T) {
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
}

func TestAuthHandler_Refresh(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("успешное обновление токенов", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := map[string]string{"refresh_token": "valid_refresh_token"}

		expectedResponse := &auth.AuthResponse{
			AccessToken:  "new_access_token",
			RefreshToken: "new_refresh_token",
			User:         &models.User{ID: uuid.New(), Username: "testuser"},
		}

		mockService.On("RefreshTokens", mock.Anything, "valid_refresh_token").Return(expectedResponse, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Refresh(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response auth.AuthResponse
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedResponse.AccessToken, response.AccessToken)

		mockService.AssertExpectations(t)
	})

	// невалидный/протухший токен — оба ведут в 401, хватает одного кейса
	t.Run("невалидный refresh-токен", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		reqBody := map[string]string{"refresh_token": "invalid_token"}

		mockService.On("RefreshTokens", mock.Anything, "invalid_token").Return(nil, errors.ErrInvalidToken.WithMessage("invalid refresh token"))

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

	// happy-path: сервис зовётся, оба токена уходят в blacklist
	t.Run("успешный выход", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		token := "valid_access_token"
		mockService.On("Logout", mock.Anything, token, "").Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("без заголовка Authorization", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("токен уже в blacklist — logout идемпотентен", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		token := "already_blacklisted_token"
		mockService.On("Logout", mock.Anything, token, "").Return(errors.ErrUnauthorized.WithMessage("token already invalidated"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		// всё равно 204, чтобы повторный logout не падал
		assert.Equal(t, http.StatusNoContent, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestAuthHandler_Me(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("возвращает текущего пользователя", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		expectedUser := &models.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
			Role:     models.RoleUser,
		}

		token := "valid_access_token"
		mockService.On("GetUserFromToken", mock.Anything, token).Return(expectedUser, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.Me(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.User
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedUser.ID, response.ID)
		assert.Equal(t, expectedUser.Username, response.Username)
		assert.Equal(t, expectedUser.Email, response.Email)

		mockService.AssertExpectations(t)
	})
}

func TestAuthHandler_UpdateProfile(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("успешное обновление профиля", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		userID := uuid.New()

		updateReq := auth.UpdateProfileRequest{
			Email:    "newemail@example.com",
			Password: "newpassword123",
		}

		expectedUser := &models.User{
			ID:       userID,
			Username: "testuser",
			Email:    "newemail@example.com",
			Role:     models.RoleUser,
		}

		mockService.On("UpdateProfile", mock.Anything, userID.String(), &updateReq).Return(expectedUser, nil)

		body, _ := json.Marshal(updateReq)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		// userID кладёт в контекст auth-middleware
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.UpdateProfile(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.User
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedUser.ID, response.ID)
		assert.Equal(t, expectedUser.Email, response.Email)

		mockService.AssertExpectations(t)
	})

	// без userID в контексте хендлер не должен звать сервис
	t.Run("нет пользователя в контексте", func(t *testing.T) {
		mockService := new(MockAuthService)
		handler := NewAuthHandler(mockService, log)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", nil)
		w := httptest.NewRecorder()

		handler.UpdateProfile(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockService.AssertExpectations(t)
	})
}
