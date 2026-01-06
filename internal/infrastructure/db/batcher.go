package db

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/google/uuid"
)

// QueryBatcher группирует запросы для пакетной обработки
type QueryBatcher struct {
	db          *DB
	batchSize   int
	flushPeriod time.Duration

	mu       sync.Mutex
	queries  []batchedQuery
	done     chan struct{}
	flushing bool
}

type batchedQuery struct {
	query    string
	args     []interface{}
	resultCh chan batchResult
}

type batchResult struct {
	rows *sql.Rows
	err  error
}

// NewQueryBatcher создаёт новый batcher
func NewQueryBatcher(db *DB, batchSize int, flushPeriod time.Duration) *QueryBatcher {
	qb := &QueryBatcher{
		db:          db,
		batchSize:   batchSize,
		flushPeriod: flushPeriod,
		queries:     make([]batchedQuery, 0, batchSize),
		done:        make(chan struct{}),
	}

	// Запускаем периодический flush
	go qb.periodicFlush()

	return qb
}

// Query добавляет запрос в batch
func (qb *QueryBatcher) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	resultCh := make(chan batchResult, 1)

	qb.mu.Lock()
	qb.queries = append(qb.queries, batchedQuery{
		query:    query,
		args:     args,
		resultCh: resultCh,
	})

	shouldFlush := len(qb.queries) >= qb.batchSize
	qb.mu.Unlock()

	if shouldFlush {
		qb.Flush(ctx)
	}

	// Ждём результат
	select {
	case result := <-resultCh:
		return result.rows, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Flush выполняет все накопленные запросы
func (qb *QueryBatcher) Flush(ctx context.Context) {
	qb.mu.Lock()
	if qb.flushing || len(qb.queries) == 0 {
		qb.mu.Unlock()
		return
	}
	qb.flushing = true
	queries := qb.queries
	qb.queries = make([]batchedQuery, 0, qb.batchSize)
	qb.mu.Unlock()

	defer func() {
		qb.mu.Lock()
		qb.flushing = false
		qb.mu.Unlock()
	}()

	// Выполняем каждый запрос
	for _, q := range queries {
		rows, err := qb.db.QueryContext(ctx, q.query, q.args...)
		q.resultCh <- batchResult{rows: rows, err: err}
	}
}

// periodicFlush периодически сбрасывает накопленные запросы
func (qb *QueryBatcher) periodicFlush() {
	ticker := time.NewTicker(qb.flushPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			qb.Flush(context.Background())
		case <-qb.done:
			return
		}
	}
}

// Close останавливает batcher
func (qb *QueryBatcher) Close() {
	close(qb.done)
	qb.Flush(context.Background())
}

// BulkInserter выполняет пакетную вставку с использованием транзакций
type BulkInserter struct {
	db        *DB
	tableName string
	columns   []string
	batchSize int
	query     string

	mu     sync.Mutex
	values [][]interface{}
}

// NewBulkInserter создаёт новый bulk inserter
func NewBulkInserter(db *DB, tableName string, columns []string, batchSize int) *BulkInserter {
	return &BulkInserter{
		db:        db,
		tableName: tableName,
		columns:   columns,
		batchSize: batchSize,
		values:    make([][]interface{}, 0, batchSize),
	}
}

// Add добавляет строку для вставки
func (bi *BulkInserter) Add(values ...interface{}) {
	bi.mu.Lock()
	bi.values = append(bi.values, values)
	shouldFlush := len(bi.values) >= bi.batchSize
	bi.mu.Unlock()

	if shouldFlush {
		_ = bi.Flush(context.Background())
	}
}

// Flush вставляет все накопленные строки
func (bi *BulkInserter) Flush(ctx context.Context) error {
	bi.mu.Lock()
	if len(bi.values) == 0 {
		bi.mu.Unlock()
		return nil
	}
	values := bi.values
	bi.values = make([][]interface{}, 0, bi.batchSize)
	bi.mu.Unlock()

	// Используем транзакцию для batch insert
	tx, err := bi.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Prepare statement
	stmt, err := tx.PrepareContext(ctx, bi.buildInsertQuery())
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Вставляем все строки
	for _, row := range values {
		if _, err := stmt.ExecContext(ctx, row...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// buildInsertQuery строит SQL запрос для вставки
func (bi *BulkInserter) buildInsertQuery() string {
	if bi.query != "" {
		return bi.query
	}

	// Строим запрос: INSERT INTO table (col1, col2) VALUES ($1, $2)
	query := "INSERT INTO " + bi.tableName + " ("
	placeholders := "VALUES ("

	for i, col := range bi.columns {
		if i > 0 {
			query += ", "
			placeholders += ", "
		}
		query += col
		placeholders += "$" + string(rune('1'+i))
	}

	bi.query = query + ") " + placeholders + ")"
	return bi.query
}

// IDLoader загружает множество объектов по ID одним запросом
type IDLoader[T any] struct {
	db         *DB
	loadFunc   func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]T, error)
	batchSize  int
	waitPeriod time.Duration

	mu      sync.Mutex
	pending map[uuid.UUID][]chan loadResult[T]
	timer   *time.Timer
}

type loadResult[T any] struct {
	value T
	err   error
}

// ErrNotFound возвращается когда объект не найден
var ErrNotFound = sql.ErrNoRows

// NewIDLoader создаёт новый ID loader
func NewIDLoader[T any](
	db *DB,
	loadFunc func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]T, error),
	batchSize int,
	waitPeriod time.Duration,
) *IDLoader[T] {
	return &IDLoader[T]{
		db:         db,
		loadFunc:   loadFunc,
		batchSize:  batchSize,
		waitPeriod: waitPeriod,
		pending:    make(map[uuid.UUID][]chan loadResult[T]),
	}
}

// Load загружает объект по ID, группируя запросы
func (l *IDLoader[T]) Load(ctx context.Context, id uuid.UUID) (T, error) {
	resultCh := make(chan loadResult[T], 1)

	l.mu.Lock()
	l.pending[id] = append(l.pending[id], resultCh)
	shouldFlush := len(l.pending) >= l.batchSize

	// Устанавливаем таймер если это первый запрос
	if l.timer == nil && !shouldFlush {
		l.timer = time.AfterFunc(l.waitPeriod, func() {
			l.flush(context.Background())
		})
	}
	l.mu.Unlock()

	if shouldFlush {
		l.flush(ctx)
	}

	select {
	case result := <-resultCh:
		return result.value, result.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// flush загружает все pending ID
func (l *IDLoader[T]) flush(ctx context.Context) {
	l.mu.Lock()
	if len(l.pending) == 0 {
		l.mu.Unlock()
		return
	}

	pending := l.pending
	l.pending = make(map[uuid.UUID][]chan loadResult[T])
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	l.mu.Unlock()

	// Собираем все ID
	ids := make([]uuid.UUID, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}

	// Загружаем все объекты
	results, err := l.loadFunc(ctx, ids)

	// Отправляем результаты
	for id, channels := range pending {
		var result loadResult[T]
		if err != nil {
			result.err = err
		} else if val, ok := results[id]; ok {
			result.value = val
		} else {
			result.err = ErrNotFound
		}

		for _, ch := range channels {
			ch <- result
		}
	}
}
