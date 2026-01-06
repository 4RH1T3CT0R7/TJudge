package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// AuthService интерфейс для auth service
type AuthService interface {
	Register(ctx context.Context, req *auth.RegisterRequest) (*auth.AuthResponse, error)
	Login(ctx context.Context, req *auth.LoginRequest) (*auth.AuthResponse, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*auth.AuthResponse, error)
	Logout(ctx context.Context, token string) error
	GetUserFromToken(ctx context.Context, token string) (*domain.User, error)
	ValidateToken(token string) (*auth.Claims, error)
}

// AuthHandler обрабатывает запросы аутентификации
type AuthHandler struct {
	authService AuthService
	log         *logger.Logger
}

// NewAuthHandler создаёт новый auth handler
func NewAuthHandler(authService AuthService, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         log,
	}
}

// Register обрабатывает регистрацию пользователя
// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req auth.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Регистрируем пользователя
	resp, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		h.log.LogError("Failed to register user", err)
		writeError(w, err)
		return
	}

	h.log.Info("User registered",
		zap.String("user_id", resp.User.ID.String()),
		zap.String("username", resp.User.Username),
	)

	writeJSON(w, http.StatusCreated, resp)
}

// Login обрабатывает вход пользователя
// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Выполняем вход
	resp, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		h.log.LogError("Failed to login", err,
			zap.String("username", req.Username),
		)
		writeError(w, err)
		return
	}

	h.log.Info("User logged in",
		zap.String("user_id", resp.User.ID.String()),
		zap.String("username", resp.User.Username),
	)

	writeJSON(w, http.StatusOK, resp)
}

// Refresh обрабатывает обновление токена
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Обновляем токены
	resp, err := h.authService.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		h.log.LogError("Failed to refresh tokens", err)
		writeError(w, err)
		return
	}

	h.log.Info("Tokens refreshed",
		zap.String("user_id", resp.User.ID.String()),
	)

	writeJSON(w, http.StatusOK, resp)
}

// Logout обрабатывает выход пользователя
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Извлекаем токен из заголовка
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	token := authHeader[7:] // Remove "Bearer "

	// Выполняем выход
	if err := h.authService.Logout(r.Context(), token); err != nil {
		// Для idempotency возвращаем success даже если токен уже в blacklist
		appErr := errors.GetAppError(err)
		if appErr != nil && appErr.Code == http.StatusUnauthorized {
			// Token invalid or already in blacklist - это OK для logout
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.log.LogError("Failed to logout", err)
		writeError(w, err)
		return
	}

	h.log.Info("User logged out")

	w.WriteHeader(http.StatusNoContent)
}

// Me возвращает информацию о текущем пользователе
// GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Извлекаем токен из заголовка
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	token := authHeader[7:] // Remove "Bearer "

	// Получаем пользователя
	user, err := h.authService.GetUserFromToken(r.Context(), token)
	if err != nil {
		h.log.LogError("Failed to get user by token", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, user)
}
