package worker

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/events"
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
}

// OutboxDispatcher добивает зависшие outbox-задачи - обновления рейтингов,
// которые потерялись если процесс упал между записью результата матча и
// fast-path обработкой.
// ключевое тут - идемпотентность: проверяется rating_history по match_id,
// и если записи уже есть (процесс упал после коммита рейтинга но до пометки
// done), рейтинг второй раз не применяется, только переотправляется событие -
// оно могло потеряться вместе с процессом, а на нём висят кэш и вебсокет
type OutboxDispatcher struct {
	outbox        OutboxStore
	matchRepo     MatchRepository
	ratingRepo    RatingRepository
	ratingService RatingService
	notifier      events.Notifier
	log           *logger.Logger

	interval  time.Duration
	olderThan time.Duration
	batchSize int

	cancel context.CancelFunc
	done   chan struct{}
}

// NewOutboxDispatcher создаёт диспетчер.
// olderThan 10с - задачи свежее добирает fast-path воркера, диспетчер в них
// не лезет чтобы не гоняться с ним за одну задачу
func NewOutboxDispatcher(
	outbox OutboxStore,
	matchRepo MatchRepository,
	ratingRepo RatingRepository,
	ratingService RatingService,
	notifier events.Notifier,
	log *logger.Logger,
) *OutboxDispatcher {
	return &OutboxDispatcher{
		outbox:        outbox,
		matchRepo:     matchRepo,
		ratingRepo:    ratingRepo,
		ratingService: ratingService,
		notifier:      notifier,
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

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n := d.RunOnce(ctx); n > 0 {
					d.log.Info("Outbox dispatcher processed stale entries", zap.Int("count", n))
				}
			}
		}
	}()
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

	// ключевое место: если в rating_history уже есть записи по матчу, значит
	// рейтинг посчитан - второй раз применять нельзя, задвоится история и
	// ело перестанет сходиться. а вот событие надо переотправить
	history, err := d.ratingRepo.GetByMatchID(ctx, entry.MatchID)
	if err != nil {
		return err
	}
	if len(history) > 0 {
		d.republishEvent(ctx, match, history)
		return nil
	}

	rating1, rating2, err := d.ratingRepo.GetParticipantRatings(
		ctx, match.TournamentID, match.Program1ID, match.Program2ID,
	)
	if err != nil {
		return err
	}

	// ProcessMatchResult сам шлёт MatchResultProcessed после коммита,
	// отсюда дублировать не надо
	return d.ratingService.ProcessMatchResult(ctx, match, rating1, rating2)
}

// republishEvent собирает потерянное событие из уже записанной истории -
// рейтинги берутся из history, никакого пересчёта
func (d *OutboxDispatcher) republishEvent(ctx context.Context, match *models.Match, history []*models.RatingHistory) {
	var newRating1, newRating2 int
	for _, h := range history {
		switch h.ProgramID {
		case match.Program1ID:
			newRating1 = h.NewRating
		case match.Program2ID:
			newRating2 = h.NewRating
		}
	}

	d.notifier.MatchResultProcessed(ctx, events.MatchResultProcessed{
		Version:      1,
		TournamentID: match.TournamentID,
		MatchID:      match.ID,
		Program1ID:   match.Program1ID,
		Program2ID:   match.Program2ID,
		NewRating1:   newRating1,
		NewRating2:   newRating2,
		Winner:       *match.Winner,
	})
}
