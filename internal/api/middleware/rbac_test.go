package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequireRole_HasRole(t *testing.T) {
	testCases := []struct {
		name          string
		userRole      domain.Role
		requiredRoles []domain.Role
		shouldPass    bool
	}{
		{
			name:          "User has exact role",
			userRole:      domain.RoleAdmin,
			requiredRoles: []domain.Role{domain.RoleAdmin},
			shouldPass:    true,
		},
		{
			name:          "User has one of multiple roles",
			userRole:      domain.RoleUser,
			requiredRoles: []domain.Role{domain.RoleAdmin, domain.RoleUser},
			shouldPass:    true,
		},
		{
			name:          "User does not have required role",
			userRole:      domain.RoleUser,
			requiredRoles: []domain.Role{domain.RoleAdmin},
			shouldPass:    false,
		},
		{
			name:          "Admin accessing user route",
			userRole:      domain.RoleAdmin,
			requiredRoles: []domain.Role{domain.RoleUser, domain.RoleAdmin},
			shouldPass:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			handler := middleware.RequireRole(tc.requiredRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			ctx := middleware.WithRole(req.Context(), tc.userRole)
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if tc.shouldPass {
				assert.True(t, handlerCalled, "Handler should be called")
				assert.Equal(t, http.StatusOK, rr.Code)
			} else {
				assert.False(t, handlerCalled, "Handler should not be called")
				assert.Equal(t, http.StatusForbidden, rr.Code)
			}
		})
	}
}

func TestRequireRole_NoRoleInContext(t *testing.T) {
	handler := middleware.RequireRole(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	// No role in context
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAdmin(t *testing.T) {
	testCases := []struct {
		name       string
		userRole   domain.Role
		shouldPass bool
	}{
		{
			name:       "Admin user",
			userRole:   domain.RoleAdmin,
			shouldPass: true,
		},
		{
			name:       "Regular user",
			userRole:   domain.RoleUser,
			shouldPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			handler := middleware.RequireAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			ctx := middleware.WithRole(req.Context(), tc.userRole)
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if tc.shouldPass {
				assert.True(t, handlerCalled)
				assert.Equal(t, http.StatusOK, rr.Code)
			} else {
				assert.False(t, handlerCalled)
				assert.Equal(t, http.StatusForbidden, rr.Code)
			}
		})
	}
}

func TestWithRole(t *testing.T) {
	ctx := context.Background()
	role := domain.RoleAdmin

	newCtx := middleware.WithRole(ctx, role)

	gotRole, ok := newCtx.Value(middleware.RoleKey).(domain.Role)
	assert.True(t, ok)
	assert.Equal(t, role, gotRole)
}

func TestRequireRoleValue(t *testing.T) {
	t.Run("Role in context", func(t *testing.T) {
		ctx := middleware.WithRole(context.Background(), domain.RoleAdmin)

		role, err := middleware.RequireRoleValue(ctx)

		assert.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, role)
	})

	t.Run("No role in context", func(t *testing.T) {
		ctx := context.Background()

		_, err := middleware.RequireRoleValue(ctx)

		assert.Error(t, err)
	})
}

func TestMiddlewareChain(t *testing.T) {
	// Test that auth and rbac middleware work together
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID, Role: domain.RoleAdmin}

	mockAuth.On("ValidateToken", "admin-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "admin-token").Return(false, nil)

	// Chain: Auth -> RequireAdmin -> Handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := middleware.RequireRoleValue(r.Context())
		userID, _ := middleware.GetUserID(r.Context())
		assert.Equal(t, domain.RoleAdmin, role)
		assert.NotEqual(t, uuid.UUID{}, userID)
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Auth(mockAuth, log)(
		middleware.RequireAdmin()(finalHandler),
	)

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockAuth.AssertExpectations(t)
}

func TestMiddlewareChain_NonAdmin(t *testing.T) {
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID, Role: domain.RoleUser} // Regular user, not admin

	mockAuth.On("ValidateToken", "user-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "user-token").Return(false, nil)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for non-admin")
	})

	handler := middleware.Auth(mockAuth, log)(
		middleware.RequireAdmin()(finalHandler),
	)

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	mockAuth.AssertExpectations(t)
}
