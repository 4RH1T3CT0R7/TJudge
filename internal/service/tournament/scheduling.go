package tournament

import (
	"context"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SchedulingService - планирование матчей: round-robin, раунды по отдельным играм
// и перезапуск упавших
type SchedulingService struct {
	tournamentRepo  TournamentRepository
	matchRepo       MatchRepository
	queueManager    QueueManager
	gameRepo        GameRepository
	distributedLock DistributedLock
	notifier        events.Notifier
	log             *logger.Logger
}

// matchIDs - вытаскивает id матчей для отката (компенсация при ошибке очереди)
func matchIDs(matches []*models.Match) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	return ids
}

func NewSchedulingService(
	tournamentRepo TournamentRepository,
	matchRepo MatchRepository,
	queueManager QueueManager,
	gameRepo GameRepository,
	distributedLock DistributedLock,
	notifier events.Notifier,
	log *logger.Logger,
) *SchedulingService {
	return &SchedulingService{
		tournamentRepo:  tournamentRepo,
		matchRepo:       matchRepo,
		queueManager:    queueManager,
		gameRepo:        gameRepo,
		distributedLock: distributedLock,
		notifier:        notifier,
		log:             log,
	}
}

// RunAllMatches - гоняет все pending матчи турнира (ручка админа).
// если pending нет - генерит новый раунд round-robin
func (ss *SchedulingService) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// лок от дублирования матчей
	lockKey := fmt.Sprintf("tournament:run_matches:%s", tournamentID.String())

	var enqueued int
	lockErr := ss.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		var err error
		enqueued, err = ss.runAllMatchesLocked(ctx, tournamentID)
		return err
	})

	return enqueued, lockErr
}

func (ss *SchedulingService) runAllMatchesLocked(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// сначала берётся то что уже висит в pending
	matches, err := ss.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// id матчей, созданных именно в этом вызове - только они откатываются при ошибке enqueue.
	// старые pending из бд не трогаются, их recovery-worker подберёт
	var createdIDs []uuid.UUID

	// pending пусто - генерируется новый раунд
	if len(matches) == 0 {
		ss.log.Info("No pending matches, generating new round",
			zap.String("tournament_id", tournamentID.String()),
		)

		// турнир из бд
		tournament, err := ss.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get tournament: %w", err)
		}

		// раунд гоняется только для активного турнира
		if tournament.Status != models.TournamentActive {
			return 0, errors.ErrConflict.WithMessage("tournament is not active")
		}

		// участники сгруппированы по играм, чтобы не сводить проги разных игр
		participantsByGame, err := ss.tournamentRepo.GetLatestParticipantsGroupedByGame(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get participants: %w", err)
		}

		if len(participantsByGame) == 0 {
			return 0, errors.ErrValidation.WithMessage("need at least 2 participants to run matches")
		}

		// сбрасываются все игры до генерации, тк иначе может выйти частичный сброс,
		// если у какой-то игры остались running матчи
		for gameType := range participantsByGame {
			if err := ss.gameRepo.ResetGameByType(ctx, tournamentID, gameType); err != nil {
				return 0, fmt.Errorf("failed to reset game %s: %w", gameType, err)
			}
		}

		// матчи генерируются по каждой игре отдельно
		for gameType, participants := range participantsByGame {
			if len(participants) < 2 {
				ss.log.Warn("Skipping game with fewer than 2 participants",
					zap.String("game_type", gameType),
					zap.Int("participants", len(participants)),
				)
				continue
			}

			gameMatches, err := ss.generateRoundRobinMatchesForGame(tournament, participants, gameType, models.PriorityMedium)
			if err != nil {
				return 0, fmt.Errorf("failed to generate matches for game %s: %w", gameType, err)
			}

			if len(gameMatches) == 0 {
				continue
			}

			if err := ss.matchRepo.CreateBatch(ctx, gameMatches); err != nil {
				// откат того что уже успели создать в этом вызове
				if len(createdIDs) > 0 {
					if delErr := ss.matchRepo.DeleteBatch(ctx, createdIDs); delErr != nil {
						ss.log.Error("Failed to rollback partially-created matches",
							zap.Error(delErr),
							zap.Int("orphaned_matches", len(createdIDs)),
						)
					}
				}
				return 0, fmt.Errorf("failed to create matches for game %s: %w", gameType, err)
			}

			createdIDs = append(createdIDs, matchIDs(gameMatches)...)
			matches = append(matches, gameMatches...)

			ss.log.Info("Generated new round of matches for game",
				zap.String("tournament_id", tournamentID.String()),
				zap.String("game_type", gameType),
				zap.Int("matches_count", len(gameMatches)),
			)
		}
	}

	// всё в очередь батчем (один pipeline).
	// при ошибке enqueue откатываются только свежесозданные;
	// старые pending остаются, их подберёт recovery-worker
	if err := ss.queueManager.EnqueueBatch(ctx, matches); err != nil {
		if len(createdIDs) > 0 {
			if delErr := ss.matchRepo.DeleteBatch(ctx, createdIDs); delErr != nil {
				ss.log.Error("Failed to rollback matches after enqueue error",
					zap.Error(delErr),
					zap.Int("orphaned_matches", len(createdIDs)),
					zap.String("tournament_id", tournamentID.String()),
				)
			}
		}
		return 0, fmt.Errorf("failed to enqueue matches: %w", err)
	}

	ss.log.Info("Admin triggered all matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("total_pending", len(matches)),
		zap.Int("enqueued", len(matches)),
	)

	return len(matches), nil
}

