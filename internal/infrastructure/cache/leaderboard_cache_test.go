package cache

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaderboardCache_UpdateRating(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()
	programID := uuid.New()

	err := lc.UpdateRating(ctx, tournamentID, programID, 1500)
	require.NoError(t, err)

	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, 1, entries[0].Rank)
	assert.Equal(t, programID, entries[0].ProgramID)
	assert.Equal(t, 1500, entries[0].Rating)
}

func TestLeaderboardCache_GetTop(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// Add multiple entries with different ratings.
	programs := []struct {
		id     uuid.UUID
		rating int
	}{
		{uuid.New(), 1200},
		{uuid.New(), 1800},
		{uuid.New(), 1500},
		{uuid.New(), 2000},
		{uuid.New(), 1000},
	}

	for _, p := range programs {
		err := lc.UpdateRating(ctx, tournamentID, p.id, p.rating)
		require.NoError(t, err)
	}

	t.Run("returns entries ordered by rating descending", func(t *testing.T) {
		entries, err := lc.GetTop(ctx, tournamentID, 10)
		require.NoError(t, err)
		require.Len(t, entries, 5)

		// Highest rating first.
		assert.Equal(t, 2000, entries[0].Rating)
		assert.Equal(t, 1800, entries[1].Rating)
		assert.Equal(t, 1500, entries[2].Rating)
		assert.Equal(t, 1200, entries[3].Rating)
		assert.Equal(t, 1000, entries[4].Rating)

		// Ranks are sequential starting at 1.
		for i, entry := range entries {
			assert.Equal(t, i+1, entry.Rank)
		}
	})

	t.Run("limit restricts number of results", func(t *testing.T) {
		entries, err := lc.GetTop(ctx, tournamentID, 3)
		require.NoError(t, err)
		require.Len(t, entries, 3)

		assert.Equal(t, 2000, entries[0].Rating)
		assert.Equal(t, 1800, entries[1].Rating)
		assert.Equal(t, 1500, entries[2].Rating)
	})
}

func TestLeaderboardCache_GetTop_Empty(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()

	entries, err := lc.GetTop(ctx, uuid.New(), 10)
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestLeaderboardCache_IncrementRating(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()
	programID := uuid.New()

	// Set initial rating.
	err := lc.UpdateRating(ctx, tournamentID, programID, 1500)
	require.NoError(t, err)

	// Increment by 50.
	err = lc.IncrementRating(ctx, tournamentID, programID, 50)
	require.NoError(t, err)

	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 1550, entries[0].Rating)
}

func TestLeaderboardCache_Remove(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	programA := uuid.New()
	programB := uuid.New()

	err := lc.UpdateRating(ctx, tournamentID, programA, 1500)
	require.NoError(t, err)
	err = lc.UpdateRating(ctx, tournamentID, programB, 1600)
	require.NoError(t, err)

	// Remove programA.
	err = lc.Remove(ctx, tournamentID, programA)
	require.NoError(t, err)

	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, programB, entries[0].ProgramID)
	assert.Equal(t, 1600, entries[0].Rating)
}

func TestLeaderboardCache_Clear(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// Add several entries.
	for i := 0; i < 5; i++ {
		err := lc.UpdateRating(ctx, tournamentID, uuid.New(), 1000+i*100)
		require.NoError(t, err)
	}

	// Clear the leaderboard.
	err := lc.Clear(ctx, tournamentID)
	require.NoError(t, err)

	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	assert.Nil(t, entries)
}
