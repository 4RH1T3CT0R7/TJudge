package db

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- QueryBatcher ---

func TestQueryBatcher_Query_SingleQuery(t *testing.T) {
	db, mock := newTestDB(t)

	rows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	qb := NewQueryBatcher(db, 1, time.Second)
	defer qb.Close()

	result, err := qb.Query(context.Background(), "SELECT 1")
	require.NoError(t, err)
	require.NotNil(t, result)
	result.Close()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryBatcher_Flush_Empty(t *testing.T) {
	db, _ := newTestDB(t)
	qb := NewQueryBatcher(db, 10, time.Second)
	defer qb.Close()

	// Flushing empty batcher should not panic
	require.NotPanics(t, func() {
		qb.Flush(context.Background())
	})
}

func TestQueryBatcher_Flush_BatchSize(t *testing.T) {
	db, mock := newTestDB(t)
	mock.MatchExpectationsInOrder(false)

	// Two queries that should auto-flush at batch size 2
	rows1 := sqlmock.NewRows([]string{"id"}).AddRow(1)
	rows2 := sqlmock.NewRows([]string{"id"}).AddRow(2)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows1)
	mock.ExpectQuery("SELECT 2").WillReturnRows(rows2)

	qb := NewQueryBatcher(db, 2, 10*time.Second)
	defer qb.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	var r1 *sql.Rows
	var err1 error
	go func() {
		defer wg.Done()
		r1, err1 = qb.Query(context.Background(), "SELECT 1")
	}()

	var r2 *sql.Rows
	var err2 error
	go func() {
		defer wg.Done()
		r2, err2 = qb.Query(context.Background(), "SELECT 2")
	}()

	wg.Wait()

	require.NoError(t, err1)
	require.NoError(t, err2)
	if r1 != nil {
		r1.Close()
	}
	if r2 != nil {
		r2.Close()
	}
}

func TestQueryBatcher_PeriodicFlush(t *testing.T) {
	db, mock := newTestDB(t)

	rows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	// Batch size large, but flush period short — should auto-flush
	qb := NewQueryBatcher(db, 100, 50*time.Millisecond)
	defer qb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := qb.Query(ctx, "SELECT 1")
	require.NoError(t, err)
	require.NotNil(t, result)
	result.Close()
}

func TestQueryBatcher_Close(t *testing.T) {
	db, _ := newTestDB(t)
	qb := NewQueryBatcher(db, 10, time.Second)

	// Close should not panic even when empty
	require.NotPanics(t, func() {
		qb.Close()
	})
}

func TestQueryBatcher_ContextCancellation(t *testing.T) {
	db, _ := newTestDB(t)

	// Large batch size, long flush period — won't auto-flush
	qb := NewQueryBatcher(db, 100, 10*time.Second)
	defer qb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := qb.Query(ctx, "SELECT 1")
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// --- BulkInserter ---

func TestBulkInserter_BuildInsertQuery(t *testing.T) {
	db, _ := newTestDB(t)
	bi := NewBulkInserter(db, "users", []string{"name", "email"}, 10)

	query := bi.buildInsertQuery()
	assert.Contains(t, query, "INSERT INTO users")
	assert.Contains(t, query, "name")
	assert.Contains(t, query, "email")
	assert.Contains(t, query, "$1")
	assert.Contains(t, query, "$2")
}

func TestBulkInserter_BuildInsertQuery_SingleColumn(t *testing.T) {
	db, _ := newTestDB(t)
	bi := NewBulkInserter(db, "tags", []string{"name"}, 10)

	query := bi.buildInsertQuery()
	assert.Contains(t, query, "INSERT INTO tags (name) VALUES ($1)")
}

func TestBulkInserter_BuildInsertQuery_Cached(t *testing.T) {
	db, _ := newTestDB(t)
	bi := NewBulkInserter(db, "users", []string{"name"}, 10)

	q1 := bi.buildInsertQuery()
	q2 := bi.buildInsertQuery()
	assert.Equal(t, q1, q2) // Should return cached query
}

func TestBulkInserter_Flush_Empty(t *testing.T) {
	db, _ := newTestDB(t)
	bi := NewBulkInserter(db, "users", []string{"name"}, 10)

	err := bi.Flush(context.Background())
	assert.NoError(t, err)
}

func TestBulkInserter_Flush_WithData(t *testing.T) {
	db, mock := newTestDB(t)
	bi := NewBulkInserter(db, "users", []string{"name"}, 10)

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO users")
	mock.ExpectExec("INSERT INTO users").WithArgs("alice").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("bob").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	bi.Add("alice")
	bi.Add("bob")

	err := bi.Flush(context.Background())
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkInserter_Add_AutoFlush(t *testing.T) {
	db, mock := newTestDB(t)
	bi := NewBulkInserter(db, "users", []string{"name"}, 2)

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO users")
	mock.ExpectExec("INSERT INTO users").WithArgs("alice").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("bob").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	bi.Add("alice")
	bi.Add("bob") // Should trigger auto-flush at batch size 2

	// Give a moment for the background flush
	time.Sleep(10 * time.Millisecond)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkInserter_Flush_BeginTxError(t *testing.T) {
	db, mock := newTestDB(t)
	bi := NewBulkInserter(db, "users", []string{"name"}, 10)

	mock.ExpectBegin().WillReturnError(&testError{msg: "tx error"})

	bi.Add("alice")
	err := bi.Flush(context.Background())
	assert.Error(t, err)
}

// --- IDLoader ---

func TestIDLoader_Load_Single(t *testing.T) {
	db, _ := newTestDB(t)
	id1 := uuid.New()

	loadFunc := func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		result := make(map[uuid.UUID]string)
		for _, id := range ids {
			result[id] = "found:" + id.String()
		}
		return result, nil
	}

	loader := NewIDLoader[string](db, loadFunc, 1, 100*time.Millisecond)

	val, err := loader.Load(context.Background(), id1)
	require.NoError(t, err)
	assert.Equal(t, "found:"+id1.String(), val)
}

func TestIDLoader_Load_Batch(t *testing.T) {
	db, _ := newTestDB(t)
	id1, id2 := uuid.New(), uuid.New()

	var loadedIDs []uuid.UUID
	var mu sync.Mutex

	loadFunc := func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		mu.Lock()
		loadedIDs = append(loadedIDs, ids...)
		mu.Unlock()

		result := make(map[uuid.UUID]string)
		for _, id := range ids {
			result[id] = "val:" + id.String()
		}
		return result, nil
	}

	loader := NewIDLoader[string](db, loadFunc, 2, 5*time.Second)

	var wg sync.WaitGroup
	wg.Add(2)

	var val1, val2 string
	var err1, err2 error

	go func() {
		defer wg.Done()
		val1, err1 = loader.Load(context.Background(), id1)
	}()
	go func() {
		defer wg.Done()
		val2, err2 = loader.Load(context.Background(), id2)
	}()

	wg.Wait()

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, "val:"+id1.String(), val1)
	assert.Equal(t, "val:"+id2.String(), val2)
}

