package queue

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	sharedMetrics     *metrics.Metrics
	sharedMetricsOnce sync.Once
)

// MockCache mocks the cache.Cache for testing
type MockCache struct {
	mock.Mock
}

func (m *MockCache) LPush(ctx context.Context, key string, values ...any) error {
	args := m.Called(ctx, key, values)
	return args.Error(0)
}

func (m *MockCache) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	args := m.Called(ctx, timeout, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCache) LLen(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCache) Del(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

// CacheInterface is the interface that QueueManager uses
type CacheInterface interface {
	LPush(ctx context.Context, key string, values ...any) error
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error)
	LLen(ctx context.Context, key string) (int64, error)
	Del(ctx context.Context, keys ...string) error
}

// testLogger creates a test logger
func testLogger() *logger.Logger {
	log, _ := logger.New("debug", "json")
	return log
}

// testMetrics creates test metrics (singleton to avoid duplicate registration)
func testMetrics() *metrics.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetrics = metrics.New()
	})
	return sharedMetrics
}

// testMatch creates a test match
func testMatch(priority domain.MatchPriority) *domain.Match {
	return &domain.Match{
		ID:       uuid.New(),
		Priority: priority,
		Status:   domain.MatchPending,
		GameType: "tictactoe",
	}
}

func TestQueueManager_GetQueueKey(t *testing.T) {
	cache := new(MockCache)
	qm := NewQueueManager(nil, testLogger(), testMetrics())

	tests := []struct {
		priority domain.MatchPriority
		expected string
	}{
		{domain.PriorityHigh, "queue:high"},
		{domain.PriorityMedium, "queue:medium"},
		{domain.PriorityLow, "queue:low"},
	}

	for _, tc := range tests {
		t.Run(string(tc.priority), func(t *testing.T) {
			key := qm.getQueueKey(tc.priority)
			assert.Equal(t, tc.expected, key)
		})
	}

	_ = cache // Use cache to avoid unused warning
}

func TestQueueManager_GetQueueSize(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// All queues start empty
	for _, priority := range []domain.MatchPriority{domain.PriorityHigh, domain.PriorityMedium, domain.PriorityLow} {
		size, err := qm.GetQueueSize(ctx, priority)
		require.NoError(t, err)
		assert.Equal(t, int64(0), size)
	}

	// Enqueue matches to different priorities
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityMedium)))

	highSize, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(2), highSize)

	medSize, err := qm.GetQueueSize(ctx, domain.PriorityMedium)
	require.NoError(t, err)
	assert.Equal(t, int64(1), medSize)

	lowSize, err := qm.GetQueueSize(ctx, domain.PriorityLow)
	require.NoError(t, err)
	assert.Equal(t, int64(0), lowSize)
}

func TestMatch_Serialization(t *testing.T) {
	match := testMatch(domain.PriorityHigh)

	// Serialize
	data, err := json.Marshal(match)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize
	var deserialized domain.Match
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	assert.Equal(t, match.ID, deserialized.ID)
	assert.Equal(t, match.Priority, deserialized.Priority)
	assert.Equal(t, match.Status, deserialized.Status)
	assert.Equal(t, match.GameType, deserialized.GameType)
}

func TestPriority_Order(t *testing.T) {
	// Test that priorities are ordered correctly
	priorities := []domain.MatchPriority{
		domain.PriorityHigh,
		domain.PriorityMedium,
		domain.PriorityLow,
	}

	assert.Equal(t, "high", string(priorities[0]))
	assert.Equal(t, "medium", string(priorities[1]))
	assert.Equal(t, "low", string(priorities[2]))
}

// InMemoryQueue implements a simple in-memory queue for testing
type InMemoryQueue struct {
	queues map[string][]string
}

func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{
		queues: make(map[string][]string),
	}
}

func (q *InMemoryQueue) LPush(ctx context.Context, key string, values ...any) error {
	if q.queues[key] == nil {
		q.queues[key] = make([]string, 0)
	}
	for _, v := range values {
		q.queues[key] = append([]string{v.(string)}, q.queues[key]...)
	}
	return nil
}

func (q *InMemoryQueue) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	for _, key := range keys {
		if queue, exists := q.queues[key]; exists && len(queue) > 0 {
			value := queue[len(queue)-1]
			q.queues[key] = queue[:len(queue)-1]
			return []string{key, value}, nil
		}
	}
	return nil, nil
}

