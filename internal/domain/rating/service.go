package rating

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RatingRepository интерфейс для работы с рейтингами в БД
type RatingRepository interface {
	Create(ctx context.Context, history *domain.RatingHistory) error
	GetByProgramID(ctx context.Context, programID uuid.UUID) ([]*domain.RatingHistory, error)
	UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, newRating int) error
	UpdateParticipantStats(ctx context.Context, tournamentID, programID uuid.UUID, won bool, draw bool) error
}

// Service - сервис для работы с рейтингами
type Service struct {
	calculator       *EloCalculator
	repo             RatingRepository
	leaderboardCache *cache.LeaderboardCache
	log              *logger.Logger
}

// NewService создаёт новый сервис рейтингов
func NewService(repo RatingRepository, leaderboardCache *cache.LeaderboardCache, log *logger.Logger) *Service {
	return &Service{
		calculator:       NewDefaultEloCalculator(),
		repo:             repo,
		leaderboardCache: leaderboardCache,
		log:              log,
	}
}

// ProcessMatchResult обрабатывает результат матча и обновляет рейтинги
func (s *Service) ProcessMatchResult(ctx context.Context, match *domain.Match, rating1, rating2 int) error {
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

	// Обновляем рейтинг первого участника
	if err := s.updateParticipantRating(ctx, match, match.Program1ID, rating1, newRating1, change1); err != nil {
		return err
	}

	// Обновляем рейтинг второго участника
	if err := s.updateParticipantRating(ctx, match, match.Program2ID, rating2, newRating2, change2); err != nil {
		return err
	}

	// Обновляем статистику (wins/losses/draws)
	if err := s.updateMatchStats(ctx, match); err != nil {
		s.log.LogError("Failed to update match stats", err,
			zap.String("match_id", match.ID.String()),
		)
		// Не возвращаем ошибку, так как основные обновления успешны
	}

	// Обновляем leaderboard в кэше
	if err := s.leaderboardCache.UpdateRating(ctx, match.TournamentID, match.Program1ID, newRating1); err != nil {
		s.log.LogError("Failed to update leaderboard cache for program1", err)
	}

	if err := s.leaderboardCache.UpdateRating(ctx, match.TournamentID, match.Program2ID, newRating2); err != nil {
		s.log.LogError("Failed to update leaderboard cache for program2", err)
	}

	return nil
}

// updateParticipantRating обновляет рейтинг участника в БД
func (s *Service) updateParticipantRating(
	ctx context.Context,
	match *domain.Match,
	programID uuid.UUID,
	oldRating, newRating, change int,
) error {
	// Создаём запись в истории рейтингов
	history := &domain.RatingHistory{
		ID:           uuid.New(),
		ProgramID:    programID,
		TournamentID: match.TournamentID,
		OldRating:    oldRating,
		NewRating:    newRating,
		Change:       change,
		MatchID:      &match.ID,
		CreatedAt:    time.Now(),
	}

	if err := s.repo.Create(ctx, history); err != nil {
		return err
	}

	// Обновляем текущий рейтинг участника
	return s.repo.UpdateParticipantRating(ctx, match.TournamentID, programID, newRating)
}

// updateMatchStats обновляет статистику матчей (wins/losses/draws)
func (s *Service) updateMatchStats(ctx context.Context, match *domain.Match) error {
	winner := *match.Winner

	// Обновляем статистику для первого игрока
	var won1, draw1 bool
	if winner == 1 {
		won1 = true
	} else if winner == 0 {
		draw1 = true
	}

	if err := s.repo.UpdateParticipantStats(ctx, match.TournamentID, match.Program1ID, won1, draw1); err != nil {
		return err
	}

	// Обновляем статистику для второго игрока
	var won2, draw2 bool
	if winner == 2 {
		won2 = true
	} else if winner == 0 {
		draw2 = true
	}

	return s.repo.UpdateParticipantStats(ctx, match.TournamentID, match.Program2ID, won2, draw2)
}

// GetRatingHistory получает историю рейтинга программы
func (s *Service) GetRatingHistory(ctx context.Context, programID uuid.UUID) ([]*domain.RatingHistory, error) {
	return s.repo.GetByProgramID(ctx, programID)
}

// CalculateExpectedScore вычисляет ожидаемый результат матча
func (s *Service) CalculateExpectedScore(rating1, rating2 int) float64 {
	return s.calculator.CalculateExpectedScore(rating1, rating2)
}
