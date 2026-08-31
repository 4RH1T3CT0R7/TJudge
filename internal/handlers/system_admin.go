package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// recoveryStuckThreshold — после этого времени матч считается зависшим в running
// (синхронизировано с RecoveryService воркера: 120s > worker timeout 90s)
const recoveryStuckThreshold = 2 * time.Minute

// AuditLogReader описывает то, что нужно эндпоинту от репозитория
type AuditLogReader interface {
	List(ctx context.Context, limit int) ([]*models.AuditLogEntry, error)
}

// AuditHandler отдаёт записи admin audit log'а.
// Эндпоинт GET /admin/audit доступен только админам (middleware в routes.go).
type AuditHandler struct {
	repo AuditLogReader
	log  *logger.Logger
}

// NewAuditHandler создаёт handler чтения audit log'а
func NewAuditHandler(repo AuditLogReader, log *logger.Logger) *AuditHandler {
	return &AuditHandler{repo: repo, log: log}
}

// List возвращает последние N записей audit log'а
// @Summary Получить audit log (admin-only)
// @Description Возвращает последние записи admin-действий.
// @Tags admin
// @Produce json
// @Param limit query int false "Лимит записей (1-500, default 100)"
// @Success 200 {array} models.AuditLogEntry
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

// повтор ошибочных outbox-задач
type RecoveryOutboxRepo interface {
	RetryErrors(ctx context.Context) (int64, error)
}

// программы, зависшие в компиляции
type RecoveryProgramRepo interface {
	GetStuckCompiling(ctx context.Context, olderThan time.Duration, limit int) ([]*models.Program, error)
}

// постановка программ в очередь компиляции
type RecoveryCompileQueue interface {
	Enqueue(ctx context.Context, programID uuid.UUID) error
}

// зависшие матчи
type RecoveryMatchRepo interface {
	GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*models.Match, error)
	ResetToPending(ctx context.Context, id uuid.UUID) error
}

// возврат матчей в очередь и чистка dead-letter
type RecoveryQueueManager interface {
	Enqueue(ctx context.Context, match *models.Match) error
	ClearDeadLetter(ctx context.Context) (int64, error)
}

// SystemRecoveryHandler — кнопки восстановления в админ-панели: прикладные
// поломки (зависшие матчи/компиляция, ошибки outbox, dead-letter) чинятся
// прямо из интерфейса, без SSH и ручного SQL.
type SystemRecoveryHandler struct {
	outboxRepo   RecoveryOutboxRepo
	programRepo  RecoveryProgramRepo
	compileQueue RecoveryCompileQueue
	matchRepo    RecoveryMatchRepo
	queueManager RecoveryQueueManager
	log          *logger.Logger
}

// NewSystemRecoveryHandler создаёт handler восстановления
func NewSystemRecoveryHandler(
	outboxRepo RecoveryOutboxRepo,
	programRepo RecoveryProgramRepo,
	compileQueue RecoveryCompileQueue,
	matchRepo RecoveryMatchRepo,
	queueManager RecoveryQueueManager,
	log *logger.Logger,
) *SystemRecoveryHandler {
	return &SystemRecoveryHandler{
		outboxRepo:   outboxRepo,
		programRepo:  programRepo,
		compileQueue: compileQueue,
		matchRepo:    matchRepo,
		queueManager: queueManager,
		log:          log,
	}
}

// RetryOutboxErrors возвращает ошибочные outbox-задачи в обработку
// @Summary Повторить ошибочные outbox-задачи (admin)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{retried=int}
// @Router /system/recovery/outbox-retry [post]
func (h *SystemRecoveryHandler) RetryOutboxErrors(w http.ResponseWriter, r *http.Request) {
	retried, err := h.outboxRepo.RetryErrors(r.Context())
	if err != nil {
		h.log.LogError("recovery: outbox retry", err)
		writeError(w, err)
		return
	}

	h.log.Info("recovery: outbox errors retried", zap.Int64("count", retried))
	writeJSON(w, http.StatusOK, map[string]int64{"retried": retried})
}

// RequeueCompiling возвращает все compiling-программы в очередь компиляции
// @Summary Перезапустить зависшую компиляцию (admin)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{requeued=int}
// @Router /system/recovery/requeue-compiling [post]
func (h *SystemRecoveryHandler) RequeueCompiling(w http.ResponseWriter, r *http.Request) {
	// olderThan=0: берутся все compiling-программы — кнопка жмётся осознанно,
	// дедупликацию дублей обеспечивает идемпотентность compile-worker'а
	// (статус-проверка перед компиляцией).
	programs, err := h.programRepo.GetStuckCompiling(r.Context(), 0, 500)
	if err != nil {
		h.log.LogError("recovery: list compiling programs", err)
		writeError(w, err)
		return
	}

	requeued := int64(0)
	for _, p := range programs {
		if err := h.compileQueue.Enqueue(r.Context(), p.ID); err != nil {
			h.log.LogError("recovery: requeue compile", err, zap.String("program_id", p.ID.String()))
			continue
		}
		requeued++
	}

	h.log.Info("recovery: compiling programs requeued", zap.Int64("count", requeued))
	writeJSON(w, http.StatusOK, map[string]int64{"requeued": requeued})
}

// ResetStuckMatches сбрасывает зависшие running-матчи в pending и возвращает в очередь
// @Summary Сбросить зависшие матчи (admin)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{reset=int}
// @Router /system/recovery/reset-stuck-matches [post]
func (h *SystemRecoveryHandler) ResetStuckMatches(w http.ResponseWriter, r *http.Request) {
	stuck, err := h.matchRepo.GetStuckRunning(r.Context(), recoveryStuckThreshold, 1000)
	if err != nil {
		h.log.LogError("recovery: list stuck matches", err)
		writeError(w, err)
		return
	}

	reset := int64(0)
	for _, m := range stuck {
		if err := h.matchRepo.ResetToPending(r.Context(), m.ID); err != nil {
			h.log.LogError("recovery: reset match", err, zap.String("match_id", m.ID.String()))
			continue
		}
		m.Status = models.MatchPending
		if err := h.queueManager.Enqueue(r.Context(), m); err != nil {
			// не страшно: pending-матч подберёт периодический recovery воркера
			h.log.LogError("recovery: enqueue match", err, zap.String("match_id", m.ID.String()))
		}
		reset++
	}

	h.log.Info("recovery: stuck matches reset", zap.Int64("count", reset))
	writeJSON(w, http.StatusOK, map[string]int64{"reset": reset})
}

// ClearDeadLetter очищает dead-letter очередь
// @Summary Очистить dead-letter очередь (admin)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{cleared=int}
// @Router /system/recovery/clear-dead-letter [post]
func (h *SystemRecoveryHandler) ClearDeadLetter(w http.ResponseWriter, r *http.Request) {
	cleared, err := h.queueManager.ClearDeadLetter(r.Context())
	if err != nil {
		h.log.LogError("recovery: clear dead-letter", err)
		writeError(w, err)
		return
	}

	h.log.Info("recovery: dead-letter cleared", zap.Int64("count", cleared))
	writeJSON(w, http.StatusOK, map[string]int64{"cleared": cleared})
}
