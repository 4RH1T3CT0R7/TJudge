package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TournamentGameService is the interface for linking games to tournaments.
type TournamentGameService interface {
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Game, error)
	AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
}

// TournamentGameOwnerRepo checks tournament ownership for authorization.
type TournamentGameOwnerRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
}

// TournamentGameHandler handles linking/unlinking games with tournaments.
type TournamentGameHandler struct {
	gameService    TournamentGameService
	tournamentRepo TournamentGameOwnerRepo
	log            *logger.Logger
}

// NewTournamentGameHandler creates a new handler for tournament-game operations.
func NewTournamentGameHandler(
	gameService TournamentGameService,
	tournamentRepo TournamentGameOwnerRepo,
	log *logger.Logger,
) *TournamentGameHandler {
	return &TournamentGameHandler{
		gameService:    gameService,
		tournamentRepo: tournamentRepo,
		log:            log,
	}
}

// GetTournamentGames returns games for a tournament.
// @Summary Игры турнира
// @Description Возвращает список игр, привязанных к турниру
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {array} domain.Game
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games [get]
func (h *TournamentGameHandler) GetTournamentGames(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	games, err := h.gameService.GetByTournamentID(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament games", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, games)
}

// AddGameToTournamentRequest is the request body for adding a game to a tournament.
type AddGameToTournamentRequest struct {
	GameID uuid.UUID `json:"game_id"`
}

// AddGameToTournament adds a game to a tournament.
// @Summary Добавить игру в турнир
// @Description Привязывает игру к турниру (админ или создатель турнира)
// @Tags games
// @Accept json
// @Param id path string true "Tournament ID" format(uuid)
// @Param request body AddGameToTournamentRequest true "ID игры"
// @Security BearerAuth
// @Success 204 "Игра добавлена"
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments/{id}/games [post]
func (h *TournamentGameHandler) AddGameToTournament(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}
	userRole, _ := r.Context().Value(middleware.RoleKey).(domain.Role)

	isAdmin := userRole == domain.RoleAdmin
	isCreator := false

	if !isAdmin && h.tournamentRepo != nil {
		tournament, err := h.tournamentRepo.GetByID(r.Context(), tournamentID)
		if err != nil {
			h.log.LogError("Failed to get tournament", err)
			writeError(w, err)
			return
		}
		if tournament.CreatorID != nil && *tournament.CreatorID == userID {
			isCreator = true
		}
	}

	if !isAdmin && !isCreator {
		writeError(w, errors.ErrForbidden.WithMessage("only admins or tournament creator can add games"))
		return
	}

	var req AddGameToTournamentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	if err := h.gameService.AddToTournament(r.Context(), tournamentID, req.GameID); err != nil {
		h.log.LogError("Failed to add game to tournament", err)
		writeError(w, err)
		return
	}

	h.log.Info("Game added to tournament",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", req.GameID.String()),
		zap.String("added_by", userID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// RemoveGameFromTournament removes a game from a tournament.
// @Summary Удалить игру из турнира
// @Description Отвязывает игру от турнира (только для админов)
// @Tags games
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Игра удалена из турнира"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId} [delete]
func (h *TournamentGameHandler) RemoveGameFromTournament(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	gameID, ok := httputil.ParseUUIDParam(w, r, "gameId", "game")
	if !ok {
		return
	}

	if err := h.gameService.RemoveFromTournament(r.Context(), tournamentID, gameID); err != nil {
		h.log.LogError("Failed to remove game from tournament", err)
		writeError(w, err)
		return
	}

	h.log.Info("Game removed from tournament",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", gameID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}
