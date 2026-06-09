package handlers

import (
	"context"
	"os"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ProgramRepository интерфейс для работы с программами
type ProgramRepository interface {
	Create(ctx context.Context, program *domain.Program) error
	CreateWithAtomicVersion(ctx context.Context, program *domain.Program) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Program, error)
	Update(ctx context.Context, program *domain.Program) error
	Delete(ctx context.Context, id uuid.UUID) error
	CheckOwnership(ctx context.Context, programID, userID uuid.UUID) (bool, error)
	GetLatestVersion(ctx context.Context, teamID, gameID uuid.UUID) (int, error)
	GetAllVersionsByTeamAndGame(ctx context.Context, teamID, gameID uuid.UUID) ([]*domain.Program, error)
	ClearErrorMessages(ctx context.Context, tournamentID uuid.UUID) (int64, error)
}

// TournamentParticipantAdder интерфейс для добавления участников в турнир
type TournamentParticipantAdder interface {
	AddParticipant(ctx context.Context, participant *domain.TournamentParticipant) error
}

// TournamentStatusChecker интерфейс для проверки статуса турнира
type TournamentStatusChecker interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
}

// MatchScheduler интерфейс для создания матчей
type MatchScheduler interface {
	ScheduleNewProgramMatches(ctx context.Context, tournamentID, gameID, newProgramID, teamID uuid.UUID) error
}

// GameLookup интерфейс для получения информации об игре
type GameLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
}

// MatchExistenceChecker интерфейс для проверки существования матчей
type MatchExistenceChecker interface {
	HasStartedMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (bool, error)
	HasAnyRunningMatches(ctx context.Context, tournamentID uuid.UUID) (bool, error)
	GetActiveGameType(ctx context.Context, tournamentID uuid.UUID) (string, error)
}

// RoundCompletionChecker интерфейс для проверки завершения раунда игры
type RoundCompletionChecker interface {
	IsRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error)
}

// TeamMembershipChecker интерфейс для проверки членства в команде
type TeamMembershipChecker interface {
	IsUserInTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error)
	IsTeamDisqualified(ctx context.Context, teamID uuid.UUID) (bool, error)
}

// AutoRoundChecker интерфейс для проверки статуса авто-раунда
type AutoRoundChecker interface {
	IsAutoRoundEnabled(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error)
}

// CompileEnqueuer ставит загруженную программу в очередь асинхронной
// компиляции (выполняется worker'ом в Docker-песочнице).
type CompileEnqueuer interface {
	Enqueue(ctx context.Context, programID uuid.UUID) error
}

// ProgramHandler обрабатывает запросы программ
type ProgramHandler struct {
	programRepo      ProgramRepository
	tournamentRepo   TournamentParticipantAdder
	tournamentStatus TournamentStatusChecker
	matchScheduler   MatchScheduler
	gameLookup       GameLookup
	matchChecker     MatchExistenceChecker
	roundChecker     RoundCompletionChecker
	teamChecker      TeamMembershipChecker
	autoRoundChecker AutoRoundChecker
	compileQueue     CompileEnqueuer
	uploadDir        string
	maxFileSize      int64
	log              *logger.Logger
}

// NewProgramHandler создаёт новый program handler
func NewProgramHandler(
	programRepo ProgramRepository,
	tournamentRepo TournamentParticipantAdder,
	tournamentStatus TournamentStatusChecker,
	matchScheduler MatchScheduler,
	gameLookup GameLookup,
	matchChecker MatchExistenceChecker,
	roundChecker RoundCompletionChecker,
	teamChecker TeamMembershipChecker,
	autoRoundChecker AutoRoundChecker,
	compileQueue CompileEnqueuer,
	uploadDir string,
	log *logger.Logger,
) *ProgramHandler {
	if uploadDir == "" {
		uploadDir = "/data/programs"
	}

	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		log.Error("Failed to create upload directory", zap.Error(err))
	}

	return &ProgramHandler{
		programRepo:      programRepo,
		tournamentRepo:   tournamentRepo,
		tournamentStatus: tournamentStatus,
		matchScheduler:   matchScheduler,
		gameLookup:       gameLookup,
		matchChecker:     matchChecker,
		roundChecker:     roundChecker,
		teamChecker:      teamChecker,
		autoRoundChecker: autoRoundChecker,
		compileQueue:     compileQueue,
		uploadDir:        uploadDir,
		maxFileSize:      10 * 1024 * 1024, // 10MB
		log:              log,
	}
}
