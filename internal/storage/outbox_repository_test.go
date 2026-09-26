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
		t.Cleanup(func() {
			_, _ = database.ExecContext(context.Background(), "DELETE FROM match_outbox WHERE match_id = $1", row.matchID)
		})
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

// забираются только старые pending с запасом попыток; взятая задача под lease
// повторно не выдаётся
func TestOutboxRepository_ClaimPending(t *testing.T) {
	database := setupTestDB(t)
	repo := storage.NewOutboxRepository(database)
	ctx := context.Background()

	stale, fresh, exhausted := uuid.New(), uuid.New(), uuid.New()
	for _, row := range []struct {
		matchID  uuid.UUID
		age      string
		attempts int
	}{
		{stale, "1 hour", 0},
		{fresh, "0 seconds", 0},
		{exhausted, "1 hour", 10},
	} {
		_, err := database.ExecContext(ctx,
			"INSERT INTO match_outbox (match_id, attempts, created_at) VALUES ($1, $2, NOW() - $3::interval)",
			row.matchID, row.attempts, row.age)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = database.ExecContext(context.Background(), "DELETE FROM match_outbox WHERE match_id = $1", row.matchID)
		})
	}

	ours := func(entries []*storage.OutboxEntry) []*storage.OutboxEntry {
		var out []*storage.OutboxEntry
		for _, e := range entries {
			if e.MatchID == stale || e.MatchID == fresh || e.MatchID == exhausted {
				out = append(out, e)
			}
		}
		return out
	}

	claimed, err := repo.ClaimPending(ctx, time.Minute, 100)
	require.NoError(t, err)
	claimed = ours(claimed)
	require.Len(t, claimed, 1)
	assert.Equal(t, stale, claimed[0].MatchID)
	assert.Equal(t, 1, claimed[0].Attempts)
	assert.Equal(t, storage.OutboxKindRatingUpdate, claimed[0].Kind)

	again, err := repo.ClaimPending(ctx, time.Minute, 100)
	require.NoError(t, err)
	assert.Empty(t, ours(again))
}
