package worker

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RecoveryMatchRepository интерфейс для работы с матчами при восстановлении
type RecoveryMatchRepository interface {
	GetPending(ctx context.Context, limit int) ([]*domain.Match, error)
	GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*domain.Match, error)
	BatchUpdateStatus(ctx context.Context, matchIDs []uuid.UUID, status domain.MatchStatus) error
}

// RecoveryQueueManager интерфейс для добавления матчей в очередь
type RecoveryQueueManager interface {
	Enqueue(ctx context.Context, match *domain.Match) error
	GetTotalQueueSize(ctx context.Context) (int64, error)
}

// RecoveryService сервис восстановления застрявших матчей
type RecoveryService struct {
	matchRepo    RecoveryMatchRepository
	queueManager RecoveryQueueManager
	log          *logger.Logger

	// Конфигурация
	stuckDuration    time.Duration // Время, после которого running матч считается застрявшим
	batchSize        int           // Размер батча для восстановления
	periodicInterval time.Duration // Интервал периодической проверки

	// Для graceful shutdown
	stopCh chan struct{}
}

// RecoveryConfig конфигурация сервиса восстановления
type RecoveryConfig struct {
	StuckDuration    time.Duration // По умолчанию 10 минут
	BatchSize        int           // По умолчанию 1000
	PeriodicInterval time.Duration // Интервал периодической проверки (0 = отключено)
}

// NewRecoveryService создаёт новый сервис восстановления
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

// RecoverOnStartup выполняет восстановление при запуске worker'а
// 1. Сбрасывает "застрявшие" running матчи в pending
// 2. Добавляет все pending матчи в очередь Redis
func (s *RecoveryService) RecoverOnStartup(ctx context.Context) error {
	s.log.Info("Starting match recovery...")

	// Проверяем текущий размер очереди
	queueSize, err := s.queueManager.GetTotalQueueSize(ctx)
	if err != nil {
		s.log.LogError("Failed to get queue size during recovery", err)
		// Продолжаем, это не критичная ошибка
	} else {
		s.log.Info("Current queue size before recovery", zap.Int64("queue_size", queueSize))
	}

	// 1. Восстанавливаем застрявшие running матчи
	stuckRecovered, err := s.recoverStuckRunning(ctx)
	if err != nil {
		s.log.LogError("Failed to recover stuck running matches", err)
		// Продолжаем с pending матчами
	}

	// 2. Добавляем pending матчи в очередь
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

// recoverStuckRunning сбрасывает застрявшие running матчи в pending
func (s *RecoveryService) recoverStuckRunning(ctx context.Context) (int, error) {
	// Получаем застрявшие running матчи
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

	// Собираем ID для batch update
	matchIDs := make([]uuid.UUID, len(stuckMatches))
	for i, match := range stuckMatches {
		matchIDs[i] = match.ID
		s.log.Debug("Recovering stuck match",
			zap.String("match_id", match.ID.String()),
			zap.Time("started_at", *match.StartedAt),
		)
	}

	// Сбрасываем статус в pending
	if err := s.matchRepo.BatchUpdateStatus(ctx, matchIDs, domain.MatchPending); err != nil {
		return 0, err
	}

	s.log.Info("Reset stuck matches to pending",
		zap.Int("count", len(matchIDs)),
	)

	return len(matchIDs), nil
}

// enqueuePendingMatches добавляет все pending матчи из БД в очередь Redis
func (s *RecoveryService) enqueuePendingMatches(ctx context.Context) (int, error) {
	// Получаем pending матчи
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

	// Добавляем в очередь
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

// Start запускает периодическое восстановление в фоне
func (s *RecoveryService) Start() {
	s.log.Info("Starting periodic recovery service",
		zap.Duration("interval", s.periodicInterval),
		zap.Duration("stuck_threshold", s.stuckDuration),
	)

	go s.runPeriodic()
}

// Stop останавливает периодическое восстановление
func (s *RecoveryService) Stop() {
	s.log.Info("Stopping periodic recovery service...")
	close(s.stopCh)
}

// runPeriodic выполняет периодическую проверку застрявших матчей
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

// runPeriodicRecovery выполняет одну итерацию периодического восстановления
func (s *RecoveryService) runPeriodicRecovery() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Только восстанавливаем застрявшие running матчи
	// Pending матчи уже должны быть в очереди после startup recovery
	stuckRecovered, err := s.recoverStuckRunning(ctx)
	if err != nil {
		s.log.LogError("Periodic recovery failed", err)
		return
	}

	if stuckRecovered > 0 {
		// Если были застрявшие матчи, добавляем их в очередь
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