func (q *InMemoryQueue) LLen(ctx context.Context, key string) (int64, error) {
	if queue, exists := q.queues[key]; exists {
		return int64(len(queue)), nil
	}
	return 0, nil
}

func (q *InMemoryQueue) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(q.queues, key)
	}
	return nil
}

func TestInMemoryQueue_Operations(t *testing.T) {
	q := NewInMemoryQueue()
	ctx := context.Background()

	t.Run("LPush and LLen", func(t *testing.T) {
		err := q.LPush(ctx, "test", "value1")
		require.NoError(t, err)

		len, err := q.LLen(ctx, "test")
		require.NoError(t, err)
		assert.Equal(t, int64(1), len)

		err = q.LPush(ctx, "test", "value2")
		require.NoError(t, err)

		len, err = q.LLen(ctx, "test")
		require.NoError(t, err)
		assert.Equal(t, int64(2), len)
	})

	t.Run("BRPop returns oldest first", func(t *testing.T) {
		q := NewInMemoryQueue()
		_ = q.LPush(ctx, "test", "first")
		_ = q.LPush(ctx, "test", "second")

		result, err := q.BRPop(ctx, time.Second, "test")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "first", result[1])
	})

	t.Run("BRPop empty queue returns nil", func(t *testing.T) {
		q := NewInMemoryQueue()

		result, err := q.BRPop(ctx, time.Second, "empty")
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Del removes queue", func(t *testing.T) {
		q := NewInMemoryQueue()
		_ = q.LPush(ctx, "test", "value")

		err := q.Del(ctx, "test")
		require.NoError(t, err)

		len, err := q.LLen(ctx, "test")
		require.NoError(t, err)
		assert.Equal(t, int64(0), len)
	})
}

func TestInMemoryQueue_MatchSerialization(t *testing.T) {
	q := NewInMemoryQueue()
	ctx := context.Background()

	match := testMatch(domain.PriorityHigh)

	// Serialize and push
	data, err := json.Marshal(match)
	require.NoError(t, err)

	err = q.LPush(ctx, "queue:high", string(data))
	require.NoError(t, err)

	// Pop and deserialize
	result, err := q.BRPop(ctx, time.Second, "queue:high")
	require.NoError(t, err)
	require.NotNil(t, result)

	var deserialized domain.Match
	err = json.Unmarshal([]byte(result[1]), &deserialized)
	require.NoError(t, err)

	assert.Equal(t, match.ID, deserialized.ID)
}

func TestInMemoryQueue_PriorityOrder(t *testing.T) {
	q := NewInMemoryQueue()
	ctx := context.Background()

	// Add matches with different priorities
	highMatch := testMatch(domain.PriorityHigh)
	medMatch := testMatch(domain.PriorityMedium)
	lowMatch := testMatch(domain.PriorityLow)

	// Push to respective queues
	highData, _ := json.Marshal(highMatch)
	medData, _ := json.Marshal(medMatch)
	lowData, _ := json.Marshal(lowMatch)

	_ = q.LPush(ctx, "queue:low", string(lowData))
	_ = q.LPush(ctx, "queue:medium", string(medData))
	_ = q.LPush(ctx, "queue:high", string(highData))

	// Dequeue in priority order (HIGH -> MEDIUM -> LOW)
	priorities := []string{"queue:high", "queue:medium", "queue:low"}

	for _, priority := range priorities {
		result, err := q.BRPop(ctx, time.Second, priority)
		require.NoError(t, err)

		if result != nil {
			var match domain.Match
			err = json.Unmarshal([]byte(result[1]), &match)
			require.NoError(t, err)

			switch priority {
			case "queue:high":
				assert.Equal(t, highMatch.ID, match.ID)
			case "queue:medium":
				assert.Equal(t, medMatch.ID, match.ID)
			case "queue:low":
				assert.Equal(t, lowMatch.ID, match.ID)
			}
		}
	}
}

// --- Integration tests using miniredis ---

func setupTestQueueManager(t *testing.T) *QueueManager {
	t.Helper()

	mr := miniredis.RunT(t)
	log, _ := logger.New("error", "json")
	m := testMetrics()

	// Parse miniredis address into host and port for config.RedisConfig.
	host, portStr, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	cfg := &config.RedisConfig{
		Host: host,
		Port: port,
	}

	realCache, err := cache.New(cfg, log, m)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = realCache.Close()
	})

	return NewQueueManager(realCache, log, m)
}

