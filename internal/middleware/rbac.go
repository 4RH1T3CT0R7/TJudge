package middleware

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// RequireRole middleware проверяет, что у пользователя есть требуемая роль
func RequireRole(requiredRoles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем роль из контекста
			role, ok := r.Context().Value(RoleKey).(models.Role)
			if !ok {
				httputil.WriteError(w, errors.ErrUnauthorized.WithMessage("role not found in context"))
				return
			}

			// Проверяем, есть ли роль в списке разрешённых
			hasRole := slices.Contains(requiredRoles, role)

			if !hasRole {
				httputil.WriteError(w, errors.ErrForbidden.WithMessage("insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin middleware - shortcut для RequireRole(models.RoleAdmin)
func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole(models.RoleAdmin)
}

// WithRole добавляет роль в контекст запроса
func WithRole(ctx context.Context, role models.Role) context.Context {
	return context.WithValue(ctx, RoleKey, role)
}

// RequireRoleValue извлекает роль из контекста
func RequireRoleValue(ctx context.Context) (models.Role, error) {
	role, ok := ctx.Value(RoleKey).(models.Role)
	if !ok {
		return "", errors.ErrUnauthorized.WithMessage("role not found in context")
	}
	return role, nil
}

// UserRoleChecker интерфейс для проверки актуальной роли пользователя из БД
type UserRoleChecker interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// roleCacheEntry хранит кешированную роль с временем добавления
type roleCacheEntry struct {
	role      models.Role
	expiresAt time.Time
}

// VerifiedAdminChecker проверяет admin-роль из БД с кешированием.
// Защищает от использования отозванных admin-прав через не-истёкший JWT.
type VerifiedAdminChecker struct {
	userRepo UserRoleChecker
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[uuid.UUID]roleCacheEntry
}

// NewVerifiedAdminChecker создаёт checker с указанным TTL кеша
func NewVerifiedAdminChecker(userRepo UserRoleChecker, cacheTTL time.Duration) *VerifiedAdminChecker {
	return &VerifiedAdminChecker{
		userRepo: userRepo,
		cacheTTL: cacheTTL,
		cache:    make(map[uuid.UUID]roleCacheEntry),
	}
}

// RequireVerifiedAdmin middleware проверяет admin-роль из БД (не только из JWT).
// Кеширует результат на cacheTTL для уменьшения нагрузки на БД.
func (v *VerifiedAdminChecker) RequireVerifiedAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Сначала быстрая проверка из JWT
			role, ok := r.Context().Value(RoleKey).(models.Role)
			if !ok || role != models.RoleAdmin {
				httputil.WriteError(w, errors.ErrForbidden.WithMessage("insufficient permissions"))
				return
			}

			userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
			if !ok {
				httputil.WriteError(w, errors.ErrUnauthorized)
				return
			}

			// Проверяем кеш
			v.mu.RLock()
			entry, cached := v.cache[userID]
			v.mu.RUnlock()

			if cached && time.Now().Before(entry.expiresAt) {
				if entry.role != models.RoleAdmin {
					httputil.WriteError(w, errors.ErrForbidden.WithMessage("admin privileges have been revoked"))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем в БД
			user, err := v.userRepo.GetByID(r.Context(), userID)
			if err != nil {
				httputil.WriteError(w, errors.ErrForbidden.WithMessage("insufficient permissions"))
				return
			}

			// Обновляем кеш с lazy eviction
			v.mu.Lock()
			v.cache[userID] = roleCacheEntry{
				role:      user.Role,
				expiresAt: time.Now().Add(v.cacheTTL),
			}
			if len(v.cache) > 1000 {
				now := time.Now()
				for id, e := range v.cache {
					if now.After(e.expiresAt) {
						delete(v.cache, id)
					}
				}
			}
			v.mu.Unlock()

			if user.Role != models.RoleAdmin {
				httputil.WriteError(w, errors.ErrForbidden.WithMessage("admin privileges have been revoked"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
