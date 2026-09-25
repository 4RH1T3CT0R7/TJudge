package worker

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/executor"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrMatchNotFound - матч удалили из базы, это не ошибка, матч просто пропускается
var ErrMatchNotFound = stderrors.New("match not found in database")

// ErrProgramFailed - терминальная ошибка программы участника (ненулевой exit,
// мусорный вывод, таймаут). ретраить бессмысленно, матч уже помечен failed
var ErrProgramFailed = stderrors.New("match failed: program error")

// MatchRepository - всё что воркеру нужно от репозитория матчей
// (процессор, аутбокс и recovery смотрят на один и тот же *storage.MatchRepository,
// раньше у каждого был свой интерфейс-огрызок - склеил)
type MatchRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Match, error)
	GetPending(ctx context.Context, limit int) ([]*models.Match, error)
	GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*models.Match, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.MatchStatus) error
	BatchUpdateStatus(ctx context.Context, matchIDs []uuid.UUID, status models.MatchStatus) error
	UpdateResult(ctx context.Context, id uuid.UUID, result *models.MatchResult) error
	// результат + outbox-задача рейтинга в одной транзакции, чтобы рейтинг
	// не потерялся при падении
	UpdateResultWithOutbox(ctx context.Context, id uuid.UUID, result *models.MatchResult) error
	MarkRatingApplied(ctx context.Context, matchID uuid.UUID) error
	ResetToPending(ctx context.Context, id uuid.UUID) error
}

type RatingRepository interface {
	GetParticipantRatings(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID) (int, int, error)
	GetByMatchID(ctx context.Context, matchID uuid.UUID) ([]*models.RatingHistory, error)
}

type RatingService interface {
	ProcessMatchResult(ctx context.Context, match *models.Match, rating1, rating2 int) error
}

// Executor гоняет матч в докере
type Executor interface {
	Execute(ctx context.Context, match *models.Match, program1Path, program2Path string) (*models.MatchResult, error)
}

type ProgramRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Program, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.Program, error)
	UpdateCompileResult(ctx context.Context, id uuid.UUID, status models.ProgramStatus, codePath string, errorMessage *string) error
	GetStuckCompiling(ctx context.Context, olderThan time.Duration, limit int) ([]*models.Program, error)
}

// Processor обрабатывает матчи
type Processor struct {
	matchRepo     MatchRepository
	ratingRepo    RatingRepository
	programRepo   ProgramRepository
	ratingService RatingService
	executor      Executor
	log           *logger.Logger
}

func NewProcessor(
	matchRepo MatchRepository,
	ratingRepo RatingRepository,
	programRepo ProgramRepository,
	ratingService RatingService,
	executor Executor,
	log *logger.Logger,
) *Processor {
	return &Processor{
		matchRepo:     matchRepo,
		ratingRepo:    ratingRepo,
		programRepo:   programRepo,
		ratingService: ratingService,
		executor:      executor,
		log:           log,
	}
}

