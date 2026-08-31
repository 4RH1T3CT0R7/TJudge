package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/tournament"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TournamentService - контракт сервиса турниров, всё что нужно хендлеру.
type TournamentService interface {
	Create(ctx context.Context, req *tournament.CreateRequest) (*models.Tournament, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error)
	List(ctx context.Context, filter models.TournamentFilter) ([]*models.Tournament, error)
	Join(ctx context.Context, req *tournament.JoinRequest) error
	Start(ctx context.Context, tournamentID uuid.UUID) error
	Complete(ctx context.Context, tournamentID uuid.UUID) error
	Delete(ctx context.Context, tournamentID uuid.UUID) error
	GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error)
	GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error)
	CreateMatch(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID, priority models.MatchPriority) (*models.Match, error)
	GetMatches(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*models.Match, error)
	GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*models.MatchRound, error)
}

// SchedulingService раскладывает пары round-robin и толкает матчи в очередь
// (полный прогон, прогон по одной игре, ретрай упавших).
type SchedulingService interface {
	RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error)
	RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error)
	RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error)
}

// TournamentHandler - HTTP-ручки всего, что связано с турниром.
//
// файл раздулся до бога-обработчика: тут и CRUD турнира, и лидерборды, и запуск
// матчей, и в конце ещё история рейтинга. по-хорошему давно просится распил на
// несколько файлов, но пока так и остаётся - каждый раз откладываю на потом.
type TournamentHandler struct {
	tournamentService TournamentService
	schedulingService SchedulingService
	log               *logger.Logger
}

// NewTournamentHandler собирает хендлер из сервисов турниров и планировщика.
func NewTournamentHandler(tournamentService TournamentService, schedulingService SchedulingService, log *logger.Logger) *TournamentHandler {
	return &TournamentHandler{
		tournamentService: tournamentService,
		schedulingService: schedulingService,
		log:               log,
	}
}

// Create создаёт турнир (доступно только админам).
// @Summary Создать турнир
// @Description Создаёт новый турнир (только для админов)
// @Tags tournaments
// @Accept json
// @Produce json
// @Param request body tournament.CreateRequest true "Данные турнира"
// @Security BearerAuth
// @Success 201 {object} models.Tournament
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

	// id создателя берётся из контекста аутентификаци, если он там есть
	if userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID); ok {
		req.CreatorID = &userID
	}

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

// List отдаёт турниры с фильтрами и постраничкой.
// @Summary Список турниров
// @Description Возвращает список турниров с фильтрацией и пагинацией
// @Tags tournaments
// @Produce json
// @Param status query string false "Фильтр по статусу (pending, active, completed, cancelled)"
// @Param game_type query string false "Фильтр по типу игры"
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} models.Tournament
// @Failure 400 {object} object{error=string}
// @Router /tournaments [get]
func (h *TournamentHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := models.TournamentFilter{}

	// статус приходит строкой из query - прогоняется через whitelist, иначе 400,
	// чтобы мусоный фильтр не улетал в БД
	if status := r.URL.Query().Get("status"); status != "" {
		s := models.TournamentStatus(status)
		switch s {
		case models.TournamentPending, models.TournamentActive, models.TournamentCompleted, models.TournamentCancelled:
			filter.Status = s
		default:
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid status filter, must be one of: pending, active, completed, cancelled"))
			return
		}
	}

	filter.GameType = r.URL.Query().Get("game_type")

	pg := pagination.ParseLimitOffset(r, 50, 0)
	filter.Limit = pg.Limit
	filter.Offset = pg.Offset

	tournaments, err := h.tournamentService.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get tournaments list", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tournaments)
}

