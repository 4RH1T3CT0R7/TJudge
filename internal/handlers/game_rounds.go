package handlers

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// здесь собрана вся возня с раундами игр: статус, активная игра, лидерборды,
// личные встречи, сброс и авто-раунд. файл заметно толще соседей, но так весь
// жизненый цикл раунда лежит в одном месте и его проще читать целиком

// GameRoundLookupService предоставляет lookup игр для round-related handler'ов.
type GameRoundLookupService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error)
}

// GameRoundHandler обрабатывает leaderboard, матчи, программы, статус и управление раундами игр.
type GameRoundHandler struct {
	gameService              GameRoundLookupService
	leaderboardRepo          GameLeaderboardRepository
	matchRepo                GameMatchRepository
	programRepo              GameProgramRepository
	tournamentGameStatusRepo TournamentGameStatusRepository
	notifier                 events.Notifier
	uploadDir                string
	log                      *logger.Logger
}

// NewGameRoundHandler создаёт handler для операций round/status игр.
func NewGameRoundHandler(
	gameService GameRoundLookupService,
	leaderboardRepo GameLeaderboardRepository,
	matchRepo GameMatchRepository,
	programRepo GameProgramRepository,
	tournamentGameStatusRepo TournamentGameStatusRepository,
	notifier events.Notifier,
	uploadDir string,
	log *logger.Logger,
) *GameRoundHandler {
	return &GameRoundHandler{
		gameService:              gameService,
		leaderboardRepo:          leaderboardRepo,
		matchRepo:                matchRepo,
		programRepo:              programRepo,
		tournamentGameStatusRepo: tournamentGameStatusRepo,
		notifier:                 notifier,
		uploadDir:                uploadDir,
		log:                      log,
	}
}

// parseTournamentGameIDs - хелпер для парсинга ID турнира и игры из URL-параметров.
func (h *GameRoundHandler) parseTournamentGameIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}

	gameID, ok := parseUUIDParam(w, r, "gameId", "game")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}

	return tournamentID, gameID, true
}

// TournamentGameWithDetails содержит данные связи турнир-игра с детализацией игры.
type TournamentGameWithDetails struct {
	TournamentID          uuid.UUID `json:"tournament_id"`
	GameID                uuid.UUID `json:"game_id"`
	GameName              string    `json:"game_name"`
	GameDisplayName       string    `json:"game_display_name"`
	IsActive              bool      `json:"is_active"`
	RoundCompleted        bool      `json:"round_completed"`
	RoundCompletedAt      *string   `json:"round_completed_at,omitempty"`
	AutoRoundEnabled      bool      `json:"auto_round_enabled"`
	AutoRoundIntervalSecs int       `json:"auto_round_interval_seconds"`
	AutoRoundLastRunAt    *string   `json:"auto_round_last_run_at,omitempty"`
}

