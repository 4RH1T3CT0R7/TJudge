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
	score1, score2, winner := 10, 5, 1
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
		CreatedAt:    time.Now().Truncate(time.Second),
	}
}

func newTestMatchResult() *domain.MatchResult {
	return &domain.MatchResult{
		MatchID:  uuid.New(),
		Score1:   10,
		Score2:   5,
		Winner:   1,
		Duration: 2 * time.Second,
	}
}

func TestMatchCache_SetGet(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()
	result := newTestMatchResult()

	require.NoError(t, mc.Set(ctx, result.MatchID, result))

	got, err := mc.Get(ctx, result.MatchID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, result.MatchID, got.MatchID)
	assert.Equal(t, result.Score1, got.Score1)
	assert.Equal(t, result.Score2, got.Score2)
	assert.Equal(t, result.Winner, got.Winner)
	assert.Equal(t, result.Duration, got.Duration)
}

func TestMatchCache_Get_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)

	got, err := mc.Get(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMatchCache_SetGetMatch(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()
	match := newTestMatch(domain.MatchCompleted)

	require.NoError(t, mc.SetMatch(ctx, match))

	got, err := mc.GetMatch(ctx, match.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, match.ID, got.ID)
	assert.Equal(t, match.TournamentID, got.TournamentID)
	assert.Equal(t, match.GameType, got.GameType)
	assert.Equal(t, match.Status, got.Status)
	assert.Equal(t, match.Priority, got.Priority)
	assert.Equal(t, *match.Score1, *got.Score1)
	assert.Equal(t, *match.Winner, *got.Winner)
}

func TestMatchCache_GetMatch_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)

	got, err := mc.GetMatch(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

// разные статусы должны сериализоваться и читаться как есть
func TestMatchCache_SetGetStatus(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()

	for _, st := range []domain.MatchStatus{domain.MatchPending, domain.MatchRunning, domain.MatchFailed} {
		match := newTestMatch(st)
		require.NoError(t, mc.SetMatch(ctx, match))

		got, err := mc.GetMatch(ctx, match.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, st, got.Status)
	}
}

func TestMatchCache_Delete(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	mc := NewMatchCache(c)
	ctx := context.Background()
	result := newTestMatchResult()

	require.NoError(t, mc.Set(ctx, result.MatchID, result))
	require.NoError(t, mc.Delete(ctx, result.MatchID))

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

	require.NoError(t, mc.Set(ctx, result.MatchID, result))

	exists, err := mc.Exists(ctx, result.MatchID)
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, mc.Delete(ctx, result.MatchID))

	exists, err = mc.Exists(ctx, result.MatchID)
	require.NoError(t, err)
	assert.False(t, exists)
}
