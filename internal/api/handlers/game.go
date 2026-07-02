package handlers

import (
	"context"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/game"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
)

// GameService - полный интерфейс domain-сервиса игр.
// Удовлетворяет GameCRUDService, TournamentGameService и GameRoundLookupService.
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

// GameLeaderboardRepository - интерфейс для leaderboard конкретной игры.
type GameLeaderboardRepository interface {
	GetLeaderboardByGameType(ctx context.Context, tournamentID uuid.UUID, gameType string, limit int) ([]*domain.LeaderboardEntry, error)
	GetHeadToHead(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.HeadToHeadCell, error)
}

// GameMatchRepository - интерфейс для листинга матчей игры.
type GameMatchRepository interface {
	List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error)
}

// GameTournamentRepository - интерфейс для проверки владения турниром.
type GameTournamentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
}

// GameProgramRepository - интерфейс для получения программ по турниру и игре.
type GameProgramRepository interface {
	GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error)
}

// TournamentGameStatusRepository - интерфейс управления статусом игр и раундами.
type TournamentGameStatusRepository interface {
	GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGame, error)
	GetTournamentGamesWithDetails(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGameWithDetails, error)
	MarkRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) error
	SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error
	GetActiveGame(ctx context.Context, tournamentID uuid.UUID) (*domain.TournamentGame, error)
	ResetGameRound(ctx context.Context, tournamentID, gameID uuid.UUID) error
	ResetGameRoundFull(ctx context.Context, tournamentID, gameID uuid.UUID, gameType string) (matchesDeleted, participantsReset, ratingHistoryDeleted int64, err error)
	DeactivateAllGames(ctx context.Context, tournamentID uuid.UUID) error
	// Авто-раунд
	SetAutoRound(ctx context.Context, tournamentID, gameID uuid.UUID, enabled bool, intervalSecs int) error
	GetTournamentGame(ctx context.Context, tournamentID, gameID uuid.UUID) (*domain.TournamentGame, error)
}

// GameHandler - фасад, встраивающий три специализированных sub-handler'а:
//   - GameCRUDHandler: Create/List/Get/Update/Delete для игр
//   - TournamentGameHandler: привязка/отвязка игр к турнирам
//   - GameRoundHandler: leaderboard, матчи, программы, статус, управление раундом
//
// Все имена HTTP-методов сохранены, поэтому routes.go не требует изменений.
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
	eventBus events.Bus,
	uploadDir string,
	log *logger.Logger,
) *GameHandler {
	return &GameHandler{
		GameCRUDHandler:       NewGameCRUDHandler(gameService, log),
		TournamentGameHandler: NewTournamentGameHandler(gameService, tournamentRepo, log),
		GameRoundHandler:      NewGameRoundHandler(gameService, leaderboardRepo, matchRepo, programRepo, tournamentGameStatusRepo, eventBus, uploadDir, log),
	}
}
