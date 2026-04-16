package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// PasswordResetService интерфейс для password reset (P1.11).
type PasswordResetService interface {
	Request(ctx context.Context, req auth.PasswordResetRequest, requesterIP string) error
	Confirm(ctx context.Context, req auth.PasswordResetConfirm) error
}

// PasswordResetHandler обрабатывает /auth/password-reset/* endpoints.
type PasswordResetHandler struct {
	service PasswordResetService
	log     *logger.Logger
}

// NewPasswordResetHandler создаёт handler.
func NewPasswordResetHandler(service PasswordResetService, log *logger.Logger) *PasswordResetHandler {
	return &PasswordResetHandler{service: service, log: log}
}

// Request начинает процесс восстановления пароля.
// @Summary Запрос восстановления пароля
// @Description Отправляет на указанный email ссылку для сброса пароля.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.PasswordResetRequest true "Email для восстановления"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Router /auth/password-reset/request [post]
func (h *PasswordResetHandler) Request(w http.ResponseWriter, r *http.Request) {
	var req auth.PasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid password-reset request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	ip := clientIPFromRequest(r)
	if err := h.service.Request(r.Context(), req, ip); err != nil {
		writeError(w, err)
		return
	}

	// Ответ одинаков и для найденного, и для несуществующего email —
	// предотвращает user enumeration.
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// Confirm завершает восстановление пароля.
// @Summary Подтверждение восстановления пароля
// @Description Устанавливает новый пароль по одноразовому токену из email.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.PasswordResetConfirm true "Токен + новый пароль"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string} "Неверный или истёкший токен"
// @Router /auth/password-reset/confirm [post]
func (h *PasswordResetHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	var req auth.PasswordResetConfirm
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid password-reset confirm body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	if err := h.service.Confirm(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// clientIPFromRequest извлекает IP запроса — используется для аудита
// (в теле password-reset-log'а). chi's RealIP уже разобрал X-Forwarded-For.
func clientIPFromRequest(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
