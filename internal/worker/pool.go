package worker

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"go.uber.org/zap"
)

// QueueManager интерфейс для работы с очередями
type QueueManager interface {
	Dequeue(ctx context.Context) (*domain.Match, error)
	GetTotalQueueSize(ctx context.Context) (int64, error)
}

// MatchProcessor интерфейс для обработки матчей
type MatchProcessor interface {
	Process(ctx context.Context, match *domain.Match) error
}

// Pool - пул воркеров для обработки матчей
type Pool struct {
	config           config.WorkerConfig
	queue            QueueManager
	processor        MatchProcessor
	log              *logger.Logger
	metrics          *metrics.Metrics
	ctx              context.Context
	cancel           context.CancelFunc
	shutdownCtx      context.Context    // stays alive during graceful shutdown; cancelled after grace period
	shutdownCancel   context.CancelFunc // cancels shutdownCtx
	wg               sync.WaitGroup
	activeWorkers    atomic.Int32
	totalWorkers     atomic.Int32
	matchesProcessed atomic.Int64
	matchesFailed    atomic.Int64

	// Per-worker cancellation for scale-down support.
	workerMu      sync.Mutex
	workerCancels []context.CancelFunc
}

// NewPool создаёт новый пул воркеров
func NewPool(
	cfg config.WorkerConfig,
	queue QueueManager,
	processor MatchProcessor,
	log *logger.Logger,
	m *metrics.Metrics,
) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	return &Pool{
		config:         cfg,
		queue:          queue,
		processor:      processor,
		log:            log,
		metrics:        m,
		ctx:            ctx,
		cancel:         cancel,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}
}

// Start запускает пул воркеров
func (p *Pool) Start() {
	p.log.Info("Starting worker pool",
		zap.Int("min_workers", p.config.MinWorkers),
		zap.Int("max_workers", p.config.MaxWorkers),
	)

	// Запускаем минимальное количество воркеров
	for i := 0; i < p.config.MinWorkers; i++ {
		p.spawnWorker()
	}

	// Запускаем автоскейлер
	go p.autoScaler()

	// Запускаем монитор метрик
	go p.metricsMonitor()

	p.log.Info("Worker pool started",
		zap.Int32("workers", p.totalWorkers.Load()),
	)
}

// Stop останавливает пул воркеров.
// P1.7: graceful drain с observability — фиксируем число in-flight матчей
// на момент Stop и длительность drain. При превышении grace period
// отменяем оставшиеся in-flight матчи через shutdownCtx.
func (p *Pool) Stop() {
	inFlight := int(p.activeWorkers.Load())
	p.log.Info("Stopping worker pool (draining)",
		zap.Int("in_flight_matches", inFlight),
	)
	p.metrics.SetWorkerInFlightOnStop(inFlight)
	p.metrics.SetWorkerDraining(true)
	defer p.metrics.SetWorkerDraining(false)

	drainStart := time.Now()
	defer func() {
		p.metrics.RecordWorkerDrainDuration(time.Since(drainStart))
	}()

	// Cancel the dequeue loop so workers stop picking up new matches.
	p.cancel()

	// Wait for all workers (including in-flight matches) to finish.
	// In-flight matches use shutdownCtx, which is still alive at this point,
	// so they continue processing until their individual timeout expires.
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Give in-flight matches up to Timeout to finish, then force-cancel.
	grace := p.config.Timeout
	if grace == 0 {
		grace = 30 * time.Second
	}
	select {
	case <-done:
		// All workers finished within grace period
	case <-time.After(grace):
		remaining := int(p.activeWorkers.Load())
		p.log.Warn("Grace period expired, cancelling in-flight matches",
			zap.Duration("grace_period", grace),
			zap.Int("remaining_in_flight", remaining),
		)
		p.shutdownCancel()
		<-done // Wait for workers to react to cancellation
	}

	// Ensure shutdownCtx is always cleaned up.
	p.shutdownCancel()

	p.log.Info("Worker pool stopped",
		zap.Int64("matches_processed", p.matchesProcessed.Load()),
		zap.Int64("matches_failed", p.matchesFailed.Load()),
		zap.Duration("drain_duration", time.Since(drainStart)),
	)
}

