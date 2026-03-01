package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
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

// MockQueueManager mocks the QueueManager interface
type MockQueueManager struct {
	mock.Mock
	mu      sync.Mutex
	matches []*domain.Match
}

func NewMockQueueManager() *MockQueueManager {
	return &MockQueueManager{
		matches: make([]*domain.Match, 0),
	}
}

func (m *MockQueueManager) Dequeue(ctx context.Context) (*domain.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Match), args.Error(1)
}

func (m *MockQueueManager) GetTotalQueueSize(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQueueManager) EnqueueMatch(match *domain.Match) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.matches = append(m.matches, match)
}

// MockMatchProcessor mocks the MatchProcessor interface
type MockMatchProcessor struct {
	mock.Mock
	processedMatches atomic.Int32
	failCount        atomic.Int32
}

func NewMockMatchProcessor() *MockMatchProcessor {
	return &MockMatchProcessor{}
}

func (m *MockMatchProcessor) Process(ctx context.Context, match *domain.Match) error {
	args := m.Called(ctx, match)
	if args.Error(0) == nil {
		m.processedMatches.Add(1)
	} else {
		m.failCount.Add(1)
	}
	return args.Error(0)
}

func (m *MockMatchProcessor) GetProcessedCount() int32 {
	return m.processedMatches.Load()
}

func (m *MockMatchProcessor) GetFailCount() int32 {
	return m.failCount.Load()
}

// Helper function to create test config
func testConfig() config.WorkerConfig {
	return config.WorkerConfig{
		MinWorkers:    2,
		MaxWorkers:    10,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
	}
}

// Helper function to create test metrics (singleton to avoid duplicate registration)
func testMetrics() *metrics.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetrics = metrics.New()
	})
	return sharedMetrics
}

// Helper function to create test logger
func testLogger() *logger.Logger {
	log, _ := logger.New("debug", "json")
	return log
}

// Helper function to create a test match
func testMatch() *domain.Match {
	return &domain.Match{
		ID:       uuid.New(),
		Priority: domain.PriorityMedium,
		Status:   domain.MatchPending,
		GameType: "tictactoe",
	}
}

func TestNewPool(t *testing.T) {
	cfg := testConfig()
	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	assert.NotNil(t, pool)
	assert.Equal(t, cfg, pool.config)
}

func TestPool_StartStop(t *testing.T) {
	cfg := testConfig()
	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	// Setup queue to return nil (empty)
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Start pool
	pool.Start()

	// Wait for workers to start
	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	stats := pool.GetStats()
	assert.GreaterOrEqual(t, stats.TotalWorkers, cfg.MinWorkers)

	// Stop pool
	pool.Stop()

	// All workers should be stopped
	stats = pool.GetStats()
	assert.Equal(t, 0, stats.TotalWorkers)
}

func TestPool_ProcessMatch(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 2

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	match := testMatch()

	// First call returns match, subsequent calls return nil
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Processor should successfully process the match
	processor.On("Process", mock.Anything, match).Return(nil)

	pool.Start()

	// Wait for processing
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 1
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// Verify match was processed
	assert.Equal(t, int32(1), processor.GetProcessedCount())
}

func TestPool_RetryOnFailure(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 3
	cfg.RetryDelay = 10 * time.Millisecond

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	match := testMatch()

	// Return match once then nil
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Fail first two attempts, succeed on third
	processor.On("Process", mock.Anything, match).Return(errors.New("temporary error")).Twice()
	processor.On("Process", mock.Anything, match).Return(nil).Once()

	pool.Start()

	// Wait for processing with retries
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 1
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// Final attempt succeeded (3 total calls to Process)
	assert.Equal(t, int32(1), processor.GetProcessedCount())
}

func TestPool_GetStats(t *testing.T) {
	cfg := testConfig()
	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	pool.Start()

	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	stats := pool.GetStats()

	assert.GreaterOrEqual(t, stats.TotalWorkers, cfg.MinWorkers)
	assert.Equal(t, int64(0), stats.MatchesProcessed)
	assert.Equal(t, int64(0), stats.MatchesFailed)

	pool.Stop()
}

func TestPool_ConcurrentProcessing(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 4
	cfg.MaxWorkers = 4

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	// Return 10 matches then nil
	for i := 0; i < 10; i++ {
		queue.On("Dequeue", mock.Anything).Return(testMatch(), nil).Once()
	}
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Processor should process all matches
	processor.On("Process", mock.Anything, mock.AnythingOfType("*domain.Match")).Return(nil)

	pool.Start()

	// Wait for all matches to be processed
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 10
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// All matches should be processed
	assert.Equal(t, int32(10), processor.GetProcessedCount())
}

