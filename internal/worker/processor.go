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
	ResetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) (int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.MatchStatus) error
	UpdateResult(ctx context.Context, id uuid.UUID, result *models.MatchResult) error
	// результат + outbox-задача рейтинга в одной транзакции, чтобы рейтинг
	// не потерялся при падении
	UpdateResultWithOutbox(ctx context.Context, id uuid.UUID, result *models.MatchResult) error
	ResetToPending(ctx context.Context, id uuid.UUID) error
}

// RatingService применяет рейтинг за матч, повторный вызов по тому же матчу
// ничего не меняет
type RatingService interface {
	ProcessMatchResult(ctx context.Context, match *models.Match) error
}

// Executor гоняет матч в докере, по одному или пачкой в одном контейнере
type Executor interface {
	Execute(ctx context.Context, match *models.Match, program1Path, program2Path string) (*models.MatchResult, error)
	ExecuteBatch(ctx context.Context, items []executor.BatchItem) ([]executor.BatchResult, error)
}

type ProgramRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Program, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.Program, error)
	UpdateCompileResult(ctx context.Context, id uuid.UUID, status models.ProgramStatus, codePath string, errorMessage *string) (bool, error)
	GetStuckCompiling(ctx context.Context, olderThan time.Duration, limit int) ([]*models.Program, error)
	SaveCheckResult(ctx context.Context, id uuid.UUID, status models.CheckStatus, message *string) (bool, error)
	ResetStaleChecks(ctx context.Context, olderThan time.Duration) (int64, error)
}

// Processor обрабатывает матчи
type Processor struct {
	matchRepo     MatchRepository
	programRepo   ProgramRepository
	ratingService RatingService
	executor      Executor
	log           *logger.Logger
}

func NewProcessor(
	matchRepo MatchRepository,
	programRepo ProgramRepository,
	ratingService RatingService,
	executor Executor,
	log *logger.Logger,
) *Processor {
	return &Processor{
		matchRepo:     matchRepo,
		programRepo:   programRepo,
		ratingService: ratingService,
		executor:      executor,
		log:           log,
	}
}

// Process обрабатывает один матч
func (p *Processor) Process(ctx context.Context, match *models.Match) error {
	p.log.Debug("Processing match",
		zap.String("match_id", match.ID.String()),
		zap.String("tournament_id", match.TournamentID.String()),
	)

	if claimed, err := p.claim(ctx, match); !claimed {
		return err
	}

	// дальше матч running. любая ошибка, кроме терминальной ошибки программы,
	// возвращает его в pending: иначе ретрай пула упрётся в
	// ErrMatchAlreadyProcessed на переводе в running и засчитает матч как
	// успешный, а сам матч провисит running до recovery
	if err := p.play(ctx, match); err != nil {
		if !stderrors.Is(err, ErrProgramFailed) {
			p.resetToPending(ctx, match.ID)
		}
		return err
	}
	return nil
}

// claim переводит матч в running, причём только из pending - если из очереди
// прилетел дубль, второй перевод не пройдёт и матч просто пропускается.
// false - матч не взят: nil (дубль), ErrMatchNotFound или ошибка базы
func (p *Processor) claim(ctx context.Context, match *models.Match) (bool, error) {
	err := p.matchRepo.UpdateStatus(ctx, match.ID, models.MatchRunning)
	switch {
	case err == nil:
		return true, nil
	case stderrors.Is(err, models.ErrMatchAlreadyProcessed):
		p.log.Debug("Match already processed or in progress, skipping duplicate",
			zap.String("match_id", match.ID.String()),
		)
		return false, nil
	case isNotFoundError(err):
		p.log.Warn("Match not found in database, skipping (likely deleted)",
			zap.String("match_id", match.ID.String()),
		)
		return false, ErrMatchNotFound
	default:
		return false, fmt.Errorf("failed to update match status: %w", err)
	}
}

