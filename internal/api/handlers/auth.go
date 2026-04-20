package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
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
	Logout(ctx context.Context, accessToken, refreshToken string) error
	GetUserFromToken(ctx context.Context, token string) (*domain.User, error)
	ValidateToken(token string) (*auth.Claims, error)
	UpdateProfile(ctx context.Context, userID string, req *auth.UpdateProfileRequest) (*domain.User, error)
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
// @Summary Регистрация пользователя
// @Description Создаёт нового пользователя и возвращает JWT токены
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RegisterRequest true "Данные регистрации"
// @Success 201 {object} auth.AuthResponse
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string} "Пользователь уже существует"
// @Router /auth/register [post]
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
// @Summary Вход в систему
// @Description Аутентификация по username/email и паролю, возвращает JWT токены
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.LoginRequest true "Данные для входа"
// @Success 200 {object} auth.AuthResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string} "Неверные учётные данные"
// @Router /auth/login [post]
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
		// PII намеренно не логируется: предотвращаем user enumeration.
		h.log.LogError("Failed to login", err)
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
// @Summary Обновление токенов
// @Description Обновляет access и refresh токены по refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object{refresh_token=string} true "Refresh token"
// @Success 200 {object} auth.AuthResponse
// @Failure 401 {object} object{error=string} "Невалидный refresh token"
// @Router /auth/refresh [post]
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
// @Summary Выход из системы
// @Description Инвалидирует access и refresh токены
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object{refresh_token=string} false "Refresh token (опционально)"
// @Security BearerAuth
// @Success 200 {object} object{message=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Извлекаем access token из заголовка
	accessToken := middleware.ExtractToken(r)
	if accessToken == "" {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	// Извлекаем refresh token из body (опционально)
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	// Игнорируем ошибки декодирования - refresh token опционален
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Выполняем выход (blacklist обоих токенов)
	if err := h.authService.Logout(r.Context(), accessToken, req.RefreshToken); err != nil {
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
// @Summary Текущий пользователь
// @Description Возвращает информацию о текущем аутентифицированном пользователе
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.User
// @Failure 401 {object} object{error=string}
// @Router /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Извлекаем токен из заголовка (middleware уже валидировал)
	token := middleware.ExtractToken(r)
	if token == "" {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	// Получаем пользователя
	user, err := h.authService.GetUserFromToken(r.Context(), token)
	if err != nil {
		h.log.LogError("Failed to get user by token", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// UpdateProfile обновляет профиль пользователя
// @Summary Обновление профиля
// @Description Обновляет профиль текущего пользователя (email, username)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.UpdateProfileRequest true "Данные для обновления профиля"
// @Security BearerAuth
// @Success 200 {object} domain.User
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/profile [put]
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	// Получаем user ID из контекста (установлен auth middleware)
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	var req auth.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Обновляем профиль
	user, err := h.authService.UpdateProfile(r.Context(), userID.String(), &req)
	if err != nil {
		h.log.LogError("Failed to update profile", err)
		writeError(w, err)
		return
	}

	h.log.Info("Profile updated",
		zap.String("user_id", user.ID.String()),
	)

	writeJSON(w, http.StatusOK, user)
}
