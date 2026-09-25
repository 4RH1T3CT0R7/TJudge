package worker

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// OutboxStore - таблица match_outbox
type OutboxStore interface {
	ClaimPending(ctx context.Context, olderThan time.Duration, limit int) ([]*storage.OutboxEntry, error)
	MarkDone(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
	PurgeDone(ctx context.Context, olderThan time.Duration) (int64, error)
}

// outboxRetention - сколько хранятся выполненные задачи (для done_last_24h
// в системном статусе хватает и суток, неделя - с запасом на разбор)
const outboxRetention = 7 * 24 * time.Hour

// OutboxDispatcher добивает зависшие outbox-задачи - обновления рейтингов,
// которые потерялись если процесс упал между записью результата матча и
// fast-path обработкой.
// идемпотентность держит сам ProcessMatchResult: задача гасится в одной
// транзакции с применением рейтинга, так что гонка с fast path не задвоит ELO
type OutboxDispatcher struct {
	outbox        OutboxStore
	matchRepo     MatchRepository
	ratingService RatingService
	log           *logger.Logger

	interval  time.Duration
	olderThan time.Duration
	batchSize int

	cancel context.CancelFunc
	done   chan struct{}
}

// NewOutboxDispatcher создаёт диспетчер.
// olderThan 10с - задачи свежее добирает fast-path воркера, диспетчер в них
// обычно не лезет (а если и полезет, второй раз рейтинг не применится)
func NewOutboxDispatcher(
	outbox OutboxStore,
	matchRepo MatchRepository,
	ratingService RatingService,
	log *logger.Logger,
) *OutboxDispatcher {
	return &OutboxDispatcher{
		outbox:        outbox,
		matchRepo:     matchRepo,
		ratingService: ratingService,
		log:           log,
		interval:      15 * time.Second,
		olderThan:     10 * time.Second,
		batchSize:     50,
		done:          make(chan struct{}),
	}
}

// Start запускает периодическую обработку в фоне
func (d *OutboxDispatcher) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go func() {
		defer close(d.done)
		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()
		purge := time.NewTicker(time.Hour)
		defer purge.Stop()

		// первая чистка сразу: рестарт процесса сбрасывает часовой тикер, и при
		// частых деплоях до него дело могло бы не доходить
		d.purgeDone(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n := d.RunOnce(ctx); n > 0 {
					d.log.Info("Outbox dispatcher processed stale entries", zap.Int("count", n))
				}
			case <-purge.C:
				d.purgeDone(ctx)
			}
		}
	}()
}

// purgeDone удаляет старые выполненные задачи. повтор из второй реплики безвреден
func (d *OutboxDispatcher) purgeDone(ctx context.Context) {
	if n, err := d.outbox.PurgeDone(ctx, outboxRetention); err != nil {
		d.log.LogError("Outbox: failed to purge done entries", err)
	} else if n > 0 {
		d.log.Info("Outbox: purged done entries", zap.Int64("count", n))
	}
}

// Stop гасит диспетчер и ждёт пока дообработается текущий цикл
func (d *OutboxDispatcher) Stop() {
	if d.cancel != nil {
		d.cancel()
		<-d.done
	}
}

// RunOnce обрабатывает одну пачку, возвращает сколько задач закрыл
func (d *OutboxDispatcher) RunOnce(ctx context.Context) int {
	entries, err := d.outbox.ClaimPending(ctx, d.olderThan, d.batchSize)
	if err != nil {
		d.log.LogError("Outbox: failed to claim pending entries", err)
		return 0
	}

	processed := 0
	for _, entry := range entries {
		if err := d.processEntry(ctx, entry); err != nil {
			d.log.LogError("Outbox: failed to process entry", err,
				zap.Int64("outbox_id", entry.ID),
				zap.String("match_id", entry.MatchID.String()),
				zap.Int("attempts", entry.Attempts),
			)
			if markErr := d.outbox.MarkFailed(ctx, entry.ID, err.Error()); markErr != nil {
				d.log.LogError("Outbox: failed to mark entry failed", markErr)
			}
			continue
		}
		if err := d.outbox.MarkDone(ctx, entry.ID); err != nil {
			d.log.LogError("Outbox: failed to mark entry done", err)
			continue
		}
		processed++
	}

	return processed
}

// processEntry разбирает одну задачу. return nil означает «задачу можно
// гасить» - в том числе для неактуальных (матч удалён/не завершён), иначе
// они крутились бы в очереди вечно
func (d *OutboxDispatcher) processEntry(ctx context.Context, entry *storage.OutboxEntry) error {
	if entry.Kind != storage.OutboxKindRatingUpdate {
		d.log.Warn("Outbox: unknown entry kind, skipping",
			zap.String("kind", entry.Kind),
			zap.Int64("outbox_id", entry.ID),
		)
		return nil
	}

	match, err := d.matchRepo.GetByID(ctx, entry.MatchID)
	if err != nil {
		if isNotFoundError(err) {
			// матч удалили - задача неактуальна
			return nil
		}
		return err
	}

	// рейтинг только для завершённых матчей с победителем (или ничьёй)
	if match.Status != models.MatchCompleted || match.Winner == nil || *match.Winner < 0 {
		return nil
	}

	// ProcessMatchResult сам шлёт MatchResultProcessed после коммита,
	// отсюда дублировать не надо
	return d.ratingService.ProcessMatchResult(ctx, match)
}
