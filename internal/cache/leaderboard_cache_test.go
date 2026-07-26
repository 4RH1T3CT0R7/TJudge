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

func TestLeaderboardCache_GetTop(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	for _, r := range []int{1200, 1800, 1500, 2000, 1000} {
		require.NoError(t, lc.UpdateRating(ctx, tournamentID, uuid.New(), r))
	}

	// отсортировано по убыванию рейтинга, ранги с 1
	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 5)
	assert.Equal(t, 2000, entries[0].Rating)
	assert.Equal(t, 1000, entries[4].Rating)
	for i, e := range entries {
		assert.Equal(t, i+1, e.Rank)
	}

	// лимит режет выдачу
	top3, err := lc.GetTop(ctx, tournamentID, 3)
	require.NoError(t, err)
	require.Len(t, top3, 3)
	assert.Equal(t, 1500, top3[2].Rating)
}

func TestLeaderboardCache_GetTop_Empty(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)

	entries, err := lc.GetTop(context.Background(), uuid.New(), 10)
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

	require.NoError(t, lc.UpdateRating(ctx, tournamentID, programID, 1500))
	require.NoError(t, lc.IncrementRating(ctx, tournamentID, programID, 50))

	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	// заодно проверяем что UpdateRating кладёт member/score корректно
	assert.Equal(t, 1, entries[0].Rank)
	assert.Equal(t, programID, entries[0].ProgramID)
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

	require.NoError(t, lc.UpdateRating(ctx, tournamentID, programA, 1500))
	require.NoError(t, lc.UpdateRating(ctx, tournamentID, programB, 1600))

	require.NoError(t, lc.Remove(ctx, tournamentID, programA))

	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, programB, entries[0].ProgramID)
}

// Clear сносит и sorted set, и json-кэши по лимитам (через InvalidateFullLeaderboard)
func TestLeaderboardCache_Clear(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	full := []*domain.LeaderboardEntry{{Rank: 1, ProgramID: uuid.New(), ProgramName: "Alpha", Rating: 2000}}
	for i := range 5 {
		require.NoError(t, lc.UpdateRating(ctx, tournamentID, uuid.New(), 1000+i*100))
	}
	require.NoError(t, lc.SetFullLeaderboard(ctx, tournamentID, 100, full))
	require.NoError(t, lc.SetFullLeaderboard(ctx, tournamentID, 50, full))

	require.NoError(t, lc.Clear(ctx, tournamentID))

	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	assert.Nil(t, entries)

	// json по всем лимитам тоже снесён
	r1, _ := lc.GetFullLeaderboard(ctx, tournamentID, 100)
	r2, _ := lc.GetFullLeaderboard(ctx, tournamentID, 50)
	assert.Nil(t, r1)
	assert.Nil(t, r2)
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

func TestLeaderboardCache_FullCrossGameLeaderboard(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	entries := []*domain.CrossGameLeaderboardEntry{
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

	// формат ключа полного лидерборда: "leaderboard:full:<id>:<limit>"
	limit := 100
	key := fmt.Sprintf("leaderboard:full:%s:%d", tournamentID.String(), limit)
	require.NoError(t, mr.Set(key, "not-json{{{"))

	// битый json автоудаляется, наружу nil без ошибки
	result, err := lc.GetFullLeaderboard(ctx, tournamentID, limit)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.False(t, mr.Exists(key), "битый ключ должен быть удалён")
}

func TestLeaderboardCache_GetTop_InvalidUUID(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	lc := NewLeaderboardCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// ключ sorted set-а: "leaderboard:<id>"
	key := fmt.Sprintf("leaderboard:%s", tournamentID.String())
	_, err := mr.ZAdd(key, 1500, "not-a-valid-uuid")
	require.NoError(t, err)

	// битый uuid просто пропускаем
	entries, err := lc.GetTop(ctx, tournamentID, 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
