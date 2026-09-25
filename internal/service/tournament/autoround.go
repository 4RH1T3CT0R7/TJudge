package tournament

import (
	"context"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// AutoRoundScheduler - раз в N секунд смотрит игры с включённым авто-раундом
// и запускает новый раунд, когда предыдущий доигран
type AutoRoundScheduler struct {
	schedulingService *SchedulingService
	gameRepo          GameRepository
	log               *logger.Logger

	pollInterval time.Duration
	stopCh       chan struct{}
	stopOnce     sync.Once
}

func NewAutoRoundScheduler(
	schedulingService *SchedulingService,
	gameRepo GameRepository,
	log *logger.Logger,
	pollInterval time.Duration,
) *AutoRoundScheduler {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	return &AutoRoundScheduler{
		schedulingService: schedulingService,
		gameRepo:          gameRepo,
		log:               log,
		pollInterval:      pollInterval,
		stopCh:            make(chan struct{}),
	}
}

// Start - поднимает планировщик в фоновой горутине
func (s *AutoRoundScheduler) Start(ctx context.Context) {
	s.log.Info("Starting auto-round scheduler",
		zap.Duration("poll_interval", s.pollInterval),
	)
	go s.run(ctx)
}

// Stop - гасит планировщик, безопасно звать несколько раз (sync.Once)
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

// tick - один проход по всем авто-раунд играм
func (s *AutoRoundScheduler) tick(ctx context.Context) {
	games, err := s.gameRepo.GetAutoRoundEnabledGames(ctx)
	if err != nil {
		s.log.Error("Auto-round: failed to get enabled games", zap.Error(err))
		return
	}

	// TODO: все игры дёргаются по очереди, при большом кол-ве стоило бы пачками
	for _, g := range games {
		s.processGame(ctx, g)
	}
}

// processGame - обрабатывает одну игру с авто-раундом.
// порядок проверок важен, не переставлять
func (s *AutoRoundScheduler) processGame(ctx context.Context, g *models.AutoRoundGameInfo) {
	// 1. есть ли активные (pending/running) матчи по этой игре?
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
		return // ещё крутятся, надо ждать
	}

	// 2. прошёл ли cooldown с прошлого раунда?
	if g.LastRunAt != nil {
		elapsed := time.Since(*g.LastRunAt)
		if elapsed < time.Duration(g.IntervalSeconds)*time.Second {
			return // рано ещё
		}
	}

	// 3. появились ли новые проги после прошлого раунда?
	since := time.Time{} // на первом запуске - от начала времён
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
		return // новых прог нет, перезапускать нечего
	}

	// 4. запуск раунда через RunGameMatches, лок планирования турнира он берёт сам
	enqueued, err := s.schedulingService.RunGameMatches(ctx, g.TournamentID, g.GameType)
	if err != nil {
		// 4xx - штатные ситуации (лок занят, не хватает участников, идут матчи),
		// остальное - реальная поломка, её должно быть видно в логах
		logFn := s.log.Warn
		if appErr := errors.GetAppError(err); appErr != nil && appErr.Code < 500 {
			logFn = s.log.Debug
		}
		logFn("Auto-round: round not started",
			zap.Error(err),
			zap.String("tournament_id", g.TournamentID.String()),
			zap.String("game_type", g.GameType),
		)
		return
	}

	// обновляется время последнего запуска
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
}
