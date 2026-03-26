package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TournamentGameWithDetails contains tournament-game link info with game details.
type TournamentGameWithDetails struct {
	TournamentID          uuid.UUID `json:"tournament_id"`
	GameID                uuid.UUID `json:"game_id"`
	GameName              string    `json:"game_name"`
	GameDisplayName       string    `json:"game_display_name"`
	IsActive              bool      `json:"is_active"`
	RoundCompleted        bool      `json:"round_completed"`
	RoundCompletedAt      *string   `json:"round_completed_at,omitempty"`
	CurrentRound          int       `json:"current_round"`
	AutoRoundEnabled      bool      `json:"auto_round_enabled"`
	AutoRoundIntervalSecs int       `json:"auto_round_interval_seconds"`
	AutoRoundLastRunAt    *string   `json:"auto_round_last_run_at,omitempty"`
}

// GetTournamentGamesWithStatus returns games with their round status.
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
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
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
			CurrentRound:          d.CurrentRound,
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

// GetActiveGame returns the currently active game for a tournament.
// @Summary Активная игра турнира
// @Description Возвращает текущую активную игру турнира (null если нет)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {object} TournamentGameWithDetails
// @Failure 500 {object} object{error=string}
// @Router /tournaments/{id}/active-game [get]
func (h *GameRoundHandler) GetActiveGame(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
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
		CurrentRound:          activeGame.CurrentRound,
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

// SetActiveGameRequest is the request for setting the active game.
type SetActiveGameRequest struct {
	GameID uuid.UUID `json:"game_id"`
}

// SetActiveGame sets the active game for a tournament.
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
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
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

// GetGameLeaderboard returns the leaderboard for a specific game in a tournament.
// @Summary Рейтинг по игре
// @Description Возвращает таблицу лидеров для конкретной игры в турнире
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Param limit query int false "Лимит записей" default(100)
// @Success 200 {array} domain.LeaderboardEntry
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

// GetGameMatches returns matches for a specific game in a tournament.
// @Summary Матчи по игре
// @Description Возвращает матчи для конкретной игры в турнире с фильтрацией по статусу
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Param status query string false "Фильтр по статусу (pending, running, completed, failed, cancelled)"
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} domain.Match
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

	filter := domain.MatchFilter{
		TournamentID: &tournamentID,
		GameType:     g.Name,
	}

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

// GetGamePrograms returns programs for a specific game in a tournament.
// @Summary Программы по игре
// @Description Возвращает программы для конкретной игры в турнире (только для админов)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {array} domain.Program
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
