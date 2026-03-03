package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
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

func TestLeaderboardCache_FullLeaderboard(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	entries := []*domain.LeaderboardEntry{
		{Rank: 1, ProgramID: uuid.New(), ProgramName: "Alpha", Rating: 2000, Wins: 10, Losses: 2},
		{Rank: 2, ProgramID: uuid.New(), ProgramName: "Beta", Rating: 1800, Wins: 8, Losses: 4},
	}

	t.Run("miss returns nil", func(t *testing.T) {
		result, err := lc.GetFullLeaderboard(ctx, tournamentID, 100)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("set and get", func(t *testing.T) {
		err := lc.SetFullLeaderboard(ctx, tournamentID, 100, entries)
		require.NoError(t, err)

		result, err := lc.GetFullLeaderboard(ctx, tournamentID, 100)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "Alpha", result[0].ProgramName)
		assert.Equal(t, 2000, result[0].Rating)
		assert.Equal(t, 10, result[0].Wins)
		assert.Equal(t, "Beta", result[1].ProgramName)
	})

	t.Run("different limits are separate cache entries", func(t *testing.T) {
		result, err := lc.GetFullLeaderboard(ctx, tournamentID, 50)
		require.NoError(t, err)
		assert.Nil(t, result) // limit=50 was never set
	})

	t.Run("TTL expiration", func(t *testing.T) {
		mr.FastForward(fullLeaderboardTTL)

		result, err := lc.GetFullLeaderboard(ctx, tournamentID, 100)
		require.NoError(t, err)
		assert.Nil(t, result) // expired
	})

	t.Run("invalidate clears all limits", func(t *testing.T) {
		_ = lc.SetFullLeaderboard(ctx, tournamentID, 100, entries)
		_ = lc.SetFullLeaderboard(ctx, tournamentID, 50, entries[:1])

		err := lc.InvalidateFullLeaderboard(ctx, tournamentID)
		require.NoError(t, err)

		r1, _ := lc.GetFullLeaderboard(ctx, tournamentID, 100)
		r2, _ := lc.GetFullLeaderboard(ctx, tournamentID, 50)
		assert.Nil(t, r1)
		assert.Nil(t, r2)
	})
}

func TestLeaderboardCache_FullCrossGameLeaderboard(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	entries := []*domain.CrossGameLeaderboardEntry{
		{Rank: 1, ProgramID: uuid.New(), ProgramName: "Alpha", TotalRating: 3500, TotalWins: 15},
	}

	t.Run("miss returns nil", func(t *testing.T) {
		result, err := lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("set and get", func(t *testing.T) {
		err := lc.SetFullCrossGameLeaderboard(ctx, tournamentID, entries)
		require.NoError(t, err)

		result, err := lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Alpha", result[0].ProgramName)
		assert.Equal(t, 3500, result[0].TotalRating)
	})

	t.Run("TTL expiration", func(t *testing.T) {
		mr.FastForward(fullLeaderboardTTL)

		result, err := lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Clear removes cross-game cache too", func(t *testing.T) {
		_ = lc.SetFullCrossGameLeaderboard(ctx, tournamentID, entries)

		err := lc.Clear(ctx, tournamentID)
		require.NoError(t, err)

		result, _ := lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
		assert.Nil(t, result)
	})
}

func TestLeaderboardCache_GetFullLeaderboard_CorruptJSON(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// The full leaderboard key format is "leaderboard:full:<id>:<limit>".
	limit := 100
	key := fmt.Sprintf("leaderboard:full:%s:%d", tournamentID.String(), limit)

	// Set corrupt JSON directly via miniredis.
	mr.Set(key, "not-json{{{")
	require.True(t, mr.Exists(key), "key should exist before GetFullLeaderboard")

	// GetFullLeaderboard should auto-delete the corrupt key and return nil, nil.
	result, err := lc.GetFullLeaderboard(ctx, tournamentID, limit)
	assert.NoError(t, err)
	assert.Nil(t, result)

	// The key should have been deleted.
	assert.False(t, mr.Exists(key), "corrupt key should be deleted")
}

func TestLeaderboardCache_GetTop_InvalidUUID(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// The sorted set key is "leaderboard:<id>".
	key := fmt.Sprintf("leaderboard:%s", tournamentID.String())

	// Add a member with an invalid UUID string to the sorted set.
	mr.ZAdd(key, 1500, "not-a-valid-uuid")

	// GetTop should skip the invalid UUID member and return an empty slice.
	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	assert.Empty(t, entries, "invalid UUID members should be skipped")
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
