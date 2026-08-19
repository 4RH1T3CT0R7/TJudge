package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ручные фейки коллабораторов ---

type fakeTournamentCache struct {
	setIDs        []uuid.UUID
	invalidateIDs []uuid.UUID
	err           error
}

func (f *fakeTournamentCache) Set(_ context.Context, t *models.Tournament) error {
	f.setIDs = append(f.setIDs, t.ID)
	return f.err
}

func (f *fakeTournamentCache) Invalidate(_ context.Context, id uuid.UUID) error {
	f.invalidateIDs = append(f.invalidateIDs, id)
	return f.err
}

type fakeLeaderboard struct {
	updateRating   int
	batchUpdates   int
	clearIDs       []uuid.UUID
	invalidateFull []uuid.UUID
	err            error
}

func (f *fakeLeaderboard) UpdateRating(_ context.Context, _, _ uuid.UUID, _ int) error {
	f.updateRating++
	return f.err
}

func (f *fakeLeaderboard) UpdateRatingsBatch(_ context.Context, updates []cache.RatingUpdate) error {
	f.batchUpdates += len(updates)
	return f.err
}

func (f *fakeLeaderboard) Clear(_ context.Context, id uuid.UUID) error {
	f.clearIDs = append(f.clearIDs, id)
	return f.err
}

func (f *fakeLeaderboard) InvalidateFullLeaderboard(_ context.Context, id uuid.UUID) error {
	f.invalidateFull = append(f.invalidateFull, id)
	return f.err
}

type broadcastCall struct {
	tournamentID uuid.UUID
	messageType  string
	payload      any
}

type fakeBroadcaster struct {
	calls []broadcastCall
}

func (f *fakeBroadcaster) Broadcast(tid uuid.UUID, mt string, p any) {
	f.calls = append(f.calls, broadcastCall{tid, mt, p})
}

// fakeRedisPub ловит то что уходит в редис (payload envelope)
type fakeRedisPub struct {
	channel  string
	payloads [][]byte
}

func (f *fakeRedisPub) Publish(_ context.Context, channel string, message any) error {
	f.channel = channel
	f.payloads = append(f.payloads, []byte(message.([]byte)))
	return nil
}

func testLog(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	return log
}

// --- топология апи: кэш турниров + лидерборд + broadcaster, без редиса ---

func TestSyncNotifier_ApiTopology(t *testing.T) {
	ctx := context.Background()
	tc := &fakeTournamentCache{}
	lb := &fakeLeaderboard{}
	br := &fakeBroadcaster{}
	n := &SyncNotifier{TournamentCache: tc, Leaderboard: lb, Broadcaster: br, Log: testLog(t)}

	tid := uuid.New()

	n.TournamentCreated(ctx, TournamentCreated{Version: 1, Tournament: &models.Tournament{ID: tid}})
	assert.Equal(t, []uuid.UUID{tid}, tc.setIDs)

	n.TournamentStarted(ctx, TournamentStarted{Version: 1, TournamentID: tid, Status: models.TournamentActive})
	n.TournamentCompleted(ctx, TournamentCompleted{Version: 1, TournamentID: tid})
	// оба инвалидируют кэш и шлют tournament_update
	assert.Len(t, tc.invalidateIDs, 2)
	require.Len(t, br.calls, 2)
	assert.Equal(t, "tournament_update", br.calls[0].messageType)
	assert.Equal(t, "tournament_update", br.calls[1].messageType)

	n.TournamentDeleted(ctx, TournamentDeleted{Version: 1, TournamentID: tid})
	assert.Contains(t, lb.clearIDs, tid)

	n.ParticipantJoined(ctx, ParticipantJoined{Version: 1, TournamentID: tid, ProgramID: uuid.New(), InitialRating: 1500})
	assert.Equal(t, 1, lb.updateRating)
	assert.Contains(t, lb.invalidateFull, tid)

	n.MatchesCreated(ctx, MatchesCreated{Version: 1, TournamentID: tid, ProgramID: uuid.New(), MatchCount: 4})
	last := br.calls[len(br.calls)-1]
	assert.Equal(t, "matches_created", last.messageType)
	payload := last.payload.(map[string]any)
	assert.Equal(t, 4, payload["matches_count"])

	n.GameRoundReset(ctx, GameRoundReset{Version: 1, TournamentID: tid, GameID: uuid.New()})
	// сброс раунда чистит лидерборд (второй Clear)
	assert.Len(t, lb.clearIDs, 2)
}

// --- топология воркера: лидерборд + редис, без кэша турниров и вебсокета ---

