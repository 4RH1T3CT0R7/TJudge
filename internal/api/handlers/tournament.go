package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/tournament"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TournamentService интерфейс для tournament service
type TournamentService interface {
	Create(ctx context.Context, req *tournament.CreateRequest) (*domain.Tournament, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
	List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error)
	Join(ctx context.Context, req *tournament.JoinRequest) error
	Start(ctx context.Context, tournamentID uuid.UUID) error
	Complete(ctx context.Context, tournamentID uuid.UUID) error
	Delete(ctx context.Context, tournamentID uuid.UUID) error
	GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error)
	GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error)
	CreateMatch(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID, priority domain.MatchPriority) (*domain.Match, error)
	GetMatches(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error)
	GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error)
}

// SchedulingService интерфейс для сервиса планирования матчей
type SchedulingService interface {
	RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error)
	RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error)
	RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error)
}

// TournamentHandler обрабатывает запросы турниров
type TournamentHandler struct {
	tournamentService TournamentService
	schedulingService SchedulingService
	log               *logger.Logger
}

// NewTournamentHandler создаёт новый tournament handler
func NewTournamentHandler(tournamentService TournamentService, schedulingService SchedulingService, log *logger.Logger) *TournamentHandler {
	return &TournamentHandler{
		tournamentService: tournamentService,
		schedulingService: schedulingService,
		log:               log,
	}
}

// Create обрабатывает создание турнира
// @Summary Создать турнир
// @Description Создаёт новый турнир (только для админов)
// @Tags tournaments
// @Accept json
// @Produce json
// @Param request body tournament.CreateRequest true "Данные турнира"
// @Security BearerAuth
// @Success 201 {object} domain.Tournament
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments [post]
func (h *TournamentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req tournament.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Получаем ID создателя из контекста
	if userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID); ok {
		req.CreatorID = &userID
	}

	// Создаём турнир
	t, err := h.tournamentService.Create(r.Context(), &req)
	if err != nil {
		h.log.LogError("Failed to create tournament", err)
		writeError(w, err)
		return
	}

	h.log.Info("Tournament created",
		zap.String("tournament_id", t.ID.String()),
		zap.String("name", t.Name),
	)

	writeJSON(w, http.StatusCreated, t)
}

// List обрабатывает получение списка турниров
// @Summary Список турниров
// @Description Возвращает список турниров с фильтрацией и пагинацией
// @Tags tournaments
// @Produce json
// @Param status query string false "Фильтр по статусу (pending, active, completed, cancelled)"
// @Param game_type query string false "Фильтр по типу игры"
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} domain.Tournament
// @Failure 400 {object} object{error=string}
// @Router /tournaments [get]
func (h *TournamentHandler) List(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры фильтрации
	filter := domain.TournamentFilter{}

	// Фильтр по статусу
	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.TournamentStatus(status)
		switch s {
		case domain.TournamentPending, domain.TournamentActive, domain.TournamentCompleted, domain.TournamentCancelled:
			filter.Status = s
		default:
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid status filter, must be one of: pending, active, completed, cancelled"))
			return
		}
	}

	// Фильтр по типу игры
	filter.GameType = r.URL.Query().Get("game_type")

	// Пагинация
	pg := pagination.ParseLimitOffset(r, 50, 0)
	filter.Limit = pg.Limit
	filter.Offset = pg.Offset

	// Получаем список турниров
	tournaments, err := h.tournamentService.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get tournaments list", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tournaments)
}

