package storage

import (
	"context"
	"database/sql"
	stderrors "errors"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// statusCacheTTL - агрегаты по matches и match_outbox считаются проходом по
// всей таблице, а админка опрашивает статус каждые 10с, поэтому результат
// переиспользуется между запросами
const statusCacheTTL = 30 * time.Second

// SystemStatusRepository - агрегированные показатели состояния системы
// для admin-эндпоинта /system/status, CLI-команды make status и Grafana.
type SystemStatusRepository struct {
	db *DB

	matchCounts cachedAggregate[map[string]int64]
	outbox      cachedAggregate[*OutboxStatus]
}

// cachedAggregate - значение, которое пересчитывается не чаще раза в
// statusCacheTTL. пересчёт под мьютексом, так что параллельные запросы
// не гоняют один и тот же агрегат одновременно
type cachedAggregate[T any] struct {
	mu  sync.Mutex
	val T
	at  time.Time
}

func (c *cachedAggregate[T]) get(ctx context.Context, load func(context.Context) (T, error)) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.at.IsZero() && time.Since(c.at) < statusCacheTTL {
		return c.val, nil
	}
	val, err := load(ctx)
	if err != nil {
		return val, err
	}
	c.val, c.at = val, time.Now()
	return val, nil
}

func NewSystemStatusRepository(database *DB) *SystemStatusRepository {
	return &SystemStatusRepository{db: database}
}

// SchemaVersion возвращает текущую версию миграций (golang-migrate
// хранит её в schema_migrations) и флаг dirty.
func (r *SystemStatusRepository) SchemaVersion(ctx context.Context) (version int64, dirty bool, err error) {
	row := r.db.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1")
	if scanErr := row.Scan(&version, &dirty); scanErr != nil {
		if stderrors.Is(scanErr, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, errors.Wrap(scanErr, "failed to read schema version")
	}
	return version, dirty, nil
}

// MatchCountsByStatus возвращает число матчей в каждом статусе (кэш statusCacheTTL).
func (r *SystemStatusRepository) MatchCountsByStatus(ctx context.Context) (map[string]int64, error) {
	return r.matchCounts.get(ctx, func(ctx context.Context) (map[string]int64, error) {
		return r.countsByStatus(ctx, "SELECT status, COUNT(*) FROM matches GROUP BY status")
	})
}

// ProgramCountsByStatus возвращает число программ в каждом статусе
// (compiling/ready/failed).
func (r *SystemStatusRepository) ProgramCountsByStatus(ctx context.Context) (map[string]int64, error) {
	return r.countsByStatus(ctx, "SELECT status, COUNT(*) FROM programs GROUP BY status")
}

func (r *SystemStatusRepository) countsByStatus(ctx context.Context, query string) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to count by status")
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, errors.Wrap(err, "failed to scan status count")
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// OutboxStatus - состояние outbox-задач пост-обработки матчей.
type OutboxStatus struct {
	Pending          int64      `json:"pending"`
	Errors           int64      `json:"errors"`
	DoneLast24h      int64      `json:"done_last_24h"`
	OldestPendingAge *float64   `json:"oldest_pending_age_seconds,omitempty"`
	LastProcessedAt  *time.Time `json:"last_processed_at,omitempty"`
}

// OutboxStats возвращает состояние match_outbox: зависшие pending-задачи
// и терминальные ошибки - прямой сигнал, что рейтинги могли не примениться.
// кэш statusCacheTTL
func (r *SystemStatusRepository) OutboxStats(ctx context.Context) (*OutboxStatus, error) {
	return r.outbox.get(ctx, r.outboxStats)
}

func (r *SystemStatusRepository) outboxStats(ctx context.Context) (*OutboxStatus, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending')                                        AS pending,
			COUNT(*) FILTER (WHERE status = 'error')                                          AS errors,
			COUNT(*) FILTER (WHERE status = 'done' AND processed_at > NOW() - interval '24h') AS done_24h,
			EXTRACT(EPOCH FROM (NOW() - MIN(created_at) FILTER (WHERE status = 'pending')))   AS oldest_pending_age,
			MAX(processed_at)                                                                 AS last_processed_at
		FROM match_outbox
	`

	var s OutboxStatus
	row := r.db.QueryRowContext(ctx, query)
	if err := row.Scan(&s.Pending, &s.Errors, &s.DoneLast24h, &s.OldestPendingAge, &s.LastProcessedAt); err != nil {
		return nil, errors.Wrap(err, "failed to read outbox stats")
	}
	return &s, nil
}

// LastCompletedMatchAt возвращает время последнего завершённого матча
// (nil, если матчей ещё не было).
func (r *SystemStatusRepository) LastCompletedMatchAt(ctx context.Context) (*time.Time, error) {
	var ts *time.Time
	row := r.db.QueryRowContext(ctx, "SELECT MAX(completed_at) FROM matches WHERE status = 'completed'")
	if err := row.Scan(&ts); err != nil {
		return nil, errors.Wrap(err, "failed to read last completed match time")
	}
	return ts, nil
}

// StuckRunningCount возвращает число матчей, зависших в running дольше
// olderThan (признак умершего worker'а или потерянного результата).
func (r *SystemStatusRepository) StuckRunningCount(ctx context.Context, olderThan time.Duration) (int64, error) {
	var count int64
	row := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM matches WHERE status = 'running' AND started_at < NOW() - $1::interval",
		olderThan.String(),
	)
	if err := row.Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count stuck running matches")
	}
	return count, nil
}

func (r *SystemStatusRepository) ConnectionStats() sql.DBStats {
	return r.db.Stats()
}

func (r *SystemStatusRepository) Healthy(ctx context.Context) bool {
	return r.db.Health(ctx) == nil
}
