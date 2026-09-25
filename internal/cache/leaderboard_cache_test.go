package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// инвалидация сносит json-кэши по всем лимитам и кросс-гейм
func TestLeaderboardCache_InvalidateFullLeaderboard(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	full := []*models.LeaderboardEntry{{Rank: 1, ProgramID: uuid.New(), ProgramName: "Alpha", Rating: 2000}}
	cross := []*models.CrossGameLeaderboardEntry{{Rank: 1, ProgramID: uuid.New(), TotalRating: 3500}}
	require.NoError(t, lc.SetFullLeaderboard(ctx, tournamentID, 100, full))
	require.NoError(t, lc.SetFullLeaderboard(ctx, tournamentID, 50, full))
	require.NoError(t, lc.SetFullCrossGameLeaderboard(ctx, tournamentID, cross))

	require.NoError(t, lc.InvalidateFullLeaderboard(ctx, tournamentID))

	r1, _ := lc.GetFullLeaderboard(ctx, tournamentID, 100)
	r2, _ := lc.GetFullLeaderboard(ctx, tournamentID, 50)
	r3, _ := lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
	assert.Nil(t, r1)
	assert.Nil(t, r2)
	assert.Nil(t, r3)
}

func TestLeaderboardCache_FullLeaderboard(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	entries := []*models.LeaderboardEntry{
		{Rank: 1, ProgramID: uuid.New(), ProgramName: "Alpha", Rating: 2000, Wins: 10, Losses: 2},
		{Rank: 2, ProgramID: uuid.New(), ProgramName: "Beta", Rating: 1800, Wins: 8, Losses: 4},
	}

	// промах до записи
	result, err := lc.GetFullLeaderboard(ctx, tournamentID, 100)
	require.NoError(t, err)
	assert.Nil(t, result)

	require.NoError(t, lc.SetFullLeaderboard(ctx, tournamentID, 100, entries))

	result, err = lc.GetFullLeaderboard(ctx, tournamentID, 100)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Alpha", result[0].ProgramName)
	assert.Equal(t, 10, result[0].Wins)

	// разные лимиты — разные ключи
	other, err := lc.GetFullLeaderboard(ctx, tournamentID, 50)
	require.NoError(t, err)
	assert.Nil(t, other)

	// короткий ttl протухает
	mr.FastForward(fullLeaderboardTTL)
	result, err = lc.GetFullLeaderboard(ctx, tournamentID, 100)
	require.NoError(t, err)
	assert.Nil(t, result)
}

// запись с другим limit не продлевает жизнь уже лежащим полям
func TestLeaderboardCache_FullLeaderboard_TTLNotExtended(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()
	entries := []*models.LeaderboardEntry{{Rank: 1, ProgramID: uuid.New(), Rating: 2000}}

	require.NoError(t, lc.SetFullLeaderboard(ctx, tournamentID, 100, entries))
	mr.FastForward(fullLeaderboardTTL - time.Second)
	require.NoError(t, lc.SetFullLeaderboard(ctx, tournamentID, 50, entries))
	mr.FastForward(time.Second)

	result, err := lc.GetFullLeaderboard(ctx, tournamentID, 100)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestLeaderboardCache_FullCrossGameLeaderboard(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	entries := []*models.CrossGameLeaderboardEntry{
		{Rank: 1, ProgramID: uuid.New(), ProgramName: "Alpha", TotalRating: 3500, TotalWins: 15},
	}

	result, err := lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
	require.NoError(t, err)
	assert.Nil(t, result)

	require.NoError(t, lc.SetFullCrossGameLeaderboard(ctx, tournamentID, entries))

	result, err = lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, 3500, result[0].TotalRating)

	mr.FastForward(fullLeaderboardTTL)
	result, err = lc.GetFullCrossGameLeaderboard(ctx, tournamentID)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestLeaderboardCache_GetFullLeaderboard_CorruptJSON(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// полные лидерборды - hash "leaderboard:full:<id>", поле = limit
	limit := 100
	key := fmt.Sprintf("leaderboard:full:%s", tournamentID.String())
	mr.HSet(key, "100", "not-json{{{")

	// битый json автоудаляется, наружу nil без ошибки
	result, err := lc.GetFullLeaderboard(ctx, tournamentID, limit)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.False(t, mr.Exists(key), "битое поле должно быть удалено")
}
