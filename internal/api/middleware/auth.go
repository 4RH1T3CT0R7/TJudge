package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ContextKey тип для ключей контекста
type ContextKey string

const (
	// UserIDKey ключ для user ID в контексте
	UserIDKey ContextKey = "user_id"
	// UserKey ключ для User в контексте
	UserKey ContextKey = "user"
	// RoleKey ключ для роли в контексте
	RoleKey ContextKey = "user_role"
)

// AuthService интерфейс для работы с аутентификацией
type AuthService interface {
	ValidateToken(tokenString string) (*auth.Claims, error)
	GetUserByToken(ctx context.Context, tokenString string) (*domain.User, error)
	GetUserFromToken(ctx context.Context, tokenString string) (*domain.User, error)
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
}

// Auth middleware для проверки JWT токена
func Auth(authService AuthService, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string

			// Сначала проверяем заголовок Authorization
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				// Проверяем формат "Bearer <token>"
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					token = parts[1]
				}
			}

			// Если токена нет в header, проверяем query параметр (для WebSocket)
			if token == "" {
				token = r.URL.Query().Get("token")
			}

			if token == "" {
				log.Info("Missing authorization token")
				writeError(w, errors.ErrUnauthorized)
				return
			}

			// Валидируем токен
			claims, err := authService.ValidateToken(token)
			if err != nil {
				log.Info("Invalid token", zap.Error(err))
				writeError(w, errors.ErrInvalidToken)
				return
			}

			// Проверяем, не находится ли токен в чёрном списке
			blacklisted, err := authService.IsTokenBlacklisted(r.Context(), token)
			if err != nil {
				log.LogError("Failed to check token blacklist", err)
				writeError(w, errors.ErrInternal)
				return
			}
			if blacklisted {
				log.Info("Token is blacklisted", zap.String("user_id", claims.UserID.String()))
				writeError(w, errors.ErrUnauthorized.WithMessage("token has been revoked"))
				return
			}

			// Получаем пользователя для получения роли
			user, err := authService.GetUserFromToken(r.Context(), token)
			if err != nil {
				log.LogError("Failed to get user from token", err)
				writeError(w, errors.ErrUnauthorized.WithError(err))
				return
			}

			// Добавляем user ID и роль в контекст
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, user.Role)

			// Передаём управление следующему обработчику
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth middleware для опциональной аутентификации
// Если токен есть - валидирует и добавляет в контекст (включая роль), если нет - пропускает
func OptionalAuth(authService AuthService, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]
			claims, err := authService.ValidateToken(token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем чёрный список
			blacklisted, err := authService.IsTokenBlacklisted(r.Context(), token)
			if err != nil || blacklisted {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем пользователя для получения роли (важно для проверки админ-прав)
			user, err := authService.GetUserFromToken(r.Context(), token)
			if err != nil {
				// Если не удалось получить пользователя, всё равно пропускаем запрос
				// но без установки роли в контекст
				ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Добавляем user ID и роль в контекст
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, user.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID извлекает user ID из контекста
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// RequireUserID извлекает user ID из контекста или возвращает ошибку
func RequireUserID(ctx context.Context) (uuid.UUID, error) {
	userID, ok := GetUserID(ctx)
	if !ok {
		return uuid.Nil, errors.ErrUnauthorized
	}
	return userID, nil
}

// extractToken извлекает токен из заголовка Authorization
func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// writeError пишет ошибку в ответ
func writeError(w http.ResponseWriter, err error) {
	appErr := errors.ToAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)
	_, _ = w.Write([]byte(`{"error":"` + appErr.Message + `"}`))
}
