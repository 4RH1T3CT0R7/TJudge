package tournament

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

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
	log             *logger.Logger
}

func NewSchedulingService(
	tournamentRepo TournamentRepository,
	matchRepo MatchRepository,
	queueManager QueueManager,
	gameRepo GameRepository,
	distributedLock DistributedLock,
	log *logger.Logger,
) *SchedulingService {
	return &SchedulingService{
		tournamentRepo:  tournamentRepo,
		matchRepo:       matchRepo,
		queueManager:    queueManager,
		gameRepo:        gameRepo,
		distributedLock: distributedLock,
		log:             log,
	}
}

// scheduleLockKey - общий лок планирования турнира. RunAll, RunGame (и авто-раунд через
// него), повтор упавших, ручной сброс раунда и завершение турнира под ним не
// пересекаются: иначе игра может получить два раунда, сброс - снести только что
// поставленный в очередь раунд, а завершённый турнир - новые матчи
func scheduleLockKey(tournamentID uuid.UUID) string {
	return "tournament:schedule:" + tournamentID.String()
}

// RunAllMatches - гоняет все pending матчи турнира (ручка админа).
// если pending нет - генерит новый раунд round-robin по всем играм
func (ss *SchedulingService) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var enqueued int
	lockErr := ss.distributedLock.WithLock(ctx, scheduleLockKey(tournamentID), 60*time.Second, func(ctx context.Context) error {
		var err error
		enqueued, err = ss.runAllMatchesLocked(ctx, tournamentID)
		return err
	})

	return enqueued, lockErr
}

func (ss *SchedulingService) runAllMatchesLocked(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// проверка до перепостановки: pending завершённого турнира в очередь не уходят
	tournament, err := ss.getActiveTournament(ctx, tournamentID)
	if err != nil {
		return 0, err
	}

	// недоигранный раунд: висящие pending просто ставятся в очередь заново
	pending, err := ss.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}
	if len(pending) > 0 {
		return ss.enqueue(ctx, tournamentID, pending)
	}

	ss.log.Info("No pending matches, generating new round",
		zap.String("tournament_id", tournamentID.String()),
	)

	// участники сгруппированы по играм, чтобы не сводить проги разных игр
	participantsByGame, err := ss.tournamentRepo.GetLatestParticipantsGroupedByGame(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get participants: %w", err)
	}

	// все проверки до сброса: игры, где играть некому, не сбрасываются вовсе
	var gameTypes []string
	var matches []*models.Match
	for _, gameType := range slices.Sorted(maps.Keys(participantsByGame)) {
		participants := participantsByGame[gameType]
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
		gameTypes = append(gameTypes, gameType)
		matches = append(matches, gameMatches...)
	}

	if len(matches) == 0 {
		return 0, errors.ErrValidation.WithMessage("need at least 2 participants to run matches")
	}

	return ss.startRound(ctx, tournamentID, gameTypes, matches)
}

// RunGameMatches - запускает матчи для одной конкретной игры турнира
func (ss *SchedulingService) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	var enqueued int
	lockErr := ss.distributedLock.WithLock(ctx, scheduleLockKey(tournamentID), 60*time.Second, func(ctx context.Context) error {
		var err error
		enqueued, err = ss.runGameMatchesLocked(ctx, tournamentID, gameType)
		return err
	})

	return enqueued, lockErr
}

func (ss *SchedulingService) runGameMatchesLocked(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	pending, matches, _, err := ss.planGameRound(ctx, tournamentID, gameType)
	if err != nil {
		return 0, err
	}
	if len(pending) > 0 {
		return ss.enqueue(ctx, tournamentID, pending)
	}

	ss.log.Info("No pending matches for game, resetting and generating new round",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_type", gameType),
	)

	return ss.startRound(ctx, tournamentID, []string{gameType}, matches)
}

// planGameRound - общий расчёт запуска раунда игры для RunGameMatches и его dry-run:
// висящие pending этой игры уходят в очередь как есть, иначе новый round-robin по
// последним готовым программам команд
func (ss *SchedulingService) planGameRound(ctx context.Context, tournamentID uuid.UUID, gameType string) (pending, matches []*models.Match, participants int, err error) {
	tournament, err := ss.getActiveTournament(ctx, tournamentID)
	if err != nil {
		return nil, nil, 0, err
	}

	pending, err = ss.matchRepo.GetPendingByTournamentAndGame(ctx, tournamentID, gameType)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get pending matches: %w", err)
	}
	if len(pending) > 0 {
		return pending, nil, 0, nil
	}

	list, err := ss.tournamentRepo.GetLatestParticipantsByGame(ctx, tournamentID, gameType)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get participants: %w", err)
	}
	if len(list) < 2 {
		return nil, nil, 0, errors.ErrValidation.WithMessage("need at least 2 participants with programs for this game")
	}

	// ручной запуск - высокий приоритет
	matches, err = ss.generateRoundRobinMatchesForGame(tournament, list, gameType, models.PriorityHigh)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to generate matches: %w", err)
	}
	return nil, matches, len(list), nil
}

// RoundPreview - dry-run запуска раунда игры для диалога подтверждения
type RoundPreview struct {
	GameType string `json:"game_type"`
	// Pending - матчи недоигранного раунда: запуск только вернёт их в очередь,
	// новый раунд не создаётся и прошлые результаты не трогаются
	Pending int `json:"pending"`
	// Participants - команды с готовой программой, играющие новый раунд
	Participants   int   `json:"participants"`
	MatchesCreated int   `json:"matches_created"`
	MatchesDeleted int64 `json:"matches_deleted"` // матчи прошлого раунда вместе с историей рейтинга
}

