package worker

import (
	"context"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MatchRepository интерфейс для работы с матчами
type MatchRepository interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus) error
	UpdateResult(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error
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

	// Обновляем статус на "running"
	if err := p.matchRepo.UpdateStatus(ctx, match.ID, domain.MatchRunning); err != nil {
		return fmt.Errorf("failed to update match status: %w", err)
	}

	// Получаем программы
	program1, err := p.programRepo.GetByID(ctx, match.Program1ID)
	if err != nil {
		return fmt.Errorf("failed to get program1: %w", err)
	}

	program2, err := p.programRepo.GetByID(ctx, match.Program2ID)
	if err != nil {
		return fmt.Errorf("failed to get program2: %w", err)
	}

	// Выполняем матч через executor
	result, err := p.executor.Execute(ctx, match, program1.CodePath, program2.CodePath)
	if err != nil {
		// Сохраняем ошибку в БД
		errorResult := &domain.MatchResult{
			MatchID:      match.ID,
			ErrorCode:    1,
			ErrorMessage: err.Error(),
		}
		_ = p.matchRepo.UpdateResult(ctx, match.ID, errorResult)
		return fmt.Errorf("failed to execute match: %w", err)
	}

	// Обновляем результат в БД
	if err := p.matchRepo.UpdateResult(ctx, match.ID, result); err != nil {
		return fmt.Errorf("failed to update match result: %w", err)
	}

	// Кэшируем результат
	if err := p.matchCache.Set(ctx, match.ID, result); err != nil {
		p.log.LogError("Failed to cache match result", err)
	}

	// Если матч успешно завершён, обновляем рейтинги
	if result.ErrorCode == 0 && result.Winner >= 0 {
		if err := p.updateRatings(ctx, match, result); err != nil {
			p.log.LogError("Failed to update ratings", err,
				zap.String("match_id", match.ID.String()),
			)
			// Не возвращаем ошибку, так как матч уже выполнен
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
