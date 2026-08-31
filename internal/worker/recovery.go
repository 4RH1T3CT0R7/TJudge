package worker

import (
	"context"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RecoveryMatchRepository - матчи для восстановления
type RecoveryMatchRepository interface {
	GetPending(ctx context.Context, limit int) ([]*models.Match, error)
	GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*models.Match, error)
	BatchUpdateStatus(ctx context.Context, matchIDs []uuid.UUID, status models.MatchStatus) error
}

// RecoveryQueueManager - постановка матчей обратно в очередь
type RecoveryQueueManager interface {
	Enqueue(ctx context.Context, match *models.Match) error
	GetTotalQueueSize(ctx context.Context) (int64, error)
}

// RecoveryService возвращает застрявшие матчи в работу: если воркер умер
// посреди матча, матч навсегда остался бы running - этот сервис сбрасывает
// такие обратно в pending и перезакидывает в очередь
type RecoveryService struct {
	matchRepo    RecoveryMatchRepository
	queueManager RecoveryQueueManager
	log          *logger.Logger

	stuckDuration    time.Duration // сколько running считается застрявшим
	batchSize        int
	periodicInterval time.Duration

	// mu не даёт startup- и periodic-восстановлению перекрыться
	mu sync.Mutex

	stopCh chan struct{}
}

// RecoveryConfig - настройки восстановления
type RecoveryConfig struct {
	StuckDuration    time.Duration
	BatchSize        int
	PeriodicInterval time.Duration
}

// NewRecoveryService создаёт сервис. реальные пороги задаются из main
// (120с > таймаута воркера), дефолты тут скорее на всякий случай
func NewRecoveryService(
	matchRepo RecoveryMatchRepository,
	queueManager RecoveryQueueManager,
	log *logger.Logger,
	cfg RecoveryConfig,
) *RecoveryService {
	if cfg.StuckDuration == 0 {
		cfg.StuckDuration = 10 * time.Minute
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 1000
	}
	if cfg.PeriodicInterval == 0 {
		cfg.PeriodicInterval = 5 * time.Minute
	}

	return &RecoveryService{
		matchRepo:        matchRepo,
		queueManager:     queueManager,
		log:              log,
		stuckDuration:    cfg.StuckDuration,
		batchSize:        cfg.BatchSize,
		periodicInterval: cfg.PeriodicInterval,
		stopCh:           make(chan struct{}),
	}
}

// RecoverOnStartup - восстановление при старте воркера: сперва застрявшие
// running сбрасываются в pending, потом все pending уходят в очередь
func (s *RecoveryService) RecoverOnStartup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info("Starting match recovery...")

	queueSize, err := s.queueManager.GetTotalQueueSize(ctx)
	if err != nil {
		s.log.LogError("Failed to get queue size during recovery", err)
		// не критично, дальше по плану
	} else {
		s.log.Info("Current queue size before recovery", zap.Int64("queue_size", queueSize))
	}

	stuckRecovered, err := s.recoverStuckRunning(ctx)
	if err != nil {
		s.log.LogError("Failed to recover stuck running matches", err)
		// pending всё равно стоит прогнать
	}

	pendingEnqueued, err := s.enqueuePendingMatches(ctx)
	if err != nil {
		return err
	}

	s.log.Info("Match recovery completed",
		zap.Int("stuck_recovered", stuckRecovered),
		zap.Int("pending_enqueued", pendingEnqueued),
	)

	return nil
}

// recoverStuckRunning сбрасывает застрявшие running в pending
// FIXME: если застрявших больше batchSize, за раз берётся только первая пачка,
// остальные подождут следующего тика
func (s *RecoveryService) recoverStuckRunning(ctx context.Context) (int, error) {
	stuckMatches, err := s.matchRepo.GetStuckRunning(ctx, s.stuckDuration, s.batchSize)
	if err != nil {
		return 0, err
	}

	if len(stuckMatches) == 0 {
		s.log.Info("No stuck running matches found")
		return 0, nil
	}

	s.log.Info("Found stuck running matches",
		zap.Int("count", len(stuckMatches)),
		zap.Duration("stuck_threshold", s.stuckDuration),
	)

	matchIDs := make([]uuid.UUID, len(stuckMatches))
	for i, match := range stuckMatches {
		matchIDs[i] = match.ID
		s.log.Debug("Recovering stuck match",
			zap.String("match_id", match.ID.String()),
			zap.Time("started_at", *match.StartedAt),
		)
	}

	if err := s.matchRepo.BatchUpdateStatus(ctx, matchIDs, models.MatchPending); err != nil {
		return 0, err
	}

	s.log.Info("Reset stuck matches to pending",
		zap.Int("count", len(matchIDs)),
	)

	return len(matchIDs), nil
}

// enqueuePendingMatches закидывает pending матчи из базы в очередь редиса
func (s *RecoveryService) enqueuePendingMatches(ctx context.Context) (int, error) {
	pendingMatches, err := s.matchRepo.GetPending(ctx, s.batchSize)
	if err != nil {
		return 0, err
	}

	if len(pendingMatches) == 0 {
		s.log.Info("No pending matches to enqueue")
		return 0, nil
	}

	s.log.Info("Found pending matches to enqueue",
		zap.Int("count", len(pendingMatches)),
	)

	// ошибка на одном матче не валит остальные
	enqueued := 0
	for _, match := range pendingMatches {
		if err := s.queueManager.Enqueue(ctx, match); err != nil {
			s.log.LogError("Failed to enqueue match during recovery", err,
				zap.String("match_id", match.ID.String()),
			)
			continue
		}
		enqueued++
	}

	s.log.Info("Enqueued pending matches",
		zap.Int("enqueued", enqueued),
		zap.Int("total", len(pendingMatches)),
	)

	return enqueued, nil
}

// Start запускает периодическую проверку в фоне
func (s *RecoveryService) Start() {
	s.log.Info("Starting periodic recovery service",
		zap.Duration("interval", s.periodicInterval),
		zap.Duration("stuck_threshold", s.stuckDuration),
	)

	go s.runPeriodic()
}

// Stop гасит периодику. без join - горутина дозакончит сама
func (s *RecoveryService) Stop() {
	s.log.Info("Stopping periodic recovery service...")
	close(s.stopCh)
}

func (s *RecoveryService) runPeriodic() {
	ticker := time.NewTicker(s.periodicInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			s.log.Info("Periodic recovery service stopped")
			return
		case <-ticker.C:
			s.runPeriodicRecovery()
		}
	}
}

func (s *RecoveryService) runPeriodicRecovery() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// периодика трогает только застрявшие running - pending уже в очереди
	// после startup-восстановления
	stuckRecovered, err := s.recoverStuckRunning(ctx)
	if err != nil {
		s.log.LogError("Periodic recovery failed", err)
		return
	}

	if stuckRecovered > 0 {
		// появились новые сброшенные - их надо и в очередь
		enqueued, err := s.enqueuePendingMatches(ctx)
		if err != nil {
			s.log.LogError("Failed to enqueue recovered matches", err)
			return
		}

		s.log.Info("Periodic recovery completed",
			zap.Int("stuck_recovered", stuckRecovered),
			zap.Int("enqueued", enqueued),
		)
	}
}
