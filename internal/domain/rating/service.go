package rating

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ParticipantUpdate содержит данные для обновления рейтинга и статистики одного участника
type ParticipantUpdate struct {
	ProgramID    uuid.UUID
	TournamentID uuid.UUID
	History      *domain.RatingHistory
	RatingDelta  int
	Won          bool
	Draw         bool
}

// RatingRepository интерфейс для работы с рейтингами в БД
type RatingRepository interface {
	Create(ctx context.Context, history *domain.RatingHistory) error
	GetByProgramID(ctx context.Context, programID uuid.UUID) ([]*domain.RatingHistory, error)
	UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int) error
	UpdateParticipantStats(ctx context.Context, tournamentID, programID uuid.UUID, won bool, draw bool) error
	UpdateParticipantRatingAndStats(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int, won bool, draw bool) error
	// ProcessMatchResultAtomic выполняет все обновления рейтингов и статистики для обоих участников
	// матча в одной транзакции. Это гарантирует ELO-инвариант (нулевую сумму).
	ProcessMatchResultAtomic(ctx context.Context, update1, update2 *ParticipantUpdate) error
}

// Service - сервис для работы с рейтингами
type Service struct {
	calculator *EloCalculator
	repo       RatingRepository
	eventBus   events.Bus
	log        *logger.Logger
}

// NewService создаёт новый сервис рейтингов
func NewService(repo RatingRepository, eventBus events.Bus, log *logger.Logger) *Service {
	return &Service{
		calculator: NewDefaultEloCalculator(),
		repo:       repo,
		eventBus:   eventBus,
		log:        log,
	}
}

// ProcessMatchResult обрабатывает результат матча и обновляет рейтинги
func (s *Service) ProcessMatchResult(ctx context.Context, match *domain.Match, rating1, rating2 int) error {
	if match.Winner == nil {
		return errors.ErrValidation.WithMessage("match has no winner")
	}

	// Вычисляем новые рейтинги
	newRating1, newRating2, change1, change2 := s.calculator.ProcessMatch(rating1, rating2, *match.Winner)

	s.log.Info("Processing match result",
		zap.String("match_id", match.ID.String()),
		zap.Int("rating1_old", rating1),
		zap.Int("rating1_new", newRating1),
		zap.Int("rating1_change", change1),
		zap.Int("rating2_old", rating2),
		zap.Int("rating2_new", newRating2),
		zap.Int("rating2_change", change2),
	)

	winner := *match.Winner

	// Определяем статистику для каждого участника
	var won1, draw1, won2, draw2 bool
	if winner == 1 {
		won1 = true
	} else if winner == 0 {
		draw1 = true
		draw2 = true
	}
	if winner == 2 {
		won2 = true
	}

	// Обновляем рейтинг и статистику обоих участников атомарно в одной транзакции.
	// Это гарантирует ELO-инвариант: если обновление одного участника упадёт,
	// обновление другого тоже откатится.
	update1 := &ParticipantUpdate{
		ProgramID:    match.Program1ID,
		TournamentID: match.TournamentID,
		History: &domain.RatingHistory{
			ID:           uuid.New(),
			ProgramID:    match.Program1ID,
			TournamentID: match.TournamentID,
			OldRating:    rating1,
			NewRating:    newRating1,
			Change:       change1,
			MatchID:      &match.ID,
			CreatedAt:    time.Now(),
		},
		RatingDelta: change1,
		Won:         won1,
		Draw:        draw1,
	}

	update2 := &ParticipantUpdate{
		ProgramID:    match.Program2ID,
		TournamentID: match.TournamentID,
		History: &domain.RatingHistory{
			ID:           uuid.New(),
			ProgramID:    match.Program2ID,
			TournamentID: match.TournamentID,
			OldRating:    rating2,
			NewRating:    newRating2,
			Change:       change2,
			MatchID:      &match.ID,
			CreatedAt:    time.Now(),
		},
		RatingDelta: change2,
		Won:         won2,
		Draw:        draw2,
	}

	if err := s.repo.ProcessMatchResultAtomic(ctx, update1, update2); err != nil {
		return err
	}

	s.eventBus.Publish(ctx, events.MatchResultProcessed{
		TournamentID: match.TournamentID,
		MatchID:      match.ID,
		Program1ID:   match.Program1ID,
		Program2ID:   match.Program2ID,
		NewRating1:   newRating1,
		NewRating2:   newRating2,
		Winner:       *match.Winner,
	})

	return nil
}

// GetRatingHistory получает историю рейтинга программы
func (s *Service) GetRatingHistory(ctx context.Context, programID uuid.UUID) ([]*domain.RatingHistory, error) {
	return s.repo.GetByProgramID(ctx, programID)
}

// CalculateExpectedScore вычисляет ожидаемый результат матча
func (s *Service) CalculateExpectedScore(rating1, rating2 int) float64 {
	return s.calculator.CalculateExpectedScore(rating1, rating2)
}
