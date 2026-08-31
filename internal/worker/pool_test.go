package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	sharedMetrics     *metrics.Metrics
	sharedMetricsOnce sync.Once
)

// MockQueueManager - мок интерфейса QueueManager
type MockQueueManager struct {
	mock.Mock
	mu      sync.Mutex
	matches []*models.Match
}

func NewMockQueueManager() *MockQueueManager {
	return &MockQueueManager{
		matches: make([]*models.Match, 0),
	}
}

func (m *MockQueueManager) Enqueue(ctx context.Context, match *models.Match) error {
	args := m.Called(ctx, match)
	return args.Error(0)
}

func (m *MockQueueManager) Dequeue(ctx context.Context) (*models.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Match), args.Error(1)
}

func (m *MockQueueManager) GetTotalQueueSize(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQueueManager) EnqueueMatch(match *models.Match) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.matches = append(m.matches, match)
}

// MockMatchProcessor - мок интерфейса MatchProcessor
type MockMatchProcessor struct {
	mock.Mock
	processedMatches atomic.Int32
	failCount        atomic.Int32
}

func NewMockMatchProcessor() *MockMatchProcessor {
	return &MockMatchProcessor{}
}

func (m *MockMatchProcessor) Process(ctx context.Context, match *models.Match) error {
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

// testConfig создаёт тестовую конфигурацию
func testConfig() config.WorkerConfig {
	return config.WorkerConfig{
		MinWorkers:    2,
		MaxWorkers:    10,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
	}
}

// testMetrics создаёт тестовые метрики (singleton, чтобы избежать дублирования регистрации)
func testMetrics() *metrics.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetrics = metrics.New()
	})
	return sharedMetrics
}

// testLogger создаёт тестовый логгер
func testLogger() *logger.Logger {
	log, _ := logger.New("debug", "json")
	return log
}

// testMatch создаёт тестовый матч
func testMatch() *models.Match {
	return &models.Match{
		ID:       uuid.New(),
		Priority: models.PriorityMedium,
		Status:   models.MatchPending,
		GameType: "tictactoe",
	}
}

// newTestPool собирает пул с моками очереди и процессора
func newTestPool(t *testing.T, cfg config.WorkerConfig) (*Pool, *MockQueueManager, *MockMatchProcessor) {
	t.Helper()
	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	pool := NewPool(cfg, queue, processor, testLogger(), testMetrics())
	return pool, queue, processor
}

// после старта воркеры поднимаются до min, после Stop счётчик обнуляется
func TestPool_StartStop(t *testing.T) {
	pool, queue, _ := newTestPool(t, testConfig())

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	pool.Start()
	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= testConfig().MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	assert.GreaterOrEqual(t, pool.GetStats().TotalWorkers, testConfig().MinWorkers)

	pool.Stop()
	assert.Equal(t, 0, pool.GetStats().TotalWorkers)
}

// два падения подряд ретраятся, третья попытка успешна
func TestPool_RetryOnFailure(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 3
	cfg.RetryDelay = 10 * time.Millisecond

	pool, queue, processor := newTestPool(t, cfg)
	match := testMatch()

	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	processor.On("Process", mock.Anything, match).Return(errors.New("temporary error")).Twice()
	processor.On("Process", mock.Anything, match).Return(nil).Once()

	pool.Start()
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 1
	}, 5*time.Second, 10*time.Millisecond)
	pool.Stop()

	// матч зачтён один раз (всего 3 вызова Process)
	assert.Equal(t, int32(1), processor.GetProcessedCount())
}

// без retry падение матча инкрементит metric MatchesFailed, а не MatchesProcessed
func TestPool_FailedMatchCounting(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 1

	pool, queue, processor := newTestPool(t, cfg)
	match := testMatch()

	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

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

// -race: четыре воркера параллельно разбирают десять матчей
func TestPool_ConcurrentProcessing(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 4
	cfg.MaxWorkers = 4

	pool, queue, processor := newTestPool(t, cfg)

	for range 10 {
		queue.On("Dequeue", mock.Anything).Return(testMatch(), nil).Once()
	}
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	processor.On("Process", mock.Anything, mock.AnythingOfType("*models.Match")).Return(nil)

	pool.Start()
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 10
	}, 5*time.Second, 10*time.Millisecond)
	pool.Stop()

	assert.Equal(t, int32(10), processor.GetProcessedCount())
}

// Stop дожидается завершения матча, запущенного в обработку
func TestPool_GracefulShutdown(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 4

	pool, queue, processor := newTestPool(t, cfg)

	match := testMatch()
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// обработка занимает 200мс
	processor.On("Process", mock.Anything, match).Run(func(args mock.Arguments) {
		time.Sleep(200 * time.Millisecond)
	}).Return(nil)

	pool.Start()
	time.Sleep(50 * time.Millisecond) // воркер успевает взять матч

	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Pool did not stop in time")
	}

	assert.Equal(t, int32(1), processor.GetProcessedCount())
}

