package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// DB оборачивает sqlx.DB и добавляет метрики
type DB struct {
	*sqlx.DB
	log     *logger.Logger
	metrics *metrics.Metrics
	done    chan struct{}
}

// New подключается к базе с ретраями. после рестарта постгрес недоступен
// пару секунд (recovery/WAL) - лучше подождать чем сразу падать фаталом,
// иначе api и воркер валятся каскадом на каждом рестарте бд
func New(cfg *config.DatabaseConfig, log *logger.Logger, m *metrics.Metrics) (*DB, error) {
	const (
		connectTimeout = 60 * time.Second
		retryInterval  = 2 * time.Second
	)
	var db *sqlx.DB
	var err error
	deadline := time.Now().Add(connectTimeout)
	for {
		db, err = sqlx.Connect("postgres", cfg.DSN())
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("failed to connect to database after %s: %w", connectTimeout, err)
		}
		log.Warn("Postgres недоступен, повтор подключения",
			zap.Error(err),
			zap.Duration("retry_in", retryInterval),
		)
		time.Sleep(retryInterval)
	}

	// настраиваем пул соединений
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(cfg.MaxLifetime)

	// проверяем соединение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connected successfully",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Name),
	)

	d := &DB{
		DB:      db,
		log:     log,
		metrics: m,
		done:    make(chan struct{}),
	}

	// запускаем мониторинг метрик пула
	go d.monitorConnectionPool()

	return d, nil
}

// monitorConnectionPool периодически обновляет метрики пула соединений
func (db *DB) monitorConnectionPool() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := db.Stats()
			db.metrics.SetDBConnections(
				stats.InUse,
				stats.Idle,
				stats.OpenConnections,
			)
		case <-db.done:
			return
		}
	}
}

// slowQueryThreshold - порог, выше которого запрос считаем медленным и логируем
const slowQueryThreshold = 500 * time.Millisecond

// ExecWithMetrics выполняет запрос с записью метрик
func (db *DB) ExecWithMetrics(ctx context.Context, queryType string, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := db.ExecContext(ctx, query, args...)
	duration := time.Since(start)
	db.metrics.RecordDBQuery(queryType, duration)

	if duration > slowQueryThreshold {
		db.log.Warn("Slow query detected",
			zap.String("query", queryType),
			zap.Duration("duration", duration),
		)
	}

	if err != nil {
		db.log.LogError("Database exec failed", err,
			zap.String("query_type", queryType),
		)
	}

	return result, err
}

// QueryWithMetrics выполняет запрос с записью метрик
func (db *DB) QueryWithMetrics(ctx context.Context, queryType string, dest any, query string, args ...any) error {
	start := time.Now()
	err := db.SelectContext(ctx, dest, query, args...)
	duration := time.Since(start)
	db.metrics.RecordDBQuery(queryType, duration)

	if duration > slowQueryThreshold {
		db.log.Warn("Slow query detected",
			zap.String("query", queryType),
			zap.Duration("duration", duration),
		)
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		db.log.LogError("Database query failed", err,
			zap.String("query_type", queryType),
		)
	}

	return err
}

// QueryRowWithMetrics выполняет запрос одной строки с записью метрик
func (db *DB) QueryRowWithMetrics(ctx context.Context, queryType string, dest any, query string, args ...any) error {
	start := time.Now()
	err := db.GetContext(ctx, dest, query, args...)
	duration := time.Since(start)
	db.metrics.RecordDBQuery(queryType, duration)

	if duration > slowQueryThreshold {
		db.log.Warn("Slow query detected",
			zap.String("query", queryType),
			zap.Duration("duration", duration),
		)
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		db.log.LogError("Database query row failed", err,
			zap.String("query_type", queryType),
		)
	}

	return err
}

func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	return db.DB.BeginTxx(ctx, opts)
}

// RunInTx гоняет fn в транзакции, при ошибке откатывает
func (db *DB) RunInTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (db *DB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return db.PingContext(ctx)
}

