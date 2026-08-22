package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ContextKey - свой тип для ключей контекста чтобы не пересекаться со строками других пакетов
type ContextKey string

const (
	// UserIDKey - под этим ключом в контексте лежит uuid юзера
	UserIDKey ContextKey = "user_id"
	// RoleKey - роль юзера (из jwt)
	RoleKey ContextKey = "user_role"

	bearerPrefix = "Bearer"
)

// AuthService - что мидлварь спрашивает у сервиса авторизации
type AuthService interface {
	ValidateToken(tokenString string) (*auth.Claims, error)
	GetUserFromToken(ctx context.Context, tokenString string) (*models.User, error)
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
}

// Auth проверяет jwt токен. без валидного токена дальше не пускаем
func Auth(authService AuthService, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string

			// обычный путь - заголовок Authorization: Bearer <token>
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == bearerPrefix {
					token = parts[1]
				}
			}

			// для вебсокета токен приезжает сабпротоколом access_token.<jwt>,
			// потому что js-клиент не умеет ставить заголовки на ws-хендшейк
			if token == "" {
				if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
					for p := range strings.SplitSeq(proto, ",") {
						p = strings.TrimSpace(p)
						if after, ok := strings.CutPrefix(p, "access_token."); ok {
							token = after
							break
						}
					}
				}
			}

			if token == "" {
				log.Info("Missing authorization token")
				httputil.WriteError(w, errors.ErrUnauthorized)
				return
			}

			claims, err := authService.ValidateToken(token)
			if err != nil {
				log.Info("Invalid token", zap.Error(err))
				httputil.WriteError(w, errors.ErrInvalidToken)
				return
			}

			// чёрный список (разлогиненные токены). если редис упал - отдаём 500,
			// НЕ пропускаем: иначе отозванный токен прошёл бы пока редис лежит
			blacklisted, err := authService.IsTokenBlacklisted(r.Context(), token)
			if err != nil {
				log.LogError("Failed to check token blacklist", err)
				httputil.WriteError(w, errors.ErrInternal)
				return
			}
			if blacklisted {
				log.Info("Token is blacklisted", zap.String("user_id", claims.UserID.String()))
				httputil.WriteError(w, errors.ErrUnauthorized.WithMessage("token has been revoked"))
				return
			}

			// айди и роль берём прямо из jwt, в базу не ходим
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth - необязательная авторизация: есть валидный токен - положим юзера
// в контекст, нет - пропустим как анонима. для публичных ручек где залогиненным
// можно показать чуть больше
func OptionalAuth(authService AuthService, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != bearerPrefix {
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]
			claims, err := authService.ValidateToken(token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// тут при ошибке блэклиста НЕ 500 как в Auth, а просто пропускаем анонимом:
			// ручка публичная, ронять её из-за редиса глупо. токену при этом не доверяем
			blacklisted, err := authService.IsTokenBlacklisted(r.Context(), token)
			if err != nil {
				log.Warn("Blacklist check failed, proceeding without authentication",
					zap.Error(err),
					zap.String("user_id", claims.UserID.String()),
				)
				next.ServeHTTP(w, r)
				return
			}
			if blacklisted {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID достаёт user id из контекста
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// RequireUserID - то же самое но с ошибкой если юзера нет
func RequireUserID(ctx context.Context) (uuid.UUID, error) {
	userID, ok := GetUserID(ctx)
	if !ok {
		return uuid.Nil, errors.ErrUnauthorized
	}
	return userID, nil
}

// ExtractToken вытаскивает голый токен из Authorization (нужен хендлеру logout)
func ExtractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != bearerPrefix {
		return ""
	}

	return parts[1]
}