// большая очередь: scale() поднимает число воркеров выше стартового
func TestPool_Scale_UpOnLargeQueue(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 20

	pool, queue, _ := newTestPool(t, cfg)

	// Start() не вызывается: состояние выставляется вручную, чтобы не плодить настоящих воркеров
	pool.totalWorkers.Store(2)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(150), nil)

	pool.scale()

	// target = current(2) + 10 = 12; spawn-горутины увеличивают totalWorkers
	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers > 2
	}, 2*time.Second, 10*time.Millisecond)

	pool.Stop()
}

// пустая очередь: scale() медленно опускает число воркеров (гистерезис)
func TestPool_Scale_DownOnEmptyQueue(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 20

	pool, queue, _ := newTestPool(t, cfg)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// пул из 12 воркеров, пустая очередь, 0 активных; workerCancels заполняется
	// заглушечными cancel, чтобы scale-down было что отменять
	pool.totalWorkers.Store(12)
	pool.activeWorkers.Store(0) // 0 < 12/2, кандидат на scale-down

	dummyCancels := make([]context.CancelFunc, 12)
	for i := range dummyCancels {
		_, cancel := context.WithCancel(context.Background())
		dummyCancels[i] = cancel
	}
	pool.workerMu.Lock()
	pool.workerCancels = dummyCancels
	pool.workerMu.Unlock()

	pool.scale()

	// queueSize==0 и activeWorkers*3 < current: target = 12 - 2 = 10
	pool.workerMu.Lock()
	remaining := len(pool.workerCancels)
	pool.workerMu.Unlock()
	assert.Equal(t, 10, remaining)

	pool.Stop()
}

// scale не опускается ниже min даже при пустой очереди
func TestPool_Scale_NeverBelowMin(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 3
	cfg.MaxWorkers = 20

	pool, queue, _ := newTestPool(t, cfg)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	pool.Start()
	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	pool.scale()
	pool.scale()

	assert.GreaterOrEqual(t, pool.GetStats().TotalWorkers, cfg.MinWorkers)

	pool.Stop()
}

// scale не превышает max даже при переполненной очереди
func TestPool_Scale_NeverAboveMax(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 5

	pool, queue, _ := newTestPool(t, cfg)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(200), nil)

	pool.Start()
	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	pool.scale()
	pool.scale()
	pool.scale()

	assert.LessOrEqual(t, pool.GetStats().TotalWorkers, cfg.MaxWorkers)

	pool.Stop()
}

// ErrMatchNotFound: матч пропускается без retry и засчитывается как успех
func TestPool_ProcessWithRetry_MatchNotFound(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 3

	pool, queue, processor := newTestPool(t, cfg)
	match := testMatch()

	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	processor.On("Process", mock.Anything, match).Return(ErrMatchNotFound)

	pool.Start()
	// MatchNotFound = успех (nil), поэтому matchesProcessed растёт
	require.Eventually(t, func() bool {
		return pool.GetStats().MatchesProcessed >= 1
	}, 5*time.Second, 10*time.Millisecond)
	pool.Stop()

	assert.Equal(t, int64(0), pool.GetStats().MatchesFailed) // не считается как failed
	processor.AssertNumberOfCalls(t, "Process", 1)           // без retry
}

// паника в воркере ловится, воркер пересоздаётся и продолжает обработку
func TestPool_PanicRecovery_Respawns(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 4
	cfg.RetryAttempts = 1

	pool, queue, processor := newTestPool(t, cfg)

	var callCount atomic.Int32

	// матчей хватает, чтобы и пересозданный воркер взял работу
	for range 20 {
		queue.On("Dequeue", mock.Anything).Return(testMatch(), nil).Once()
	}
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// первый вызов паникует, остальные успешны
	processor.On("Process", mock.Anything, mock.AnythingOfType("*models.Match")).Run(func(args mock.Arguments) {
		if callCount.Add(1) == 1 {
			panic("test panic for recovery")
		}
	}).Return(nil)

	pool.Start()

	// после паники число воркеров возвращается к MinWorkers
	assert.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 100*time.Millisecond)

	// пересозданный воркер продолжил обработку
	assert.Eventually(t, func() bool {
		return callCount.Load() > 1
	}, 5*time.Second, 100*time.Millisecond)

	pool.Stop()
}

// отмена контекста во время retry-backoff возвращает context.Canceled
func TestPool_ProcessWithRetry_ContextCancelled(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 3
	cfg.RetryDelay = 2 * time.Second // длинная задержка, чтобы успеть отменить

	pool, _, processor := newTestPool(t, cfg)
	match := testMatch()

	processor.On("Process", mock.Anything, match).Return(errors.New("always fails"))

	ctx, cancel := context.WithCancel(context.Background())
	// отмена во время backoff между попыткой 1 и попыткой 2
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := pool.processWithRetry(ctx, match)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