// EnsureMatchPartitions создаёт партиции таблицы matches для текущего и следующего месяца.
// Вызывается при старте приложения для гарантии наличия партиций.
func (db *DB) EnsureMatchPartitions(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "SELECT create_matches_partition_if_needed()")
	if err != nil {
		return fmt.Errorf("ensure match partitions: %w", err)
	}
	db.log.Info("Match partitions verified")
	return nil
}

// EnsureRatingHistoryPartitions создаёт партиции таблицы rating_history для текущего и следующего месяца.
// Вызывается при старте приложения для гарантии наличия партиций.
func (db *DB) EnsureRatingHistoryPartitions(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "SELECT create_rating_history_partition_if_needed()")
	if err != nil {
		return fmt.Errorf("ensure rating_history partitions: %w", err)
	}
	db.log.Info("Rating history partitions verified")
	return nil
}

// DropOldPartitions удаляет партиции matches/rating_history старше
// retentionMonths месяцев (функция drop_old_partitions, миграция 000041).
// При retentionMonths <= 0 ничего не делает.
func (db *DB) DropOldPartitions(ctx context.Context, retentionMonths int) error {
	if retentionMonths <= 0 {
		return nil
	}

	for _, table := range []string{"matches", "rating_history"} {
		var dropped int
		if err := db.QueryRowContext(ctx,
			"SELECT drop_old_partitions($1, $2)", table, retentionMonths,
		).Scan(&dropped); err != nil {
			return fmt.Errorf("drop old partitions of %s: %w", table, err)
		}
		if dropped > 0 {
			db.log.Info("Dropped old partitions",
				zap.String("table", table),
				zap.Int("dropped", dropped),
				zap.Int("retention_months", retentionMonths),
			)
		}
	}

	return nil
}

// StartPartitionMaintenance раз в сутки досоздаёт партиции на текущий и
// следующий месяц (иначе при долгой работе без рестарта ловим
// partition-not-found). retentionMonths > 0 ещё и удаляет партиции старше
// этого числа месяцев (DB_PARTITION_RETENTION_MONTHS; 0 - выключено)
func (db *DB) StartPartitionMaintenance(retentionMonths int) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
				if err := db.EnsureMatchPartitions(ctx1); err != nil {
					db.log.Error("Periodic match partition maintenance failed", zap.Error(err))
				}
				cancel1()

				ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
				if err := db.EnsureRatingHistoryPartitions(ctx2); err != nil {
					db.log.Error("Periodic rating_history partition maintenance failed", zap.Error(err))
				}
				cancel2()

				ctx3, cancel3 := context.WithTimeout(context.Background(), 60*time.Second)
				if err := db.DropOldPartitions(ctx3, retentionMonths); err != nil {
					db.log.Error("Periodic partition retention failed", zap.Error(err))
				}
				cancel3()
			case <-db.done:
				return
			}
		}
	}()
}

func (db *DB) Close() error {
	close(db.done)
	db.log.Info("Closing database connection")
	return db.DB.Close()
}

// PreparedStatement - кэш для prepared statement
type PreparedStatement struct {
	stmt *sqlx.NamedStmt
	db   *DB
}

func (db *DB) PrepareNamed(query string) (*PreparedStatement, error) {
	stmt, err := db.DB.PrepareNamed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare named statement: %w", err)
	}

	return &PreparedStatement{
		stmt: stmt,
		db:   db,
	}, nil
}

func (ps *PreparedStatement) ExecContext(ctx context.Context, queryType string, arg any) (sql.Result, error) {
	start := time.Now()
	result, err := ps.stmt.ExecContext(ctx, arg)
	ps.db.metrics.RecordDBQuery(queryType, time.Since(start))

	if err != nil {
		ps.db.log.LogError("Prepared statement exec failed", err,
			zap.String("query_type", queryType),
		)
	}

	return result, err
}

func (ps *PreparedStatement) QueryContext(ctx context.Context, queryType string, dest any, arg any) error {
	start := time.Now()
	err := ps.stmt.SelectContext(ctx, dest, arg)
	ps.db.metrics.RecordDBQuery(queryType, time.Since(start))

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ps.db.log.LogError("Prepared statement query failed", err,
			zap.String("query_type", queryType),
		)
	}

	return err
}

func (ps *PreparedStatement) Close() error {
	return ps.stmt.Close()
}