func TestQueueManager_EnqueueDequeue_PriorityOrdering(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// Enqueue matches with different priorities in arbitrary order:
	// low first, then medium, then high.
	lowMatch := testMatch(domain.PriorityLow)
	medMatch := testMatch(domain.PriorityMedium)
	highMatch := testMatch(domain.PriorityHigh)

	require.NoError(t, qm.Enqueue(ctx, lowMatch))
	require.NoError(t, qm.Enqueue(ctx, medMatch))
	require.NoError(t, qm.Enqueue(ctx, highMatch))

	// Dequeue should respect priority: high -> medium -> low.
	first, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, highMatch.ID, first.ID, "first dequeued match should be high priority")

	second, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, medMatch.ID, second.ID, "second dequeued match should be medium priority")

	third, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, third)
	assert.Equal(t, lowMatch.ID, third.ID, "third dequeued match should be low priority")

	// Queue should now be empty.
	empty, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	assert.Nil(t, empty, "queue should be empty after all matches dequeued")
}

func TestQueueManager_GetStats(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// Enqueue matches of different priorities.
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

func TestQueueManager_GetStats_Empty(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	stats, err := qm.GetStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, int64(0), stats.High)
	assert.Equal(t, int64(0), stats.Medium)
	assert.Equal(t, int64(0), stats.Low)
	assert.Equal(t, int64(0), stats.Total)
}

func TestQueueManager_GetQueueSize_Integration(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// Initially empty.
	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	// Add a match.
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))

	size, err = qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)
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

func TestQueueManager_Clear(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityHigh)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityMedium)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityLow)))

	err := qm.Clear(ctx)
	require.NoError(t, err)

	total, err := qm.GetTotalQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestQueueManager_Health(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	err := qm.Health(ctx)
	assert.NoError(t, err)
}

func TestQueueManager_Dequeue_EmptyQueue(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	match, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	assert.Nil(t, match)
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

	// Within the same priority, matches should be dequeued in FIFO order.
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

func TestQueueManager_EnqueueBatch(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	matches := []*domain.Match{
		testMatch(domain.PriorityHigh),
		testMatch(domain.PriorityHigh),
		testMatch(domain.PriorityMedium),
		testMatch(domain.PriorityLow),
	}

	err := qm.EnqueueBatch(ctx, matches)
	require.NoError(t, err)

	// Verify queue sizes
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

	// Enqueue match1 individually first
	require.NoError(t, qm.Enqueue(ctx, match1))

	// Now batch-enqueue both match1 (duplicate) and match2 (new)
	err := qm.EnqueueBatch(ctx, []*domain.Match{match1, match2})
	require.NoError(t, err)

	// Queue should have 2 (original match1 + new match2), not 3
	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(2), size)
}

func TestQueueManager_EnqueueBatch_AllDuplicates(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	match1 := testMatch(domain.PriorityHigh)
	match2 := testMatch(domain.PriorityMedium)

	// Enqueue both individually
	require.NoError(t, qm.Enqueue(ctx, match1))
	require.NoError(t, qm.Enqueue(ctx, match2))

	// Batch-enqueue тех же матчей: все должны быть пропущены
	err := qm.EnqueueBatch(ctx, []*domain.Match{match1, match2})
	require.NoError(t, err)

	// Queue sizes should remain the same
	highSize, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), highSize)

	medSize, err := qm.GetQueueSize(ctx, domain.PriorityMedium)
	require.NoError(t, err)
	assert.Equal(t, int64(1), medSize)
}

