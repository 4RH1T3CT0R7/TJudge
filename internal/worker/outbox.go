package worker

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// OutboxStore - доступ к таблице match_outbox.
type OutboxStore interface {
	ClaimPending(ctx context.Context, olderThan time.Duration, limit int) ([]*storage.OutboxEntry, error)
	MarkDone(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
}

// OutboxMatchRepository - чтение матча для пост-обработки.
type OutboxMatchRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error)
}

// OutboxRatingRepository - данные рейтингов для пост-обработки.
type OutboxRatingRepository interface {
	GetParticipantRatings(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID) (int, int, error)
	GetByMatchID(ctx context.Context, matchID uuid.UUID) ([]*domain.RatingHistory, error)
}

// OutboxDispatcher доводит до конца зависшие outbox-задачи: обновления
// рейтингов, потерянные из-за сбоя между записью результата матча и
// fast-path обработкой в Process.
//
// Идемпотентность: перед обработкой проверяется rating_history по match_id.
// Если записи уже есть (краш случился после коммита рейтинга, но до пометки
// задачи done) - рейтинг не применяется повторно, но доменное событие
// MatchResultProcessed переотправляется: оно могло потеряться вместе с
// процессом, а кэш и WebSocket-клиенты зависят от него.
type OutboxDispatcher struct {
	outbox        OutboxStore
	matchRepo     OutboxMatchRepository
	ratingRepo    OutboxRatingRepository
	ratingService RatingService
	eventBus      events.Bus
	log           *logger.Logger

	interval  time.Duration
	olderThan time.Duration
	batchSize int

	cancel context.CancelFunc
	done   chan struct{}
}

// NewOutboxDispatcher создаёт диспетчер outbox-задач.
// interval - период опроса; olderThan - минимальный возраст pending-задачи
// (свежие задачи обрабатывает fast path воркера, диспетчер их не трогает).
func NewOutboxDispatcher(
	outbox OutboxStore,
	matchRepo OutboxMatchRepository,
	ratingRepo OutboxRatingRepository,
	ratingService RatingService,
	eventBus events.Bus,
	log *logger.Logger,
) *OutboxDispatcher {
	return &OutboxDispatcher{
		outbox:        outbox,
		matchRepo:     matchRepo,
		ratingRepo:    ratingRepo,
		ratingService: ratingService,
		eventBus:      eventBus,
		log:           log,
		interval:      15 * time.Second,
		olderThan:     10 * time.Second,
		batchSize:     50,
		done:          make(chan struct{}),
	}
}

// Start запускает периодическую обработку в фоне.
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

// Stop останавливает диспетчер и дожидается завершения текущего цикла.
func (d *OutboxDispatcher) Stop() {
	if d.cancel != nil {
		d.cancel()
		<-d.done
	}
}

// RunOnce обрабатывает одну пачку зависших задач; возвращает число обработанных.
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
			// Матч удалён - задача неактуальна.
			return nil
		}
		return err
	}

	// Рейтинг применяется только к успешно завершённым матчам с победителем.
	if match.Status != domain.MatchCompleted || match.Winner == nil || *match.Winner < 0 {
		return nil
	}

	// Идемпотентный guard: rating_history уже содержит записи матча -
	// рейтинг применён, но событие могло потеряться. Переотправляем его.
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

	// ProcessMatchResult сам публикует MatchResultProcessed после коммита.
	return d.ratingService.ProcessMatchResult(ctx, match, rating1, rating2)
}

// republishEvent восстанавливает потерянное событие MatchResultProcessed
// из уже записанной rating_history.
func (d *OutboxDispatcher) republishEvent(ctx context.Context, match *domain.Match, history []*domain.RatingHistory) {
	var newRating1, newRating2 int
	for _, h := range history {
		switch h.ProgramID {
		case match.Program1ID:
			newRating1 = h.NewRating
		case match.Program2ID:
			newRating2 = h.NewRating
		}
	}

	d.eventBus.Publish(ctx, events.MatchResultProcessed{
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
