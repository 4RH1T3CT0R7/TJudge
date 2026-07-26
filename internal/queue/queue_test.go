package queue

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// одни метрики на весь пакет, иначе prometheus ругается на повторную регистрацию
var (
	sharedMetrics     *metrics.Metrics
	sharedMetricsOnce sync.Once
)

func testMetrics() *metrics.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetrics = metrics.New()
	})
	return sharedMetrics
}

func testLogger() *logger.Logger {
	log, _ := logger.New("error", "json")
	return log
}

func testMatch(priority domain.MatchPriority) *domain.Match {
	return &domain.Match{
		ID:       uuid.New(),
		Priority: priority,
		Status:   domain.MatchPending,
		GameType: "tictactoe",
	}
}

// поднимаем реальный Cache поверх miniredis, моки тут не нужны
func setupTestQueueManager(t *testing.T) *QueueManager {
	t.Helper()

	mr := miniredis.RunT(t)
	log := testLogger()
	m := testMetrics()

	host, portStr, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	cfg := &config.RedisConfig{Host: host, Port: port}

	realCache, err := cache.New(cfg, log, m)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = realCache.Close()
	})

	return NewQueueManager(realCache, log, m)
}

func TestQueueManager_GetQueueKey(t *testing.T) {
	qm := NewQueueManager(nil, testLogger(), testMetrics())

	tests := []struct {
		priority domain.MatchPriority
		want     string
	}{
		{domain.PriorityHigh, "queue:high"},
		{domain.PriorityMedium, "queue:medium"},
		{domain.PriorityLow, "queue:low"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, qm.getQueueKey(tc.priority))
	}
}

func TestQueueManager_EnqueueDequeue_PriorityOrdering(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// кладём в обратном порядке: low, medium, high
	lowMatch := testMatch(domain.PriorityLow)
	medMatch := testMatch(domain.PriorityMedium)
	highMatch := testMatch(domain.PriorityHigh)

	require.NoError(t, qm.Enqueue(ctx, lowMatch))
	require.NoError(t, qm.Enqueue(ctx, medMatch))
	require.NoError(t, qm.Enqueue(ctx, highMatch))

	// достаём: high -> medium -> low
	first, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, highMatch.ID, first.ID)

	second, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, medMatch.ID, second.ID)

	third, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, third)
	assert.Equal(t, lowMatch.ID, third.ID)

	// очередь пустая
	empty, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	assert.Nil(t, empty)
}

func TestQueueManager_FIFO_WithinSamePriority(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	first := testMatch(domain.PriorityMedium)
	second := testMatch(domain.PriorityMedium)
	third := testMatch(domain.PriorityMedium)

	require.NoError(t, qm.Enqueue(ctx, first))
	require.NoError(t, qm.Enqueue(ctx, second))
	require.NoError(t, qm.Enqueue(ctx, third))

	// внутри одного приоритета порядок FIFO
	got1, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, got1)
	assert.Equal(t, first.ID, got1.ID)

	got2, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, second.ID, got2.ID)

	got3, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, got3)
	assert.Equal(t, third.ID, got3.ID)
}

func TestQueueManager_GetQueueSize(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// пока пусто
	for _, p := range []domain.MatchPriority{domain.PriorityHigh, domain.PriorityMedium, domain.PriorityLow} {
		size, err := qm.GetQueueSize(ctx, p)
		require.NoError(t, err)
		assert.Equal(t, int64(0), size)
	}

	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityMedium)))

	high, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(2), high)

	med, err := qm.GetQueueSize(ctx, domain.PriorityMedium)
	require.NoError(t, err)
	assert.Equal(t, int64(1), med)

	low, err := qm.GetQueueSize(ctx, domain.PriorityLow)
	require.NoError(t, err)
	assert.Equal(t, int64(0), low)
}

func TestQueueManager_GetTotalQueueSize(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityMedium)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityLow)))

	total, err := qm.GetTotalQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
}

func TestQueueManager_GetStats(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	for range 3 {
		require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	}
	for range 2 {
		require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityMedium)))
	}
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityLow)))

	stats, err := qm.GetStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, int64(3), stats.High)
	assert.Equal(t, int64(2), stats.Medium)
	assert.Equal(t, int64(1), stats.Low)
	assert.Equal(t, int64(6), stats.Total)
}