// Get обрабатывает получение турнира
// @Summary Получить турнир
// @Description Возвращает турнир по ID
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {object} domain.Tournament
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id} [get]
func (h *TournamentHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Получаем турнир
	t, err := h.tournamentService.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get tournament", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// Join обрабатывает присоединение к турниру
// @Summary Присоединиться к турниру
// @Description Присоединяет программу к турниру
// @Tags tournaments
// @Accept json
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param request body object{program_id=string} true "ID программы для участия"
// @Security BearerAuth
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/join [post]
func (h *TournamentHandler) Join(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Декодируем тело запроса
	var req struct {
		ProgramID uuid.UUID `json:"program_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Присоединяемся
	joinReq := &tournament.JoinRequest{
		TournamentID: tournamentID,
		ProgramID:    req.ProgramID,
	}

	if err := h.tournamentService.Join(r.Context(), joinReq); err != nil {
		h.log.LogError("Failed to join tournament", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("program_id", req.ProgramID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Joined tournament",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("program_id", req.ProgramID.String()),
	)

	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

// Start обрабатывает запуск турнира
// @Summary Запустить турнир
// @Description Переводит турнир в статус active (только для админов)
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/start [post]
func (h *TournamentHandler) Start(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Запускаем турнир
	if err := h.tournamentService.Start(r.Context(), id); err != nil {
		h.log.LogError("Failed to start tournament", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Tournament started",
		zap.String("tournament_id", id.String()),
	)

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// Complete обрабатывает завершение турнира
// @Summary Завершить турнир
// @Description Переводит турнир в статус completed (только для админов)
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/complete [post]
func (h *TournamentHandler) Complete(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Завершаем турнир
	if err := h.tournamentService.Complete(r.Context(), id); err != nil {
		h.log.LogError("Failed to complete tournament", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Tournament completed",
		zap.String("tournament_id", id.String()),
	)

	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// Delete обрабатывает удаление турнира
// @Summary Удалить турнир
// @Description Удаляет турнир по ID (только для админов)
// @Tags tournaments
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Турнир удалён"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id} [delete]
func (h *TournamentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Удаляем турнир
	if err := h.tournamentService.Delete(r.Context(), id); err != nil {
		h.log.LogError("Failed to delete tournament", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Tournament deleted",
		zap.String("tournament_id", id.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetLeaderboard обрабатывает получение таблицы лидеров
// @Summary Таблица лидеров турнира
// @Description Возвращает таблицу лидеров для турнира
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param limit query int false "Лимит записей" default(100)
// @Success 200 {array} domain.LeaderboardEntry
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/leaderboard [get]
func (h *TournamentHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Получаем limit из query параметров
	pg := pagination.ParseLimitOffset(r, 100, 0)

	// Получаем leaderboard
	leaderboard, err := h.tournamentService.GetLeaderboard(r.Context(), id, pg.Limit)
	if err != nil {
		h.log.LogError("Failed to get leaderboard", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, leaderboard)
}

// CreateMatch обрабатывает создание матча
// @Summary Создать матч
// @Description Создаёт матч между двумя программами в турнире (только для админов)
// @Tags tournaments
// @Accept json
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param request body object{program1_id=string,program2_id=string,priority=string} true "Данные матча"
// @Security BearerAuth
// @Success 201 {object} domain.Match
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments/{id}/matches [post]
func (h *TournamentHandler) CreateMatch(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Декодируем тело запроса
	var req struct {
		Program1ID uuid.UUID            `json:"program1_id"`
		Program2ID uuid.UUID            `json:"program2_id"`
		Priority   domain.MatchPriority `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Устанавливаем приоритет по умолчанию, если не указан
	if req.Priority == "" {
		req.Priority = domain.PriorityMedium
	}

	// Создаём матч
	match, err := h.tournamentService.CreateMatch(r.Context(), tournamentID, req.Program1ID, req.Program2ID, req.Priority)
	if err != nil {
		h.log.LogError("Failed to create match", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Match created",
		zap.String("match_id", match.ID.String()),
		zap.String("tournament_id", tournamentID.String()),
	)

	writeJSON(w, http.StatusCreated, match)
}

// CrossGameLeaderboardEntry представляет строку кросс-игрового рейтинга
type CrossGameLeaderboardEntry struct {
	Rank        int            `json:"rank"`
	TeamID      *uuid.UUID     `json:"team_id,omitempty"`
	TeamName    string         `json:"team_name"`
	ProgramName string         `json:"program_name"`
	GameRatings map[string]int `json:"game_ratings"` // game_id -> rating
	TotalRating int            `json:"total_rating"`
	TotalWins   int            `json:"total_wins"`
	TotalLosses int            `json:"total_losses"`
	TotalGames  int            `json:"total_games"`
}

// GetCrossGameLeaderboard получает кросс-игровой рейтинг турнира
// @Summary Кросс-игровой рейтинг
// @Description Возвращает общий рейтинг по всем играм турнира
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {array} domain.CrossGameLeaderboardEntry
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/cross-game-leaderboard [get]
func (h *TournamentHandler) GetCrossGameLeaderboard(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Получаем кросс-игровой рейтинг
	entries, err := h.tournamentService.GetCrossGameLeaderboard(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get cross-game leaderboard", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

// GetMatches обрабатывает получение списка матчей турнира
// @Summary Матчи турнира
// @Description Возвращает список матчей турнира с пагинацией
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} domain.Match
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/matches [get]
func (h *TournamentHandler) GetMatches(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Получаем параметры пагинации
	pg := pagination.ParseLimitOffset(r, 50, 0)

	// Получаем матчи
	matches, err := h.tournamentService.GetMatches(r.Context(), tournamentID, pg.Limit, pg.Offset)
	if err != nil {
		h.log.LogError("Failed to get matches", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, matches)
}

// GetMatchesByRounds обрабатывает получение матчей турнира сгруппированных по раундам
// @Summary Матчи по раундам
// @Description Возвращает матчи турнира, сгруппированные по раундам
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {array} domain.MatchRound
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/matches/rounds [get]
func (h *TournamentHandler) GetMatchesByRounds(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Получаем матчи по раундам
	rounds, err := h.tournamentService.GetMatchesByRounds(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get matches by rounds", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, rounds)
}

// RunAllMatches запускает все ожидающие матчи турнира
// @Summary Запустить все матчи
// @Description Добавляет все ожидающие матчи турнира в очередь (только для админов)
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} object{status=string,enqueued=int}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/run-matches [post]
func (h *TournamentHandler) RunAllMatches(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Запускаем все матчи
	enqueued, err := h.schedulingService.RunAllMatches(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to run all matches", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Started all pending matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("enqueued", enqueued),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "started",
		"enqueued": enqueued,
	})
}

// RunGameMatches запускает матчи для конкретной игры в турнире
// @Summary Запустить матчи для игры
// @Description Добавляет матчи конкретной игры в очередь (только для админов)
// @Tags tournaments
// @Accept json
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param request body object{game_type=string} true "Тип игры"
// @Security BearerAuth
// @Success 200 {object} object{status=string,game_type=string,enqueued=int}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments/{id}/run-game-matches [post]
func (h *TournamentHandler) RunGameMatches(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Декодируем тело запроса для получения game_type
	var req struct {
		GameType string `json:"game_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("game_type is required"))
		return
	}

	if req.GameType == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("game_type is required"))
		return
	}

	// Запускаем матчи для игры
	enqueued, err := h.schedulingService.RunGameMatches(r.Context(), tournamentID, req.GameType)
	if err != nil {
		h.log.LogError("Failed to run game matches", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", req.GameType),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Started game matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_type", req.GameType),
		zap.Int("enqueued", enqueued),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "started",
		"game_type": req.GameType,
		"enqueued":  enqueued,
	})
}

// RetryFailedMatches перезапускает все неудачные матчи турнира
// @Summary Перезапустить неудачные матчи
// @Description Повторно добавляет в очередь все матчи со статусом failed (только для админов)
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} object{status=string,enqueued=int}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/retry-matches [post]
func (h *TournamentHandler) RetryFailedMatches(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	enqueued, err := h.schedulingService.RetryFailedMatches(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to retry failed matches", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Retried failed matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("enqueued", enqueued),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "retried",
		"enqueued": enqueued,
	})
}
