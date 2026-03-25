package handlers

import (
	"context"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
)

// GameRoundLookupService provides game lookup needed by round-related handlers.
type GameRoundLookupService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
}

// GameRoundHandler handles game leaderboard, matches, programs, status, and round management.
type GameRoundHandler struct {
	gameService              GameRoundLookupService
	leaderboardRepo          GameLeaderboardRepository
	matchRepo                GameMatchRepository
	programRepo              GameProgramRepository
	tournamentGameStatusRepo TournamentGameStatusRepository
	eventBus                 events.Bus
	uploadDir                string
	log                      *logger.Logger
}

// NewGameRoundHandler creates a handler for game round and status operations.
func NewGameRoundHandler(
	gameService GameRoundLookupService,
	leaderboardRepo GameLeaderboardRepository,
	matchRepo GameMatchRepository,
	programRepo GameProgramRepository,
	tournamentGameStatusRepo TournamentGameStatusRepository,
	eventBus events.Bus,
	uploadDir string,
	log *logger.Logger,
) *GameRoundHandler {
	return &GameRoundHandler{
		gameService:              gameService,
		leaderboardRepo:          leaderboardRepo,
		matchRepo:                matchRepo,
		programRepo:              programRepo,
		tournamentGameStatusRepo: tournamentGameStatusRepo,
		eventBus:                 eventBus,
		uploadDir:                uploadDir,
		log:                      log,
	}
}

// parseTournamentGameIDs is a helper to parse tournament and game IDs from URL params.
func (h *GameRoundHandler) parseTournamentGameIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}

	gameID, ok := httputil.ParseUUIDParam(w, r, "gameId", "game")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}

	return tournamentID, gameID, true
}
