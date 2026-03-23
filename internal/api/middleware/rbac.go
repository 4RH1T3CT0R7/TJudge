package middleware

import (
	"context"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// RequireRole middleware проверяет, что у пользователя есть требуемая роль
func RequireRole(requiredRoles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем роль из контекста
			role, ok := r.Context().Value(RoleKey).(domain.Role)
			if !ok {
				httputil.WriteError(w, errors.ErrUnauthorized.WithMessage("role not found in context"))
				return
			}

			// Проверяем, есть ли роль в списке разрешённых
			hasRole := false
			for _, required := range requiredRoles {
				if role == required {
					hasRole = true
					break
				}
			}

			if !hasRole {
				httputil.WriteError(w, errors.ErrForbidden.WithMessage("insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin middleware - shortcut для RequireRole(domain.RoleAdmin)
func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole(domain.RoleAdmin)
}

// WithRole добавляет роль в контекст запроса
func WithRole(ctx context.Context, role domain.Role) context.Context {
	return context.WithValue(ctx, RoleKey, role)
}

// RequireRoleValue извлекает роль из контекста
func RequireRoleValue(ctx context.Context) (domain.Role, error) {
	role, ok := ctx.Value(RoleKey).(domain.Role)
	if !ok {
		return "", errors.ErrUnauthorized.WithMessage("role not found in context")
	}
	return role, nil
}
