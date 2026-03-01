package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
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
// GET /api/v1/tournaments/{id}/games
func (h *TournamentGameHandler) GetTournamentGames(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
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
// POST /api/v1/tournaments/{id}/games
func (h *TournamentGameHandler) AddGameToTournament(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
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
// DELETE /api/v1/tournaments/{id}/games/{gameId}
func (h *TournamentGameHandler) RemoveGameFromTournament(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game ID"))
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