func TestQueueManager_Clear(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityMedium)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityLow)))

	require.NoError(t, qm.Clear(ctx))

	total, err := qm.GetTotalQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestQueueManager_Health(t *testing.T) {
	qm := setupTestQueueManager(t)
	assert.NoError(t, qm.Health(context.Background()))
}

func TestQueueManager_EnqueueBatch(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	matches := []*domain.Match{
		testMatch(domain.PriorityHigh),
		testMatch(domain.PriorityHigh),
		testMatch(domain.PriorityMedium),
		testMatch(domain.PriorityLow),
	}

	require.NoError(t, qm.EnqueueBatch(ctx, matches))

	high, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(2), high)

	med, err := qm.GetQueueSize(ctx, domain.PriorityMedium)
	require.NoError(t, err)
	assert.Equal(t, int64(1), med)

	low, err := qm.GetQueueSize(ctx, domain.PriorityLow)
	require.NoError(t, err)
	assert.Equal(t, int64(1), low)
}

func TestQueueManager_EnqueueBatch_DedupSkipsDuplicates(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	match1 := testMatch(domain.PriorityHigh)
	match2 := testMatch(domain.PriorityHigh)

	// match1 сначала ставим по одиночке
	require.NoError(t, qm.Enqueue(ctx, match1))

	// батчем кладём и дубль match1, и новый match2
	require.NoError(t, qm.EnqueueBatch(ctx, []*domain.Match{match1, match2}))

	// в очереди должно быть 2 (match1 + match2), а не 3
	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(2), size)
}

func TestQueueManager_EnqueueBatch_Empty(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	require.NoError(t, qm.EnqueueBatch(ctx, nil))
	require.NoError(t, qm.EnqueueBatch(ctx, []*domain.Match{}))

	total, err := qm.GetTotalQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestQueueManager_WeightedFairQueueing_NoStarvation(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// наполняем все три очереди по 20 матчей
	for range 20 {
		require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
		require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityMedium)))
		require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityLow)))
	}

	// достаём все 60 и считаем по приоритетам, low не должна остаться голодной
	counts := map[domain.MatchPriority]int{}
	for i := range 60 {
		match, err := qm.Dequeue(ctx)
		require.NoError(t, err)
		require.NotNil(t, match, "ожидали матч на итерации %d", i)
		counts[match.Priority]++
	}

	assert.Equal(t, 20, counts[domain.PriorityHigh])
	assert.Equal(t, 20, counts[domain.PriorityMedium])
	assert.Equal(t, 20, counts[domain.PriorityLow])
}

func TestQueueManager_WeightedQueueKeys_Cycle(t *testing.T) {
	qm := NewQueueManager(nil, testLogger(), testMetrics())

	high := qm.getQueueKey(domain.PriorityHigh)
	medium := qm.getQueueKey(domain.PriorityMedium)
	low := qm.getQueueKey(domain.PriorityLow)

	// 9-шаговый цикл ротации 5:3:1
	expected := [][]string{
		{high, medium, low}, // 0 - HIGH first
		{high, medium, low}, // 1 - HIGH first
		{high, medium, low}, // 2 - HIGH first
		{high, medium, low}, // 3 - HIGH first
		{high, medium, low}, // 4 - HIGH first
		{medium, high, low}, // 5 - MEDIUM first
		{medium, high, low}, // 6 - MEDIUM first
		{medium, high, low}, // 7 - MEDIUM first
		{low, high, medium}, // 8 - LOW first
	}

	qm.dequeueCount = 0

	for i, exp := range expected {
		got := qm.weightedQueueKeys()
		assert.Equal(t, exp, got, "позиция цикла %d", i)
	}

	// на позиции 9 цикл повторяется (9 mod 9 = 0)
	got := qm.weightedQueueKeys()
	assert.Equal(t, []string{high, medium, low}, got, "цикл должен повториться на позиции 9")
}

func TestQueueManager_Dequeue_MalformedJSON_DeadLetter(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// пихаем битый json прямо в high очередь
	queueKey := qm.getQueueKey(domain.PriorityHigh)
	require.NoError(t, qm.cache.LPush(ctx, queueKey, "not-valid-json{{{"))

	// dequeue должен вернуть ошибку и переложить запись в dead-letter
	match, err := qm.Dequeue(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal match")
	assert.Nil(t, match)

	dlSize, err := qm.cache.LLen(ctx, "queue:dead_letter")
	require.NoError(t, err)
	assert.Equal(t, int64(1), dlSize)
}

func TestQueueManager_Enqueue_Dedup_SkipsDuplicate(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	match := testMatch(domain.PriorityHigh)

	require.NoError(t, qm.Enqueue(ctx, match))

	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)

	// повторный enqueue того же матча пропускается по дедупу (setnx)
	require.NoError(t, qm.Enqueue(ctx, match))

	size, err = qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size) // всё ещё 1, не 2
}

