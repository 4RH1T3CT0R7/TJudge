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
	creatorID := uuid.New()
	maxPart := 16
	now := time.Now().Truncate(time.Second)
	return &domain.Tournament{
		ID:              uuid.New(),
		Name:            "Test Tournament",
		Code:            "TEST01",
		Description:     "A test tournament",
		GameType:        "prisoners_dilemma",
		Status:          domain.TournamentPending,
		MaxParticipants: &maxPart,
		MaxTeamSize:     3,
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

	require.NoError(t, tc.Set(ctx, tournament))

	got, err := tc.Get(ctx, tournament.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tournament.ID, got.ID)
	assert.Equal(t, tournament.Name, got.Name)
	assert.Equal(t, tournament.Code, got.Code)
	assert.Equal(t, tournament.Status, got.Status)
	assert.Equal(t, *tournament.MaxParticipants, *got.MaxParticipants)
	assert.Equal(t, *tournament.CreatorID, *got.CreatorID)
	assert.Equal(t, tournament.Version, got.Version)
}

func TestTournamentCache_Get_NotFound(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)

	got, err := tc.Get(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTournamentCache_Exists(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournament := newTestTournament()

	require.NoError(t, tc.Set(ctx, tournament))

	exists, err := tc.Exists(ctx, tournament.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, tc.Invalidate(ctx, tournament.ID))

	exists, err = tc.Exists(ctx, tournament.ID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTournamentCache_ParticipantsCount(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// нет ключа -> сентинел -1
	count, err := tc.GetParticipantsCount(ctx, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, -1, count)

	require.NoError(t, tc.SetParticipantsCount(ctx, tournamentID, 42))
	count, err = tc.GetParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)
	assert.Equal(t, 42, count)

	require.NoError(t, tc.IncrementParticipantsCount(ctx, tournamentID))
	count, err = tc.GetParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)
	assert.Equal(t, 43, count)

	// инкремент без ключа создаёт его со значением 1
	newID := uuid.New()
	require.NoError(t, tc.IncrementParticipantsCount(ctx, newID))
	count, err = tc.GetParticipantsCount(ctx, newID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestTournamentCache_IncrementParticipantsCount_NoExpiryReset(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// первый инкремент: создаёт ключ =1 и выставляет ttl
	require.NoError(t, tc.IncrementParticipantsCount(ctx, tournamentID))

	key := fmt.Sprintf("tournament:%s:participants_count", tournamentID.String())
	ttlAfterFirst := mr.TTL(key)
	assert.Greater(t, ttlAfterFirst, time.Duration(0), "после первого инкремента ttl должен стоять")

	// проматываем время, ttl уменьшается
	mr.FastForward(10 * time.Second)
	ttlBefore := mr.TTL(key)

	// второй инкремент: значение 2, ttl НЕ сбрасывается (он ставится только на val==1)
	require.NoError(t, tc.IncrementParticipantsCount(ctx, tournamentID))

	count, err := tc.GetParticipantsCount(ctx, tournamentID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	ttlAfter := mr.TTL(key)
	assert.LessOrEqual(t, ttlAfter, ttlBefore, "ttl не должен сбрасываться на повторных инкрементах")
}

func TestTournamentCache_MatchStatistics(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()
	tournamentID := uuid.New()

	// промах
	got, err := tc.GetMatchStatistics(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)

	stats := map[string]int{"total": 100, "completed": 80, "pending": 15, "failed": 5}
	require.NoError(t, tc.SetMatchStatistics(ctx, tournamentID, stats))

	got, err = tc.GetMatchStatistics(ctx, tournamentID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 100, got["total"])
	assert.Equal(t, 80, got["completed"])
}

func TestTournamentCache_List(t *testing.T) {
	c := setupTestCache(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	// промах
	got, err := tc.GetList(ctx, "nonexistent:filter")
	require.NoError(t, err)
	assert.Nil(t, got)

	tournaments := []*domain.Tournament{newTestTournament(), newTestTournament()}
	tournaments[0].Name = "Alpha"
	tournaments[1].Name = "Beta"

	filter := "status:active"
	require.NoError(t, tc.SetList(ctx, filter, tournaments))

	got, err = tc.GetList(ctx, filter)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Alpha", got[0].Name)
	assert.Equal(t, "Beta", got[1].Name)
	assert.Equal(t, tournaments[0].ID, got[0].ID)
}

func TestTournamentCache_InvalidateList(t *testing.T) {
	c, mr := setupTestCacheWithMR(t)
	defer c.Close()

	tc := NewTournamentCache(c)
	ctx := context.Background()

	// вручную кладём ключи под паттерн "tournaments:list:*"
	require.NoError(t, mr.Set("tournaments:list:status:active", "data1"))
	require.NoError(t, mr.Set("tournaments:list:status:pending", "data2"))
	require.NoError(t, mr.Set("tournaments:list:all", "data3"))

	require.NoError(t, tc.InvalidateList(ctx))

	assert.False(t, mr.Exists("tournaments:list:status:active"))
	assert.False(t, mr.Exists("tournaments:list:status:pending"))
	assert.False(t, mr.Exists("tournaments:list:all"))
}