func TestQueueManager_EnqueueBatch_Empty(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	err := qm.EnqueueBatch(ctx, nil)
	require.NoError(t, err)

	err = qm.EnqueueBatch(ctx, []*domain.Match{})
	require.NoError(t, err)

	total, err := qm.GetTotalQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestQueueManager_WeightedFairQueueing_NoStarvation(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// Наполняем все три очереди по 20 матчей
	highMatches := make([]*domain.Match, 20)
	medMatches := make([]*domain.Match, 20)
	lowMatches := make([]*domain.Match, 20)

	for i := range 20 {
		highMatches[i] = testMatch(domain.PriorityHigh)
		medMatches[i] = testMatch(domain.PriorityMedium)
		lowMatches[i] = testMatch(domain.PriorityLow)

		require.NoError(t, qm.Enqueue(ctx, highMatches[i]))
		require.NoError(t, qm.Enqueue(ctx, medMatches[i]))
		require.NoError(t, qm.Enqueue(ctx, lowMatches[i]))
	}

	// Dequeue all 60 matches and track which priority was dequeued
	priorityCounts := map[domain.MatchPriority]int{}
	for i := range 60 {
		match, err := qm.Dequeue(ctx)
		require.NoError(t, err)
		require.NotNil(t, match, "expected match at iteration %d", i)
		priorityCounts[match.Priority]++
	}

	// Verify all matches were dequeued
	assert.Equal(t, 20, priorityCounts[domain.PriorityHigh])
	assert.Equal(t, 20, priorityCounts[domain.PriorityMedium])
	assert.Equal(t, 20, priorityCounts[domain.PriorityLow])
}

func TestQueueManager_WeightedQueueKeys_Cycle(t *testing.T) {
	qm := NewQueueManager(nil, testLogger(), testMetrics())

	// Verify the 9-step cycle pattern
	high := qm.getQueueKey(domain.PriorityHigh)
	medium := qm.getQueueKey(domain.PriorityMedium)
	low := qm.getQueueKey(domain.PriorityLow)

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

	// Reset counter
	qm.dequeueCount = 0

	for i, exp := range expected {
		got := qm.weightedQueueKeys()
		assert.Equal(t, exp, got, "cycle position %d", i)
	}

	// Cycle repeats
	got := qm.weightedQueueKeys() // position 9 = 0 mod 9
	assert.Equal(t, []string{high, medium, low}, got, "cycle should repeat at position 9")
}

func BenchmarkInMemoryQueue_LPush(b *testing.B) {
	q := NewInMemoryQueue()
	ctx := context.Background()
	match := testMatch(domain.PriorityMedium)
	data, _ := json.Marshal(match)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = q.LPush(ctx, "test", string(data))
	}
}

func BenchmarkInMemoryQueue_BRPop(b *testing.B) {
	q := NewInMemoryQueue()
	ctx := context.Background()
	match := testMatch(domain.PriorityMedium)
	data, _ := json.Marshal(match)

	// Pre-fill queue
	for i := 0; i < b.N; i++ {
		_ = q.LPush(ctx, "test", string(data))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = q.BRPop(ctx, time.Second, "test")
	}
}

// --- Dequeue dead-letter, dedup, purge tests ---

func TestQueueManager_Dequeue_MalformedJSON_DeadLetter(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// Push invalid JSON directly into the high priority queue
	queueKey := qm.getQueueKey(domain.PriorityHigh)
	err := qm.cache.LPush(ctx, queueKey, "not-valid-json{{{")
	require.NoError(t, err)

	// Dequeue should return an error and move the item to dead-letter queue
	match, err := qm.Dequeue(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal match")
	assert.Nil(t, match)

	// Verify the dead-letter queue has the item
	dlSize, err := qm.cache.LLen(ctx, "queue:dead_letter")
	require.NoError(t, err)
	assert.Equal(t, int64(1), dlSize)
}

func TestQueueManager_Enqueue_Dedup_SkipsDuplicate(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	match := testMatch(domain.PriorityHigh)

	// First enqueue should succeed
	err := qm.Enqueue(ctx, match)
	require.NoError(t, err)

	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)

	// Second enqueue of the same match should be skipped (dedup)
	err = qm.Enqueue(ctx, match)
	require.NoError(t, err)

	size, err = qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size) // still 1, not 2
}

func TestQueueManager_PurgeInvalidMatches_AllValid(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	m1 := testMatch(domain.PriorityHigh)
	m2 := testMatch(domain.PriorityHigh)
	require.NoError(t, qm.Enqueue(ctx, m1))
	require.NoError(t, qm.Enqueue(ctx, m2))

	// All matches are valid
	purged, err := qm.PurgeInvalidMatches(ctx, func(matchID string) bool {
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), purged)

	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(2), size)
}

