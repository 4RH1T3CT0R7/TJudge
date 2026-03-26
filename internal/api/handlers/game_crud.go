package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/game"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GameCRUDService is the minimal interface needed for CRUD operations on games.
type GameCRUDService interface {
	Create(ctx context.Context, req *game.CreateRequest) (*domain.Game, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
	GetByName(ctx context.Context, name string) (*domain.Game, error)
	List(ctx context.Context, filter domain.GameFilter) ([]*domain.Game, error)
	Update(ctx context.Context, id uuid.UUID, req *game.UpdateRequest) (*domain.Game, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// GameCRUDHandler handles Create/List/Get/Update/Delete for games.
type GameCRUDHandler struct {
	gameService GameCRUDService
	log         *logger.Logger
}

// NewGameCRUDHandler creates a new CRUD handler for games.
func NewGameCRUDHandler(gameService GameCRUDService, log *logger.Logger) *GameCRUDHandler {
	return &GameCRUDHandler{
		gameService: gameService,
		log:         log,
	}
}

// Create creates a new game.
// @Summary Создать игру
// @Description Создаёт новую игру (только для админов)
// @Tags games
// @Accept json
// @Produce json
// @Param request body game.CreateRequest true "Данные игры"
// @Security BearerAuth
// @Success 201 {object} domain.Game
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /games [post]
func (h *GameCRUDHandler) Create(w http.ResponseWriter, r *http.Request) {
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

// List returns a list of games.
// @Summary Список игр
// @Description Возвращает список доступных игр с фильтрацией и пагинацией
// @Tags games
// @Produce json
// @Param name query string false "Фильтр по имени"
// @Param limit query int false "Лимит записей" default(50)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} domain.Game
// @Router /games [get]
func (h *GameCRUDHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.GameFilter{}
	filter.Name = r.URL.Query().Get("name")

	pg := pagination.ParseLimitOffset(r, 50, 0)
	filter.Limit = pg.Limit
	filter.Offset = pg.Offset

	games, err := h.gameService.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to list games", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, games)
}

// Get returns a game by ID.
// @Summary Получить игру
// @Description Возвращает игру по ID
// @Tags games
// @Produce json
// @Param id path string true "Game ID" format(uuid)
// @Success 200 {object} domain.Game
// @Failure 404 {object} object{error=string}
// @Router /games/{id} [get]
func (h *GameCRUDHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseUUIDParam(w, r, "id", "game")
	if !ok {
		return
	}

	g, err := h.gameService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, g)
}

// GetByName returns a game by name.
// @Summary Получить игру по имени
// @Description Возвращает игру по системному имени
// @Tags games
// @Produce json
// @Param name path string true "Имя игры"
// @Success 200 {object} domain.Game
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /games/name/{name} [get]
func (h *GameCRUDHandler) GetByName(w http.ResponseWriter, r *http.Request) {
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

// Update updates a game.
// @Summary Обновить игру
// @Description Обновляет параметры игры (только для админов)
// @Tags games
// @Accept json
// @Produce json
// @Param id path string true "Game ID" format(uuid)
// @Param request body game.UpdateRequest true "Данные для обновления"
// @Security BearerAuth
// @Success 200 {object} domain.Game
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /games/{id} [put]
func (h *GameCRUDHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseUUIDParam(w, r, "id", "game")
	if !ok {
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

// Delete deletes a game.
// @Summary Удалить игру
// @Description Удаляет игру по ID (только для админов)
// @Tags games
// @Param id path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Игра удалена"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /games/{id} [delete]
func (h *GameCRUDHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseUUIDParam(w, r, "id", "game")
	if !ok {
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
