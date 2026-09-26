package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/game"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GameService - полный интерфейс domain-сервиса игр.
// Удовлетворяет GameCRUDService, TournamentGameService и GameRoundLookupService.
type GameService interface {
	Create(ctx context.Context, req *game.CreateRequest) (*models.Game, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error)
	GetByName(ctx context.Context, name string) (*models.Game, error)
	List(ctx context.Context, filter models.GameFilter) ([]*models.Game, error)
	Update(ctx context.Context, id uuid.UUID, req *game.UpdateRequest) (*models.Game, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Game, error)
	AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
}

// GameLeaderboardRepository - интерфейс для leaderboard конкретной игры.
type GameLeaderboardRepository interface {
	GetLeaderboardByGameType(ctx context.Context, tournamentID uuid.UUID, gameType string, limit int) ([]*models.LeaderboardEntry, error)
	GetHeadToHead(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*models.HeadToHeadCell, error)
}

type GameMatchRepository interface {
	List(ctx context.Context, filter models.MatchFilter) ([]*models.Match, error)
}

type GameTournamentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error)
}

type GameProgramRepository interface {
	GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*models.Program, error)
	UserProgramLister
}

// GameRoundResetter - ручной сброс раунда игры под общим локом планирования турнира.
type GameRoundResetter interface {
	ResetGameRound(ctx context.Context, tournamentID uuid.UUID, gameType string) (matchesDeleted, participantsReset, ratingHistoryDeleted int64, err error)
}

// TournamentGameStatusRepository - интерфейс управления статусом игр и раундами.
type TournamentGameStatusRepository interface {
	GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*models.TournamentGame, error)
	GetTournamentGamesWithDetails(ctx context.Context, tournamentID uuid.UUID) ([]*models.TournamentGameWithDetails, error)
	MarkRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) error
	SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error
	GetActiveGame(ctx context.Context, tournamentID uuid.UUID) (*models.TournamentGame, error)
	DeactivateAllGames(ctx context.Context, tournamentID uuid.UUID) error
	// авто-раунд
	SetAutoRound(ctx context.Context, tournamentID, gameID uuid.UUID, enabled bool, intervalSecs int) error
	GetTournamentGame(ctx context.Context, tournamentID, gameID uuid.UUID) (*models.TournamentGame, error)
}

// GameHandler - фасад, встраивающий три специализированных sub-handler'а:
//   - GameCRUDHandler: Create/List/Get/Update/Delete для игр
//   - TournamentGameHandler: привязка/отвязка игр к турнирам
//   - GameRoundHandler: leaderboard, матчи, программы, статус, управление раундом
//
// имена HTTP-методов сохранены, поэтому routes.go не требует изменений
type GameHandler struct {
	*GameCRUDHandler
	*TournamentGameHandler
	*GameRoundHandler
}

// NewGameHandler создаёт фасад GameHandler, делегирующий специализированным sub-handler'ам.
func NewGameHandler(
	gameService GameService,
	leaderboardRepo GameLeaderboardRepository,
	matchRepo GameMatchRepository,
	tournamentRepo GameTournamentRepository,
	programRepo GameProgramRepository,
	tournamentGameStatusRepo TournamentGameStatusRepository,
	roundResetter GameRoundResetter,
	notifier events.Notifier,
	uploadDir string,
	log *logger.Logger,
) *GameHandler {
	return &GameHandler{
		GameCRUDHandler:       NewGameCRUDHandler(gameService, log),
		TournamentGameHandler: NewTournamentGameHandler(gameService, tournamentRepo, log),
		GameRoundHandler:      NewGameRoundHandler(gameService, leaderboardRepo, matchRepo, programRepo, tournamentGameStatusRepo, roundResetter, notifier, uploadDir, log),
	}
}

// GameCRUDService - минимальный интерфейс для CRUD-операций над играми.
type GameCRUDService interface {
	Create(ctx context.Context, req *game.CreateRequest) (*models.Game, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error)
	GetByName(ctx context.Context, name string) (*models.Game, error)
	List(ctx context.Context, filter models.GameFilter) ([]*models.Game, error)
	Update(ctx context.Context, id uuid.UUID, req *game.UpdateRequest) (*models.Game, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type GameCRUDHandler struct {
	gameService GameCRUDService
	log         *logger.Logger
}

func NewGameCRUDHandler(gameService GameCRUDService, log *logger.Logger) *GameCRUDHandler {
	return &GameCRUDHandler{
		gameService: gameService,
		log:         log,
	}
}

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

func (h *GameCRUDHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := models.GameFilter{}
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

func (h *GameCRUDHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "game")
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

func (h *GameCRUDHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "game")
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

func (h *GameCRUDHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "game")
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

// TournamentGameService - интерфейс для связывания игр с турнирами.
type TournamentGameService interface {
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Game, error)
	AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
}

// TournamentGameOwnerRepo проверяет владельца турнира для авторизации.
type TournamentGameOwnerRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error)
}

type TournamentGameHandler struct {
	gameService    TournamentGameService
	tournamentRepo TournamentGameOwnerRepo
	log            *logger.Logger
}

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

func (h *TournamentGameHandler) GetTournamentGames(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
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

// AddGameToTournamentRequest - тело запроса на добавление игры в турнир.
type AddGameToTournamentRequest struct {
	GameID uuid.UUID `json:"game_id"`
}

// AddGameToTournament добавляет игру в турнир.
func (h *TournamentGameHandler) AddGameToTournament(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}
	userRole, _ := r.Context().Value(middleware.RoleKey).(models.Role)

	isAdmin := userRole == models.RoleAdmin
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

func (h *TournamentGameHandler) RemoveGameFromTournament(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	gameID, ok := parseUUIDParam(w, r, "gameId", "game")
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
