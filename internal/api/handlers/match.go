package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MatchRepository интерфейс для работы с матчами
type MatchRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error)
	List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error)
	GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*db.MatchStatistics, error)
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

// NewMatchHandler создаёт новый match handler
func NewMatchHandler(matchRepo MatchRepository, matchCache MatchCache, log *logger.Logger) *MatchHandler {
	return &MatchHandler{
		matchRepo:  matchRepo,
		matchCache: matchCache,
		log:        log,
	}
}

// NewMatchHandlerWithProgramLookup создаёт match handler с возможностью фильтрации ошибок
func NewMatchHandlerWithProgramLookup(matchRepo MatchRepository, matchCache MatchCache, programLookup MatchProgramLookup, log *logger.Logger) *MatchHandler {
	return &MatchHandler{
		matchRepo:     matchRepo,
		matchCache:    matchCache,
		programLookup: programLookup,
		log:           log,
	}
}

// NewMatchHandlerFull создаёт match handler со всеми зависимостями
func NewMatchHandlerFull(matchRepo MatchRepository, matchCache MatchCache, programLookup MatchProgramLookup, queueManager MatchQueueManager, log *logger.Logger) *MatchHandler {
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
// GET /api/v1/matches/:id
func (h *MatchHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid match ID"))
		return
	}

	// Проверяем кэш результата
	cachedResult, err := h.matchCache.Get(r.Context(), id)
	if err == nil && cachedResult != nil {
		h.log.Info("Match result from cache",
			zap.String("match_id", id.String()),
		)
		writeJSON(w, http.StatusOK, cachedResult)
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
// GET /api/v1/matches
func (h *MatchHandler) List(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры фильтрации
	filter := domain.MatchFilter{}

	// Tournament ID filter
	if tournamentIDStr := r.URL.Query().Get("tournament_id"); tournamentIDStr != "" {
		id, err := uuid.Parse(tournamentIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
			return
		}
		filter.TournamentID = &id
	}

	// Program ID filter
	if programIDStr := r.URL.Query().Get("program_id"); programIDStr != "" {
		id, err := uuid.Parse(programIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
			return
		}
		filter.ProgramID = &id
	}

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = domain.MatchStatus(status)
	}

	// Game type filter
	filter.GameType = r.URL.Query().Get("game_type")

	// Pagination
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	filter.Limit = limit

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}
	filter.Offset = offset

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
// GET /api/v1/matches/statistics
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
// GET /api/v1/matches/queue/stats
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
// POST /api/v1/matches/queue/clear
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
// POST /api/v1/matches/queue/purge
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Invalid matches purged successfully",
		"purged_count": purged,
	})
}