// spawnWorker создаёт нового воркера
func (p *Pool) spawnWorker() {
	// Create a per-worker context derived from the pool context.
	// This allows individual workers to be cancelled during scale-down
	// without stopping the entire pool.
	workerCtx, workerCancel := context.WithCancel(p.ctx)

	p.workerMu.Lock()
	p.workerCancels = append(p.workerCancels, workerCancel)
	p.workerMu.Unlock()

	current := p.totalWorkers.Add(1)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.totalWorkers.Add(-1)
		defer func() {
			if r := recover(); r != nil {
				p.log.Error("Worker panic recovered",
					zap.Int32("worker_id", current),
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				// Respawn worker if pool is still running and below minimum capacity
				if p.ctx.Err() == nil {
					time.AfterFunc(time.Second, func() {
						if p.ctx.Err() != nil {
							return // Pool was stopped, do not respawn
						}
						if int(p.totalWorkers.Load()) < p.config.MinWorkers {
							p.log.Info("Respawning worker after panic",
								zap.Int32("current_workers", p.totalWorkers.Load()),
								zap.Int("min_workers", p.config.MinWorkers),
							)
							p.spawnWorker()
						}
					})
				}
			}
		}()

		workerID := current

		p.log.Debug("Worker started", zap.Int32("worker_id", workerID))

		for {
			select {
			case <-workerCtx.Done():
				p.log.Debug("Worker stopped", zap.Int32("worker_id", workerID))
				return
			default:
			}

			idle := p.processNext(workerCtx, workerID)
			if idle {
				// Queue was empty; back off before polling again to avoid
				// a tight busy-wait loop that wastes CPU.
				select {
				case <-workerCtx.Done():
					p.log.Debug("Worker stopped", zap.Int32("worker_id", workerID))
					return
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
	}()
}

// processNext обрабатывает следующий матч из очереди.
// It returns true when the queue was empty (the worker is idle),
// allowing the caller to back off before the next poll.
func (p *Pool) processNext(workerCtx context.Context, workerID int32) (idle bool) {
	// Получаем матч из очереди
	ctx, cancel := context.WithTimeout(workerCtx, 5*time.Second)
	defer cancel()

	match, err := p.queue.Dequeue(ctx)
	if err != nil {
		p.log.LogError("Failed to dequeue match", err, zap.Int32("worker_id", workerID))
		time.Sleep(time.Second)
		return true
	}

	// Очередь пустая
	if match == nil {
		return true
	}

	// A match was dequeued -- mark the worker as actively processing.
	// Only count workers as active when they are actually handling a
	// match, not while polling an empty queue.
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)

	// Обрабатываем матч
	p.log.Info("Processing match",
		zap.Int32("worker_id", workerID),
		zap.String("match_id", match.ID.String()),
		zap.String("priority", string(match.Priority)),
	)

	start := time.Now()
	p.metrics.RecordMatchStart()

	// Derive processCtx from shutdownCtx so that scale-down (workerCtx cancel)
	// does NOT kill in-flight matches, but pool shutdown CAN signal them after
	// the grace period expires. Each match also has its individual timeout.
	processCtx, processCancel := context.WithTimeout(p.shutdownCtx, p.config.Timeout)
	defer processCancel()

	// Обрабатываем с retry
	err = p.processWithRetry(processCtx, match)

	duration := time.Since(start)
	status := "completed"
	if err != nil {
		status = "failed"
		p.matchesFailed.Add(1)
		p.log.LogError("Match processing failed", err,
			zap.Int32("worker_id", workerID),
			zap.String("match_id", match.ID.String()),
		)
	} else {
		p.matchesProcessed.Add(1)
	}

	p.metrics.RecordMatchComplete(match.GameType, status, duration)

	p.log.Info("Match processed",
		zap.Int32("worker_id", workerID),
		zap.String("match_id", match.ID.String()),
		zap.String("status", status),
		zap.Duration("duration", duration),
	)

	return false
}

// processWithRetry обрабатывает матч с повторными попытками
func (p *Pool) processWithRetry(ctx context.Context, match *domain.Match) error {
	var lastErr error

	const maxRetryDelay = 30 * time.Second
	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		if attempt > 1 {
			delay := p.config.RetryDelay * time.Duration(attempt)
			if delay > maxRetryDelay {
				p.log.Warn("Retry delay capped",
					zap.String("match_id", match.ID.String()),
					zap.Duration("computed_delay", delay),
					zap.Duration("capped_to", maxRetryDelay),
				)
				delay = maxRetryDelay
			}
			p.log.Info("Retrying match",
				zap.String("match_id", match.ID.String()),
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := p.processor.Process(ctx, match)
		if err == nil {
			return nil
		}

		// Если матч не найден в БД - пропускаем без retry
		// Это означает, что матч или турнир был удалён
		if errors.Is(err, ErrMatchNotFound) {
			p.log.Info("Match skipped (not found in database)",
				zap.String("match_id", match.ID.String()),
			)
			return nil // Возвращаем nil чтобы не считать это ошибкой
		}

		lastErr = err
		p.log.LogError("Match processing attempt failed", err,
			zap.String("match_id", match.ID.String()),
			zap.Int("attempt", attempt),
		)
	}

	return lastErr
}

// autoScaler автоматически масштабирует количество воркеров
func (p *Pool) autoScaler() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.scale()
		}
	}
}