// Get возвращает один турнир по id.
// @Summary Получить турнир
// @Description Возвращает турнир по ID
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {object} models.Tournament
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id} [get]
func (h *TournamentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	var req struct {
		ProgramID uuid.UUID `json:"program_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

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
	id, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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
	id, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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

// Delete сносит турнир целиком (только админ).
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
	id, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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

// GetLeaderboard - живой лидерборд турнира (limit из query).
// @Summary Таблица лидеров турнира
// @Description Возвращает таблицу лидеров для турнира
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param limit query int false "Лимит записей" default(100)
// @Success 200 {array} models.LeaderboardEntry
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/leaderboard [get]
func (h *TournamentHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	pg := pagination.ParseLimitOffset(r, 100, 0)

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

// @Summary Создать матч
// @Description Создаёт матч между двумя программами в турнире (только для админов)
// @Tags tournaments
// @Accept json
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param request body object{program1_id=string,program2_id=string,priority=string} true "Данные матча"
// @Security BearerAuth
// @Success 201 {object} models.Match
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments/{id}/matches [post]
func (h *TournamentHandler) CreateMatch(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	var req struct {
		Program1ID uuid.UUID            `json:"program1_id"`
		Program2ID uuid.UUID            `json:"program2_id"`
		Priority   models.MatchPriority `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// приоритет не обязателен: если не прислали - ставится medium
	if req.Priority == "" {
		req.Priority = models.PriorityMedium
	}

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

// CrossGameLeaderboardEntry - строка сводного рейтинга по всем играм турнира.
type CrossGameLeaderboardEntry struct {
	Rank        int            `json:"rank"`
	TeamID      *uuid.UUID     `json:"team_id,omitempty"`
	TeamName    string         `json:"team_name"`
	ProgramName string         `json:"program_name"`
	GameRatings map[string]int `json:"game_ratings"` // ключ - game_id, значение - рейтинг
	TotalRating int            `json:"total_rating"`
	TotalWins   int            `json:"total_wins"`
	TotalLosses int            `json:"total_losses"`
	TotalGames  int            `json:"total_games"`
}

// @Summary Кросс-игровой рейтинг
// @Description Возвращает общий рейтинг по всем играм турнира
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {array} models.CrossGameLeaderboardEntry
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/cross-game-leaderboard [get]
func (h *TournamentHandler) GetCrossGameLeaderboard(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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

// @Summary Матчи турнира
// @Description Возвращает список матчей турнира с пагинацией
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} models.Match
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/matches [get]
func (h *TournamentHandler) GetMatches(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	pg := pagination.ParseLimitOffset(r, 50, 0)

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

// GetMatchesByRounds группирует матчи турнира по раундам round-robin.
// @Summary Матчи по раундам
// @Description Возвращает матчи турнира, сгруппированные по раундам
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {array} models.MatchRound
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/matches/rounds [get]
func (h *TournamentHandler) GetMatchesByRounds(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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

// RunAllMatches ставит в очередь весь пул ожидающих матчей турнира.
//
// сам round-robin (все пары в обе ориентации AB/BA) раскладывает планировщик -
// хендлер только дёргает его и отдаёт число поставленных в очередь матчей.
// операция админская и потенциально тяжёлая: на N участниках это N*(N-1)
// матчей на каждую игру турнира.
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
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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

// RunGameMatches запускает round-robin только для одной игры турнира.
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
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// game_type обязателен - без него планировщику нечего раскладывать
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

// RetryFailedMatches перекидывает упавшие матчи (failed) обратно в очередь.
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
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
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

// --- история рейтинга (переехало из rating_history.go) ---

// RatingHistoryRepository - доступ к истории рейтинга для графиков.
type RatingHistoryRepository interface {
	GetByProgramAndTournament(ctx context.Context, programID, tournamentID uuid.UUID, limit int) ([]*models.RatingHistory, error)
}

// RatingHistoryHandler отдаёт историю рейтинга программы в турнире -
// данные для графика динамики (хронологический порядок).
type RatingHistoryHandler struct {
	repo RatingHistoryRepository
	log  *logger.Logger
}

func NewRatingHistoryHandler(repo RatingHistoryRepository, log *logger.Logger) *RatingHistoryHandler {
	return &RatingHistoryHandler{repo: repo, log: log}
}

// GetProgramRatingHistory возвращает историю рейтинга программы в турнире.
// @Summary История рейтинга программы
// @Description Хронология изменений ELO программы в турнире (для графика)
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param programId path string true "Program ID" format(uuid)
// @Param limit query int false "Максимум последних точек" default(200)
// @Success 200 {array} models.RatingHistory
// @Failure 400 {object} object{error=string}
// @Router /tournaments/{id}/programs/{programId}/rating-history [get]
func (h *RatingHistoryHandler) GetProgramRatingHistory(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}
	programID, ok := parseUUIDParam(w, r, "programId", "program")
	if !ok {
		return
	}

	pg := pagination.ParseLimitOffset(r, 200, 0)

	history, err := h.repo.GetByProgramAndTournament(r.Context(), programID, tournamentID, pg.Limit)
	if err != nil {
		h.log.LogError("Failed to get program rating history", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("program_id", programID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, history)
}
