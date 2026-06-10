package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// recoveryStuckThreshold - матч считается зависшим в running после этого
// времени (синхронизировано с RecoveryService воркера: 120s > worker timeout 90s).
const recoveryStuckThreshold = 2 * time.Minute

// RecoveryOutboxRepo - повтор ошибочных outbox-задач.
type RecoveryOutboxRepo interface {
	RetryErrors(ctx context.Context) (int64, error)
}

// RecoveryProgramRepo - программы, зависшие в компиляции.
type RecoveryProgramRepo interface {
	GetStuckCompiling(ctx context.Context, olderThan time.Duration, limit int) ([]*domain.Program, error)
}

// RecoveryCompileQueue - постановка программ в очередь компиляции.
type RecoveryCompileQueue interface {
	Enqueue(ctx context.Context, programID uuid.UUID) error
}

// RecoveryMatchRepo - зависшие матчи.
type RecoveryMatchRepo interface {
	GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*domain.Match, error)
	ResetToPending(ctx context.Context, id uuid.UUID) error
}

// RecoveryQueueManager - возврат матчей в очередь и чистка dead-letter.
type RecoveryQueueManager interface {
	Enqueue(ctx context.Context, match *domain.Match) error
	ClearDeadLetter(ctx context.Context) (int64, error)
}

// SystemRecoveryHandler - кнопки восстановления в админ-панели: прикладные
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

// NewSystemRecoveryHandler создаёт handler восстановления.
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

// RetryOutboxErrors возвращает ошибочные outbox-задачи в обработку.
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

// RequeueCompiling возвращает все compiling-программы в очередь компиляции.
// @Summary Перезапустить зависшую компиляцию (admin)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{requeued=int}
// @Router /system/recovery/requeue-compiling [post]
func (h *SystemRecoveryHandler) RequeueCompiling(w http.ResponseWriter, r *http.Request) {
	// olderThan=0: берём ВСЕ compiling-программы - кнопка жмётся осознанно,
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

// ResetStuckMatches сбрасывает зависшие running-матчи в pending и возвращает в очередь.
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
		m.Status = domain.MatchPending
		if err := h.queueManager.Enqueue(r.Context(), m); err != nil {
			// Не страшно: pending-матч подберёт периодический recovery воркера.
			h.log.LogError("recovery: enqueue match", err, zap.String("match_id", m.ID.String()))
		}
		reset++
	}

	h.log.Info("recovery: stuck matches reset", zap.Int64("count", reset))
	writeJSON(w, http.StatusOK, map[string]int64{"reset": reset})
}

// ClearDeadLetter очищает dead-letter очередь.
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