// GetTournamentGamesWithStatus возвращает игры с их round-статусом.
// @Summary Статус игр турнира
// @Description Возвращает игры турнира с информацией о раундах и авто-раунде
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {array} TournamentGameWithDetails
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tournaments/{id}/games/status [get]
func (h *GameRoundHandler) GetTournamentGamesWithStatus(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	details, err := h.tournamentGameStatusRepo.GetTournamentGamesWithDetails(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament games status", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	result := make([]TournamentGameWithDetails, 0, len(details))
	for _, d := range details {
		item := TournamentGameWithDetails{
			TournamentID:          d.TournamentID,
			GameID:                d.GameID,
			GameName:              d.GameName,
			GameDisplayName:       d.GameDisplayName,
			IsActive:              d.IsActive,
			RoundCompleted:        d.RoundCompleted,
			AutoRoundEnabled:      d.AutoRoundEnabled,
			AutoRoundIntervalSecs: d.AutoRoundIntervalSecs,
		}
		if d.RoundCompletedAt != nil {
			formatted := d.RoundCompletedAt.Format("2006-01-02T15:04:05Z07:00")
			item.RoundCompletedAt = &formatted
		}
		if d.AutoRoundLastRunAt != nil {
			formatted := d.AutoRoundLastRunAt.Format("2006-01-02T15:04:05Z07:00")
			item.AutoRoundLastRunAt = &formatted
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, result)
}

// GetActiveGame возвращает текущую активную игру турнира.
// @Summary Активная игра турнира
// @Description Возвращает текущую активную игру турнира (null если нет)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {object} TournamentGameWithDetails
// @Failure 500 {object} object{error=string}
// @Router /tournaments/{id}/active-game [get]
func (h *GameRoundHandler) GetActiveGame(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	activeGame, err := h.tournamentGameStatusRepo.GetActiveGame(r.Context(), tournamentID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeJSON(w, http.StatusOK, nil)
			return
		}
		h.log.LogError("Failed to get active game", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	g, err := h.gameService.GetByID(r.Context(), activeGame.GameID)
	if err != nil {
		h.log.LogError("Failed to get game details", err,
			zap.String("game_id", activeGame.GameID.String()),
		)
		writeError(w, err)
		return
	}

	result := TournamentGameWithDetails{
		TournamentID:          activeGame.TournamentID,
		GameID:                activeGame.GameID,
		GameName:              g.Name,
		GameDisplayName:       g.DisplayName,
		IsActive:              activeGame.IsActive,
		RoundCompleted:        activeGame.RoundCompleted,
		AutoRoundEnabled:      activeGame.AutoRoundEnabled,
		AutoRoundIntervalSecs: activeGame.AutoRoundIntervalSecs,
	}
	if activeGame.RoundCompletedAt != nil {
		formatted := activeGame.RoundCompletedAt.Format("2006-01-02T15:04:05Z07:00")
		result.RoundCompletedAt = &formatted
	}
	if activeGame.AutoRoundLastRunAt != nil {
		formatted := activeGame.AutoRoundLastRunAt.Format("2006-01-02T15:04:05Z07:00")
		result.AutoRoundLastRunAt = &formatted
	}

	writeJSON(w, http.StatusOK, result)
}

// SetActiveGameRequest - запрос на установку активной игры.
type SetActiveGameRequest struct {
	GameID uuid.UUID `json:"game_id"`
}

// SetActiveGame устанавливает активную игру турнира.
// @Summary Установить активную игру
// @Description Устанавливает активную игру для турнира (только для админов)
// @Tags games
// @Accept json
// @Param id path string true "Tournament ID" format(uuid)
// @Param request body SetActiveGameRequest true "ID игры"
// @Security BearerAuth
// @Success 204 "Активная игра установлена"
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments/{id}/active-game [post]
func (h *GameRoundHandler) SetActiveGame(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	var req SetActiveGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("game_id is required"))
		return
	}

	if err := h.tournamentGameStatusRepo.SetActiveGame(r.Context(), tournamentID, req.GameID); err != nil {
		h.log.LogError("Failed to set active game", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", req.GameID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Active game set",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", req.GameID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetGameLeaderboard возвращает leaderboard для конкретной игры в турнире.
// @Summary Рейтинг по игре
// @Description Возвращает таблицу лидеров для конкретной игры в турнире
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Param limit query int false "Лимит записей" default(100)
// @Success 200 {array} models.LeaderboardEntry
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/leaderboard [get]
func (h *GameRoundHandler) GetGameLeaderboard(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.leaderboardRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("leaderboard repository not configured"))
		return
	}

	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game", err)
		writeError(w, err)
		return
	}

	pg := pagination.ParseLimitOffset(r, 100, 0)

	leaderboard, err := h.leaderboardRepo.GetLeaderboardByGameType(r.Context(), tournamentID, g.Name, pg.Limit)
	if err != nil {
		h.log.LogError("Failed to get game leaderboard", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
			zap.String("game_type", g.Name),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, leaderboard)
}

// GetHeadToHead возвращает матрицу личных встреч команд в игре турнира.
//
// репозиторий уже сливает обе ориентации матча (AB и BA) в одну ячейку, так что
// на выходе плоский список ячеек, а не полноценная матрица - фронт сам раскладывает
// её по строкам и столбцам, здесь только отдаётся агрегат как есть
// @Summary Head-to-head матрица по игре
// @Description Агрегат личных встреч всех пар команд (обе ориентации матчей слиты)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Success 200 {array} models.HeadToHeadCell
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/head-to-head [get]
func (h *GameRoundHandler) GetHeadToHead(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.leaderboardRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("leaderboard repository not configured"))
		return
	}

	// матрица считается по game_type (системному имени), а не по gameID,
	// поэтому сначала резолвится игра и достаётся её Name
	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game", err)
		writeError(w, err)
		return
	}

	cells, err := h.leaderboardRepo.GetHeadToHead(r.Context(), tournamentID, g.Name)
	if err != nil {
		h.log.LogError("Failed to get head-to-head", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", g.Name),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, cells)
}

// GetGameMatches возвращает матчи для конкретной игры в турнире.
// @Summary Матчи по игре
// @Description Возвращает матчи для конкретной игры в турнире с фильтрацией по статусу
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Param status query string false "Фильтр по статусу (pending, running, completed, failed, cancelled)"
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} models.Match
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/matches [get]
func (h *GameRoundHandler) GetGameMatches(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.matchRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("match repository not configured"))
		return
	}

	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game", err)
		writeError(w, err)
		return
	}

	filter := models.MatchFilter{
		TournamentID: &tournamentID,
		GameType:     g.Name,
	}

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

	pg := pagination.ParseLimitOffset(r, 50, 0)
	filter.Limit = pg.Limit
	filter.Offset = pg.Offset

	matches, err := h.matchRepo.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get game matches", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
			zap.String("game_type", g.Name),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, matches)
}

