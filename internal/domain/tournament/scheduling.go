package tournament

import (
	"context"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ProgramRepository - чтобы достать программы турнира по конкретной игре
type ProgramRepository interface {
	GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error)
}

type ScheduleNewProgramMatchesRequest struct {
	TournamentID uuid.UUID
	GameID       uuid.UUID
	NewProgramID uuid.UUID
	TeamID       uuid.UUID
}

// SchedulingService - планирование матчей: round-robin, раунды по отдельным играм,
// перезапуск упавших и досоздание матчей для новых программ
type SchedulingService struct {
	tournamentRepo  TournamentRepository
	matchRepo       MatchRepository
	queueManager    QueueManager
	gameRepo        GameRepository
	distributedLock DistributedLock
	eventBus        events.Bus
	log             *logger.Logger
}

// matchIDs - вытаскивает id матчей для отката (компенсация при ошибке очереди)
func matchIDs(matches []*domain.Match) []uuid.UUID {
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
	eventBus events.Bus,
	log *logger.Logger,
) *SchedulingService {
	return &SchedulingService{
		tournamentRepo:  tournamentRepo,
		matchRepo:       matchRepo,
		queueManager:    queueManager,
		gameRepo:        gameRepo,
		distributedLock: distributedLock,
		eventBus:        eventBus,
		log:             log,
	}
}

