package handlers

import (
	"context"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/game"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
)

// GameService is the full interface for the game domain service.
// It satisfies GameCRUDService, TournamentGameService, and GameRoundLookupService.
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

// GameLeaderboardRepository is the interface for game-specific leaderboards.
type GameLeaderboardRepository interface {
	GetLeaderboardByGameType(ctx context.Context, tournamentID uuid.UUID, gameType string, limit int) ([]*domain.LeaderboardEntry, error)
}

// GameMatchRepository is the interface for listing game matches.
type GameMatchRepository interface {
	List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error)
}

// GameTournamentRepository is the interface for tournament ownership checks.
type GameTournamentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
}

// GameProgramRepository is the interface for getting programs by tournament and game.
type GameProgramRepository interface {
	GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error)
}

// TournamentGameStatusRepository is the interface for game status and round management.
type TournamentGameStatusRepository interface {
	GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGame, error)
	GetTournamentGamesWithDetails(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGameWithDetails, error)
	MarkRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) error
	SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error
	GetActiveGame(ctx context.Context, tournamentID uuid.UUID) (*domain.TournamentGame, error)
	ResetGameRound(ctx context.Context, tournamentID, gameID uuid.UUID) error
	ResetGameRoundFull(ctx context.Context, tournamentID, gameID uuid.UUID, gameType string) (matchesDeleted, participantsReset, ratingHistoryDeleted int64, err error)
	DeactivateAllGames(ctx context.Context, tournamentID uuid.UUID) error
	// Auto-round
	SetAutoRound(ctx context.Context, tournamentID, gameID uuid.UUID, enabled bool, intervalSecs int) error
	GetTournamentGame(ctx context.Context, tournamentID, gameID uuid.UUID) (*domain.TournamentGame, error)
}

// GameHandler is a facade that embeds three focused sub-handlers:
//   - GameCRUDHandler: Create/List/Get/Update/Delete for games
//   - TournamentGameHandler: linking/unlinking games with tournaments
//   - GameRoundHandler: leaderboard, matches, programs, status, round management
//
// All HTTP method names are preserved, so routes.go needs no changes.
type GameHandler struct {
	*GameCRUDHandler
	*TournamentGameHandler
	*GameRoundHandler
}

// NewGameHandler creates a GameHandler facade that delegates to focused sub-handlers.
func NewGameHandler(
	gameService GameService,
	leaderboardRepo GameLeaderboardRepository,
	matchRepo GameMatchRepository,
	tournamentRepo GameTournamentRepository,
	programRepo GameProgramRepository,
	tournamentGameStatusRepo TournamentGameStatusRepository,
	eventBus events.Bus,
	log *logger.Logger,
) *GameHandler {
	return &GameHandler{
		GameCRUDHandler:       NewGameCRUDHandler(gameService, log),
		TournamentGameHandler: NewTournamentGameHandler(gameService, tournamentRepo, log),
		GameRoundHandler:      NewGameRoundHandler(gameService, leaderboardRepo, matchRepo, programRepo, tournamentGameStatusRepo, eventBus, log),
	}
}