// ProcessBatch играет матчи пачкой в одном контейнере (Executor.ExecuteBatch).
// Ошибка по каждому матчу - как у Process: nil - итог записан или матч уже не
// наш, ErrProgramFailed - терминальная ошибка программы, остальное - матч не
// сыгран и возвращён в pending, его доигрывают по одному
func (p *Processor) ProcessBatch(ctx context.Context, matches []*models.Match) []error {
	errs := make([]error, len(matches))
	var claimed []int
	var ids []uuid.UUID
	for i, m := range matches {
		ok, err := p.claim(ctx, m)
		if !ok {
			if !stderrors.Is(err, ErrMatchNotFound) {
				errs[i] = err
			}
			continue
		}
		claimed = append(claimed, i)
		ids = append(ids, m.Program1ID, m.Program2ID)
	}
	if len(claimed) == 0 {
		return errs
	}
	// сбой до записи итога: взятые матчи возвращаются в pending
	release := func(idx []int, err error) []error {
		for _, i := range idx {
			p.resetToPending(ctx, matches[i].ID)
			errs[i] = err
		}
		return errs
	}

	programs, err := p.programRepo.GetByIDs(ctx, ids)
	if err != nil {
		return release(claimed, fmt.Errorf("failed to get programs: %w", err))
	}
	paths := make(map[uuid.UUID]string, len(programs))
	for _, prog := range programs {
		paths[prog.ID] = prog.CodePath
	}
	var items []executor.BatchItem
	var inBatch []int
	for _, i := range claimed {
		m := matches[i]
		path1, ok1 := paths[m.Program1ID]
		path2, ok2 := paths[m.Program2ID]
		if !ok1 || !ok2 {
			release([]int{i}, fmt.Errorf("programs of match %s not found", m.ID))
			continue
		}
		items = append(items, executor.BatchItem{Match: m, Program1Path: path1, Program2Path: path2})
		inBatch = append(inBatch, i)
	}
	if len(items) == 0 {
		return errs
	}

	results, err := p.executor.ExecuteBatch(ctx, items)
	if err != nil {
		return release(inBatch, fmt.Errorf("transient executor error: %w", err))
	}
	for k, i := range inBatch {
		err := p.record(ctx, matches[i], results[k].Result, results[k].Err)
		if err != nil && !stderrors.Is(err, ErrProgramFailed) {
			p.resetToPending(ctx, matches[i].ID)
		}
		errs[i] = err
	}
	return errs
}

// play гоняет матч, уже переведённый в running, и пишет результат
func (p *Processor) play(ctx context.Context, match *models.Match) error {
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
	return p.record(ctx, match, result, err)
}

// record пишет итог матча, сыгранного по одному или в пачке
func (p *Processor) record(ctx context.Context, match *models.Match, result *models.MatchResult, err error) error {
	// итог пишется и при истёкшем ctx матча (shutdown, таймаут воркера),
	// иначе готовый результат теряется, а матч остаётся running до recovery
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err != nil {
		// тут важно различать два вида ошибок. инфраструктурная (докер лёг,
		// образа нет) - программа не виновата, матч возвращается в pending,
		// его повторит ретрай пула или recovery. а ошибка самой программы
		// (упала, мусор в выводе) - терминальная, матч помечается failed
		if executor.IsInfraError(err) {
			return fmt.Errorf("transient executor error: %w", err)
		}

		errorResult := &models.MatchResult{
			MatchID:      match.ID,
			ErrorCode:    1,
			ErrorMessage: err.Error(),
		}
		if dbErr := p.matchRepo.UpdateResult(writeCtx, match.ID, errorResult); dbErr != nil {
			if stderrors.Is(dbErr, models.ErrMatchAlreadyProcessed) {
				// матч отменили или удалили, пока он играл, - штатный случай
				p.log.Info("Match is no longer running, error result discarded",
					zap.String("match_id", match.ID.String()),
				)
				return nil
			}
			p.log.Error("Failed to save error result to database",
				zap.String("match_id", match.ID.String()),
				zap.Error(dbErr),
			)
		}
		return fmt.Errorf("%w: %s", ErrProgramFailed, err.Error())
	}

	// результат + outbox-задача «обновить рейтинг» одной транзакцией
	if err := p.matchRepo.UpdateResultWithOutbox(writeCtx, match.ID, result); err != nil {
		if stderrors.Is(err, models.ErrMatchAlreadyProcessed) {
			// пока матч играл, его отменили (дисквалификация) или удалили
			// (сброс раунда) - результат выбрасывается, рейтинг не трогается
			p.log.Info("Match is no longer running, result discarded",
				zap.String("match_id", match.ID.String()),
			)
			return nil
		}
		return fmt.Errorf("failed to update match result: %w", err)
	}

	// fast-path рейтинга. если тут что-то упадёт - не страшно, outbox-задача
	// осталась pending и диспетчер её добьёт
	if result.ErrorCode == 0 && result.Winner >= 0 {
		match.Winner = &result.Winner
		match.Score1 = &result.Score1
		match.Score2 = &result.Score2
		if err := p.ratingService.ProcessMatchResult(ctx, match); err != nil {
			p.log.LogError("Failed to update ratings, outbox dispatcher will retry", err,
				zap.String("match_id", match.ID.String()),
			)
		}
	}

	p.log.Debug("Match processed successfully",
		zap.String("match_id", match.ID.String()),
		zap.Int("winner", result.Winner),
	)

	return nil
}

// resetToPending возвращает running-матч в pending. контекст отвязан от
// контекста матча: тот к этому моменту мог уже истечь по таймауту воркера
func (p *Processor) resetToPending(ctx context.Context, matchID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := p.matchRepo.ResetToPending(ctx, matchID); err != nil {
		p.log.LogError("Failed to reset match to pending", err,
			zap.String("match_id", matchID.String()),
		)
	}
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
