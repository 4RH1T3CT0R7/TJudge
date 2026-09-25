package middleware

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// RequireRole пускает дальше только если роль юзера из списка разрешённых
func RequireRole(requiredRoles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// роль положил в контекст Auth, если её нет - значит Auth не отработал
			role, ok := r.Context().Value(RoleKey).(models.Role)
			if !ok {
				writeError(w, errors.ErrUnauthorized.WithMessage("role not found in context"))
				return
			}

			hasRole := slices.Contains(requiredRoles, role)

			if !hasRole {
				writeError(w, errors.ErrForbidden.WithMessage("insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin - сокращение для RequireRole(admin)
func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole(models.RoleAdmin)
}

// WithRole кладёт роль в контекст (в основном для тестов)
func WithRole(ctx context.Context, role models.Role) context.Context {
	return context.WithValue(ctx, RoleKey, role)
}

// RequireRoleValue достаёт роль из контекста
func RequireRoleValue(ctx context.Context) (models.Role, error) {
	role, ok := ctx.Value(RoleKey).(models.Role)
	if !ok {
		return "", errors.ErrUnauthorized.WithMessage("role not found in context")
	}
	return role, nil
}

// UserRoleChecker - актуальная роль юзера из базы
type UserRoleChecker interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type roleCacheEntry struct {
	role      models.Role
	expiresAt time.Time
}

// VerifiedAdminChecker сверяет админскую роль с базой а не только с jwt.
// смысл: jwt живёт сутки, и если у админа отобрали права, по одному jwt он
// оставался бы админом до истечения токена. тут перепроверка базы с кэшом
type VerifiedAdminChecker struct {
	userRepo UserRoleChecker
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[uuid.UUID]roleCacheEntry
}

// NewVerifiedAdminChecker создаёт чекер, ttl кэша задаётся снаружи (в main 5 минут)
func NewVerifiedAdminChecker(userRepo UserRoleChecker, cacheTTL time.Duration) *VerifiedAdminChecker {
	return &VerifiedAdminChecker{
		userRepo: userRepo,
		cacheTTL: cacheTTL,
		cache:    make(map[uuid.UUID]roleCacheEntry),
	}
}

// RequireVerifiedAdmin - как RequireAdmin, но с перепроверкой роли по базе
func (v *VerifiedAdminChecker) RequireVerifiedAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// сначала дешёвая проверка по jwt - если в токене не админ,
			// до базы дело не доходит
			role, ok := r.Context().Value(RoleKey).(models.Role)
			if !ok || role != models.RoleAdmin {
				writeError(w, errors.ErrForbidden.WithMessage("insufficient permissions"))
				return
			}

			userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
			if !ok {
				writeError(w, errors.ErrUnauthorized)
				return
			}

			admin, err := v.isAdmin(r.Context(), userID)
			if err != nil {
				writeError(w, errors.ErrForbidden.WithMessage("insufficient permissions"))
				return
			}
			if !admin {
				writeError(w, errors.ErrForbidden.WithMessage("admin privileges have been revoked"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// VerifyRole ставится сразу после Auth/OptionalAuth: роль admin из jwt
// сверяется с базой, и если права отозваны (или база не ответила), в контекст
// кладётся RoleUser. так проверки админа внутри хендлеров по RoleKey тоже
// не верят одному jwt
func (v *VerifiedAdminChecker) VerifyRole() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(RoleKey).(models.Role)
			userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
			if role == models.RoleAdmin && ok {
				if admin, err := v.isAdmin(r.Context(), userID); err != nil || !admin {
					r = r.WithContext(context.WithValue(r.Context(), RoleKey, models.RoleUser))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isAdmin - актуальная роль из базы, с кэшем на cacheTTL
func (v *VerifiedAdminChecker) isAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	v.mu.RLock()
	entry, cached := v.cache[userID]
	v.mu.RUnlock()

	if cached && time.Now().Before(entry.expiresAt) {
		return entry.role == models.RoleAdmin, nil
	}

	// в кэше нет или протухло - запрос в базу
	user, err := v.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// запись в кэш. заодно ленивая чистка: когда записей за тысячу,
	// протухшие выкидываются (отдельную горутину заводить лень, да и незачем)
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

	return user.Role == models.RoleAdmin, nil
}
