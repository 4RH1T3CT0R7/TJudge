package rating

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ParticipantUpdate - данные для обновления рейтинга и статы одного участника
type ParticipantUpdate struct {
	ProgramID    uuid.UUID
	TournamentID uuid.UUID
	History      *models.RatingHistory
	RatingDelta  int
	Won          bool
	Draw         bool
}

type RatingRepository interface {
	Create(ctx context.Context, history *models.RatingHistory) error
	GetByProgramID(ctx context.Context, programID uuid.UUID) ([]*models.RatingHistory, error)
	UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int) error
	UpdateParticipantStats(ctx context.Context, tournamentID, programID uuid.UUID, won bool, draw bool) error
	UpdateParticipantRatingAndStats(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int, won bool, draw bool) error
	// рейтинг за матч применяется ровно один раз: репозиторий в одной транзакции
	// гасит outbox-задачу, блокирует рейтинги обоих участников и отдаёт их в calc.
	// false - рейтинг уже применён (fast path и диспетчер разошлись) или матча нет
	ApplyMatchResult(ctx context.Context, match *models.Match, calc func(rating1, rating2 int) (*ParticipantUpdate, *ParticipantUpdate)) (bool, error)
}

type Service struct {
	calculator *EloCalculator
	repo       RatingRepository
	notifier   events.Notifier
	log        *logger.Logger
}

func NewService(repo RatingRepository, notifier events.Notifier, log *logger.Logger) *Service {
	return &Service{
		calculator: NewDefaultEloCalculator(),
		repo:       repo,
		notifier:   notifier,
		log:        log,
	}
}

// ProcessMatchResult считает новые рейтинги и обновляет обоих участников.
// рейтинги до матча читаются под блокировкой внутри транзакции репозитория,
// поэтому параллельные матчи одной программы считаются от актуальных значений
func (s *Service) ProcessMatchResult(ctx context.Context, match *models.Match) error {
	if match.Winner == nil {
		return errors.ErrValidation.WithMessage("match has no winner")
	}

	winner := *match.Winner

	// результат раскладывается: 1 - выиграл первый, 0 - ничья обоим, 2 - выиграл второй
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

	var newRating1, newRating2 int
	applied, err := s.repo.ApplyMatchResult(ctx, match, func(rating1, rating2 int) (*ParticipantUpdate, *ParticipantUpdate) {
		var change1, change2 int
		newRating1, newRating2, change1, change2 = s.calculator.ProcessMatch(rating1, rating2, winner)

		s.log.Info("Processing match result",
			zap.String("match_id", match.ID.String()),
			zap.Int("rating1_old", rating1),
			zap.Int("rating1_new", newRating1),
			zap.Int("rating1_change", change1),
			zap.Int("rating2_old", rating2),
			zap.Int("rating2_new", newRating2),
			zap.Int("rating2_change", change2),
		)

		update1 := &ParticipantUpdate{
			ProgramID:    match.Program1ID,
			TournamentID: match.TournamentID,
			History: &models.RatingHistory{
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
			History: &models.RatingHistory{
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
		return update1, update2
	})
	if err != nil {
		return err
	}
	if !applied {
		s.log.Info("Match rating already applied, skipping",
			zap.String("match_id", match.ID.String()),
		)
		return nil
	}

	// событие уходит только после успешного апдейта, от него зависит кэш и вебсокет
	s.notifier.MatchResultProcessed(ctx, events.MatchResultProcessed{
		Version:      1,
		TournamentID: match.TournamentID,
		MatchID:      match.ID,
		Program1ID:   match.Program1ID,
		Program2ID:   match.Program2ID,
		NewRating1:   newRating1,
		NewRating2:   newRating2,
		Winner:       winner,
	})

	return nil
}

func (s *Service) GetRatingHistory(ctx context.Context, programID uuid.UUID) ([]*models.RatingHistory, error) {
	return s.repo.GetByProgramID(ctx, programID)
}

func (s *Service) CalculateExpectedScore(rating1, rating2 int) float64 {
	return s.calculator.CalculateExpectedScore(rating1, rating2)
}
