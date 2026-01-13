package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService implements middleware.AuthService for testing
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) ValidateToken(tokenString string) (*auth.Claims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Claims), args.Error(1)
}

func (m *MockAuthService) GetUserByToken(ctx context.Context, tokenString string) (*domain.User, error) {
	args := m.Called(ctx, tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockAuthService) GetUserFromToken(ctx context.Context, tokenString string) (*domain.User, error) {
	args := m.Called(ctx, tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockAuthService) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	args := m.Called(ctx, token)
	return args.Bool(0), args.Error(1)
}

func newTestLogger() *logger.Logger {
	log, _ := logger.New("error", "json")
	return log
}

func TestAuth_ValidToken(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID}
	user := &domain.User{ID: userID, Role: domain.RoleUser}

	mockAuth.On("ValidateToken", "valid-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "valid-token").Return(false, nil)
	mockAuth.On("GetUserFromToken", mock.Anything, "valid-token").Return(user, nil)

	var capturedUserID uuid.UUID
	var capturedRole domain.Role
	handler := middleware.Auth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID, _ = middleware.GetUserID(r.Context())
		capturedRole, _ = middleware.RequireRoleValue(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, userID, capturedUserID)
	assert.Equal(t, domain.RoleUser, capturedRole)
	mockAuth.AssertExpectations(t)
}

func TestAuth_MissingToken(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	handler := middleware.Auth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuth_InvalidTokenFormat(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	handler := middleware.Auth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	// Test with missing "Bearer" prefix
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "invalid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuth_InvalidToken(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	mockAuth.On("ValidateToken", "invalid-token").Return(nil, errors.ErrInvalidToken)

	handler := middleware.Auth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	mockAuth.AssertExpectations(t)
}

func TestAuth_BlacklistedToken(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID}

	mockAuth.On("ValidateToken", "blacklisted-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "blacklisted-token").Return(true, nil)

	handler := middleware.Auth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer blacklisted-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	mockAuth.AssertExpectations(t)
}

func TestAuth_TokenFromQueryParam(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID}
	user := &domain.User{ID: userID, Role: domain.RoleUser}

	mockAuth.On("ValidateToken", "query-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "query-token").Return(false, nil)
	mockAuth.On("GetUserFromToken", mock.Anything, "query-token").Return(user, nil)

	handler := middleware.Auth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Token in query parameter (for WebSocket)
	req := httptest.NewRequest("GET", "/?token=query-token", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockAuth.AssertExpectations(t)
}

func TestAuth_AdminRole(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID}
	user := &domain.User{ID: userID, Role: domain.RoleAdmin}

	mockAuth.On("ValidateToken", "admin-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "admin-token").Return(false, nil)
	mockAuth.On("GetUserFromToken", mock.Anything, "admin-token").Return(user, nil)

	var capturedRole domain.Role
	handler := middleware.Auth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRole, _ = middleware.RequireRoleValue(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, domain.RoleAdmin, capturedRole)
	mockAuth.AssertExpectations(t)
}

func TestOptionalAuth_NoToken(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	handler := middleware.OptionalAuth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.GetUserID(r.Context())
		assert.False(t, ok, "User ID should not be in context")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOptionalAuth_ValidToken(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID}
	user := &domain.User{ID: userID, Role: domain.RoleUser}

	mockAuth.On("ValidateToken", "valid-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "valid-token").Return(false, nil)
	mockAuth.On("GetUserFromToken", mock.Anything, "valid-token").Return(user, nil)

	var capturedUserID uuid.UUID
	handler := middleware.OptionalAuth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID, _ = middleware.GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, userID, capturedUserID)
	mockAuth.AssertExpectations(t)
}

func TestOptionalAuth_InvalidToken(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	mockAuth.On("ValidateToken", "invalid-token").Return(nil, errors.ErrInvalidToken)

	handler := middleware.OptionalAuth(mockAuth, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := middleware.GetUserID(r.Context())
		assert.False(t, ok, "User ID should not be in context for invalid token")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should still pass (optional auth)
	assert.Equal(t, http.StatusOK, rr.Code)
	mockAuth.AssertExpectations(t)
}

func TestGetUserID(t *testing.T) {
	userID := uuid.New()

	// With user ID in context
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
	gotID, ok := middleware.GetUserID(ctx)
	assert.True(t, ok)
	assert.Equal(t, userID, gotID)

	// Without user ID in context
	emptyCtx := context.Background()
	_, ok = middleware.GetUserID(emptyCtx)
	assert.False(t, ok)
}

func TestRequireUserID(t *testing.T) {
	userID := uuid.New()

	// With user ID in context
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
	gotID, err := middleware.RequireUserID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, userID, gotID)

	// Without user ID in context
	emptyCtx := context.Background()
	_, err = middleware.RequireUserID(emptyCtx)
	assert.Error(t, err)
}
