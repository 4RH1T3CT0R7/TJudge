package worker

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// QueueManager - очередь матчей
type QueueManager interface {
	Dequeue(ctx context.Context) (*models.Match, error)
	GetTotalQueueSize(ctx context.Context) (int64, error)
}

// MatchProcessor обрабатывает один матч
type MatchProcessor interface {
	Process(ctx context.Context, match *models.Match) error
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
	shutdownCtx      context.Context // живёт во время graceful shutdown, отменяется когда grace вышел
	shutdownCancel   context.CancelFunc
	wg               sync.WaitGroup
	activeWorkers    atomic.Int32
	totalWorkers     atomic.Int32
	matchesProcessed atomic.Int64
	matchesFailed    atomic.Int64

	// отмена отдельных воркеров, нужна для scale-down
	workerMu      sync.Mutex
	workerCancels []context.CancelFunc

	// scaleMu сериализует scale(): автоскейлер тикает в одной горутине, но
	// тесты (и возможные будущие триггеры) зовут scale параллельно - без
	// мьютекса чтение totalWorkers гонится и пул вылетает за MaxWorkers
	scaleMu sync.Mutex

	// auxWg - отдельная группа для вспомогательных горутин (автоскейлер,
	// монитор метрик). их надо дождаться раньше wg с воркерами, иначе
	// автоскейлер может дёрнуть spawnWorker одновременно с wg.Wait -
	// а это ломает инварианты sync.WaitGroup
	auxWg sync.WaitGroup
}

// NewPool создаёт пул воркеров
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

// Start запускает пул
func (p *Pool) Start() {
	p.log.Info("Starting worker pool",
		zap.Int("min_workers", p.config.MinWorkers),
		zap.Int("max_workers", p.config.MaxWorkers),
	)

	for i := 0; i < p.config.MinWorkers; i++ {
		p.spawnWorker()
	}

	// автоскейлер. первый тик у него мгновенный, чтобы полная очередь
	// после рестарта не ждала целый интервал до масштабирования
	p.auxWg.Go(func() {
		p.autoScaler()
	})

	p.auxWg.Go(func() {
		p.metricsMonitor()
	})

	p.log.Info("Worker pool started",
		zap.Int32("workers", p.totalWorkers.Load()),
	)
}

// Stop гасит пул. graceful: in-flight матчи получают grace period на
// дообработку, потом принудительная отмена через shutdownCtx
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

	// отмена dequeue-цикла - воркеры перестают брать новые матчи
	p.cancel()

	// сначала дождаться автоскейлер и монитор, только потом смотреть на wg.
	// иначе scale->spawnWorker гонится с wg.Wait и ломает WaitGroup
	p.auxWg.Wait()

	// дальше очередь за воркерами вместе с их in-flight матчами. shutdownCtx
	// тут ещё жив, так что матчи спокойно доезжают до своего таймаута
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// in-flight матчам даётся grace до Timeout, дальше принудительная отмена
	grace := p.config.Timeout
	if grace == 0 {
		grace = 30 * time.Second
	}
	select {
	case <-done:
		// все успели
	case <-time.After(grace):
		remaining := int(p.activeWorkers.Load())
		p.log.Warn("Grace period expired, cancelling in-flight matches",
			zap.Duration("grace_period", grace),
			zap.Int("remaining_in_flight", remaining),
		)
		p.shutdownCancel()
		<-done // и дождаться пока воркеры отреагируют на отмену
	}

	// на всякий случай, чтобы ctx точно был почищен
	p.shutdownCancel()

	p.log.Info("Worker pool stopped",
		zap.Int64("matches_processed", p.matchesProcessed.Load()),
		zap.Int64("matches_failed", p.matchesFailed.Load()),
		zap.Duration("drain_duration", time.Since(drainStart)),
	)
}

