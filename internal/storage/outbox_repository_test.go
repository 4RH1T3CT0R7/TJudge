//go:build integration

package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// удаляются только done старше порога: свежие done и error остаются
func TestOutboxRepository_PurgeDone(t *testing.T) {
	database := setupTestDB(t)
	repo := storage.NewOutboxRepository(database)
	ctx := context.Background()

	oldDone, freshDone, oldError := uuid.New(), uuid.New(), uuid.New()
	for _, row := range []struct {
		matchID uuid.UUID
		status  string
		age     string
	}{
		{oldDone, "done", "8 days"},
		{freshDone, "done", "1 hour"},
		{oldError, "error", "8 days"},
	} {
		_, err := database.ExecContext(ctx,
			"INSERT INTO match_outbox (match_id, status, processed_at) VALUES ($1, $2, NOW() - $3::interval)",
			row.matchID, row.status, row.age)
		require.NoError(t, err)
		t.Cleanup(func() { cleanupTable(t, database, "match_outbox", "match_id = $1", row.matchID) })
	}

	_, err := repo.PurgeDone(ctx, 7*24*time.Hour)
	require.NoError(t, err)

	remaining := func(matchID uuid.UUID) int {
		var n int
		require.NoError(t, database.GetContext(ctx, &n, "SELECT COUNT(*) FROM match_outbox WHERE match_id = $1", matchID))
		return n
	}
	assert.Zero(t, remaining(oldDone))
	assert.Equal(t, 1, remaining(freshDone))
	assert.Equal(t, 1, remaining(oldError))
}