func TestSyncNotifier_WorkerTopology(t *testing.T) {
	ctx := context.Background()
	lb := &fakeLeaderboard{}
	pub := &fakeRedisPub{}
	n := &SyncNotifier{Leaderboard: lb, Redis: NewRedisEventPublisher(pub, testLog(t)), Log: testLog(t)}

	tid := uuid.New()
	e := MatchResultProcessed{Version: 1, TournamentID: tid, MatchID: uuid.New(), NewRating1: 1516, NewRating2: 1484, Winner: 1}
	n.MatchResultProcessed(ctx, e)

	// два рейтинга пачкой + инвалидация полного лидерборда
	assert.Equal(t, 2, lb.batchUpdates)
	assert.Contains(t, lb.invalidateFull, tid)

	// ушло в редис с правильным конвертом
	require.Len(t, pub.payloads, 1)
	assert.Equal(t, defaultChannel, pub.channel)
	var env envelope
	require.NoError(t, json.Unmarshal(pub.payloads[0], &env))
	assert.Equal(t, "MatchResultProcessed", env.Type)

	// ProgramCompiled - только редис, лидерборд не трогаем
	n.ProgramCompiled(ctx, ProgramCompiled{Version: 1, TournamentID: tid, ProgramID: uuid.New(), Status: "ready"})
	require.Len(t, pub.payloads, 2)
	require.NoError(t, json.Unmarshal(pub.payloads[1], &env))
	assert.Equal(t, "ProgramCompiled", env.Type)
}

// --- топология моста из редиса: только broadcaster ---

func TestSyncNotifier_WsTopology(t *testing.T) {
	ctx := context.Background()
	br := &fakeBroadcaster{}
	n := &SyncNotifier{Broadcaster: br, Log: testLog(t)}

	tid := uuid.New()
	n.MatchResultProcessed(ctx, MatchResultProcessed{Version: 1, TournamentID: tid, MatchID: uuid.New(), NewRating1: 1500, NewRating2: 1500, Winner: 0})
	n.ProgramCompiled(ctx, ProgramCompiled{Version: 1, TournamentID: tid, ProgramID: uuid.New(), TeamID: uuid.New(), Status: "failed"})

	require.Len(t, br.calls, 2)
	assert.Equal(t, "match_result", br.calls[0].messageType)
	assert.Equal(t, "program_update", br.calls[1].messageType)
	// ключи payload'а match_result - замороженный контракт фронта
	mp := br.calls[0].payload.(map[string]any)
	for _, k := range []string{"match_id", "program1_id", "program2_id", "new_rating1", "new_rating2", "winner"} {
		assert.Contains(t, mp, k)
	}
}

// nil-коллабораторы просто пропускаются, ничего не паникует
func TestSyncNotifier_NilCollaborators(t *testing.T) {
	ctx := context.Background()
	n := &SyncNotifier{Log: testLog(t)}
	tid := uuid.New()

	assert.NotPanics(t, func() {
		n.TournamentCreated(ctx, TournamentCreated{Version: 1, Tournament: &models.Tournament{ID: tid}})
		n.TournamentStarted(ctx, TournamentStarted{Version: 1, TournamentID: tid})
		n.TournamentDeleted(ctx, TournamentDeleted{Version: 1, TournamentID: tid})
		n.ParticipantJoined(ctx, ParticipantJoined{Version: 1, TournamentID: tid})
		n.MatchesCreated(ctx, MatchesCreated{Version: 1, TournamentID: tid})
		n.GameRoundReset(ctx, GameRoundReset{Version: 1, TournamentID: tid})
		n.MatchResultProcessed(ctx, MatchResultProcessed{Version: 1, TournamentID: tid})
		n.ProgramCompiled(ctx, ProgramCompiled{Version: 1, TournamentID: tid})
	})
}

// ошибка кэша логируется и глотается, наружу не летит и не паникует
func TestSyncNotifier_CacheErrorSwallowed(t *testing.T) {
	ctx := context.Background()
	tc := &fakeTournamentCache{err: assert.AnError}
	lb := &fakeLeaderboard{err: assert.AnError}
	n := &SyncNotifier{TournamentCache: tc, Leaderboard: lb, Log: testLog(t)}

	assert.NotPanics(t, func() {
		n.TournamentDeleted(ctx, TournamentDeleted{Version: 1, TournamentID: uuid.New()})
		n.ParticipantJoined(ctx, ParticipantJoined{Version: 1, TournamentID: uuid.New()})
	})
}

func TestNoopNotifier(t *testing.T) {
	ctx := context.Background()
	var n Notifier = NoopNotifier{}
	assert.NotPanics(t, func() {
		n.TournamentCreated(ctx, TournamentCreated{})
		n.TournamentStarted(ctx, TournamentStarted{})
		n.TournamentCompleted(ctx, TournamentCompleted{})
		n.TournamentDeleted(ctx, TournamentDeleted{})
		n.ParticipantJoined(ctx, ParticipantJoined{})
		n.MatchesCreated(ctx, MatchesCreated{})
		n.GameRoundReset(ctx, GameRoundReset{})
		n.MatchResultProcessed(ctx, MatchResultProcessed{})
		n.ProgramCompiled(ctx, ProgramCompiled{})
	})
}
