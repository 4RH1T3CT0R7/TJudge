package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
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

// GameLeaderboardRepository интерфейс для получения рейтинга по игре
type GameLeaderboardRepository interface {
	GetLeaderboardByGameType(ctx context.Context, tournamentID uuid.UUID, gameType string, limit int) ([]*domain.LeaderboardEntry, error)
}

// GameMatchRepository интерфейс для получения матчей по игре
type GameMatchRepository interface {
	List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error)
}

// GameTournamentRepository интерфейс для проверки владельца турнира
type GameTournamentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
}

// GameProgramRepository интерфейс для получения программ по турниру и игре
type GameProgramRepository interface {
	GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error)
}

// TournamentGameStatusRepository интерфейс для получения статуса игр в турнире
type TournamentGameStatusRepository interface {
	GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGame, error)
	MarkRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) error
}

// GameHandler обрабатывает запросы игр
type GameHandler struct {
	gameService              GameService
	leaderboardRepo          GameLeaderboardRepository
	matchRepo                GameMatchRepository
	tournamentRepo           GameTournamentRepository
	programRepo              GameProgramRepository
	tournamentGameStatusRepo TournamentGameStatusRepository
	log                      *logger.Logger
}

// NewGameHandler создаёт новый game handler
func NewGameHandler(gameService GameService, log *logger.Logger) *GameHandler {
	return &GameHandler{
		gameService: gameService,
		log:         log,
	}
}

// NewGameHandlerWithRepos создаёт game handler с репозиториями для расширенной функциональности
func NewGameHandlerWithRepos(
	gameService GameService,
	leaderboardRepo GameLeaderboardRepository,
	matchRepo GameMatchRepository,
	tournamentRepo GameTournamentRepository,
	log *logger.Logger,
) *GameHandler {
	return &GameHandler{
		gameService:     gameService,
		leaderboardRepo: leaderboardRepo,
		matchRepo:       matchRepo,
		tournamentRepo:  tournamentRepo,
		log:             log,
	}
}

// SetProgramRepo устанавливает репозиторий программ
func (h *GameHandler) SetProgramRepo(programRepo GameProgramRepository) {
	h.programRepo = programRepo
}

// SetTournamentGameStatusRepo устанавливает репозиторий для работы со статусом игр турнира
func (h *GameHandler) SetTournamentGameStatusRepo(repo TournamentGameStatusRepository) {
	h.tournamentGameStatusRepo = repo
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
// Доступно админам или создателю турнира
func (h *GameHandler) AddGameToTournament(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Получаем информацию о пользователе из контекста
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}
	userRole, _ := r.Context().Value(middleware.RoleKey).(string)

	// Проверяем права: админ или создатель турнира
	isAdmin := userRole == "admin"
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

// GetGameLeaderboard получает рейтинг по конкретной игре в турнире
// GET /api/v1/tournaments/{id}/games/{gameId}/leaderboard
func (h *GameHandler) GetGameLeaderboard(w http.ResponseWriter, r *http.Request) {
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

	// Проверяем наличие репозитория
	if h.leaderboardRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("leaderboard repository not configured"))
		return
	}

	// Получаем игру для её имени (game_type)
	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game", err)
		writeError(w, err)
		return
	}

	// Получаем limit из query параметров
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Получаем рейтинг по игре
	leaderboard, err := h.leaderboardRepo.GetLeaderboardByGameType(r.Context(), tournamentID, g.Name, limit)
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

// GetGameMatches получает матчи по конкретной игре в турнире
// GET /api/v1/tournaments/{id}/games/{gameId}/matches
func (h *GameHandler) GetGameMatches(w http.ResponseWriter, r *http.Request) {
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

	// Проверяем наличие репозитория
	if h.matchRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("match repository not configured"))
		return
	}

	// Получаем игру для её имени (game_type)
	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game", err)
		writeError(w, err)
		return
	}

	// Получаем параметры фильтрации
	filter := domain.MatchFilter{
		TournamentID: &tournamentID,
		GameType:     g.Name,
	}

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = domain.MatchStatus(status)
	}

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

	// Получаем матчи
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

// GetGamePrograms получает программы для конкретной игры в турнире (только для админов)
// GET /api/v1/tournaments/:id/games/:gameId/programs
func (h *GameHandler) GetGamePrograms(w http.ResponseWriter, r *http.Request) {
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

	// Проверяем наличие репозитория
	if h.programRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("program repository not configured"))
		return
	}

	// Получаем программы
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

// TournamentGameWithDetails содержит информацию о связи турнира с игрой и данные об игре
type TournamentGameWithDetails struct {
	TournamentID     uuid.UUID `json:"tournament_id"`
	GameID           uuid.UUID `json:"game_id"`
	GameName         string    `json:"game_name"`
	GameDisplayName  string    `json:"game_display_name"`
	RoundCompleted   bool      `json:"round_completed"`
	RoundCompletedAt *string   `json:"round_completed_at,omitempty"`
	CurrentRound     int       `json:"current_round"`
}

// GetTournamentGamesWithStatus получает игры турнира с их статусом раундов
// GET /api/v1/tournaments/{id}/games/status
func (h *GameHandler) GetTournamentGamesWithStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Проверяем наличие репозитория
	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	// Получаем связи турнира с играми
	tournamentGames, err := h.tournamentGameStatusRepo.GetTournamentGames(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament games status", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	// Обогащаем данными об играх
	result := make([]TournamentGameWithDetails, 0, len(tournamentGames))
	for _, tg := range tournamentGames {
		g, err := h.gameService.GetByID(r.Context(), tg.GameID)
		if err != nil {
			h.log.LogError("Failed to get game details", err,
				zap.String("game_id", tg.GameID.String()),
			)
			continue
		}

		item := TournamentGameWithDetails{
			TournamentID:    tg.TournamentID,
			GameID:          tg.GameID,
			GameName:        g.Name,
			GameDisplayName: g.DisplayName,
			RoundCompleted:  tg.RoundCompleted,
			CurrentRound:    tg.CurrentRound,
		}
		if tg.RoundCompletedAt != nil {
			formatted := tg.RoundCompletedAt.Format("2006-01-02T15:04:05Z07:00")
			item.RoundCompletedAt = &formatted
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, result)
}

// MarkGameRoundCompleted отмечает раунд игры как завершённый
// POST /api/v1/tournaments/{id}/games/{gameId}/complete-round
func (h *GameHandler) MarkGameRoundCompleted(w http.ResponseWriter, r *http.Request) {
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

	// Проверяем наличие репозитория
	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	// Отмечаем раунд как завершённый
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
