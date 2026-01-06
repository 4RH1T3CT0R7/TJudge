package worker

import (
	"context"
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
	wg               sync.WaitGroup
	activeWorkers    atomic.Int32
	totalWorkers     atomic.Int32
	matchesProcessed atomic.Int64
	matchesFailed    atomic.Int64
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

	return &Pool{
		config:    cfg,
		queue:     queue,
		processor: processor,
		log:       log,
		metrics:   m,
		ctx:       ctx,
		cancel:    cancel,
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

// Stop останавливает пул воркеров
func (p *Pool) Stop() {
	p.log.Info("Stopping worker pool...")

	// Отменяем контекст
	p.cancel()

	// Ждём завершения всех воркеров
	p.wg.Wait()

	p.log.Info("Worker pool stopped",
		zap.Int64("matches_processed", p.matchesProcessed.Load()),
		zap.Int64("matches_failed", p.matchesFailed.Load()),
	)
}

// spawnWorker создаёт нового воркера
func (p *Pool) spawnWorker() {
	current := p.totalWorkers.Add(1)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.totalWorkers.Add(-1)

		workerID := current

		p.log.Debug("Worker started", zap.Int32("worker_id", workerID))

		for {
			select {
			case <-p.ctx.Done():
				p.log.Debug("Worker stopped", zap.Int32("worker_id", workerID))
				return
			default:
				p.processNext(workerID)
			}
		}
	}()
}

// processNext обрабатывает следующий матч из очереди
func (p *Pool) processNext(workerID int32) {
	// Увеличиваем счётчик активных воркеров
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)

	// Получаем матч из очереди
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	match, err := p.queue.Dequeue(ctx)
	if err != nil {
		p.log.LogError("Failed to dequeue match", err, zap.Int32("worker_id", workerID))
		time.Sleep(time.Second)
		return
	}

	// Очередь пустая
	if match == nil {
		time.Sleep(100 * time.Millisecond)
		return
	}

	// Обрабатываем матч
	p.log.Info("Processing match",
		zap.Int32("worker_id", workerID),
		zap.String("match_id", match.ID.String()),
		zap.String("priority", string(match.Priority)),
	)

	start := time.Now()
	p.metrics.RecordMatchStart()

	// Создаём контекст с таймаутом для обработки
	processCtx, processCancel := context.WithTimeout(p.ctx, p.config.Timeout)
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
}

// processWithRetry обрабатывает матч с повторными попытками
func (p *Pool) processWithRetry(ctx context.Context, match *domain.Match) error {
	var lastErr error

	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		if attempt > 1 {
			p.log.Info("Retrying match",
				zap.String("match_id", match.ID.String()),
				zap.Int("attempt", attempt),
			)
			time.Sleep(p.config.RetryDelay * time.Duration(attempt))
		}

		err := p.processor.Process(ctx, match)
		if err == nil {
			return nil
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

	if queueSize > 100 {
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
	}
	// Для scale down воркеры сами завершатся при ctx.Done()
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
