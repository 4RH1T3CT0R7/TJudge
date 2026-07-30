package storage

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// с MonitorPingsOption sqlmock реально проверяет ExpectPing, без него - игнорит
func newTestDBWithPing(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	mockDB, mock, err := sqlmock.NewWithDSN("sqlmock", sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { mockDB.Close() })

	log, _ := logger.New("error", "json")
	m := metrics.New()
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")

	return &DB{
		DB:      sqlxDB,
		log:     log,
		metrics: m,
		done:    make(chan struct{}),
	}, mock
}

// --- ExecWithMetrics ---

func TestDB_ExecWithMetrics_Success(t *testing.T) {
	db, mock := newTestDB(t)

	mock.ExpectExec("INSERT INTO users").
		WithArgs("alice").
		WillReturnResult(sqlmock.NewResult(1, 1))

	result, err := db.ExecWithMetrics(context.Background(), "insert_user", "INSERT INTO users (name) VALUES ($1)", "alice")
	require.NoError(t, err)

	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rowsAffected)

	lastID, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Equal(t, int64(1), lastID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDB_ExecWithMetrics_Error(t *testing.T) {
	db, mock := newTestDB(t)

	expectedErr := fmt.Errorf("connection refused")
	mock.ExpectExec("INSERT INTO users").
		WithArgs("alice").
		WillReturnError(expectedErr)

	result, err := db.ExecWithMetrics(context.Background(), "insert_user", "INSERT INTO users (name) VALUES ($1)", "alice")
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- QueryWithMetrics ---

func TestDB_QueryWithMetrics_Success(t *testing.T) {
	db, mock := newTestDB(t)

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "alice").
		AddRow(2, "bob")

	mock.ExpectQuery("SELECT id, name FROM users").WillReturnRows(rows)

	type user struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	var results []user
	err := db.QueryWithMetrics(context.Background(), "list_users", &results, "SELECT id, name FROM users")
	require.NoError(t, err)

	require.Len(t, results, 2)
	assert.Equal(t, 1, results[0].ID)
	assert.Equal(t, "alice", results[0].Name)
	assert.Equal(t, 2, results[1].ID)
	assert.Equal(t, "bob", results[1].Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDB_QueryWithMetrics_Error(t *testing.T) {
	db, mock := newTestDB(t)

	expectedErr := fmt.Errorf("table not found")
	mock.ExpectQuery("SELECT id FROM users").WillReturnError(expectedErr)

	type user struct {
		ID int `db:"id"`
	}

	var results []user
	err := db.QueryWithMetrics(context.Background(), "list_users", &results, "SELECT id FROM users")
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Empty(t, results)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- QueryRowWithMetrics ---

func TestDB_QueryRowWithMetrics_Success(t *testing.T) {
	db, mock := newTestDB(t)

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(42, "charlie")

	mock.ExpectQuery("SELECT id, name FROM users WHERE id").WillReturnRows(rows)

	type user struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	var result user
	err := db.QueryRowWithMetrics(context.Background(), "get_user", &result, "SELECT id, name FROM users WHERE id = $1", 42)
	require.NoError(t, err)

	assert.Equal(t, 42, result.ID)
	assert.Equal(t, "charlie", result.Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDB_QueryRowWithMetrics_NoRows(t *testing.T) {
	db, mock := newTestDB(t)

	rows := sqlmock.NewRows([]string{"id", "name"}) // пустой результат

	mock.ExpectQuery("SELECT id, name FROM users WHERE id").WillReturnRows(rows)

	type user struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	var result user
	err := db.QueryRowWithMetrics(context.Background(), "get_user", &result, "SELECT id, name FROM users WHERE id = $1", 999)
	assert.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- BeginTx ---

func TestDB_BeginTx_Success(t *testing.T) {
	db, mock := newTestDB(t)

	mock.ExpectBegin()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, tx)

	// откатываем, чтобы ожидания мока остались чистыми
	mock.ExpectRollback()
	_ = tx.Rollback()

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Health ---

func TestDB_Health_Success(t *testing.T) {
	db, mock := newTestDBWithPing(t)

	mock.ExpectPing()

	err := db.Health(context.Background())
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Close ---

func TestDB_Close(t *testing.T) {
	db, mock := newTestDB(t)

	mock.ExpectClose()

	require.NotPanics(t, func() {
		err := db.Close()
		assert.NoError(t, err)
	})
}