// scale масштабирует количество воркеров
func (p *Pool) scale() {
	ctx, cancel := context.WithTimeout(p.ctx, 2*time.Second)
	defer cancel()

	// Получаем размер очереди
	queueSize, err := p.queue.GetTotalQueueSize(ctx)
	if err != nil {
		p.log.LogError("Failed to get queue size", err)
		return
	}

	currentWorkers := int(p.totalWorkers.Load())
	activeWorkers := int(p.activeWorkers.Load())

	// Логика масштабирования
	var targetWorkers int

	if currentWorkers < p.config.MinWorkers {
		// Ниже минимума (например, после паники воркера) - восстанавливаем
		targetWorkers = p.config.MinWorkers
	} else if queueSize > 100 {
		// Много задач - увеличиваем воркеры
		targetWorkers = currentWorkers + 10
	} else if queueSize > 50 {
		targetWorkers = currentWorkers + 5
	} else if queueSize < 10 && activeWorkers < currentWorkers/2 {
		// Мало задач и много простаивающих воркеров - уменьшаем
		targetWorkers = currentWorkers - 5
	} else {
		return // Ничего не меняем
	}

	// Ограничиваем минимумом и максимумом
	if targetWorkers < p.config.MinWorkers {
		targetWorkers = p.config.MinWorkers
	}
	if targetWorkers > p.config.MaxWorkers {
		targetWorkers = p.config.MaxWorkers
	}

	// Применяем изменения
	if targetWorkers > currentWorkers {
		toSpawn := targetWorkers - currentWorkers
		p.log.Info("Scaling up workers",
			zap.Int("current", currentWorkers),
			zap.Int("target", targetWorkers),
			zap.Int64("queue_size", queueSize),
		)
		for i := 0; i < toSpawn; i++ {
			p.spawnWorker()
		}
	} else if targetWorkers < currentWorkers {
		toRemove := currentWorkers - targetWorkers
		p.log.Info("Scaling down workers",
			zap.Int("current", currentWorkers),
			zap.Int("target", targetWorkers),
			zap.Int64("queue_size", queueSize),
		)

		p.workerMu.Lock()
		// Cancel excess workers from the end of the slice.
		if toRemove > len(p.workerCancels) {
			toRemove = len(p.workerCancels)
		}
		removed := p.workerCancels[len(p.workerCancels)-toRemove:]
		p.workerCancels = p.workerCancels[:len(p.workerCancels)-toRemove]
		p.workerMu.Unlock()

		for _, cancelFn := range removed {
			cancelFn()
		}
	}
}

// metricsMonitor обновляет метрики пула
func (p *Pool) metricsMonitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.metrics.SetActiveWorkers(int(p.activeWorkers.Load()))
			p.metrics.SetWorkerPoolSize(int(p.totalWorkers.Load()))
		}
	}
}

// GetStats возвращает статистику пула
func (p *Pool) GetStats() WorkerStats {
	return WorkerStats{
		TotalWorkers:     int(p.totalWorkers.Load()),
		ActiveWorkers:    int(p.activeWorkers.Load()),
		MatchesProcessed: p.matchesProcessed.Load(),
		MatchesFailed:    p.matchesFailed.Load(),
	}
}

// WorkerStats - статистика пула воркеров
type WorkerStats struct {
	TotalWorkers     int
	ActiveWorkers    int
	MatchesProcessed int64
	MatchesFailed    int64
}

// Wait ожидает завершения всех воркеров
func (p *Pool) Wait() {
	p.wg.Wait()
}

// GetMatchesProcessed возвращает количество обработанных матчей
func (p *Pool) GetMatchesProcessed() int64 {
	return p.matchesProcessed.Load()
}
