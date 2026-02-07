package cache

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMatch(status domain.MatchStatus) *domain.Match {
	now := time.Now().Truncate(time.Second)
	score1, score2 := 10, 5
	winner := 1
	return &domain.Match{
		ID:           uuid.New(),
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		GameType:     "prisoners_dilemma",
		Status:       status,
		Priority:     domain.PriorityMedium,
		RoundNumber:  1,
		Score1:       &score1,
		Score2:       &score2,
		Winner:       &winner,
		CreatedAt:    now,
	}
}

func newTestMatchResult() *domain.MatchResult {
	return &domain.MatchResult{
		MatchID:      uuid.New(),
		Score1:       10,
		Score2:       5,
		Winner:       1,
		ErrorCode:    0,
		ErrorMessage: "",
		Duration:     2 * time.Second,
	}
}

func TestMatchCache_SetGet(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	result := newTestMatchResult()

	err := mc.Set(ctx, result.MatchID, result)
	require.NoError(t, err)

	got, err := mc.Get(ctx, result.MatchID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, result.MatchID, got.MatchID)
	assert.Equal(t, result.Score1, got.Score1)
	assert.Equal(t, result.Score2, got.Score2)
	assert.Equal(t, result.Winner, got.Winner)
	assert.Equal(t, result.ErrorCode, got.ErrorCode)
	assert.Equal(t, result.ErrorMessage, got.ErrorMessage)
	assert.Equal(t, result.Duration, got.Duration)
}

func TestMatchCache_Get_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	got, err := mc.Get(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMatchCache_SetGetMatch(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	match := newTestMatch(domain.MatchCompleted)

	err := mc.SetMatch(ctx, match)
	require.NoError(t, err)

	got, err := mc.GetMatch(ctx, match.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, match.ID, got.ID)
	assert.Equal(t, match.TournamentID, got.TournamentID)
	assert.Equal(t, match.Program1ID, got.Program1ID)
	assert.Equal(t, match.Program2ID, got.Program2ID)
	assert.Equal(t, match.GameType, got.GameType)
	assert.Equal(t, match.Status, got.Status)
	assert.Equal(t, match.Priority, got.Priority)
	assert.Equal(t, match.RoundNumber, got.RoundNumber)
	assert.Equal(t, *match.Score1, *got.Score1)
	assert.Equal(t, *match.Score2, *got.Score2)
	assert.Equal(t, *match.Winner, *got.Winner)
}

func TestMatchCache_GetMatch_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	got, err := mc.GetMatch(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMatchCache_SetGetStatus(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	t.Run("pending match", func(t *testing.T) {
		match := newTestMatch(domain.MatchPending)

		err := mc.SetMatch(ctx, match)
		require.NoError(t, err)

		got, err := mc.GetMatch(ctx, match.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.MatchPending, got.Status)
	})

	t.Run("running match", func(t *testing.T) {
		match := newTestMatch(domain.MatchRunning)

		err := mc.SetMatch(ctx, match)
		require.NoError(t, err)

		got, err := mc.GetMatch(ctx, match.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.MatchRunning, got.Status)
	})

	t.Run("completed match", func(t *testing.T) {
		match := newTestMatch(domain.MatchCompleted)

		err := mc.SetMatch(ctx, match)
		require.NoError(t, err)

		got, err := mc.GetMatch(ctx, match.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.MatchCompleted, got.Status)
	})

	t.Run("failed match", func(t *testing.T) {
		match := newTestMatch(domain.MatchFailed)

		err := mc.SetMatch(ctx, match)
		require.NoError(t, err)

		got, err := mc.GetMatch(ctx, match.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.MatchFailed, got.Status)
	})
}

func TestMatchCache_GetStatus_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	got, err := mc.GetMatch(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMatchCache_Delete(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	result := newTestMatchResult()

	err := mc.Set(ctx, result.MatchID, result)
	require.NoError(t, err)

	err = mc.Delete(ctx, result.MatchID)
	require.NoError(t, err)

	got, err := mc.Get(ctx, result.MatchID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMatchCache_Exists(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	result := newTestMatchResult()

	t.Run("exists after set", func(t *testing.T) {
		err := mc.Set(ctx, result.MatchID, result)
		require.NoError(t, err)

		exists, err := mc.Exists(ctx, result.MatchID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("does not exist after delete", func(t *testing.T) {
		err := mc.Delete(ctx, result.MatchID)
		require.NoError(t, err)

		exists, err := mc.Exists(ctx, result.MatchID)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}
