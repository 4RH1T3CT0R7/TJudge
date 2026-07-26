package db

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// OutboxEntry - задача пост-обработки матча из таблицы match_outbox.
type OutboxEntry struct {
	ID       int64     `db:"id"`
	MatchID  uuid.UUID `db:"match_id"`
	Kind     string    `db:"kind"`
	Attempts int       `db:"attempts"`
}

// OutboxKindRatingUpdate - обновление рейтингов после завершения матча.
const OutboxKindRatingUpdate = "rating_update"

// outboxMaxAttempts - после этого числа неудач задача переводится в error
// (терминальный статус), чтобы «ядовитая» задача не крутилась вечно.
const outboxMaxAttempts = 10

// OutboxRepository - запасная таблица match_outbox: если воркер упал между
// записью результата матча и пересчётом рейтинга, задачу добираем отсюда
type OutboxRepository struct {
	db *DB
}

func NewOutboxRepository(database *DB) *OutboxRepository {
	return &OutboxRepository{db: database}
}

// ClaimPending атомарно забирает пачку зависших pending-задач.
// берём только старше olderThan (свежие тянет fast path воркера) и без
// живого lease (claimed_at старше минуты или NULL). FOR UPDATE SKIP LOCKED
// даёт нескольким диспетчерам работать параллельно без двойного клейма,
// а lease не даёт заклеймить повторно пока задачу обрабатывают
func (r *OutboxRepository) ClaimPending(ctx context.Context, olderThan time.Duration, limit int) ([]*OutboxEntry, error) {
	query := `
		UPDATE match_outbox
		SET attempts = attempts + 1, claimed_at = NOW()
		WHERE id IN (
			SELECT id FROM match_outbox
			WHERE status = 'pending'
			  AND attempts < $3
			  AND created_at < NOW() - $1::interval
			  AND (claimed_at IS NULL OR claimed_at < NOW() - interval '60 seconds')
			ORDER BY id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, match_id, kind, attempts
	`

	var entries []*OutboxEntry
	interval := olderThan.String()
	if err := r.db.SelectContext(ctx, &entries, query, interval, limit, outboxMaxAttempts); err != nil {
		return nil, errors.Wrap(err, "failed to claim outbox entries")
	}

	return entries, nil
}

func (r *OutboxRepository) MarkDone(ctx context.Context, id int64) error {
	query := `UPDATE match_outbox SET status = 'done', processed_at = NOW() WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return errors.Wrap(err, "failed to mark outbox entry done")
	}
	return nil
}

// MarkFailed пишет ошибку попытки, после outboxMaxAttempts задача
// уходит в терминальный error
func (r *OutboxRepository) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	query := `
		UPDATE match_outbox
		SET last_error = $2,
		    status = CASE WHEN attempts >= $3 THEN 'error' ELSE 'pending' END,
		    claimed_at = NULL
		WHERE id = $1
	`
	if _, err := r.db.ExecContext(ctx, query, id, errMsg, outboxMaxAttempts); err != nil {
		return errors.Wrap(err, "failed to mark outbox entry failed")
	}
	return nil
}

// RetryErrors возвращает застрявшие в error задачи обратно в pending со
// сбросом счётчика. дёргается кнопкой восстановления в админке: после того
// как починили причину (например бд), рейтинги добьёт OutboxDispatcher
func (r *OutboxRepository) RetryErrors(ctx context.Context) (int64, error) {
	query := `
		UPDATE match_outbox
		SET status = 'pending', attempts = 0, claimed_at = NULL, last_error = NULL
		WHERE status = 'error'
	`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, errors.Wrap(err, "failed to retry outbox errors")
	}
	return result.RowsAffected()
}
