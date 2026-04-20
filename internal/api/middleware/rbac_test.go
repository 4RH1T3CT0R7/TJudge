package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserRoleChecker - мок для тестирования VerifiedAdminChecker
type MockUserRoleChecker struct {
	mock.Mock
}

func (m *MockUserRoleChecker) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

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
	// Без роли в контексте
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
	// Проверяем, что auth и rbac middleware работают вместе
	mockAuth := new(MockAuthService)
	log := newTestLogger()

	userID := uuid.New()
	claims := &auth.Claims{UserID: userID, Role: domain.RoleAdmin}

	mockAuth.On("ValidateToken", "admin-token").Return(claims, nil)
	mockAuth.On("IsTokenBlacklisted", mock.Anything, "admin-token").Return(false, nil)

	// Цепочка: Auth -> RequireAdmin -> Handler
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

// --- VerifiedAdminChecker tests ---

// helper: build request context with role and userID
func verifiedAdminCtx(role domain.Role, userID uuid.UUID, setRole, setUserID bool) context.Context {
	ctx := context.Background()
	if setRole {
		ctx = context.WithValue(ctx, middleware.RoleKey, role)
	}
	if setUserID {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	return ctx
}

func TestVerifiedAdminChecker_AdminVerifiedInDB(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	userID := uuid.New()
	mockRepo.On("GetByID", mock.Anything, userID).Return(&domain.User{
		ID:   userID,
		Role: domain.RoleAdmin,
	}, nil)

	handlerCalled := false
	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	req = req.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.True(t, handlerCalled, "Handler should be called for verified admin")
	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertExpectations(t)
}

func TestVerifiedAdminChecker_AdminRevokedInDB(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	userID := uuid.New()
	// JWT говорит admin, но БД говорит user (admin отозван)
	mockRepo.On("GetByID", mock.Anything, userID).Return(&domain.User{
		ID:   userID,
		Role: domain.RoleUser,
	}, nil)

	handlerCalled := false
	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	req = req.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.False(t, handlerCalled, "Handler should not be called when admin is revoked")
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "admin privileges have been revoked")
	mockRepo.AssertExpectations(t)
}

func TestVerifiedAdminChecker_NonAdminJWT(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	userID := uuid.New()

	handlerCalled := false
	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	// JWT role - user, не admin
	req = req.WithContext(verifiedAdminCtx(domain.RoleUser, userID, true, true))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.False(t, handlerCalled, "Handler should not be called for non-admin JWT")
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "insufficient permissions")
	// БД НЕ должна вызываться - отклонено на уровне JWT
	mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestVerifiedAdminChecker_DBError(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	userID := uuid.New()
	// БД возвращает ошибку - fail-closed поведение
	mockRepo.On("GetByID", mock.Anything, userID).Return(nil, fmt.Errorf("connection refused"))

	handlerCalled := false
	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	req = req.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.False(t, handlerCalled, "Handler should not be called on DB error (fail-closed)")
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "insufficient permissions")
	mockRepo.AssertExpectations(t)
}

func TestVerifiedAdminChecker_CacheHitFresh(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	userID := uuid.New()
	mockRepo.On("GetByID", mock.Anything, userID).Return(&domain.User{
		ID:   userID,
		Role: domain.RoleAdmin,
	}, nil).Once() // Ожидаем ровно один вызов БД

	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Первый запрос - идёт в БД, наполняет кэш
	req1 := httptest.NewRequest("GET", "/admin", nil)
	req1 = req1.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Второй запрос - должен использовать кэш, без вызова БД
	req2 := httptest.NewRequest("GET", "/admin", nil)
	req2 = req2.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	// Проверяем, что БД была вызвана только один раз
	mockRepo.AssertNumberOfCalls(t, "GetByID", 1)
}

func TestVerifiedAdminChecker_CacheExpiry(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	cacheTTL := 100 * time.Millisecond
	checker := middleware.NewVerifiedAdminChecker(mockRepo, cacheTTL)

	userID := uuid.New()
	mockRepo.On("GetByID", mock.Anything, userID).Return(&domain.User{
		ID:   userID,
		Role: domain.RoleAdmin,
	}, nil)

	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Первый запрос - идёт в БД
	req1 := httptest.NewRequest("GET", "/admin", nil)
	req1 = req1.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)
	mockRepo.AssertNumberOfCalls(t, "GetByID", 1)

	// Ждём истечения TTL кэша
	time.Sleep(150 * time.Millisecond)

	// Второй запрос - кэш истёк, снова идёт в БД
	req2 := httptest.NewRequest("GET", "/admin", nil)
	req2 = req2.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	// Проверяем, что БД была вызвана дважды (один раз fresh, один раз после expiry)
	mockRepo.AssertNumberOfCalls(t, "GetByID", 2)
}

func TestVerifiedAdminChecker_NoRoleInContext(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	handlerCalled := false
	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	// Роли в контексте совсем нет
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.False(t, handlerCalled, "Handler should not be called without role in context")
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "insufficient permissions")
	mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestVerifiedAdminChecker_NoUserIDInContext(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	handlerCalled := false
	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	// Роль admin, но нет UserIDKey
	req = req.WithContext(verifiedAdminCtx(domain.RoleAdmin, uuid.UUID{}, true, false))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.False(t, handlerCalled, "Handler should not be called without user ID in context")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestVerifiedAdminChecker_CacheHitRevokedAdmin(t *testing.T) {
	mockRepo := new(MockUserRoleChecker)
	checker := middleware.NewVerifiedAdminChecker(mockRepo, 5*time.Minute)

	userID := uuid.New()
	// БД возвращает роль user (admin отозван)
	mockRepo.On("GetByID", mock.Anything, userID).Return(&domain.User{
		ID:   userID,
		Role: domain.RoleUser,
	}, nil).Once()

	handler := checker.RequireVerifiedAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Первый запрос - идёт в БД, кэширует роль как "user"
	req1 := httptest.NewRequest("GET", "/admin", nil)
	req1 = req1.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusForbidden, rr1.Code)
	assert.Contains(t, rr1.Body.String(), "admin privileges have been revoked")

	// Второй запрос - использует закэшированную "user" роль, всё ещё forbidden
	req2 := httptest.NewRequest("GET", "/admin", nil)
	req2 = req2.WithContext(verifiedAdminCtx(domain.RoleAdmin, userID, true, true))
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusForbidden, rr2.Code)
	assert.Contains(t, rr2.Body.String(), "admin privileges have been revoked")

	// БД вызывается только один раз - второй запрос обслуживается из кэша
	mockRepo.AssertNumberOfCalls(t, "GetByID", 1)
}
