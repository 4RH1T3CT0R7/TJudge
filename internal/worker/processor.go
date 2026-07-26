package worker

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/executor"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrMatchNotFound используется когда матч не найден в БД (был удалён)
// Это не ошибка обработки - матч просто нужно пропустить
var ErrMatchNotFound = stderrors.New("match not found in database")

// ErrProgramFailed - терминальная ошибка программы участника (ненулевой
// exit-code, мусорный вывод, превышение таймаута). Ретраи бессмысленны:
// матч уже помечен failed, пул не должен повторять обработку.
var ErrProgramFailed = stderrors.New("match failed: program error")

// MatchRepository интерфейс для работы с матчами
type MatchRepository interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus) error
	UpdateResult(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error
	// UpdateResultWithOutbox записывает результат и outbox-задачу рейтинга
	// в одной транзакции - гарантия, что рейтинг не потеряется при сбое.
	UpdateResultWithOutbox(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error
	// MarkRatingApplied закрывает outbox-задачу после успешного fast-path
	// обновления рейтинга.
	MarkRatingApplied(ctx context.Context, matchID uuid.UUID) error
	// ResetToPending возвращает матч в pending после транзиентной
	// инфраструктурной ошибки executor'а.
	ResetToPending(ctx context.Context, id uuid.UUID) error
}

// RatingRepository интерфейс для работы с рейтингами
type RatingRepository interface {
	GetParticipantRatings(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID) (int, int, error)
}

// RatingService интерфейс для обновления рейтингов
type RatingService interface {
	ProcessMatchResult(ctx context.Context, match *domain.Match, rating1, rating2 int) error
}

// Executor интерфейс для выполнения матчей
type Executor interface {
	Execute(ctx context.Context, match *domain.Match, program1Path, program2Path string) (*domain.MatchResult, error)
}

// ProgramRepository интерфейс для работы с программами
type ProgramRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Program, error)
}

// Processor обрабатывает матчи
type Processor struct {
	matchRepo     MatchRepository
	ratingRepo    RatingRepository
	programRepo   ProgramRepository
	ratingService RatingService
	executor      Executor
	matchCache    *cache.MatchCache
	log           *logger.Logger
}

// NewProcessor создаёт новый процессор матчей
func NewProcessor(
	matchRepo MatchRepository,
	ratingRepo RatingRepository,
	programRepo ProgramRepository,
	ratingService RatingService,
	executor Executor,
	matchCache *cache.MatchCache,
	log *logger.Logger,
) *Processor {
	return &Processor{
		matchRepo:     matchRepo,
		ratingRepo:    ratingRepo,
		programRepo:   programRepo,
		ratingService: ratingService,
		executor:      executor,
		matchCache:    matchCache,
		log:           log,
	}
}

// Process обрабатывает матч
func (p *Processor) Process(ctx context.Context, match *domain.Match) error {
	p.log.Info("Processing match",
		zap.String("match_id", match.ID.String()),
		zap.String("tournament_id", match.TournamentID.String()),
	)

	// Обновляем статус на "running" (только из pending - идемпотентная защита)
	if err := p.matchRepo.UpdateStatus(ctx, match.ID, domain.MatchRunning); err != nil {
		// Матч уже обрабатывается или обработан - пропускаем (дубликат из очереди)
		if stderrors.Is(err, domain.ErrMatchAlreadyProcessed) {
			p.log.Info("Match already processed or in progress, skipping duplicate",
				zap.String("match_id", match.ID.String()),
			)
			return nil
		}
		// Проверяем, не был ли матч удалён из БД
		if isNotFoundError(err) {
			p.log.Warn("Match not found in database, skipping (likely deleted)",
				zap.String("match_id", match.ID.String()),
			)
			return ErrMatchNotFound
		}
		return fmt.Errorf("failed to update match status: %w", err)
	}

	// Получаем обе программы одним запросом
	programs, err := p.programRepo.GetByIDs(ctx, []uuid.UUID{match.Program1ID, match.Program2ID})
	if err != nil {
		return fmt.Errorf("failed to get programs: %w", err)
	}

	programMap := make(map[uuid.UUID]*domain.Program, len(programs))
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

	// Выполняем матч через executor
	result, err := p.executor.Execute(ctx, match, program1.CodePath, program2.CodePath)
	if err != nil {
		// Инфраструктурная ошибка (Docker daemon, образ, контейнер):
		// программа участника не виновата - матч НЕ помечается failed,
		// а возвращается в pending. Его повторит retry-цикл пула, а если
		// попытки исчерпаются - периодический recovery-сервис.
		if executor.IsInfraError(err) {
			if resetErr := p.matchRepo.ResetToPending(ctx, match.ID); resetErr != nil {
				p.log.Error("Failed to reset match to pending after infra error",
					zap.String("match_id", match.ID.String()),
					zap.Error(resetErr),
				)
			}
			return fmt.Errorf("transient executor error: %w", err)
		}

		// Ошибка программы (exit-code, формат вывода, таймаут) - терминальна.
		errorResult := &domain.MatchResult{
			MatchID:      match.ID,
			ErrorCode:    1,
			ErrorMessage: err.Error(),
		}
		if dbErr := p.matchRepo.UpdateResult(ctx, match.ID, errorResult); dbErr != nil {
			p.log.Error("Failed to save error result to database",
				zap.String("match_id", match.ID.String()),
				zap.Error(dbErr),
			)
		}
		return fmt.Errorf("%w: %s", ErrProgramFailed, err.Error())
	}

	// Обновляем результат в БД; для успешных матчей в той же транзакции
	// создаётся outbox-задача «обновить рейтинг».
	if err := p.matchRepo.UpdateResultWithOutbox(ctx, match.ID, result); err != nil {
		return fmt.Errorf("failed to update match result: %w", err)
	}

	// Кэшируем результат
	if p.matchCache != nil {
		if err := p.matchCache.Set(ctx, match.ID, result); err != nil {
			p.log.LogError("Failed to cache match result", err)
		}
	}

	// Если матч успешно завершён, обновляем рейтинги (fast path).
	// При ошибке ничего не теряется: outbox-задача осталась pending,
	// её доведёт до конца OutboxDispatcher.
	if result.ErrorCode == 0 && result.Winner >= 0 {
		if err := p.updateRatings(ctx, match, result); err != nil {
			p.log.LogError("Failed to update ratings, outbox dispatcher will retry", err,
				zap.String("match_id", match.ID.String()),
			)
		} else if err := p.matchRepo.MarkRatingApplied(ctx, match.ID); err != nil {
			// Не страшно: диспетчер увидит rating_history и закроет задачу сам.
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

// updateRatings обновляет рейтинги участников после матча
func (p *Processor) updateRatings(ctx context.Context, match *domain.Match, result *domain.MatchResult) error {
	// Получаем текущие рейтинги участников
	rating1, rating2, err := p.ratingRepo.GetParticipantRatings(
		ctx,
		match.TournamentID,
		match.Program1ID,
		match.Program2ID,
	)
	if err != nil {
		return fmt.Errorf("failed to get participant ratings: %w", err)
	}

	// Обновляем рейтинги через сервис
	match.Winner = &result.Winner
	if err := p.ratingService.ProcessMatchResult(ctx, match, rating1, rating2); err != nil {
		return fmt.Errorf("failed to process match result: %w", err)
	}

	return nil
}

// isNotFoundError проверяет, является ли ошибка типом "not found"
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем через errors package (AppError с кодом 404)
	if errors.IsNotFound(err) {
		return true
	}

	// Проверяем sentinel error
	return stderrors.Is(err, ErrMatchNotFound)
}