// spawnWorker поднимает нового воркера
func (p *Pool) spawnWorker() {
	// у каждого воркера свой контекст от контекста пула - так scale-down
	// может погасить отдельного воркера, не трогая остальных
	// #nosec G118
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
				// воркер упал - через секунду поднимается новый, если пул жив
				// и воркеров стало меньше минимума
				if p.ctx.Err() == nil {
					time.AfterFunc(time.Second, func() {
						if p.ctx.Err() != nil {
							return // пул уже остановлен
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
				// короткий backoff перед следующим опросом. в проде BRPOP внутри
				// Dequeue и так блокируется до 2 сек, так что тут добавляется
				// всего ~10мс к задержке. а вот в тестах с моками без этого
				// получался busy-loop и сжирался процессор
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

// processNext берёт следующий матч из очереди.
// true = очередь была пуста, воркер простаивает (вызывающий сделает backoff)
func (p *Pool) processNext(workerCtx context.Context, workerID int32) (idle bool) {
	ctx, cancel := context.WithTimeout(workerCtx, 5*time.Second)
	defer cancel()

	match, err := p.queue.Dequeue(ctx)
	if err != nil {
		// отмена родительского контекста = graceful shutdown, выход без шума
		if errors.Is(err, context.Canceled) && workerCtx.Err() != nil {
			return true
		}
		p.log.LogError("Failed to dequeue match", err, zap.Int32("worker_id", workerID))
		time.Sleep(time.Second)
		return true
	}

	if match == nil {
		return true
	}

	// воркер считается активным только когда реально взял матч,
	// опрос пустой очереди не в счёт - на этом строится scale-down
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)

	p.log.Info("Processing match",
		zap.Int32("worker_id", workerID),
		zap.String("match_id", match.ID.String()),
		zap.String("priority", string(match.Priority)),
	)

	start := time.Now()
	p.metrics.RecordMatchStart()

	// processCtx производится от shutdownCtx, не от workerCtx: scale-down
	// (отмена workerCtx) не должен убивать матч на середине, а вот shutdown
	// пула после grace period - должен. плюс у матча свой таймаут
	processCtx, processCancel := context.WithTimeout(p.shutdownCtx, p.config.Timeout)
	defer processCancel()

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

// processWithRetry гоняет матч с повторами
func (p *Pool) processWithRetry(ctx context.Context, match *models.Match) error {
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

		// матча нет в базе (удалили вместе с турниром) - пропуск без ретраев,
		// и это не ошибка
		if errors.Is(err, ErrMatchNotFound) {
			p.log.Info("Match skipped (not found in database)",
				zap.String("match_id", match.ID.String()),
			)
			return nil
		}

		// терминальная ошибка программы: матч уже помечен failed, повторять
		// нет смысла. транзиентные инфра-ошибки сюда не попадают - для них
		// Process возвращает матч в pending и ретрай реально повторяет
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

// autoScaler крутит scale() раз в интервал (дефолт 2с), первый раз сразу
func (p *Pool) autoScaler() {
	interval := p.config.AutoScaleInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

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

// scale подгоняет число воркеров под очередь. вверх - быстро и
// пропорционально очереди, вниз - нарочно медленно (по 2), чтобы пул
// не дёргался на всплесках.
// после отмены p.ctx спавнить нельзя ни при каких условиях: иначе
// spawnWorker гонится со Stop/wg.Wait, плюс во время drain воркеры выходят,
// totalWorkers проседает ниже минимума и scale начал бы «восстанавливать»
// пул прямо посреди остановки
// TODO: пороги масштабирования вынести в конфиг? пока захардкожены
func (p *Pool) scale() {
	if p.ctx.Err() != nil {
		return
	}

	p.scaleMu.Lock()
	defer p.scaleMu.Unlock()

	// повторная проверка уже под мьютексом: между первой проверкой и Lock
	// мог сработать Stop, тогда масштабировать больше нечего
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
		// ниже минимума (например после паники) - восстановление
		targetWorkers = p.config.MinWorkers
	case queueSize >= 10:
		// ramp-up пропорционально очереди: queueSize/5 это +20 воркеров на
		// 100 матчей, следующий тик утроит если надо. минимум +2 чтобы
		// не топтаться на месте
		grow := max(int(queueSize)/5, 2)
		targetWorkers = currentWorkers + grow
	case queueSize == 0 && activeWorkers*3 < currentWorkers:
		// очередь пуста и простаивает больше 2/3 пула - снимается по 2
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

		// гасятся воркеры с хвоста списка
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

// GetStats отдаёт статистику пула
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

// Wait ждёт всех: сперва вспомогательные горутины (чтобы scale точно не
// дёрнул spawnWorker во время wg.Wait), потом самих воркеров
func (p *Pool) Wait() {
	p.auxWg.Wait()
	p.wg.Wait()
}

func (p *Pool) GetMatchesProcessed() int64 {
	return p.matchesProcessed.Load()
}