// Process обрабатывает один матч
func (p *Processor) Process(ctx context.Context, match *models.Match) error {
	p.log.Info("Processing match",
		zap.String("match_id", match.ID.String()),
		zap.String("tournament_id", match.TournamentID.String()),
	)

	// перевод в running, причём только из pending - если из очереди прилетел
	// дубль, второй перевод не пройдёт и матч просто пропускается
	if err := p.matchRepo.UpdateStatus(ctx, match.ID, models.MatchRunning); err != nil {
		if stderrors.Is(err, models.ErrMatchAlreadyProcessed) {
			p.log.Info("Match already processed or in progress, skipping duplicate",
				zap.String("match_id", match.ID.String()),
			)
			return nil
		}
		if isNotFoundError(err) {
			p.log.Warn("Match not found in database, skipping (likely deleted)",
				zap.String("match_id", match.ID.String()),
			)
			return ErrMatchNotFound
		}
		return fmt.Errorf("failed to update match status: %w", err)
	}

	// обе программы одним запросом
	programs, err := p.programRepo.GetByIDs(ctx, []uuid.UUID{match.Program1ID, match.Program2ID})
	if err != nil {
		return fmt.Errorf("failed to get programs: %w", err)
	}

	programMap := make(map[uuid.UUID]*models.Program, len(programs))
	for _, prog := range programs {
		programMap[prog.ID] = prog
	}

	program1, ok := programMap[match.Program1ID]
	if !ok {
		return fmt.Errorf("program1 %s not found", match.Program1ID)
	}
	program2, ok := programMap[match.Program2ID]
	if !ok {
		return fmt.Errorf("program2 %s not found", match.Program2ID)
	}

	result, err := p.executor.Execute(ctx, match, program1.CodePath, program2.CodePath)
	// итог пишется и при истёкшем ctx матча (shutdown, таймаут воркера),
	// иначе готовый результат теряется, а матч остаётся running до recovery
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err != nil {
		// тут важно различать два вида ошибок. инфраструктурная (докер лёг,
		// образа нет) - программа не виновата, матч возвращается в pending,
		// его повторит ретрай пула. а ошибка самой программы (упала, мусор
		// в выводе) - терминальная, матч помечается failed
		if executor.IsInfraError(err) {
			// при истёкшем ctx матча ретраев пула уже не будет, а pending вне
			// очереди никто не подберёт: матч остаётся running, и recovery
			// через StuckDuration сбросит его и поставит в очередь
			if ctx.Err() != nil {
				return fmt.Errorf("transient executor error: %w", err)
			}
			if resetErr := p.matchRepo.ResetToPending(ctx, match.ID); resetErr != nil {
				p.log.Error("Failed to reset match to pending after infra error",
					zap.String("match_id", match.ID.String()),
					zap.Error(resetErr),
				)
			}
			return fmt.Errorf("transient executor error: %w", err)
		}

		errorResult := &models.MatchResult{
			MatchID:      match.ID,
			ErrorCode:    1,
			ErrorMessage: err.Error(),
		}
		if dbErr := p.matchRepo.UpdateResult(writeCtx, match.ID, errorResult); dbErr != nil {
			p.log.Error("Failed to save error result to database",
				zap.String("match_id", match.ID.String()),
				zap.Error(dbErr),
			)
		}
		return fmt.Errorf("%w: %s", ErrProgramFailed, err.Error())
	}

	// результат + outbox-задача «обновить рейтинг» одной транзакцией
	if err := p.matchRepo.UpdateResultWithOutbox(writeCtx, match.ID, result); err != nil {
		return fmt.Errorf("failed to update match result: %w", err)
	}

	// fast-path рейтинга. если тут что-то упадёт - не страшно, outbox-задача
	// осталась pending и диспетчер её добьёт
	if result.ErrorCode == 0 && result.Winner >= 0 {
		if err := p.updateRatings(ctx, match, result); err != nil {
			p.log.LogError("Failed to update ratings, outbox dispatcher will retry", err,
				zap.String("match_id", match.ID.String()),
			)
		} else if err := p.matchRepo.MarkRatingApplied(ctx, match.ID); err != nil {
			// тоже не страшно - диспетчер увидит rating_history и закроет задачу
			p.log.LogError("Failed to mark outbox entry done", err,
				zap.String("match_id", match.ID.String()),
			)
		}
	}

	p.log.Info("Match processed successfully",
		zap.String("match_id", match.ID.String()),
		zap.Int("winner", result.Winner),
	)

	return nil
}

func (p *Processor) updateRatings(ctx context.Context, match *models.Match, result *models.MatchResult) error {
	rating1, rating2, err := p.ratingRepo.GetParticipantRatings(
		ctx,
		match.TournamentID,
		match.Program1ID,
		match.Program2ID,
	)
	if err != nil {
		return fmt.Errorf("failed to get participant ratings: %w", err)
	}

	match.Winner = &result.Winner
	if err := p.ratingService.ProcessMatchResult(ctx, match, rating1, rating2); err != nil {
		return fmt.Errorf("failed to process match result: %w", err)
	}

	return nil
}

// isNotFoundError - и AppError с 404, и локальный сентинел
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if errors.IsNotFound(err) {
		return true
	}

	return stderrors.Is(err, ErrMatchNotFound)
}
