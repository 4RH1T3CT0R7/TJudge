package handlers

import (
	"context"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MatchRepository интерфейс для работы с матчами
type MatchRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error)
	List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error)
	GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*storage.MatchStatistics, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Match, error)
}

// MatchQueueManager интерфейс для работы с очередью матчей
type MatchQueueManager interface {
	GetStats(ctx context.Context) (*queue.QueueStats, error)
	Clear(ctx context.Context) error
	PurgeInvalidMatches(ctx context.Context, validator func(matchID string) bool) (int64, error)
}

// MatchCache интерфейс для кэширования матчей
type MatchCache interface {
	Get(ctx context.Context, matchID uuid.UUID) (*domain.MatchResult, error)
	Set(ctx context.Context, matchID uuid.UUID, result *domain.MatchResult) error
	GetMatch(ctx context.Context, matchID uuid.UUID) (*domain.Match, error)
	SetMatch(ctx context.Context, match *domain.Match) error
}

// MatchProgramLookup интерфейс для получения владельца программы
type MatchProgramLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error)
}

// MatchHandler обрабатывает запросы матчей
type MatchHandler struct {
	matchRepo     MatchRepository
	matchCache    MatchCache
	programLookup MatchProgramLookup
	queueManager  MatchQueueManager
	log           *logger.Logger
}

// NewMatchHandler создаёт match handler. Опциональные зависимости (programLookup,
// queueManager) могут быть nil - handler корректно деградирует при их отсутствии.
func NewMatchHandler(matchRepo MatchRepository, matchCache MatchCache, programLookup MatchProgramLookup, queueManager MatchQueueManager, log *logger.Logger) *MatchHandler {
	return &MatchHandler{
		matchRepo:     matchRepo,
		matchCache:    matchCache,
		programLookup: programLookup,
		queueManager:  queueManager,
		log:           log,
	}
}

// filterMatchError фильтрует сообщение об ошибке матча в зависимости от прав пользователя
// Если пользователь владеет программой, которая вызвала ошибку, или является админом - показываем полную ошибку
// Иначе показываем "Программа оппонента завершилась с ошибкой"
func (h *MatchHandler) filterMatchError(ctx context.Context, match *domain.Match, userID uuid.UUID, isAdmin bool) *domain.Match {
	// Если нет ошибки или нет program lookup - возвращаем как есть
	if match.ErrorMessage == nil || *match.ErrorMessage == "" || h.programLookup == nil {
		return match
	}

	// Админы видят все ошибки
	if isAdmin {
		return match
	}

	// Определяем, какая программа вызвала ошибку
	// Winner = 1 означает что программа 1 выиграла (программа 2 упала)
	// Winner = 2 означает что программа 2 выиграла (программа 1 упала)
	var failedProgramID uuid.UUID
	if match.Winner != nil {
		if *match.Winner == 1 {
			failedProgramID = match.Program2ID
		} else if *match.Winner == 2 {
			failedProgramID = match.Program1ID
		}
	}

	// Если не можем определить упавшую программу - скрываем ошибку
	if failedProgramID == uuid.Nil {
		opponentError := "Ошибка выполнения матча"
		match.ErrorMessage = &opponentError
		return match
	}

	// Проверяем владельца упавшей программы
	program, err := h.programLookup.GetByID(ctx, failedProgramID)
	if err != nil {
		h.log.Warn("Failed to get program for error filtering", zap.Error(err))
		opponentError := "Ошибка выполнения матча"
		match.ErrorMessage = &opponentError
		return match
	}

	// Если пользователь владеет упавшей программой - показываем полную ошибку
	if program.UserID == userID {
		return match
	}

	// Иначе показываем обезличенное сообщение
	opponentError := "Программа оппонента завершилась с ошибкой"
	match.ErrorMessage = &opponentError
	return match
}

// filterMatchesErrors применяет фильтрацию ошибок к списку матчей
func (h *MatchHandler) filterMatchesErrors(ctx context.Context, matches []*domain.Match, userID uuid.UUID, isAdmin bool) []*domain.Match {
	for i, match := range matches {
		matches[i] = h.filterMatchError(ctx, match, userID, isAdmin)
	}
	return matches
}