// ScheduleNewProgramMatches - досоздаёт матчи для новой программы против всех остальных.
// оптимизация round-robin: не гоняем весь турнир заново, только пары с новой прогой
func (ss *SchedulingService) ScheduleNewProgramMatches(ctx context.Context, req *ScheduleNewProgramMatchesRequest, programRepo ProgramRepository) error {
	// лок чтобы параллельные запросы не наплодили дублей матчей
	lockKey := fmt.Sprintf("tournament:schedule:%s:%s", req.TournamentID.String(), req.GameID.String())

	return ss.distributedLock.WithLock(ctx, lockKey, 10*time.Second, func(ctx context.Context) error {
		// берём турнир из бд
		tournament, err := ss.tournamentRepo.GetByID(ctx, req.TournamentID)
		if err != nil {
			return err
		}

		// для завершённого турнира матчи уже не планируем
		if tournament.Status != domain.TournamentActive && tournament.Status != domain.TournamentPending {
			return errors.ErrConflict.WithMessage("cannot schedule matches for completed tournament")
		}

		// получем все программы турнира по этой игре
		programs, err := programRepo.GetByTournamentAndGame(ctx, req.TournamentID, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get programs: %w", err)
		}

		// матчи только против чужих программ (свою команду пропускаем)
		var matches []*domain.Match
		now := time.Now()

		for _, prog := range programs {
			// свою прогу и проги своей команды скипаем
			if prog.ID == req.NewProgramID {
				continue
			}
			if prog.TeamID != nil && *prog.TeamID == req.TeamID {
				continue
			}

			// матч 1: новая прога первым игроком, существующая вторым
			match1 := &domain.Match{
				ID:           uuid.New(),
				TournamentID: req.TournamentID,
				Program1ID:   req.NewProgramID,
				Program2ID:   prog.ID,
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityHigh, // новые матчи в приоритете
				CreatedAt:    now,
			}

			if err := match1.Validate(); err != nil {
				ss.log.Error("Invalid match generated",
					zap.Error(err),
					zap.String("program1_id", req.NewProgramID.String()),
					zap.String("program2_id", prog.ID.String()),
				)
				continue
			}

			// матч 2: наоборот, старая прога первым а новая вторым (важно для несимметричных игр)
			match2 := &domain.Match{
				ID:           uuid.New(),
				TournamentID: req.TournamentID,
				Program1ID:   prog.ID,
				Program2ID:   req.NewProgramID,
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityHigh, // новые матчи в приоритете
				CreatedAt:    now,
			}

			if err := match2.Validate(); err != nil {
				ss.log.Error("Invalid reverse match generated",
					zap.Error(err),
					zap.String("program1_id", prog.ID.String()),
					zap.String("program2_id", req.NewProgramID.String()),
				)
				continue
			}

			matches = append(matches, match1, match2)
		}

		if len(matches) == 0 {
			ss.log.Info("No new matches to schedule",
				zap.String("tournament_id", req.TournamentID.String()),
				zap.String("program_id", req.NewProgramID.String()),
			)
			return nil
		}

		// пишем матчи в бд
		if err := ss.matchRepo.CreateBatch(ctx, matches); err != nil {
			return fmt.Errorf("failed to create matches: %w", err)
		}

		// кидаем в очередь батчем (один pipeline в редис).
		// если enqueue упал - откатываем матчи из бд, иначе повиснут
		// в pending навсегда и в обработку не попадут
		if err := ss.queueManager.EnqueueBatch(ctx, matches); err != nil {
			ids := matchIDs(matches)
			if delErr := ss.matchRepo.DeleteBatch(ctx, ids); delErr != nil {
				ss.log.Error("Failed to rollback matches after enqueue error",
					zap.Error(delErr),
					zap.Int("orphaned_matches", len(ids)),
					zap.String("tournament_id", req.TournamentID.String()),
				)
			}
			return fmt.Errorf("failed to enqueue matches: %w", err)
		}

		ss.log.Info("New program matches scheduled",
			zap.String("tournament_id", req.TournamentID.String()),
			zap.String("program_id", req.NewProgramID.String()),
			zap.Int("matches_created", len(matches)),
		)

		// шлём событие, дальше broadcast разрулят обработчики
		ss.eventBus.Publish(ctx, events.MatchesCreated{
			Version:      1,
			TournamentID: req.TournamentID,
			ProgramID:    req.NewProgramID,
			MatchCount:   len(matches),
		})

		return nil
	})
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
	// сначала берём то что уже висит в pending
	matches, err := ss.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// id матчей, созданных именно в этом вызове - только их откатываем при ошибке enqueue.
	// старые pending из бд не трогаем, их recovery-worker подберёт
	var createdIDs []uuid.UUID

	// pending пусто - генерим новый раунд
	if len(matches) == 0 {
		ss.log.Info("No pending matches, generating new round",
			zap.String("tournament_id", tournamentID.String()),
		)

		// турнир из бд
		tournament, err := ss.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get tournament: %w", err)
		}

		// раунд гоняем только для активного турнира
		if tournament.Status != domain.TournamentActive {
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

		// сбрасываем ВСЕ игры до генерации, тк иначе может выйти частичный сброс,
		// если у какой-то игры остались running матчи
		for gameType := range participantsByGame {
			if err := ss.gameRepo.ResetGameByType(ctx, tournamentID, gameType); err != nil {
				return 0, fmt.Errorf("failed to reset game %s: %w", gameType, err)
			}
		}

		// генерим матчи по каждой игре отдельно
		for gameType, participants := range participantsByGame {
			if len(participants) < 2 {
				ss.log.Warn("Skipping game with fewer than 2 participants",
					zap.String("game_type", gameType),
					zap.Int("participants", len(participants)),
				)
				continue
			}

			roundNumber := 1

			gameMatches, err := ss.generateRoundRobinMatchesForGame(tournament, participants, gameType, roundNumber, domain.PriorityMedium, nil)
			if err != nil {
				return 0, fmt.Errorf("failed to generate matches for game %s: %w", gameType, err)
			}

			if len(gameMatches) == 0 {
				continue
			}

			if err := ss.matchRepo.CreateBatch(ctx, gameMatches); err != nil {
				// откатываем то что уже успели создать в этом вызове
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
				zap.Int("round_number", roundNumber),
				zap.Int("matches_count", len(gameMatches)),
			)
		}
	}

	// всё в очередь батчем (один pipeline).
	// при ошибке enqueue откатываем только свежесозданные;
	// старые pending оставляем, их подберёт recovery-worker
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

	// pending нет - сбрасываем старые результаты и генерим заново
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
		if tournament.Status != domain.TournamentActive {
			return 0, errors.ErrConflict.WithMessage("tournament is not active")
		}

		// сбрасываем прошлые матчи и рейтинги этой игры (если были).
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

		// после сброса раунд всегда начинается с 1
		roundNumber := 1

		// ручной запуск - высокий приоритет
		matches, err = ss.generateRoundRobinMatchesForGame(tournament, participants, gameType, roundNumber, domain.PriorityHigh, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to generate matches: %w", err)
		}

		if len(matches) == 0 {
			return 0, errors.ErrValidation.WithMessage("no matches generated for this game")
		}

		// сохраняем в бд
		if err := ss.matchRepo.CreateBatch(ctx, matches); err != nil {
			return 0, fmt.Errorf("failed to create matches: %w", err)
		}
		createdIDs = matchIDs(matches)

		ss.log.Info("Generated new round of matches for game",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", gameType),
			zap.Int("round_number", roundNumber),
			zap.Int("matches_count", len(matches)),
		)
	}

	// всё в очередь батчем (один pipeline).
	// при ошибке enqueue откатываем только свежесозданные;
	// старые pending оставляем recovery-worker'у
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

func (ss *SchedulingService) getLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.TournamentParticipant, error) {
	return ss.tournamentRepo.GetLatestParticipantsByGame(ctx, tournamentID, gameType)
}

// generateRoundRobinMatchesForGame - собирает матчи для одной игры.
// playedPairs это уже сыгранные пары "program1_id|program2_id", их пропускаем
func (ss *SchedulingService) generateRoundRobinMatchesForGame(tournament *domain.Tournament, participants []*domain.TournamentParticipant, gameType string, roundNumber int, priority domain.MatchPriority, playedPairs map[string]struct{}) ([]*domain.Match, error) {
	var matches []*domain.Match
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

			// пары что уже игрались с теми же программами - скипаем
			pairKey := participants[i].ProgramID.String() + "|" + participants[j].ProgramID.String()
			if _, played := playedPairs[pairKey]; played {
				continue
			}

			match := &domain.Match{
				ID:           uuid.New(),
				TournamentID: tournament.ID,
				Program1ID:   participants[i].ProgramID,
				Program2ID:   participants[j].ProgramID,
				GameType:     gameType,
				Status:       domain.MatchPending,
				Priority:     priority,
				RoundNumber:  roundNumber,
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

	// забираем pending и ставим в очередь
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
