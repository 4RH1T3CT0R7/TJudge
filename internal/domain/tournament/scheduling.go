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

// ProgramRepository интерфейс для работы с программами (для оптимизированного round-robin)
type ProgramRepository interface {
	GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error)
}

// ScheduleNewProgramMatchesRequest запрос на создание матчей для новой программы
type ScheduleNewProgramMatchesRequest struct {
	TournamentID uuid.UUID
	GameID       uuid.UUID
	NewProgramID uuid.UUID
	TeamID       uuid.UUID
}

// SchedulingService управляет планированием матчей: генерация round-robin,
// раунды по отдельным играм, перезапуск упавших матчей и on-demand планирование
// для новых программ.
type SchedulingService struct {
	tournamentRepo  TournamentRepository
	matchRepo       MatchRepository
	queueManager    QueueManager
	gameRepo        GameRepository
	distributedLock DistributedLock
	eventBus        events.Bus
	log             *logger.Logger
}

// matchIDs извлекает ID из списка матчей - используется для rollback/компенсации.
func matchIDs(matches []*domain.Match) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	return ids
}

// NewSchedulingService создаёт новый сервис планирования матчей
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

// ScheduleNewProgramMatches создаёт матчи для новой программы против всех существующих
// Это оптимизированный round-robin - вместо генерации всех матчей заново,
// создаются только матчи с новой программой
func (ss *SchedulingService) ScheduleNewProgramMatches(ctx context.Context, req *ScheduleNewProgramMatchesRequest, programRepo ProgramRepository) error {
	// Используем distributed lock для предотвращения гонок при создании матчей
	lockKey := fmt.Sprintf("tournament:schedule:%s:%s", req.TournamentID.String(), req.GameID.String())

	return ss.distributedLock.WithLock(ctx, lockKey, 10*time.Second, func(ctx context.Context) error {
		// Получаем турнир напрямую из БД
		tournament, err := ss.tournamentRepo.GetByID(ctx, req.TournamentID)
		if err != nil {
			return err
		}

		// Проверяем статус турнира
		if tournament.Status != domain.TournamentActive && tournament.Status != domain.TournamentPending {
			return errors.ErrConflict.WithMessage("cannot schedule matches for completed tournament")
		}

		// Получаем все программы в турнире для данной игры
		programs, err := programRepo.GetByTournamentAndGame(ctx, req.TournamentID, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get programs: %w", err)
		}

		// Создаём матчи только против других программ (не своей команды)
		var matches []*domain.Match
		now := time.Now()

		for _, prog := range programs {
			// Пропускаем свою программу и программы своей команды
			if prog.ID == req.NewProgramID {
				continue
			}
			if prog.TeamID != nil && *prog.TeamID == req.TeamID {
				continue
			}

			// Матч 1: новая программа как Program1, существующая как Program2
			match1 := &domain.Match{
				ID:           uuid.New(),
				TournamentID: req.TournamentID,
				Program1ID:   req.NewProgramID,
				Program2ID:   prog.ID,
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityHigh, // Новые матчи с высоким приоритетом
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

			// Матч 2: существующая программа как Program1, новая как Program2
			match2 := &domain.Match{
				ID:           uuid.New(),
				TournamentID: req.TournamentID,
				Program1ID:   prog.ID,
				Program2ID:   req.NewProgramID,
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityHigh, // Новые матчи с высоким приоритетом
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

		// Создаём матчи в БД
		if err := ss.matchRepo.CreateBatch(ctx, matches); err != nil {
			return fmt.Errorf("failed to create matches: %w", err)
		}

		// Добавляем матчи в очередь (batch - один Redis pipeline).
		// При ошибке Enqueue откатываем матчи из БД, иначе они останутся
		// как "pending" навсегда, не попав в очередь обработки.
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

		// Публикуем событие (broadcast обрабатывается в обработчиках событий)
		ss.eventBus.Publish(ctx, events.MatchesCreated{
			Version:      1,
			TournamentID: req.TournamentID,
			ProgramID:    req.NewProgramID,
			MatchCount:   len(matches),
		})

		return nil
	})
}

// RunAllMatches запускает все pending матчи турнира (для админа)
// Если нет pending матчей, создаёт новый раунд round-robin матчей
func (ss *SchedulingService) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// Используем distributed lock для предотвращения дублирования матчей
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
	// Получаем все pending матчи
	matches, err := ss.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// IDs матчей, созданных именно в этом вызове (для rollback при ошибке Enqueue).
	// Существующие pending-матчи из БД мы не удаляем - recovery-worker их подберёт.
	var createdIDs []uuid.UUID

	// Если нет pending матчей, создаём новый раунд
	if len(matches) == 0 {
		ss.log.Info("No pending matches, generating new round",
			zap.String("tournament_id", tournamentID.String()),
		)

		// Получаем турнир напрямую из БД
		tournament, err := ss.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get tournament: %w", err)
		}

		// Проверяем что турнир активен
		if tournament.Status != domain.TournamentActive {
			return 0, errors.ErrConflict.WithMessage("tournament is not active")
		}

		// Получаем участников сгруппированных по играм (чтобы не сводить программы разных игр)
		participantsByGame, err := ss.tournamentRepo.GetLatestParticipantsGroupedByGame(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get participants: %w", err)
		}

		if len(participantsByGame) == 0 {
			return 0, errors.ErrValidation.WithMessage("need at least 2 participants to run matches")
		}

		// Предварительная проверка: сбрасываем все игры до генерации матчей.
		// Это предотвращает частичный сброс, если одна из игр имеет running матчи.
		for gameType := range participantsByGame {
			if err := ss.gameRepo.ResetGameByType(ctx, tournamentID, gameType); err != nil {
				return 0, fmt.Errorf("failed to reset game %s: %w", gameType, err)
			}
		}

		// Генерируем матчи отдельно для каждой игры
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
				// Откатываем уже созданные в этом вызове матчи
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

	// Добавляем все матчи в очередь (batch - один Redis pipeline).
	// При ошибке Enqueue откатываем только свежесозданные матчи;
	// существующие pending оставляем - recovery-worker их подберёт.
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

// RunGameMatches запускает матчи для конкретной игры в турнире
func (ss *SchedulingService) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	// Используем distributed lock для предотвращения дублирования матчей
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
	// Получаем pending матчи для конкретной игры
	matches, err := ss.matchRepo.GetPendingByTournamentAndGame(ctx, tournamentID, gameType)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// createdIDs - IDs матчей, созданных именно в этом вызове (для rollback).
	var createdIDs []uuid.UUID

	// Если нет pending матчей, сбрасываем предыдущие результаты и генерируем заново
	if len(matches) == 0 {
		ss.log.Info("No pending matches for game, resetting and generating new round",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", gameType),
		)

		// Получаем турнир напрямую из БД
		tournament, err := ss.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get tournament: %w", err)
		}

		// Проверяем что турнир активен
		if tournament.Status != domain.TournamentActive {
			return 0, errors.ErrConflict.WithMessage("tournament is not active")
		}

		// Сбрасываем предыдущие матчи и рейтинги для этой игры (если есть).
		// При повторном запуске игры новые результаты заменяют старые.
		if err := ss.gameRepo.ResetGameByType(ctx, tournamentID, gameType); err != nil {
			return 0, fmt.Errorf("failed to reset game %s before generating matches: %w", gameType, err)
		}

		// Получаем участников (только последние версии программ каждой команды для этой игры)
		participants, err := ss.getLatestParticipantsByGame(ctx, tournamentID, gameType)
		if err != nil {
			return 0, fmt.Errorf("failed to get participants: %w", err)
		}

		if len(participants) < 2 {
			return 0, errors.ErrValidation.WithMessage("need at least 2 participants with programs for this game")
		}

		// После сброса round_number всегда начинается с 1
		roundNumber := 1

		// Генерируем матчи для этой игры с высоким приоритетом (ручной запуск)
		matches, err = ss.generateRoundRobinMatchesForGame(tournament, participants, gameType, roundNumber, domain.PriorityHigh, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to generate matches: %w", err)
		}

		if len(matches) == 0 {
			return 0, errors.ErrValidation.WithMessage("no matches generated for this game")
		}

		// Сохраняем матчи в БД
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

	// Добавляем все матчи в очередь (batch - один Redis pipeline).
	// При ошибке Enqueue откатываем только свежесозданные матчи;
	// существующие pending оставляем - recovery-worker их подберёт.
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

// getLatestParticipantsByGame получает последние версии программ участников для конкретной игры
func (ss *SchedulingService) getLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.TournamentParticipant, error) {
	return ss.tournamentRepo.GetLatestParticipantsByGame(ctx, tournamentID, gameType)
}

// generateRoundRobinMatchesForGame генерирует матчи для конкретной игры.
// playedPairs содержит пары "program1_id|program2_id", которые уже были сыграны - они пропускаются.
func (ss *SchedulingService) generateRoundRobinMatchesForGame(tournament *domain.Tournament, participants []*domain.TournamentParticipant, gameType string, roundNumber int, priority domain.MatchPriority, playedPairs map[string]struct{}) ([]*domain.Match, error) {
	var matches []*domain.Match
	now := time.Now()

	// Каждый участник играет с каждым в обе стороны (AB и BA)
	for i := range participants {
		for j := range participants {
			// Пропускаем матч против себя
			if i == j {
				continue
			}

			// Пропускаем пары, которые уже были сыграны с теми же программами
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

// RetryFailedMatches сбрасывает failed матчи в pending и ставит их в очередь
func (ss *SchedulingService) RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// Сбрасываем все failed матчи в pending
	resetCount, err := ss.matchRepo.ResetFailedMatches(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to reset failed matches: %w", err)
	}

	if resetCount == 0 {
		return 0, nil
	}

	// Получаем все pending матчи и ставим в очередь
	matches, err := ss.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// Добавляем все матчи в очередь (batch - один Redis pipeline)
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
