package storage

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// агрегат по matches в пределах statusCacheTTL считается один раз
func TestSystemStatus_MatchCountsCached(t *testing.T) {
	db, mock := newTestDB(t)
	repo := NewSystemStatusRepository(db)

	mock.ExpectQuery("SELECT status, COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).AddRow("pending", 3))

	for range 2 {
		counts, err := repo.MatchCountsByStatus(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(3), counts["pending"])
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
