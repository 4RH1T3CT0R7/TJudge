package db

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	mockDB, mock, err := sqlmock.New()
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

// --- isUndefinedFunctionError ---

func TestIsUndefinedFunctionError_Nil(t *testing.T) {
	assert.False(t, isUndefinedFunctionError(nil))
}

func TestIsUndefinedFunctionError_PqFormat(t *testing.T) {
	err := &testError{msg: "pq: function refresh_leaderboards() does not exist"}
	assert.True(t, isUndefinedFunctionError(err))
}

func TestIsUndefinedFunctionError_SQLSTATEFormat(t *testing.T) {
	err := &testError{msg: "ERROR: function refresh_leaderboards() does not exist (SQLSTATE 42883)"}
	assert.True(t, isUndefinedFunctionError(err))
}

func TestIsUndefinedFunctionError_OtherError(t *testing.T) {
	err := &testError{msg: "connection refused"}
	assert.False(t, isUndefinedFunctionError(err))
}

func TestIsUndefinedFunctionError_SimilarButDifferent(t *testing.T) {
	err := &testError{msg: "pq: function other_function() does not exist"}
	assert.False(t, isUndefinedFunctionError(err))
}

// --- NewLeaderboardRefresher ---

func TestNewLeaderboardRefresher_Fields(t *testing.T) {
	log, _ := logger.New("error", "json")
	r := NewLeaderboardRefresher(nil, 5*time.Minute, log)

	assert.NotNil(t, r)
	assert.Equal(t, 5*time.Minute, r.interval)
	assert.NotNil(t, r.stopCh)
	assert.NotNil(t, r.doneCh)
}

// --- refresh ---

func TestLeaderboardRefresher_Refresh_Success(t *testing.T) {
	db, mock := newTestDB(t)
	log, _ := logger.New("error", "json")
	r := NewLeaderboardRefresher(db, time.Hour, log)

	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r.RefreshNow()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLeaderboardRefresher_Refresh_UndefinedFunction(t *testing.T) {
	db, mock := newTestDB(t)
	log, _ := logger.New("error", "json")
	r := NewLeaderboardRefresher(db, time.Hour, log)

	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnError(&testError{msg: "pq: function refresh_leaderboards() does not exist"})

	// Should not panic, just log and skip
	require.NotPanics(t, func() {
		r.RefreshNow()
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLeaderboardRefresher_Refresh_OtherError(t *testing.T) {
	db, mock := newTestDB(t)
	log, _ := logger.New("error", "json")
	r := NewLeaderboardRefresher(db, time.Hour, log)

	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnError(&testError{msg: "connection refused"})

	require.NotPanics(t, func() {
		r.RefreshNow()
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Start / Stop lifecycle ---

func TestLeaderboardRefresher_StartStop(t *testing.T) {
	db, mock := newTestDB(t)
	log, _ := logger.New("error", "json")
	r := NewLeaderboardRefresher(db, 50*time.Millisecond, log)

	// Allow any number of refresh calls
	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnResult(sqlmock.NewResult(0, 0)).
		WillDelayFor(0)
	// For subsequent ticks
	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnResult(sqlmock.NewResult(0, 0)).
		WillDelayFor(0)
	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnResult(sqlmock.NewResult(0, 0)).
		WillDelayFor(0)
	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnResult(sqlmock.NewResult(0, 0)).
		WillDelayFor(0)
	mock.ExpectExec("SELECT refresh_leaderboards").
		WillReturnResult(sqlmock.NewResult(0, 0)).
		WillDelayFor(0)

	r.Start()
	time.Sleep(120 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		r.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Stopped successfully
	case <-time.After(2 * time.Second):
		t.Fatal("LeaderboardRefresher.Stop did not return")
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
