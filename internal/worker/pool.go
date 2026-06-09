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
	shutdownCtx      context.Context    // жив во время graceful shutdown; отменяется по истечении grace period
	shutdownCancel   context.CancelFunc // отменяет shutdownCtx
	wg               sync.WaitGroup
	activeWorkers    atomic.Int32
	totalWorkers     atomic.Int32
	matchesProcessed atomic.Int64
	matchesFailed    atomic.Int64

	// Отмена отдельных воркеров для поддержки scale-down.
	workerMu      sync.Mutex
	workerCancels []context.CancelFunc

	// scaleMu сериализует вызовы scale(): autoScaler тикает в одной
	// горутине, но тесты и потенциальные будущие триггеры (например,
	// kick-on-enqueue) могут вызывать scale параллельно, что даёт
	// TOCTOU на чтении totalWorkers и выбрасывает пул за MaxWorkers.
	scaleMu sync.Mutex

	// auxWg - отдельная группа для вспомогательных горутин пула
	// (autoScaler, metricsMonitor). Их мы должны дождаться ПЕРЕД wg.Wait
	// на воркерах, иначе autoScaler может вызвать scale()->spawnWorker
	// одновременно с wg.Wait, что нарушает инварианты sync.WaitGroup.
	auxWg sync.WaitGroup
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

	// Запускаем автоскейлер. Первый tick сделаем немедленно, чтобы burst
	// на старте (например, заполненная очередь после рестарта воркера)
	// не ждал полного AutoScaleInterval до первого масштабирования.
	p.auxWg.Go(func() {
		p.autoScaler()
	})

	// Запускаем монитор метрик
	p.auxWg.Go(func() {
		p.metricsMonitor()
	})

	p.log.Info("Worker pool started",
		zap.Int32("workers", p.totalWorkers.Load()),
	)
}

// Stop останавливает пул воркеров.
// Graceful drain с observability: фиксируем число in-flight матчей
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

	// Отменяем dequeue-цикл, чтобы воркеры перестали брать новые матчи.
	p.cancel()

	// Дожидаемся auxiliary-горутин (autoScaler, metricsMonitor) до того, как
	// смотреть на wg. Иначе autoScaler может вызвать scale->spawnWorker
	// одновременно с wg.Wait(), что нарушит инварианты sync.WaitGroup.
	p.auxWg.Wait()

	// Ждём завершения всех воркеров (включая in-flight матчи).
	// In-flight матчи используют shutdownCtx, который здесь ещё жив,
	// поэтому они продолжают работу до истечения собственного таймаута.
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Даём in-flight матчам время до Timeout на завершение, затем форсируем отмену.
	grace := p.config.Timeout
	if grace == 0 {
		grace = 30 * time.Second
	}
	select {
	case <-done:
		// Все воркеры завершились в пределах grace period
	case <-time.After(grace):
		remaining := int(p.activeWorkers.Load())
		p.log.Warn("Grace period expired, cancelling in-flight matches",
			zap.Duration("grace_period", grace),
			zap.Int("remaining_in_flight", remaining),
		)
		p.shutdownCancel()
		<-done // Ждём реакции воркеров на отмену
	}

	// Гарантируем, что shutdownCtx будет очищен в любом случае.
	p.shutdownCancel()

	p.log.Info("Worker pool stopped",
		zap.Int64("matches_processed", p.matchesProcessed.Load()),
		zap.Int64("matches_failed", p.matchesFailed.Load()),
		zap.Duration("drain_duration", time.Since(drainStart)),
	)
}

