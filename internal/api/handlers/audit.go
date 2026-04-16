package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
)

// AuditLogReader — что нужно endpoint'у от репозитория.
type AuditLogReader interface {
	List(ctx context.Context, limit int) ([]*domain.AuditLogEntry, error)
}

// AuditHandler отдаёт записи admin audit log'а.
// P1.12: endpoint GET /admin/audit — только для admin'ов (middleware в routes.go).
type AuditHandler struct {
	repo AuditLogReader
	log  *logger.Logger
}

// NewAuditHandler создаёт handler чтения audit log'а.
func NewAuditHandler(repo AuditLogReader, log *logger.Logger) *AuditHandler {
	return &AuditHandler{repo: repo, log: log}
}

// List возвращает последние N записей audit log'а.
// @Summary Получить audit log (admin-only)
// @Description Возвращает последние записи admin-действий.
// @Tags admin
// @Produce json
// @Param limit query int false "Лимит записей (1-500, default 100)"
// @Success 200 {array} domain.AuditLogEntry
// @Security BearerAuth
// @Router /admin/audit [get]
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 500 {
		limit = 500
	}

	entries, err := h.repo.List(r.Context(), limit)
	if err != nil {
		h.log.LogError("Failed to list audit log", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, entries)
}
