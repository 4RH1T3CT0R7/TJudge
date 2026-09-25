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

// поиск владельца программы для фильтрации текста ошибок
type MatchProgramLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Program, error)
}

// MatchHandler обслуживает запросы к матчам
type MatchHandler struct {
	matchRepo     MatchRepository
	programLookup MatchProgramLookup
	queueManager  MatchQueueManager
	log           *logger.Logger
}

// NewMatchHandler собирает хендлер матчей. programLookup и queueManager
// опциональны и могут быть nil - без них хендлер продолжает работать,
// просто отключая связанные с ними возможности
func NewMatchHandler(matchRepo MatchRepository, programLookup MatchProgramLookup, queueManager MatchQueueManager, log *logger.Logger) *MatchHandler {
	return &MatchHandler{
		matchRepo:     matchRepo,
		programLookup: programLookup,
		queueManager:  queueManager,
		log:           log,
	}
}

// filterMatchError прячет текст ошибки от чужих глаз: владелец упавшей
// программы и админ видят полный текст, остальным отдаётся обезличенное сообщение
func (h *MatchHandler) filterMatchError(ctx context.Context, match *models.Match, userID uuid.UUID, isAdmin bool) *models.Match {
	// нет ошибки или некому проверять владельца - отдаётся как есть
	if match.ErrorMessage == nil || *match.ErrorMessage == "" || h.programLookup == nil {
		return match
	}

	// админу показывается всё без фильтрации
	if isAdmin {
		return match
	}

	// winner=1 значит упала вторая программа, winner=2 - первая
	var failedProgramID uuid.UUID
	if match.Winner != nil {
		if *match.Winner == 1 {
			failedProgramID = match.Program2ID
		} else if *match.Winner == 2 {
			failedProgramID = match.Program1ID
		}
	}

	// победителя нет - непонятно, чья программа упала, поэтому скрывается
	if failedProgramID == uuid.Nil {
		opponentError := "Ошибка выполнения матча"
		match.ErrorMessage = &opponentError
		return match
	}

	program, err := h.programLookup.GetByID(ctx, failedProgramID)
	if err != nil {
		h.log.Warn("Failed to get program for error filtering", zap.Error(err))
		opponentError := "Ошибка выполнения матча"
		match.ErrorMessage = &opponentError
		return match
	}

	// свою ошибку пользователь видит целиком
	if program.UserID == userID {
		return match
	}

	// чужую - только общей формулировкой
	opponentError := "Программа оппонента завершилась с ошибкой"
	match.ErrorMessage = &opponentError
	return match
}

func (h *MatchHandler) filterMatchesErrors(ctx context.Context, matches []*models.Match, userID uuid.UUID, isAdmin bool) []*models.Match {
	for i, match := range matches {
		matches[i] = h.filterMatchError(ctx, match, userID, isAdmin)
	}
	return matches
}

// @Summary Получить матч
// @Description Возвращает матч по ID с фильтрацией ошибок по правам
// @Tags matches
// @Produce json
// @Param id path string true "Match ID" format(uuid)
// @Success 200 {object} models.Match
// @Failure 404 {object} object{error=string}
// @Router /matches/{id} [get]
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

	userID, _ := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	userRole, _ := r.Context().Value(middleware.RoleKey).(models.Role)
	isAdmin := userRole == models.RoleAdmin
	match = h.filterMatchError(r.Context(), match, userID, isAdmin)

	writeJSON(w, http.StatusOK, match)
}

// List отдаёт матчи постранично с набором необязательных фильтров
// @Summary Список матчей
// @Description Возвращает список матчей с фильтрацией и пагинацией
// @Tags matches
// @Produce json
// @Param tournament_id query string false "Фильтр по турниру" format(uuid)
// @Param program_id query string false "Фильтр по программе" format(uuid)
// @Param status query string false "Фильтр по статусу (pending, running, completed, failed, cancelled)"
// @Param game_type query string false "Фильтр по типу игры"
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} models.Match
// @Failure 400 {object} object{error=string}
// @Router /matches [get]
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
	userID, _ := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	userRole, _ := r.Context().Value(middleware.RoleKey).(models.Role)
	isAdmin := userRole == models.RoleAdmin
	matches = h.filterMatchesErrors(r.Context(), matches, userID, isAdmin)

	writeJSON(w, http.StatusOK, matches)
}

// @Summary Статистика матчей
// @Description Возвращает агрегированную статистику матчей (опционально по турниру)
// @Tags matches
// @Produce json
// @Param tournament_id query string false "Фильтр по турниру" format(uuid)
// @Success 200 {object} storage.MatchStatistics
// @Failure 400 {object} object{error=string}
// @Router /matches/statistics [get]
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

// @Summary Статистика очереди матчей
// @Description Возвращает статистику очереди матчей (только для админов)
// @Tags matches
// @Produce json
// @Security BearerAuth
// @Success 200 {object} queue.QueueStats
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /matches/queue/stats [get]
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

// @Summary Очистить очередь матчей
// @Description Отменяет все pending-матчи и очищает очереди (только для админов)
// @Tags matches
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{message=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /matches/queue/clear [post]
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

// @Summary Очистить невалидные матчи
// @Description Удаляет из очереди матчи, которых нет в БД (только для админов)
// @Tags matches
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{message=string,purged_count=int}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /matches/queue/purge [post]
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
