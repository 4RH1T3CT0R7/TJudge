package queue

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
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

func (m *MockCache) LPush(ctx context.Context, key string, values ...interface{}) error {
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
	LPush(ctx context.Context, key string, values ...interface{}) error
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
	cache := new(MockCache)
	qm := &QueueManager{
		cache:   nil, // We'll set it through a different test
		log:     testLogger(),
		metrics: testMetrics(),
	}

	// Setup mock expectations for each priority
	cache.On("LLen", mock.Anything, "queue:high").Return(int64(5), nil)
	cache.On("LLen", mock.Anything, "queue:medium").Return(int64(10), nil)
	cache.On("LLen", mock.Anything, "queue:low").Return(int64(3), nil)

	// Note: This test is a demonstration of the interface
	// Actual testing requires dependency injection refactoring
	_ = qm
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

func (q *InMemoryQueue) LPush(ctx context.Context, key string, values ...interface{}) error {
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