// Get обрабатывает получение матча
// @Summary Получить матч
// @Description Возвращает матч по ID с фильтрацией ошибок по правам
// @Tags matches
// @Produce json
// @Param id path string true "Match ID" format(uuid)
// @Success 200 {object} domain.Match
// @Failure 404 {object} object{error=string}
// @Router /matches/{id} [get]
func (h *MatchHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id, ok := httputil.ParseUUIDParam(w, r, "id", "match")
	if !ok {
		return
	}

	// Проверяем кэш матча
	cachedMatch, cacheErr := h.matchCache.GetMatch(r.Context(), id)
	if cacheErr == nil && cachedMatch != nil {
		h.log.Info("Match from cache",
			zap.String("match_id", id.String()),
		)
		// Фильтруем сообщение об ошибке в зависимости от прав пользователя
		userID, _ := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
		userRole, _ := r.Context().Value(middleware.RoleKey).(domain.Role)
		isAdmin := userRole == domain.RoleAdmin
		cachedMatch = h.filterMatchError(r.Context(), cachedMatch, userID, isAdmin)
		writeJSON(w, http.StatusOK, cachedMatch)
		return
	}

	// Получаем матч из БД
	match, err := h.matchRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get match", err,
			zap.String("match_id", id.String()),
		)
		writeError(w, err)
		return
	}

	// Фильтруем сообщение об ошибке в зависимости от прав пользователя
	userID, _ := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	userRole, _ := r.Context().Value(middleware.RoleKey).(domain.Role)
	isAdmin := userRole == domain.RoleAdmin
	match = h.filterMatchError(r.Context(), match, userID, isAdmin)

	writeJSON(w, http.StatusOK, match)
}

// List обрабатывает получение списка матчей
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
// @Success 200 {array} domain.Match
// @Failure 400 {object} object{error=string}
// @Router /matches [get]
func (h *MatchHandler) List(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры фильтрации
	filter := domain.MatchFilter{}

	// Фильтр по Tournament ID
	if tournamentIDStr := r.URL.Query().Get("tournament_id"); tournamentIDStr != "" {
		id, err := uuid.Parse(tournamentIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
			return
		}
		filter.TournamentID = &id
	}

	// Фильтр по Program ID
	if programIDStr := r.URL.Query().Get("program_id"); programIDStr != "" {
		id, err := uuid.Parse(programIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
			return
		}
		filter.ProgramID = &id
	}

	// Фильтр по статусу
	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.MatchStatus(status)
		switch s {
		case domain.MatchPending, domain.MatchRunning, domain.MatchCompleted, domain.MatchFailed, domain.MatchCancelled:
			filter.Status = s
		default:
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid status filter, must be one of: pending, running, completed, failed, cancelled"))
			return
		}
	}

	// Фильтр по типу игры
	filter.GameType = r.URL.Query().Get("game_type")

	// Пагинация
	pg := pagination.ParseLimitOffset(r, 50, 0)
	filter.Limit = pg.Limit
	filter.Offset = pg.Offset

	// Получаем список матчей
	matches, err := h.matchRepo.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get matches list", err)
		writeError(w, err)
		return
	}

	// Фильтруем сообщения об ошибках в зависимости от прав пользователя
	userID, _ := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	userRole, _ := r.Context().Value(middleware.RoleKey).(domain.Role)
	isAdmin := userRole == domain.RoleAdmin
	matches = h.filterMatchesErrors(r.Context(), matches, userID, isAdmin)

	writeJSON(w, http.StatusOK, matches)
}

// GetStatistics обрабатывает получение статистики матчей
// @Summary Статистика матчей
// @Description Возвращает агрегированную статистику матчей (опционально по турниру)
// @Tags matches
// @Produce json
// @Param tournament_id query string false "Фильтр по турниру" format(uuid)
// @Success 200 {object} storage.MatchStatistics
// @Failure 400 {object} object{error=string}
// @Router /matches/statistics [get]
func (h *MatchHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	// Получаем tournament_id из query параметров (опционально)
	var tournamentID *uuid.UUID
	if tournamentIDStr := r.URL.Query().Get("tournament_id"); tournamentIDStr != "" {
		id, err := uuid.Parse(tournamentIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
			return
		}
		tournamentID = &id
	}

	// Получаем статистику
	stats, err := h.matchRepo.GetStatistics(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get match statistics", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// GetQueueStats возвращает статистику очереди матчей (только для админов)
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

// ClearQueue очищает все очереди матчей (только для админов)
// @Summary Очистить очередь матчей
// @Description Очищает все очереди матчей (только для админов)
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

	if err := h.queueManager.Clear(r.Context()); err != nil {
		h.log.LogError("Failed to clear queue", err)
		writeError(w, err)
		return
	}

	h.log.Info("Queue cleared by admin")

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "All queues cleared successfully",
	})
}

// PurgeInvalidMatches удаляет из очереди матчи, которых нет в БД (только для админов)
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

	// Создаём валидатор, который проверяет существование матча в БД
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
