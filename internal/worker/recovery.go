package worker

import (
	"context"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// RecoveryService возвращает застрявшие матчи в работу: если воркер умер
// посреди матча, матч навсегда остался бы running - этот сервис сбрасывает
// такие обратно в pending и перезакидывает в очередь
type RecoveryService struct {
	matchRepo    MatchRepository
	queueManager QueueManager
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
// (порог застревания - WorkerConfig.StuckThreshold), дефолты тут на всякий случай
func NewRecoveryService(
	matchRepo MatchRepository,
	queueManager QueueManager,
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
	recovered, err := s.matchRepo.ResetStuckRunning(ctx, s.stuckDuration, s.batchSize)
	if err != nil {
		return 0, err
	}

	if recovered == 0 {
		s.log.Debug("No stuck running matches found")
		return 0, nil
	}

	s.log.Info("Reset stuck matches to pending",
		zap.Int64("count", recovered),
		zap.Duration("stuck_threshold", s.stuckDuration),
	)

	return int(recovered), nil
}

// enqueuePendingMatches закидывает pending матчи из базы в очередь редиса.
// повтор безопасен: матч, который уже лежит в очереди, отсекает dedup-ключ.
// ponytail: берётся только первая пачка (batchSize) по приоритету и возрасту;
// выпавшие из очереди матчи старше остальных, поэтому попадают в неё первыми
func (s *RecoveryService) enqueuePendingMatches(ctx context.Context) (int, error) {
	pendingMatches, err := s.matchRepo.GetPending(ctx, s.batchSize)
	if err != nil {
		return 0, err
	}

	if len(pendingMatches) == 0 {
		s.log.Debug("No pending matches to enqueue")
		return 0, nil
	}

	if err := s.queueManager.EnqueueBatch(ctx, pendingMatches); err != nil {
		return 0, err
	}

	s.log.Info("Pending matches passed to queue",
		zap.Int("count", len(pendingMatches)),
	)

	return len(pendingMatches), nil
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

	stuckRecovered, err := s.recoverStuckRunning(ctx)
	if err != nil {
		s.log.LogError("Periodic recovery failed", err)
		// pending всё равно стоит прогнать
	}

	// pending ставятся в очередь на каждом тике, а не только после сброса
	// застрявших: матч, исчерпавший ретраи пула на инфра-ошибке, остаётся
	// pending вне очереди и без этого ждал бы рестарта воркера
	enqueued, err := s.enqueuePendingMatches(ctx)
	if err != nil {
		s.log.LogError("Failed to enqueue pending matches", err)
		return
	}

	if stuckRecovered > 0 {
		s.log.Info("Periodic recovery completed",
			zap.Int("stuck_recovered", stuckRecovered),
			zap.Int("enqueued", enqueued),
		)
	}
}