func TestQueueManager_Dequeue_RemovesFromDedupSet(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	match := testMatch(domain.PriorityHigh)
	require.NoError(t, qm.Enqueue(ctx, match))

	dequeued, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, dequeued)
	assert.Equal(t, match.ID, dequeued.ID)

	// после dequeue дедуп очищен, тот же матч можно поставить заново
	require.NoError(t, qm.Enqueue(ctx, match))

	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)
}

func TestQueueManager_PurgeInvalidMatches_SomeInvalid(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	validMatch := testMatch(domain.PriorityMedium)
	invalidMatch := testMatch(domain.PriorityMedium)
	require.NoError(t, qm.Enqueue(ctx, validMatch))
	require.NoError(t, qm.Enqueue(ctx, invalidMatch))

	// валидатор пропускает только validMatch
	purged, err := qm.PurgeInvalidMatches(ctx, func(matchID string) bool {
		return matchID == validMatch.ID.String()
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged)

	size, err := qm.GetQueueSize(ctx, domain.PriorityMedium)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)
}

func TestQueueManager_PurgeInvalidMatches_MalformedJSON(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// битый json + один валидный матч
	queueKey := qm.getQueueKey(domain.PriorityHigh)
	require.NoError(t, qm.cache.LPush(ctx, queueKey, "invalid-json"))

	validMatch := testMatch(domain.PriorityHigh)
	require.NoError(t, qm.Enqueue(ctx, validMatch))

	// все настоящие матчи валидны, вычистится только битый json
	purged, err := qm.PurgeInvalidMatches(ctx, func(matchID string) bool {
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged)

	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size) // валидный матч остался
}

func TestQueueManager_ConcurrentEnqueueDequeue(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	const enqueueGoroutines = 5
	const matchesPerGoroutine = 20
	totalMatches := enqueueGoroutines * matchesPerGoroutine

	enqueuedIDs := make(chan uuid.UUID, totalMatches)

	var enqueueWg sync.WaitGroup
	enqueueWg.Add(enqueueGoroutines)

	for range enqueueGoroutines {
		go func() {
			defer enqueueWg.Done()
			for range matchesPerGoroutine {
				match := testMatch(domain.PriorityMedium)
				assert.NoError(t, qm.Enqueue(ctx, match))
				enqueuedIDs <- match.ID
			}
		}()
	}

	enqueueWg.Wait()
	close(enqueuedIDs)

	expectedIDs := make(map[uuid.UUID]struct{}, totalMatches)
	for id := range enqueuedIDs {
		expectedIDs[id] = struct{}{}
	}
	require.Equal(t, totalMatches, len(expectedIDs), "все id должны быть уникальны")

	total, err := qm.GetTotalQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(totalMatches), total)

	// параллельно вычёрпываем всё обратно
	const dequeueGoroutines = 3
	dequeuedIDs := make(chan uuid.UUID, totalMatches)

	var dequeueWg sync.WaitGroup
	dequeueWg.Add(dequeueGoroutines)

	for range dequeueGoroutines {
		go func() {
			defer dequeueWg.Done()
			for {
				match, err := qm.Dequeue(ctx)
				if err != nil {
					return
				}
				if match == nil {
					return // очередь пуста (brpop по таймауту)
				}
				dequeuedIDs <- match.ID
			}
		}()
	}

	dequeueWg.Wait()
	close(dequeuedIDs)

	gotIDs := make(map[uuid.UUID]struct{})
	for id := range dequeuedIDs {
		gotIDs[id] = struct{}{}
	}

	// каждый вычерпнутый матч должен был быть добавлен
	for id := range gotIDs {
		_, found := expectedIDs[id]
		assert.True(t, found, "вычерпнутый id %s не добавлялся", id)
	}
	assert.Equal(t, len(expectedIDs), len(gotIDs),
		"вычерпнули (%d) должно совпадать с добавленными (%d)", len(gotIDs), len(expectedIDs))
}