func TestQueueManager_PurgeInvalidMatches_SomeInvalid(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	validMatch := testMatch(domain.PriorityMedium)
	invalidMatch := testMatch(domain.PriorityMedium)
	require.NoError(t, qm.Enqueue(ctx, validMatch))
	require.NoError(t, qm.Enqueue(ctx, invalidMatch))

	// Only validMatch's ID passes the validator
	purged, err := qm.PurgeInvalidMatches(ctx, func(matchID string) bool {
		return matchID == validMatch.ID.String()
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged)

	size, err := qm.GetQueueSize(ctx, domain.PriorityMedium)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)
}

func TestQueueManager_PurgeInvalidMatches_AllInvalid(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityLow)))
	require.NoError(t, qm.Enqueue(ctx, testMatch(domain.PriorityLow)))

	purged, err := qm.PurgeInvalidMatches(ctx, func(matchID string) bool {
		return false // all invalid
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), purged)

	size, err := qm.GetQueueSize(ctx, domain.PriorityLow)
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)
}

func TestQueueManager_PurgeInvalidMatches_EmptyQueue(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	purged, err := qm.PurgeInvalidMatches(ctx, func(matchID string) bool {
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), purged)
}

func TestQueueManager_PurgeInvalidMatches_MalformedJSON(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	// Push invalid JSON directly
	queueKey := qm.getQueueKey(domain.PriorityHigh)
	err := qm.cache.LPush(ctx, queueKey, "invalid-json")
	require.NoError(t, err)

	// Also push a valid match
	validMatch := testMatch(domain.PriorityHigh)
	require.NoError(t, qm.Enqueue(ctx, validMatch))

	purged, err := qm.PurgeInvalidMatches(ctx, func(matchID string) bool {
		return true // all real matches are valid
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged) // only the invalid JSON purged

	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size) // valid match remains
}

func TestQueueManager_Dequeue_RemovesFromDedupSet(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	match := testMatch(domain.PriorityHigh)
	require.NoError(t, qm.Enqueue(ctx, match))

	// Dequeue the match
	dequeued, err := qm.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, dequeued)
	assert.Equal(t, match.ID, dequeued.ID)

	// Повторный enqueue того же матча: должен быть разрешён, так как dedup очищен
	require.NoError(t, qm.Enqueue(ctx, match))

	size, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)
}

// --- Concurrency / Race Detection Tests ---

func TestQueueManager_ConcurrentEnqueueDequeue(t *testing.T) {
	qm := setupTestQueueManager(t)
	ctx := context.Background()

	const enqueueGoroutines = 5
	const matchesPerGoroutine = 20
	totalMatches := enqueueGoroutines * matchesPerGoroutine

	// Track all enqueued match IDs for verification.
	enqueuedIDs := make(chan uuid.UUID, totalMatches)

	var enqueueWg sync.WaitGroup
	enqueueWg.Add(enqueueGoroutines)

	// Launch concurrent enqueuers.
	for range enqueueGoroutines {
		go func() {
			defer enqueueWg.Done()
			for range matchesPerGoroutine {
				match := testMatch(domain.PriorityMedium)
				err := qm.Enqueue(ctx, match)
				assert.NoError(t, err)
				enqueuedIDs <- match.ID
			}
		}()
	}

	// Wait for all enqueues to complete, then close the channel.
	enqueueWg.Wait()
	close(enqueuedIDs)

	// Collect all enqueued IDs.
	expectedIDs := make(map[uuid.UUID]struct{}, totalMatches)
	for id := range enqueuedIDs {
		expectedIDs[id] = struct{}{}
	}
	require.Equal(t, totalMatches, len(expectedIDs), "all enqueued IDs should be unique")

	// Verify total queue size.
	total, err := qm.GetTotalQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(totalMatches), total)

	// Concurrently dequeue all matches.
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
					// Queue is empty (BRPop timed out).
					return
				}
				dequeuedIDs <- match.ID
			}
		}()
	}

	dequeueWg.Wait()
	close(dequeuedIDs)

	// Collect all dequeued IDs.
	gotIDs := make(map[uuid.UUID]struct{})
	for id := range dequeuedIDs {
		gotIDs[id] = struct{}{}
	}

	// Every dequeued match must have been enqueued.
	for id := range gotIDs {
		_, found := expectedIDs[id]
		assert.True(t, found, "dequeued ID %s was not enqueued", id)
	}

	// All enqueued matches should have been dequeued.
	assert.Equal(t, len(expectedIDs), len(gotIDs),
		"total dequeued (%d) should equal total enqueued (%d)", len(gotIDs), len(expectedIDs))
}