// GetGamePrograms возвращает программы для конкретной игры в турнире.
// @Summary Программы по игре
// @Description Возвращает программы для конкретной игры в турнире (только для админов)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {array} models.Program
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/programs [get]
func (h *GameRoundHandler) GetGamePrograms(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.programRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("program repository not configured"))
		return
	}

	programs, err := h.programRepo.GetByTournamentAndGame(r.Context(), tournamentID, gameID)
	if err != nil {
		h.log.LogError("Failed to get game programs", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, programs)
}

// MarkGameRoundCompleted помечает раунд игры как завершённый.
// @Summary Завершить раунд игры
// @Description Отмечает текущий раунд игры как завершённый (только для админов)
// @Tags games
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Раунд завершён"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/complete-round [post]
func (h *GameRoundHandler) MarkGameRoundCompleted(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	if err := h.tournamentGameStatusRepo.MarkRoundCompleted(r.Context(), tournamentID, gameID); err != nil {
		h.log.LogError("Failed to mark round completed", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Game round marked as completed",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", gameID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// ResetGameRoundResponse - ответ на сброс раунда.
type ResetGameRoundResponse struct {
	MatchesDeleted     int64 `json:"matches_deleted"`
	ParticipantsReset  int64 `json:"participants_reset"`
	RatingHistoryReset int64 `json:"rating_history_reset"`
}

// ResetGameRound полностью сбрасывает раунд игры: удаляет матчи, обнуляет рейтинги и статистику.
//
// это самая разрушительная операция во всём хендлере, поэтому всё делается аккуратно:
//  1. игра резолвится, чтобы получить её game_type (сброс работает по типу, не по id)
//  2. одной транзакцией в репозитории сносятся матчи, обнуляются участники и история рейтинга
//  3. логируется сколько чего снесли (пригодится при разборе жалоб «куда делись очки»)
//  4. уходит событие GameRoundReset, чтобы подписчики сбросили кэши и лидерборды
//
// возврата назад нет - удалённые матчи не восстановить, так что ручка только для админов
// @Summary Сбросить раунд игры
// @Description Полностью сбрасывает раунд: удаляет матчи, обнуляет рейтинги и статистику (только для админов)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} ResetGameRoundResponse
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/reset-round [post]
func (h *GameRoundHandler) ResetGameRound(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	// сброс идёт по game_type, поэтому сперва достаётся сама игра
	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game details", err,
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	// вся зачистка происходит в одной транзакции внутри репозитория,
	// наружу отдаются только счётчики удалённого
	matchesDeleted, participantsReset, ratingHistoryDeleted, err := h.tournamentGameStatusRepo.ResetGameRoundFull(r.Context(), tournamentID, gameID, g.Name)
	if err != nil {
		h.log.LogError("Failed to reset game round", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Game round reset completed",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", gameID.String()),
		zap.Int64("matches_deleted", matchesDeleted),
		zap.Int64("participants_reset", participantsReset),
		zap.Int64("rating_history_reset", ratingHistoryDeleted),
	)

	// событие поднимает подписчиков (кэш, лидерборды), чтобы они не показывали
	// уже удалённые данные - сам сброс уже закоммичен, так что это best-effort
	h.notifier.GameRoundReset(r.Context(), events.GameRoundReset{
		Version:      1,
		TournamentID: tournamentID,
		GameID:       gameID,
	})

	writeJSON(w, http.StatusOK, ResetGameRoundResponse{
		MatchesDeleted:     matchesDeleted,
		ParticipantsReset:  participantsReset,
		RatingHistoryReset: ratingHistoryDeleted,
	})
}

// SetAutoRound включает или выключает авто-раунд для игры в турнире.
// @Summary Настроить авто-раунд
// @Description Включает или выключает автоматический запуск раундов для игры (только для админов)
// @Tags games
// @Accept json
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Param request body object{enabled=bool,interval_seconds=int} true "Настройки авто-раунда"
// @Security BearerAuth
// @Success 200 {object} object{enabled=bool,interval_seconds=int}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/auto-round [post]
func (h *GameRoundHandler) SetAutoRound(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	var req struct {
		Enabled         bool `json:"enabled"`
		IntervalSeconds int  `json:"interval_seconds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid request body"))
		return
	}

	if req.Enabled && (req.IntervalSeconds < 10 || req.IntervalSeconds > 3600) {
		writeError(w, errors.ErrValidation.WithMessage("interval_seconds must be between 10 and 3600"))
		return
	}

	// если выключается, ставится дефолтный интервал
	if !req.Enabled && req.IntervalSeconds == 0 {
		req.IntervalSeconds = 60
	}

	if err := h.tournamentGameStatusRepo.SetAutoRound(r.Context(), tournamentID, gameID, req.Enabled, req.IntervalSeconds); err != nil {
		h.log.LogError("Failed to set auto-round", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}

	h.log.Info("Auto-round updated",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", gameID.String()),
		zap.String("status", status),
		zap.Int("interval", req.IntervalSeconds),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":          req.Enabled,
		"interval_seconds": req.IntervalSeconds,
	})
}

// GetAutoRound возвращает статус авто-раунда для игры в турнире.
// @Summary Статус авто-раунда
// @Description Возвращает текущие настройки авто-раунда для игры (только для админов)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} object{enabled=bool,interval_seconds=int,last_run_at=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/auto-round [get]
func (h *GameRoundHandler) GetAutoRound(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	tg, err := h.tournamentGameStatusRepo.GetTournamentGame(r.Context(), tournamentID, gameID)
	if err != nil {
		h.log.LogError("Failed to get auto-round status", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":          tg.AutoRoundEnabled,
		"interval_seconds": tg.AutoRoundIntervalSecs,
		"last_run_at":      tg.AutoRoundLastRunAt,
	})
}

// DownloadAllPrograms стримит ZIP-архив со всеми программами турнира.
// @Summary Скачать все программы
// @Description Скачивает ZIP-архив со всеми программами турнира (только для админов)
// @Tags games
// @Produce application/zip
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {file} binary "ZIP-архив программ"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/programs/download-zip [get]
func (h *GameRoundHandler) DownloadAllPrograms(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}
	if h.programRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("program repository not configured"))
		return
	}

	// берутся все игры этого турнира
	games, err := h.tournamentGameStatusRepo.GetTournamentGames(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament games", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	// резолвится upload-директория для валидации путей (EvalSymlinks раскрывает симлинки)
	absUploadDir, err := filepath.EvalSymlinks(h.uploadDir)
	if err != nil {
		h.log.Error("Failed to resolve upload dir", zap.Error(err))
		writeError(w, errors.ErrInternal.WithMessage("invalid upload directory"))
		return
	}

	// заголовки ответа выставляются до записи тела
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"programs_%s.zip\"", tournamentID.String()[:8]))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	filesAdded := 0

	for _, tg := range games {
		// берутся данные игры для display-имени
		game, err := h.gameService.GetByID(r.Context(), tg.GameID)
		if err != nil {
			h.log.Error("Failed to get game details, skipping",
				zap.String("game_id", tg.GameID.String()),
				zap.Error(err),
			)
			continue
		}

		gameDirName := sanitizeZipPath(game.DisplayName)
		if gameDirName == "" {
			gameDirName = game.Name
		}

		// берутся последние программы для этой игры
		programs, err := h.programRepo.GetByTournamentAndGame(r.Context(), tournamentID, tg.GameID)
		if err != nil {
			h.log.Error("Failed to get programs for game, skipping",
				zap.String("game_id", tg.GameID.String()),
				zap.Error(err),
			)
			continue
		}

		for _, prog := range programs {
			if prog.FilePath == nil || *prog.FilePath == "" {
				continue
			}

			filePath := *prog.FilePath

			// проверка, что путь внутри upload-директории (EvalSymlinks раскрывает симлинки)
			absFilePath, err := filepath.EvalSymlinks(filePath)
			if err != nil || !strings.HasPrefix(absFilePath, absUploadDir+string(os.PathSeparator)) {
				h.log.Error("Program file path outside upload dir, skipping",
					zap.String("program_id", prog.ID.String()),
					zap.String("path", filePath),
				)
				continue
			}

			// проверка, что файл существует
			// #nosec G703 G304 -- filePath провалидирован через EvalSymlinks +
			// HasPrefix(absUploadDir) чуть выше; path-traversal невозможен.
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				h.log.Error("Program file not found on disk, skipping",
					zap.String("program_id", prog.ID.String()),
					zap.String("path", filePath),
				)
				continue
			}

			// формируется путь ZIP-записи: game_name/program_name_v{version}.ext
			ext := filepath.Ext(filePath)
			entryName := fmt.Sprintf("%s/%s_v%d%s", gameDirName, sanitizeZipPath(prog.Name), prog.Version, ext)

			// #nosec G703 G304 -- filePath провалидирован выше (EvalSymlinks +
			// HasPrefix(absUploadDir)).
			f, err := os.Open(filePath)
			if err != nil {
				h.log.Error("Failed to open program file, skipping",
					zap.String("program_id", prog.ID.String()),
					zap.Error(err),
				)
				continue
			}

			zf, err := zipWriter.Create(entryName)
			if err != nil {
				f.Close()
				h.log.Error("Failed to create ZIP entry", zap.Error(err))
				continue
			}

			if _, err := io.Copy(zf, f); err != nil {
				f.Close()
				h.log.Error("Failed to write program to ZIP", zap.Error(err))
				continue
			}
			f.Close()
			filesAdded++
		}
	}

	h.log.Info("Programs archive created",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("files_added", filesAdded),
	)
}

// DeactivateAllGames деактивирует все игры в турнире.
// @Summary Деактивировать все игры
// @Description Деактивирует все игры в турнире (только для админов)
// @Tags games
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Все игры деактивированы"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/deactivate-all [post]
func (h *GameRoundHandler) DeactivateAllGames(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	if err := h.tournamentGameStatusRepo.DeactivateAllGames(r.Context(), tournamentID); err != nil {
		h.log.LogError("Failed to deactivate all games", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("All games deactivated",
		zap.String("tournament_id", tournamentID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// sanitizeZipPath очищает имя для использования в путях ZIP-записей.
func sanitizeZipPath(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	// удаляются последовательности ".." path-traversal
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		name = LangUnknown
	}
	return name
}