func TestIDLoader_Load_NotFound(t *testing.T) {
	db, _ := newTestDB(t)
	id1 := uuid.New()

	loadFunc := func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		// Return empty — id not found
		return make(map[uuid.UUID]string), nil
	}

	loader := NewIDLoader[string](db, loadFunc, 1, 100*time.Millisecond)

	_, err := loader.Load(context.Background(), id1)
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
}

func TestIDLoader_Load_ErrorBroadcast(t *testing.T) {
	db, _ := newTestDB(t)
	id1 := uuid.New()

	loadFunc := func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		return nil, &testError{msg: "load error"}
	}

	loader := NewIDLoader[string](db, loadFunc, 1, 100*time.Millisecond)

	_, err := loader.Load(context.Background(), id1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load error")
}

func TestIDLoader_Load_TimerFlush(t *testing.T) {
	db, _ := newTestDB(t)
	id1 := uuid.New()

	loadFunc := func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		result := make(map[uuid.UUID]string)
		for _, id := range ids {
			result[id] = "timer:" + id.String()
		}
		return result, nil
	}

	// Large batch size but short wait period — timer should trigger flush
	loader := NewIDLoader[string](db, loadFunc, 100, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	val, err := loader.Load(ctx, id1)
	require.NoError(t, err)
	assert.Equal(t, "timer:"+id1.String(), val)
}

func TestIDLoader_Load_ContextCancellation(t *testing.T) {
	db, _ := newTestDB(t)
	id1 := uuid.New()

	loadFunc := func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		// Simulate slow load
		time.Sleep(5 * time.Second)
		return nil, nil
	}

	// Large batch size, long wait — won't auto-flush
	loader := NewIDLoader[string](db, loadFunc, 100, 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := loader.Load(ctx, id1)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestIDLoader_Load_Concurrent_SameID(t *testing.T) {
	db, _ := newTestDB(t)
	id1 := uuid.New()
	callCount := 0
	var mu sync.Mutex

	loadFunc := func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		mu.Lock()
		callCount++
		mu.Unlock()

		result := make(map[uuid.UUID]string)
		for _, id := range ids {
			result[id] = "shared"
		}
		return result, nil
	}

	loader := NewIDLoader[string](db, loadFunc, 2, 50*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			val, err := loader.Load(context.Background(), id1)
			assert.NoError(t, err)
			assert.Equal(t, "shared", val)
		}()
	}

	wg.Wait()
}

// --- Concurrency / Race Detection Tests ---

func TestIDLoader_ConcurrentLoad_RaceDetection(t *testing.T) {
	db, _ := newTestDB(t)

	const goroutines = 20

	// Generate unique UUIDs upfront, one per goroutine.
	ids := make([]uuid.UUID, goroutines)
	for i := 0; i < goroutines; i++ {
		ids[i] = uuid.New()
	}

	loadFunc := func(ctx context.Context, reqIDs []uuid.UUID) (map[uuid.UUID]string, error) {
		result := make(map[uuid.UUID]string, len(reqIDs))
		for _, id := range reqIDs {
			result[id] = "result:" + id.String()
		}
		return result, nil
	}

	// Use a batch size larger than goroutines so the timer flush path is exercised,
	// and a short wait period so tests complete quickly.
	loader := NewIDLoader[string](db, loadFunc, goroutines+1, 50*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]string, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = loader.Load(context.Background(), ids[idx])
		}(i)
	}

	wg.Wait()

	for i := 0; i < goroutines; i++ {
		require.NoError(t, errs[i], "goroutine %d should not error", i)
		assert.Equal(t, "result:"+ids[i].String(), results[i],
			"goroutine %d should receive correct result", i)
	}
}