// PreviewGameRound - dry-run RunGameMatches: тот же расчёт раунда, а число удаляемых
// матчей даёт сброс игры в откатываемой транзакции (с той же проверкой идущих матчей).
// лок планирования не берётся: база не меняется, а занятый лок мешал бы настоящему запуску
func (ss *SchedulingService) PreviewGameRound(ctx context.Context, tournamentID uuid.UUID, gameType string) (*RoundPreview, error) {
	pending, matches, participants, err := ss.planGameRound(ctx, tournamentID, gameType)
	if err != nil {
		return nil, err
	}

	preview := &RoundPreview{
		GameType:       gameType,
		Pending:        len(pending),
		Participants:   participants,
		MatchesCreated: len(matches),
	}
	if len(pending) == 0 {
		preview.MatchesDeleted, _, _, err = ss.gameRepo.PreviewGameReset(ctx, tournamentID, gameType)
		if err != nil {
			return nil, err
		}
	}
	return preview, nil
}

// startRound одной транзакцией заменяет прошлые результаты игр новым раундом и ставит
// его в очередь
func (ss *SchedulingService) startRound(ctx context.Context, tournamentID uuid.UUID, gameTypes []string, matches []*models.Match) (int, error) {
	if err := ss.gameRepo.StartNewRound(ctx, tournamentID, gameTypes, matches); err != nil {
		return 0, fmt.Errorf("failed to start new round: %w", err)
	}

	ss.log.Info("Generated new round of matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Strings("game_types", gameTypes),
		zap.Int("matches_count", len(matches)),
	)

	// раунд уже в бд: отмена запроса (клиент ушёл, таймаут) не должна оборвать
	// постановку в очередь. при сбое очереди матчи не откатываются: сброс прошлых
	// результатов уже закоммичен, и откат оставил бы игру пустой. pending-матчи
	// поставит в очередь периодический recovery воркера
	if _, err := ss.enqueue(context.WithoutCancel(ctx), tournamentID, matches); err != nil {
		ss.log.Error("Round created but not enqueued, worker recovery will enqueue it",
			zap.Error(err),
			zap.String("tournament_id", tournamentID.String()),
			zap.Int("matches_count", len(matches)),
		)
	}
	return len(matches), nil
}

// enqueue ставит матчи в очередь батчем (один pipeline)
func (ss *SchedulingService) enqueue(ctx context.Context, tournamentID uuid.UUID, matches []*models.Match) (int, error) {
	if err := ss.queueManager.EnqueueBatch(ctx, matches); err != nil {
		return 0, fmt.Errorf("failed to enqueue matches: %w", err)
	}

	ss.log.Info("Matches enqueued",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("enqueued", len(matches)),
	)

	return len(matches), nil
}

// getActiveTournament - турнир из бд; раунды и повторы только для активного
func (ss *SchedulingService) getActiveTournament(ctx context.Context, tournamentID uuid.UUID) (*models.Tournament, error) {
	tournament, err := ss.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament.Status != models.TournamentActive {
		return nil, errors.ErrConflict.WithMessage("tournament is not active")
	}
	return tournament, nil
}

// generateRoundRobinMatchesForGame - собирает матчи для одной игры.
// раунд всегда первый: новый раунд начинается со сброса прошлых результатов игры
func (ss *SchedulingService) generateRoundRobinMatchesForGame(tournament *models.Tournament, participants []*models.TournamentParticipant, gameType string, priority models.MatchPriority) ([]*models.Match, error) {
	var matches []*models.Match
	now := time.Now()

	// каждый с каждым, обе стороны играют (AB и BA) -
	// порядок важен для несимметричных игр
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

// ResetGameRound - ручной сброс раунда игры (кнопка админа) под локом планирования
func (ss *SchedulingService) ResetGameRound(ctx context.Context, tournamentID uuid.UUID, gameType string) (matchesDeleted, participantsReset, ratingHistoryDeleted int64, err error) {
	err = ss.distributedLock.WithLock(ctx, scheduleLockKey(tournamentID), 60*time.Second, func(ctx context.Context) error {
		var resetErr error
		matchesDeleted, participantsReset, ratingHistoryDeleted, resetErr = ss.gameRepo.ResetGameRoundFull(ctx, tournamentID, gameType)
		return resetErr
	})
	return
}

// RetryFailedMatches - переводит упавшие матчи обратно в pending и ставит в очередь.
// только для активного турнира и под локом планирования
func (ss *SchedulingService) RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var enqueued int
	lockErr := ss.distributedLock.WithLock(ctx, scheduleLockKey(tournamentID), 60*time.Second, func(ctx context.Context) error {
		if _, err := ss.getActiveTournament(ctx, tournamentID); err != nil {
			return err
		}

		// все failed -> pending
		resetCount, err := ss.matchRepo.ResetFailedMatches(ctx, tournamentID)
		if err != nil {
			return fmt.Errorf("failed to reset failed matches: %w", err)
		}
		if resetCount == 0 {
			return nil
		}

		matches, err := ss.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
		if err != nil {
			return fmt.Errorf("failed to get pending matches: %w", err)
		}

		ss.log.Info("Admin retried failed matches",
			zap.String("tournament_id", tournamentID.String()),
			zap.Int64("reset_count", resetCount),
		)

		enqueued, err = ss.enqueue(ctx, tournamentID, matches)
		return err
	})

	return enqueued, lockErr
}