// RunGameMatches - запускает матчи для одной конкретной игры турнира
func (ss *SchedulingService) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	// лок от дублирования матчей
	lockKey := fmt.Sprintf("tournament:run_game_matches:%s:%s", tournamentID.String(), gameType)

	var enqueued int
	lockErr := ss.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		var err error
		enqueued, err = ss.runGameMatchesLocked(ctx, tournamentID, gameType)
		return err
	})

	return enqueued, lockErr
}

func (ss *SchedulingService) runGameMatchesLocked(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	// pending именно этой игры
	matches, err := ss.matchRepo.GetPendingByTournamentAndGame(ctx, tournamentID, gameType)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// createdIDs - что создали в этом вызове, для отката
	var createdIDs []uuid.UUID

	// pending нет - старые результаты сбрасываются и раунд генерируется заново
	if len(matches) == 0 {
		ss.log.Info("No pending matches for game, resetting and generating new round",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", gameType),
		)

		tournament, err := ss.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get tournament: %w", err)
		}

		// турнир должен быть активен
		if tournament.Status != models.TournamentActive {
			return 0, errors.ErrConflict.WithMessage("tournament is not active")
		}

		// прошлые матчи и рейтинги этой игры сбрасываются (если были)
		// при перезапуске новые результаты затирают старые
		if err := ss.gameRepo.ResetGameByType(ctx, tournamentID, gameType); err != nil {
			return 0, fmt.Errorf("failed to reset game %s before generating matches: %w", gameType, err)
		}

		// участники - только последние версии прог каждой команды по этой игре
		participants, err := ss.getLatestParticipantsByGame(ctx, tournamentID, gameType)
		if err != nil {
			return 0, fmt.Errorf("failed to get participants: %w", err)
		}

		if len(participants) < 2 {
			return 0, errors.ErrValidation.WithMessage("need at least 2 participants with programs for this game")
		}

		// ручной запуск - высокий приоритет
		matches, err = ss.generateRoundRobinMatchesForGame(tournament, participants, gameType, models.PriorityHigh)
		if err != nil {
			return 0, fmt.Errorf("failed to generate matches: %w", err)
		}

		if len(matches) == 0 {
			return 0, errors.ErrValidation.WithMessage("no matches generated for this game")
		}

		// сохранение в бд
		if err := ss.matchRepo.CreateBatch(ctx, matches); err != nil {
			return 0, fmt.Errorf("failed to create matches: %w", err)
		}
		createdIDs = matchIDs(matches)

		ss.log.Info("Generated new round of matches for game",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", gameType),
			zap.Int("matches_count", len(matches)),
		)
	}

	// всё в очередь батчем (один pipeline).
	// при ошибке enqueue откатываются только свежесозданные;
	// старые pending остаются recovery-worker'у
	if err := ss.queueManager.EnqueueBatch(ctx, matches); err != nil {
		if len(createdIDs) > 0 {
			if delErr := ss.matchRepo.DeleteBatch(ctx, createdIDs); delErr != nil {
				ss.log.Error("Failed to rollback matches after enqueue error",
					zap.Error(delErr),
					zap.Int("orphaned_matches", len(createdIDs)),
					zap.String("tournament_id", tournamentID.String()),
					zap.String("game_type", gameType),
				)
			}
		}
		return 0, fmt.Errorf("failed to enqueue matches: %w", err)
	}

	ss.log.Info("Admin triggered game matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_type", gameType),
		zap.Int("total_pending", len(matches)),
		zap.Int("enqueued", len(matches)),
	)

	return len(matches), nil
}

func (ss *SchedulingService) getLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*models.TournamentParticipant, error) {
	return ss.tournamentRepo.GetLatestParticipantsByGame(ctx, tournamentID, gameType)
}

// generateRoundRobinMatchesForGame - собирает матчи для одной игры.
// раунд всегда первый: новый раунд начинается со сброса прошлых результатов игры
func (ss *SchedulingService) generateRoundRobinMatchesForGame(tournament *models.Tournament, participants []*models.TournamentParticipant, gameType string, priority models.MatchPriority) ([]*models.Match, error) {
	var matches []*models.Match
	now := time.Now()

	// каждый с каждым, обе стороны играют (AB и BA) -
	// порядок важен для несимметричных игр
	// TODO: N*(N-1) матчей на раунд, на больших турнирах генерация долгая
	for i := range participants {
		for j := range participants {
			// сам с собой не играет
			if i == j {
				continue
			}

			match := &models.Match{
				ID:           uuid.New(),
				TournamentID: tournament.ID,
				Program1ID:   participants[i].ProgramID,
				Program2ID:   participants[j].ProgramID,
				GameType:     gameType,
				Status:       models.MatchPending,
				Priority:     priority,
				RoundNumber:  1,
				CreatedAt:    now,
			}

			if err := match.Validate(); err != nil {
				return nil, fmt.Errorf("invalid match generated: %w", err)
			}

			matches = append(matches, match)
		}
	}

	return matches, nil
}

// RetryFailedMatches - переводит упавшие матчи обратно в pending и ставит в очередь
func (ss *SchedulingService) RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// все failed -> pending
	resetCount, err := ss.matchRepo.ResetFailedMatches(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to reset failed matches: %w", err)
	}

	if resetCount == 0 {
		return 0, nil
	}

	// pending забирается и ставится в очередь
	matches, err := ss.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// в очередь батчем (один pipeline)
	if err := ss.queueManager.EnqueueBatch(ctx, matches); err != nil {
		return 0, fmt.Errorf("failed to enqueue matches: %w", err)
	}

	ss.log.Info("Admin retried failed matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int64("reset_count", resetCount),
		zap.Int("enqueued", len(matches)),
	)

	return len(matches), nil
}
