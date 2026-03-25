package tournament

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// AutoRoundScheduler периодически проверяет игры с включённым авто-раундом
// и запускает новые раунды матчей после завершения предыдущих.
type AutoRoundScheduler struct {
	schedulingService *SchedulingService
	gameRepo          GameRepository
	distributedLock   DistributedLock
	log               *logger.Logger

	pollInterval time.Duration
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// NewAutoRoundScheduler создаёт новый планировщик авто-раундов
func NewAutoRoundScheduler(
	schedulingService *SchedulingService,
	gameRepo GameRepository,
	distributedLock DistributedLock,
	log *logger.Logger,
	pollInterval time.Duration,
) *AutoRoundScheduler {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	return &AutoRoundScheduler{
		schedulingService: schedulingService,
		gameRepo:          gameRepo,
		distributedLock:   distributedLock,
		log:               log,
		pollInterval:      pollInterval,
		stopCh:            make(chan struct{}),
	}
}

// Start запускает планировщик в фоновой горутине
func (s *AutoRoundScheduler) Start(ctx context.Context) {
	s.log.Info("Starting auto-round scheduler",
		zap.Duration("poll_interval", s.pollInterval),
	)
	go s.run(ctx)
}

// Stop останавливает планировщик (безопасно вызывать несколько раз)
func (s *AutoRoundScheduler) Stop() {
	s.stopOnce.Do(func() {
		s.log.Info("Stopping auto-round scheduler...")
		close(s.stopCh)
	})
}

func (s *AutoRoundScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Auto-round scheduler stopped (context cancelled)")
			return
		case <-s.stopCh:
			s.log.Info("Auto-round scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick — одна итерация проверки всех авто-раунд игр
func (s *AutoRoundScheduler) tick(ctx context.Context) {
	games, err := s.gameRepo.GetAutoRoundEnabledGames(ctx)
	if err != nil {
		s.log.Error("Auto-round: failed to get enabled games", zap.Error(err))
		return
	}

	for _, g := range games {
		s.processGame(ctx, g)
	}
}

// processGame обрабатывает одну игру с включённым авто-раундом
func (s *AutoRoundScheduler) processGame(ctx context.Context, g *domain.AutoRoundGameInfo) {
	// 1. Есть ли active (pending/running) матчи для этой игры?
	hasActive, err := s.gameRepo.HasActiveMatchesForGame(ctx, g.TournamentID, g.GameType)
	if err != nil {
		s.log.Error("Auto-round: failed to check active matches",
			zap.Error(err),
			zap.String("tournament_id", g.TournamentID.String()),
			zap.String("game_type", g.GameType),
		)
		return
	}
	if hasActive {
		return // матчи ещё выполняются, ждём
	}

	// 2. Прошло ли достаточно времени с последнего раунда (cooldown)?
	if g.LastRunAt != nil {
		elapsed := time.Since(*g.LastRunAt)
		if elapsed < time.Duration(g.IntervalSeconds)*time.Second {
			return // cooldown ещё не прошёл
		}
	}

	// 3. Есть ли новые программы с последнего раунда?
	since := time.Time{} // beginning of time if first run
	if g.LastRunAt != nil {
		since = *g.LastRunAt
	}
	hasNew, err := s.gameRepo.HasNewProgramsSince(ctx, g.TournamentID, g.GameType, since)
	if err != nil {
		s.log.Error("Auto-round: failed to check new programs",
			zap.Error(err),
			zap.String("tournament_id", g.TournamentID.String()),
			zap.String("game_type", g.GameType),
		)
		return
	}
	if !hasNew && g.LastRunAt != nil {
		return // нет новых программ, нечего перезапускать
	}

	// 4. Запускаем раунд через существующий RunGameMatches (он сам берёт distributed lock)
	lockKey := fmt.Sprintf("tournament:autoround:%s:%s", g.TournamentID.String(), g.GameType)
	lockErr := s.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		enqueued, err := s.schedulingService.RunGameMatches(ctx, g.TournamentID, g.GameType)
		if err != nil {
			return err
		}

		// Обновляем timestamp последнего запуска
		if updateErr := s.gameRepo.UpdateAutoRoundLastRun(ctx, g.TournamentID, g.GameID); updateErr != nil {
			s.log.Error("Auto-round: failed to update last run timestamp",
				zap.Error(updateErr),
				zap.String("tournament_id", g.TournamentID.String()),
				zap.String("game_type", g.GameType),
			)
		}

		s.log.Info("Auto-round triggered",
			zap.String("tournament_id", g.TournamentID.String()),
			zap.String("game_type", g.GameType),
			zap.Int("matches_enqueued", enqueued),
		)
		return nil
	})

	if lockErr != nil {
		// Lock не получен или ошибка — это нормально (другой процесс уже обрабатывает)
		s.log.Debug("Auto-round: skipped (lock or error)",
			zap.Error(lockErr),
			zap.String("tournament_id", g.TournamentID.String()),
			zap.String("game_type", g.GameType),
		)
	}
}
