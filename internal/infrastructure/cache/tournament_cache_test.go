package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTournament() *domain.Tournament {
	id := uuid.New()
	creatorID := uuid.New()
	maxPart := 16
	now := time.Now().Truncate(time.Second)
	return &domain.Tournament{
		ID:              id,
		Name:            "Test Tournament",
		Code:            "TEST01",
		Description:     "A test tournament",
		GameType:        "prisoners_dilemma",
		Status:          domain.TournamentPending,
		MaxParticipants: &maxPart,
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       &creatorID,
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestTournamentCache_SetGet(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournament := newTestTournament()

	err := tc.Set(ctx, tournament)
	require.NoError(t, err)

	got, err := tc.Get(ctx, tournament.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, tournament.ID, got.ID)
	assert.Equal(t, tournament.Name, got.Name)
	assert.Equal(t, tournament.Code, got.Code)
	assert.Equal(t, tournament.Description, got.Description)
	assert.Equal(t, tournament.GameType, got.GameType)
	assert.Equal(t, tournament.Status, got.Status)
	assert.Equal(t, *tournament.MaxParticipants, *got.MaxParticipants)
	assert.Equal(t, tournament.MaxTeamSize, got.MaxTeamSize)
	assert.Equal(t, tournament.IsPermanent, got.IsPermanent)
	assert.Equal(t, *tournament.CreatorID, *got.CreatorID)
	assert.Equal(t, tournament.Version, got.Version)
}

func TestTournamentCache_Get_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	got, err := tc.Get(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTournamentCache_Invalidate(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournament := newTestTournament()

	err := tc.Set(ctx, tournament)
	require.NoError(t, err)

	err = tc.Invalidate(ctx, tournament.ID)
	require.NoError(t, err)

	got, err := tc.Get(ctx, tournament.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTournamentCache_Exists(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournament := newTestTournament()

	t.Run("exists after set", func(t *testing.T) {
		err := tc.Set(ctx, tournament)
		require.NoError(t, err)

		exists, err := tc.Exists(ctx, tournament.ID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("does not exist after invalidate", func(t *testing.T) {
		err := tc.Invalidate(ctx, tournament.ID)
		require.NoError(t, err)

		exists, err := tc.Exists(ctx, tournament.ID)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestTournamentCache_SetGetParticipantsCount(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	err := tc.SetParticipantsCount(ctx, tournamentID, 42)
	require.NoError(t, err)

	count, err := tc.GetParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestTournamentCache_GetParticipantsCount_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	count, err := tc.GetParticipantsCount(ctx, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, -1, count)
}

func TestTournamentCache_IncrementParticipantsCount(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	t.Run("increment from existing value", func(t *testing.T) {
		err := tc.SetParticipantsCount(ctx, tournamentID, 5)
		require.NoError(t, err)

		err = tc.IncrementParticipantsCount(ctx, tournamentID)
		require.NoError(t, err)

		count, err := tc.GetParticipantsCount(ctx, tournamentID)
		require.NoError(t, err)
		assert.Equal(t, 6, count)
	})

	t.Run("increment when key does not exist sets to 1", func(t *testing.T) {
		newID := uuid.New()
		err := tc.IncrementParticipantsCount(ctx, newID)
		require.NoError(t, err)

		count, err := tc.GetParticipantsCount(ctx, newID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestTournamentCache_SetGetMatchStatistics(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	stats := map[string]int{
		"total":     100,
		"completed": 80,
		"pending":   15,
		"failed":    5,
	}

	err := tc.SetMatchStatistics(ctx, tournamentID, stats)
	require.NoError(t, err)

	got, err := tc.GetMatchStatistics(ctx, tournamentID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, stats["total"], got["total"])
	assert.Equal(t, stats["completed"], got["completed"])
	assert.Equal(t, stats["pending"], got["pending"])
	assert.Equal(t, stats["failed"], got["failed"])
}

func TestTournamentCache_GetMatchStatistics_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	got, err := tc.GetMatchStatistics(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTournamentCache_SetGetList(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	tournaments := []*domain.Tournament{
		newTestTournament(),
		newTestTournament(),
		newTestTournament(),
	}
	// Give them distinct names for verification.
	tournaments[0].Name = "Alpha"
	tournaments[1].Name = "Beta"
	tournaments[2].Name = "Gamma"

	filter := "status:active"

	err := tc.SetList(ctx, filter, tournaments)
	require.NoError(t, err)

	got, err := tc.GetList(ctx, filter)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got, 3)

	assert.Equal(t, "Alpha", got[0].Name)
	assert.Equal(t, "Beta", got[1].Name)
	assert.Equal(t, "Gamma", got[2].Name)

	// Verify IDs match.
	for i := range tournaments {
		assert.Equal(t, tournaments[i].ID, got[i].ID)
	}
}

func TestTournamentCache_GetList_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	got, err := tc.GetList(ctx, "nonexistent:filter")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTournamentCache_IncrementParticipantsCount_SetsExpiry(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	err := tc.IncrementParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)

	// Verify that TTL was set on the key (first increment creates the key with val=1).
	key := fmt.Sprintf("tournament:%s:participants_count", tournamentID.String())
	ttl := mr.TTL(key)
	assert.Greater(t, ttl, time.Duration(0), "TTL should be set after first increment")
}

func TestTournamentCache_IncrementParticipantsCount_NoExpiryReset(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// First increment — creates the key with value 1 and sets TTL.
	err := tc.IncrementParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)

	key := fmt.Sprintf("tournament:%s:participants_count", tournamentID.String())
	ttlAfterFirst := mr.TTL(key)
	assert.Greater(t, ttlAfterFirst, time.Duration(0), "TTL should be set after first increment")

	// Simulate some time passing so TTL decreases.
	mr.FastForward(10 * time.Second)
	ttlBeforeSecond := mr.TTL(key)

	// Second increment — value should become 2, TTL should NOT be reset.
	err = tc.IncrementParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)

	// Verify value is 2.
	count, err := tc.GetParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// TTL should not have increased (it was only set when val==1).
	ttlAfterSecond := mr.TTL(key)
	assert.LessOrEqual(t, ttlAfterSecond, ttlBeforeSecond,
		"TTL should not be reset on subsequent increments")
}

func TestTournamentCache_InvalidateList_DeletesAllKeys(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	// Manually set 3 keys matching the "tournaments:list:*" pattern.
	require.NoError(t, mr.Set("tournaments:list:status:active", "data1"))
	require.NoError(t, mr.Set("tournaments:list:status:pending", "data2"))
	require.NoError(t, mr.Set("tournaments:list:all", "data3"))

	// Verify the keys exist before invalidation.
	assert.True(t, mr.Exists("tournaments:list:status:active"))
	assert.True(t, mr.Exists("tournaments:list:status:pending"))
	assert.True(t, mr.Exists("tournaments:list:all"))

	err := tc.InvalidateList(ctx)
	require.NoError(t, err)

	// All 3 keys should be deleted.
	assert.False(t, mr.Exists("tournaments:list:status:active"))
	assert.False(t, mr.Exists("tournaments:list:status:pending"))
	assert.False(t, mr.Exists("tournaments:list:all"))
}
