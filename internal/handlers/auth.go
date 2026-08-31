package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// AuthService описывает методы сервиса аутентификации
type AuthService interface {
	Register(ctx context.Context, req *auth.RegisterRequest) (*auth.AuthResponse, error)
	Login(ctx context.Context, req *auth.LoginRequest) (*auth.AuthResponse, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*auth.AuthResponse, error)
	Logout(ctx context.Context, accessToken, refreshToken string) error
	GetUserFromToken(ctx context.Context, token string) (*models.User, error)
	ValidateToken(token string) (*auth.Claims, error)
	UpdateProfile(ctx context.Context, userID string, req *auth.UpdateProfileRequest) (*models.User, error)
}

// AuthHandler обрабатывает HTTP-запросы аутентификации
type AuthHandler struct {
	authService AuthService
	log         *logger.Logger
}

// NewAuthHandler создаёт хендлер аутентификации
func NewAuthHandler(authService AuthService, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         log,
	}
}

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
	// TODO: разбор тела и writeError повторяются во всех хендлерах — вынести в общий хелпер
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

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

	resp, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		// логин и пароль намеренно не пишутся в лог, иначе перебором можно узнать какие юзеры есть
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

	// тут ротация: старый refresh инвалидируется, в ответ уходит новая пара токенов
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
	accessToken := middleware.ExtractToken(r)
	if accessToken == "" {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	// refresh в теле опционален, поэтому ошибка декодирования просто проглатывается
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// оба токена уходят в blacklist, чтобы их нельзя было переиспользовать
	if err := h.authService.Logout(r.Context(), accessToken, req.RefreshToken); err != nil {
		// logout идемпотентен: токен мог быть уже инвалидроват — это не ошибка
		appErr := errors.GetAppError(err)
		if appErr != nil && appErr.Code == http.StatusUnauthorized {
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

// @Summary Текущий пользователь
// @Description Возвращает информацию о текущем аутентифицированном пользователе
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.User
// @Failure 401 {object} object{error=string}
// @Router /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// токен уже проверен auth-middleware, здесь только достаётся юзер
	token := middleware.ExtractToken(r)
	if token == "" {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	user, err := h.authService.GetUserFromToken(r.Context(), token)
	if err != nil {
		h.log.LogError("Failed to get user by token", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// @Summary Обновление профиля
// @Description Обновляет профиль текущего пользователя (email, username)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.UpdateProfileRequest true "Данные для обновления профиля"
// @Security BearerAuth
// @Success 200 {object} models.User
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/profile [put]
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	// userID кладёт в контекст auth-middleware
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
