package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// DB оборачивает sqlx.DB и добавляет метрики
type DB struct {
	*sqlx.DB
	log     *logger.Logger
	metrics *metrics.Metrics
}

// New создаёт новое подключение к базе данных
func New(cfg *config.DatabaseConfig, log *logger.Logger, m *metrics.Metrics) (*DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Настраиваем connection pool
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(cfg.MaxLifetime)

	// Проверяем соединение
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
	}

	// Запускаем мониторинг метрик пула
	go d.monitorConnectionPool()

	return d, nil
}

// monitorConnectionPool периодически обновляет метрики пула соединений
func (db *DB) monitorConnectionPool() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := db.Stats()
		db.metrics.SetDBConnections(
			stats.InUse,
			stats.Idle,
			stats.OpenConnections,
		)
	}
}

// ExecWithMetrics выполняет запрос с записью метрик
func (db *DB) ExecWithMetrics(ctx context.Context, queryType string, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := db.ExecContext(ctx, query, args...)
	db.metrics.RecordDBQuery(queryType, time.Since(start))

	if err != nil {
		db.log.LogError("Database exec failed", err,
			zap.String("query_type", queryType),
		)
	}

	return result, err
}

// QueryWithMetrics выполняет запрос с записью метрик
func (db *DB) QueryWithMetrics(ctx context.Context, queryType string, dest interface{}, query string, args ...interface{}) error {
	start := time.Now()
	err := db.SelectContext(ctx, dest, query, args...)
	db.metrics.RecordDBQuery(queryType, time.Since(start))

	if err != nil && err != sql.ErrNoRows {
		db.log.LogError("Database query failed", err,
			zap.String("query_type", queryType),
		)
	}

	return err
}

// QueryRowWithMetrics выполняет запрос одной строки с записью метрик
func (db *DB) QueryRowWithMetrics(ctx context.Context, queryType string, dest interface{}, query string, args ...interface{}) error {
	start := time.Now()
	err := db.GetContext(ctx, dest, query, args...)
	db.metrics.RecordDBQuery(queryType, time.Since(start))

	if err != nil && err != sql.ErrNoRows {
		db.log.LogError("Database query row failed", err,
			zap.String("query_type", queryType),
		)
	}

	return err
}

// BeginTx начинает транзакцию
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	return db.DB.BeginTxx(ctx, opts)
}

// Health проверяет здоровье базы данных
func (db *DB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return db.PingContext(ctx)
}

// Close закрывает соединение с базой данных
func (db *DB) Close() error {
	db.log.Info("Closing database connection")
	return db.DB.Close()
}

// PreparedStatement кэш для prepared statements
type PreparedStatement struct {
	stmt *sqlx.NamedStmt
	db   *DB
}

// PrepareNamed создаёт именованный prepared statement
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

// ExecContext выполняет prepared statement
func (ps *PreparedStatement) ExecContext(ctx context.Context, queryType string, arg interface{}) (sql.Result, error) {
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

// QueryContext выполняет prepared statement query
func (ps *PreparedStatement) QueryContext(ctx context.Context, queryType string, dest interface{}, arg interface{}) error {
	start := time.Now()
	err := ps.stmt.SelectContext(ctx, dest, arg)
	ps.db.metrics.RecordDBQuery(queryType, time.Since(start))

	if err != nil && err != sql.ErrNoRows {
		ps.db.log.LogError("Prepared statement query failed", err,
			zap.String("query_type", queryType),
		)
	}

	return err
}

// Close закрывает prepared statement
func (ps *PreparedStatement) Close() error {
	return ps.stmt.Close()
}