func TestPool_GracefulShutdown(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 4

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	// Setup match that takes time to process
	match := testMatch()
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Processor takes 200ms to complete
	processor.On("Process", mock.Anything, match).Run(func(args mock.Arguments) {
		time.Sleep(200 * time.Millisecond)
	}).Return(nil)

	pool.Start()

	// Wait for processing to start
	time.Sleep(50 * time.Millisecond)

	// Stop pool - should wait for current match to complete
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Pool stopped gracefully
	case <-time.After(5 * time.Second):
		t.Fatal("Pool did not stop in time")
	}

	// Verify match was processed
	assert.Equal(t, int32(1), processor.GetProcessedCount())
}

func TestPool_FailedMatchCounting(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 1 // No retries

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	match := testMatch()

	// Return match once then nil
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Processing always fails
	processor.On("Process", mock.Anything, match).Return(errors.New("processing failed"))

	pool.Start()

	require.Eventually(t, func() bool {
		return pool.GetStats().MatchesFailed >= 1
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	stats := pool.GetStats()
	assert.Equal(t, int64(1), stats.MatchesFailed)
	assert.Equal(t, int64(0), stats.MatchesProcessed)
}

func TestPool_Wait(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 2

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	pool.Start()

	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	// Cancel context
	pool.cancel()

	// Wait should return after all workers finish
	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return in time")
	}
}

func TestPool_Scale_UpOnLargeQueue(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 20

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	// Don't call Start() — set up state manually to avoid spawning real workers.
	pool.totalWorkers.Store(2)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(150), nil)

	pool.scale()

	// scale() should have spawned extra workers: target = current(2) + 10 = 12
	// Wait for spawn goroutines to increment totalWorkers.
	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers > 2
	}, 2*time.Second, 10*time.Millisecond)

	pool.Stop()
}

func TestPool_Scale_DownOnEmptyQueue(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 20

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Simulate a pool that has 12 workers but an empty queue and 0 active workers.
	// Manually populate workerCancels with dummy cancel functions so scale-down
	// has something to cancel.
	pool.totalWorkers.Store(12)
	pool.activeWorkers.Store(0) // 0 < 12/2 → eligible for scale-down

	dummyCancels := make([]context.CancelFunc, 12)
	for i := range dummyCancels {
		_, cancel := context.WithCancel(context.Background())
		dummyCancels[i] = cancel
	}
	pool.workerMu.Lock()
	pool.workerCancels = dummyCancels
	pool.workerMu.Unlock()

	pool.scale()

	// scale() with queueSize < 10 and activeWorkers < currentWorkers/2
	// → target = current(12) - 5 = 7
	// Verify that some cancel functions were removed.
	pool.workerMu.Lock()
	remaining := len(pool.workerCancels)
	pool.workerMu.Unlock()
	assert.Equal(t, 7, remaining)

	pool.Stop()
}

func TestPool_Scale_NeverBelowMin(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 3
	cfg.MaxWorkers = 20

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	pool.Start()

	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	// Even with empty queue, scale should not go below min
	pool.scale()
	pool.scale()

	assert.GreaterOrEqual(t, pool.GetStats().TotalWorkers, cfg.MinWorkers)

	pool.Stop()
}

func TestPool_Scale_NeverAboveMax(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 5

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(200), nil)

	pool.Start()

	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	// Multiple scale-ups should never exceed max
	pool.scale()
	pool.scale()
	pool.scale()

	assert.LessOrEqual(t, pool.GetStats().TotalWorkers, cfg.MaxWorkers)

	pool.Stop()
}

func TestPool_ProcessWithRetry_MatchNotFound(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 3

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	match := testMatch()

	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Process returns ErrMatchNotFound — should be skipped (not retried), counted as success
	processor.On("Process", mock.Anything, match).Return(ErrMatchNotFound)

	pool.Start()

	// Wait for the match to be dequeued and handled
	require.Eventually(t, func() bool {
		// MatchNotFound is treated as success (nil return), so matchesProcessed increments
		return pool.GetStats().MatchesProcessed >= 1
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// Should NOT be counted as failed
	assert.Equal(t, int64(0), pool.GetStats().MatchesFailed)
	// Process should only be called once (no retries)
	processor.AssertNumberOfCalls(t, "Process", 1)
}

func TestPool_GetMatchesProcessed(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	// Return 5 matches then nil
	for i := 0; i < 5; i++ {
		queue.On("Dequeue", mock.Anything).Return(testMatch(), nil).Once()
	}
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	processor.On("Process", mock.Anything, mock.AnythingOfType("*domain.Match")).Return(nil)

	pool.Start()

	require.Eventually(t, func() bool {
		return pool.GetMatchesProcessed() >= 5
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	assert.Equal(t, int64(5), pool.GetMatchesProcessed())
}
