package handlers

import (
	"context"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MatchRepository описывает доступ к матчам в хранилище
type MatchRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Match, error)
	List(ctx context.Context, filter models.MatchFilter) ([]*models.Match, error)
	GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*storage.MatchStatistics, error)
	CancelPending(ctx context.Context) (int64, error)
}

// управление очередью матчей
type MatchQueueManager interface {
	GetStats(ctx context.Context) (*queue.QueueStats, error)
	Clear(ctx context.Context) error
	PurgeInvalidMatches(ctx context.Context, validator func(matchID string) bool) (int64, error)
}

// UserProgramLister - программы юзера и всех его команд (для фильтра текста ошибок)
type UserProgramLister interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Program, error)
}

// MatchHandler обслуживает запросы к матчам
type MatchHandler struct {
	matchRepo     MatchRepository
	programLookup UserProgramLister
	queueManager  MatchQueueManager
	log           *logger.Logger
}

// NewMatchHandler собирает хендлер матчей. programLookup и queueManager
// опциональны и могут быть nil - без них хендлер продолжает работать,
// просто отключая связанные с ними возможности
func NewMatchHandler(matchRepo MatchRepository, programLookup UserProgramLister, queueManager MatchQueueManager, log *logger.Logger) *MatchHandler {
	return &MatchHandler{
		matchRepo:     matchRepo,
		programLookup: programLookup,
		queueManager:  queueManager,
		log:           log,
	}
}

// redactMatchErrors прячет текст ошибок матчей (вывод упавшей программы):
// целиком его видят админ и команда упавшей программы, остальные, включая
// анонимов, - обезличенную формулировку. через неё идут все ручки, отдающие
// матчи; programs == nil - прячется всё
func redactMatchErrors(ctx context.Context, programs UserProgramLister, matches []*models.Match) {
	if isAdmin(ctx) {
		return
	}

	var own map[uuid.UUID]bool
	for _, m := range matches {
		if m.ErrorMessage == nil || *m.ErrorMessage == "" {
			continue
		}

		// winner=1 значит упала вторая программа, winner=2 - первая
		var failed uuid.UUID
		if m.Winner != nil {
			switch *m.Winner {
			case 1:
				failed = m.Program2ID
			case 2:
				failed = m.Program1ID
			}
		}
		// победителя нет - непонятно, чья программа упала, поэтому скрывается
		if failed == uuid.Nil {
			msg := "Ошибка выполнения матча"
			m.ErrorMessage = &msg
			continue
		}

		if own == nil {
			own = ownProgramIDs(ctx, programs)
		}
		if !own[failed] {
			msg := "Программа оппонента завершилась с ошибкой"
			m.ErrorMessage = &msg
		}
	}
}

// ownProgramIDs - программы команд текущего юзера; при ошибке пусто (всё прячется)
func ownProgramIDs(ctx context.Context, programs UserProgramLister) map[uuid.UUID]bool {
	own := map[uuid.UUID]bool{}
	userID, ok := middleware.GetUserID(ctx)
	if !ok || programs == nil {
		return own
	}
	list, err := programs.GetByUserID(ctx, userID)
	if err != nil {
		return own
	}
	for _, p := range list {
		own[p.ID] = true
	}
	return own
}

func (h *MatchHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "match")
	if !ok {
		return
	}

	match, err := h.matchRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get match", err,
			zap.String("match_id", id.String()),
		)
		writeError(w, err)
		return
	}

	redactMatchErrors(r.Context(), h.programLookup, []*models.Match{match})

	writeJSON(w, http.StatusOK, match)
}

// List отдаёт матчи постранично с набором необязательных фильтров
func (h *MatchHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := models.MatchFilter{}

	// фильтр по турниру: пустая строка означает "без фильтра"
	if tournamentIDStr := r.URL.Query().Get("tournament_id"); tournamentIDStr != "" {
		id, err := uuid.Parse(tournamentIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
			return
		}
		filter.TournamentID = &id
	}

	// фильтр по програме, тоже опциональный
	if programIDStr := r.URL.Query().Get("program_id"); programIDStr != "" {
		id, err := uuid.Parse(programIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
			return
		}
		filter.ProgramID = &id
	}

	// статус берётся только из белого списка, иначе отдаётся 400
	if status := r.URL.Query().Get("status"); status != "" {
		s := models.MatchStatus(status)
		switch s {
		case models.MatchPending, models.MatchRunning, models.MatchCompleted, models.MatchFailed, models.MatchCancelled:
			filter.Status = s
		default:
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid status filter, must be one of: pending, running, completed, failed, cancelled"))
			return
		}
	}

	// тип игры принимается как есть, без валидации значения
	filter.GameType = r.URL.Query().Get("game_type")

	// TODO: вынести разбор фильтров из хендлера в отдельный парсер
	pg := pagination.ParseLimitOffset(r, 50, 0)
	filter.Limit = pg.Limit
	filter.Offset = pg.Offset

	matches, err := h.matchRepo.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get matches list", err)
		writeError(w, err)
		return
	}

	// чужие ошибки скрываются в каждом матче списка
	redactMatchErrors(r.Context(), h.programLookup, matches)

	writeJSON(w, http.StatusOK, matches)
}

func (h *MatchHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	var tournamentID *uuid.UUID
	if tournamentIDStr := r.URL.Query().Get("tournament_id"); tournamentIDStr != "" {
		id, err := uuid.Parse(tournamentIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
			return
		}
		tournamentID = &id
	}

	stats, err := h.matchRepo.GetStatistics(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get match statistics", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *MatchHandler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	// без менеджера очереди статистику отдать нечем
	if h.queueManager == nil {
		writeError(w, errors.ErrInternal.WithMessage("queue manager not configured"))
		return
	}

	stats, err := h.queueManager.GetStats(r.Context())
	if err != nil {
		h.log.LogError("Failed to get queue stats", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *MatchHandler) ClearQueue(w http.ResponseWriter, r *http.Request) {
	if h.queueManager == nil {
		writeError(w, errors.ErrInternal.WithMessage("queue manager not configured"))
		return
	}

	// сначала отмена в бд, иначе recovery воркера вернёт pending в очередь.
	// то, что останется в редисе при сбое Clear, воркер пропустит: в running
	// он переводит только из pending
	cancelled, err := h.matchRepo.CancelPending(r.Context())
	if err != nil {
		h.log.LogError("Failed to cancel pending matches", err)
		writeError(w, err)
		return
	}

	if err := h.queueManager.Clear(r.Context()); err != nil {
		h.log.LogError("Failed to clear queue", err)
		writeError(w, err)
		return
	}

	h.log.Info("Queue cleared by admin", zap.Int64("cancelled", cancelled))

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "All queues cleared successfully",
	})
}

func (h *MatchHandler) PurgeInvalidMatches(w http.ResponseWriter, r *http.Request) {
	if h.queueManager == nil {
		writeError(w, errors.ErrInternal.WithMessage("queue manager not configured"))
		return
	}

	// матч считается валидным, только если он ещё есть в базе
	validator := func(matchIDStr string) bool {
		matchID, err := uuid.Parse(matchIDStr)
		if err != nil {
			return false
		}

		_, err = h.matchRepo.GetByID(r.Context(), matchID)
		return err == nil
	}

	purged, err := h.queueManager.PurgeInvalidMatches(r.Context(), validator)
	if err != nil {
		h.log.LogError("Failed to purge invalid matches", err)
		writeError(w, err)
		return
	}

	h.log.Info("Invalid matches purged by admin",
		zap.Int64("purged_count", purged),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"message":      "Invalid matches purged successfully",
		"purged_count": purged,
	})
}
