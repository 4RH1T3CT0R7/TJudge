package db

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

// newTestDB создаёт *DB поверх go-sqlmock для unit-тестов без реального PostgreSQL.
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
