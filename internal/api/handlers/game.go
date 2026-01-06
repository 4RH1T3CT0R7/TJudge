package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/game"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GameService интерфейс для game service
type GameService interface {
	Create(ctx context.Context, req *game.CreateRequest) (*domain.Game, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
	GetByName(ctx context.Context, name string) (*domain.Game, error)
	List(ctx context.Context, filter domain.GameFilter) ([]*domain.Game, error)
	Update(ctx context.Context, id uuid.UUID, req *game.UpdateRequest) (*domain.Game, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Game, error)
	AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
}

// GameHandler обрабатывает запросы игр
type GameHandler struct {
	gameService GameService
	log         *logger.Logger
}

// NewGameHandler создаёт новый game handler
func NewGameHandler(gameService GameService, log *logger.Logger) *GameHandler {
	return &GameHandler{
		gameService: gameService,
		log:         log,
	}
}

// Create создаёт новую игру
// POST /api/v1/games
func (h *GameHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req game.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	g, err := h.gameService.Create(r.Context(), &req)
	if err != nil {
		h.log.LogError("Failed to create game", err)
		writeError(w, err)
		return
	}

	h.log.Info("Game created",
		zap.String("game_id", g.ID.String()),
		zap.String("name", g.Name),
	)

	writeJSON(w, http.StatusCreated, g)
}

// List получает список игр
// GET /api/v1/games
func (h *GameHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.GameFilter{}

	// Name filter (search)
	filter.Name = r.URL.Query().Get("name")

	// Pagination
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
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

	games, err := h.gameService.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to list games", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, games)
}

// Get получает игру по ID
// GET /api/v1/games/{id}
func (h *GameHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game ID"))
		return
	}

	g, err := h.gameService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, g)
}

// GetByName получает игру по имени
// GET /api/v1/games/name/{name}
func (h *GameHandler) GetByName(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("game name required"))
		return
	}

	g, err := h.gameService.GetByName(r.Context(), name)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, g)
}

// Update обновляет игру
// PUT /api/v1/games/{id}
func (h *GameHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game ID"))
		return
	}

	var req game.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	g, err := h.gameService.Update(r.Context(), id, &req)
	if err != nil {
		h.log.LogError("Failed to update game", err)
		writeError(w, err)
		return
	}

	h.log.Info("Game updated", zap.String("game_id", g.ID.String()))

	writeJSON(w, http.StatusOK, g)
}

// Delete удаляет игру
// DELETE /api/v1/games/{id}
func (h *GameHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game ID"))
		return
	}

	if err := h.gameService.Delete(r.Context(), id); err != nil {
		h.log.LogError("Failed to delete game", err)
		writeError(w, err)
		return
	}

	h.log.Info("Game deleted", zap.String("game_id", id.String()))

	w.WriteHeader(http.StatusNoContent)
}

// GetTournamentGames получает игры турнира
// GET /api/v1/tournaments/{id}/games
func (h *GameHandler) GetTournamentGames(w http.ResponseWriter, r *http.Request) {
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

// AddGameToTournamentRequest запрос на добавление игры в турнир
type AddGameToTournamentRequest struct {
	GameID uuid.UUID `json:"game_id"`
}

// AddGameToTournament добавляет игру в турнир
// POST /api/v1/tournaments/{id}/games
func (h *GameHandler) AddGameToTournament(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
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
	)

	w.WriteHeader(http.StatusNoContent)
}

// RemoveGameFromTournament удаляет игру из турнира
// DELETE /api/v1/tournaments/{id}/games/{gameId}
func (h *GameHandler) RemoveGameFromTournament(w http.ResponseWriter, r *http.Request) {
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