// spawnWorker создаёт нового воркера
func (p *Pool) spawnWorker() {
	// Создаём per-worker контекст, производный от контекста пула.
	// Это позволяет отменять отдельных воркеров при scale-down,
	// не останавливая весь пул.
	// #nosec G118 -- workerCancel сохраняется в p.workerCancels и вызывается
	// при scale-down (scale(): workerCancels[N:]) или Stop() (shutdownCancel);
	// не leak, lifecycle привязан к worker-goroutine.
	workerCtx, workerCancel := context.WithCancel(p.ctx)

	p.workerMu.Lock()
	p.workerCancels = append(p.workerCancels, workerCancel)
	p.workerMu.Unlock()

	current := p.totalWorkers.Add(1)

	p.wg.Go(func() {
		defer p.totalWorkers.Add(-1)
		defer func() {
			if r := recover(); r != nil {
				p.log.Error("Worker panic recovered",
					zap.Int32("worker_id", current),
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				// Пересоздаём воркера, если пул всё ещё работает и число воркеров ниже минимума
				if p.ctx.Err() == nil {
					time.AfterFunc(time.Second, func() {
						if p.ctx.Err() != nil {
							return // Пул остановлен, не пересоздаём
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
				// Короткий backoff перед следующим опросом. В проде реальный
				// BRPOP внутри Dequeue уже блокируется до 2 сек, так что этот
				// sleep добавляет лишь ~10 мс к wake-up latency. В тестах с
				// моками, где Dequeue возвращает nil без блокировки, этот
				// backoff гасит busy-loop и не даёт сжигать CPU.
				select {
				case <-workerCtx.Done():
					p.log.Debug("Worker stopped", zap.Int32("worker_id", workerID))
					return
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
	})
}

// processNext обрабатывает следующий матч из очереди.
// Возвращает true, когда очередь была пуста (воркер простаивает),
// что позволяет вызывающему сделать back off перед следующим опросом.
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

	// Матч получен из очереди - помечаем воркер как активно обрабатывающий.
	// Воркеры считаются активными только при реальной обработке матча,
	// а не при опросе пустой очереди.
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

	// processCtx производим от shutdownCtx, чтобы scale-down (отмена workerCtx)
	// НЕ убивал in-flight матчи, но shutdown пула МОГ их прервать после
	// истечения grace period. У каждого матча также есть свой таймаут.
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

		// Терминальная ошибка программы участника: матч уже помечен failed,
		// повторные попытки бессмысленны (и раньше всё равно отбивались
		// guard'ом pending→running). Транзиентные инфра-ошибки сюда не
		// попадают - для них Process возвращает матч в pending и retry
		// действительно повторяет исполнение.
		if errors.Is(err, ErrProgramFailed) {
			return err
		}

		lastErr = err
		p.log.LogError("Match processing attempt failed", err,
			zap.String("match_id", match.ID.String()),
			zap.Int("attempt", attempt),
		)
	}

	return lastErr
}

// autoScaler автоматически масштабирует количество воркеров.
// Период по умолчанию 2 секунды; переопределяется через AutoScaleInterval.
// Первый tick делается мгновенно - initial warmup под текущий queue-depth.
func (p *Pool) autoScaler() {
	interval := p.config.AutoScaleInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	// Initial warmup.
	p.scale()

	ticker := time.NewTicker(interval)
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

// scale масштабирует количество воркеров.
// Ramp-up быстрый и пропорциональный размеру очереди, scale-down намеренно
// медленный (гистерезис), чтобы не дёргать пул при всплесках.
//
// Никогда не спавнит воркеров после отмены p.ctx: это исключает гонку
// spawnWorker().wg.Add и Stop().wg.Wait при сценарии Start с последующим Stop в пределах
// одного интервала autoScaler, а также гасит "spawn-шторм" во время
// graceful drain (воркеры выходят, totalWorkers кратковременно падает ниже
// MinWorkers, scale() раньше мог начать восстановление во время shutdown).
func (p *Pool) scale() {
	if p.ctx.Err() != nil {
		return
	}

	p.scaleMu.Lock()
	defer p.scaleMu.Unlock()

	// Повторная проверка ctx после захвата mutex: между первой проверкой
	// и Lock мог сработать Stop(), и нам больше нечего масштабировать.
	if p.ctx.Err() != nil {
		return
	}

	ctx, cancel := context.WithTimeout(p.ctx, 2*time.Second)
	defer cancel()

	queueSize, err := p.queue.GetTotalQueueSize(ctx)
	if err != nil {
		p.log.LogError("Failed to get queue size", err)
		return
	}

	currentWorkers := int(p.totalWorkers.Load())
	activeWorkers := int(p.activeWorkers.Load())

	var targetWorkers int

	switch {
	case currentWorkers < p.config.MinWorkers:
		// Ниже минимума (например, после паники) - восстанавливаем.
		targetWorkers = p.config.MinWorkers
	case queueSize >= 10:
		// Ramp-up: добавляем воркеров пропорционально очереди.
		// queueSize/5 даёт +20 воркеров на 100 матчей в очереди, и ещё один тик
		// утроит пул при необходимости. Минимум +2, чтобы никогда не "топтаться".
		grow := max(int(queueSize)/5, 2)
		targetWorkers = currentWorkers + grow
	case queueSize == 0 && activeWorkers*3 < currentWorkers:
		// Scale-down: очередь пуста и простаивает >66% пула. Снимаем по 2 воркера.
		targetWorkers = currentWorkers - 2
	default:
		return
	}

	if targetWorkers < p.config.MinWorkers {
		targetWorkers = p.config.MinWorkers
	}
	if targetWorkers > p.config.MaxWorkers {
		targetWorkers = p.config.MaxWorkers
	}

	switch {
	case targetWorkers > currentWorkers:
		toSpawn := targetWorkers - currentWorkers
		p.log.Info("Scaling up workers",
			zap.Int("current", currentWorkers),
			zap.Int("target", targetWorkers),
			zap.Int64("queue_size", queueSize),
		)
		for range toSpawn {
			p.spawnWorker()
		}
	case targetWorkers < currentWorkers:
		toRemove := currentWorkers - targetWorkers
		p.log.Info("Scaling down workers",
			zap.Int("current", currentWorkers),
			zap.Int("target", targetWorkers),
			zap.Int64("queue_size", queueSize),
		)

		p.workerMu.Lock()
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

// Wait ожидает завершения всех воркеров и вспомогательных горутин пула.
// Сначала auxWg (autoScaler, metricsMonitor), чтобы scale() точно не мог
// вызвать spawnWorker в момент wg.Wait; затем сам wg (воркеры).
func (p *Pool) Wait() {
	p.auxWg.Wait()
	p.wg.Wait()
}

// GetMatchesProcessed возвращает количество обработанных матчей
func (p *Pool) GetMatchesProcessed() int64 {
	return p.matchesProcessed.Load()
}
