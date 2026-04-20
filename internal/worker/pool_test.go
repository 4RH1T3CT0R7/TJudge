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

// MockQueueManager - мок интерфейса QueueManager
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

// MockMatchProcessor - мок интерфейса MatchProcessor
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

	// Настраиваем очередь возвращать nil (пусто)
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Запускаем пул
	pool.Start()

	// Ждём запуска воркеров
	require.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 10*time.Millisecond)

	stats := pool.GetStats()
	assert.GreaterOrEqual(t, stats.TotalWorkers, cfg.MinWorkers)

	// Останавливаем пул
	pool.Stop()

	// Все воркеры должны быть остановлены
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

	// Первый вызов возвращает матч, последующие - nil
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Processor успешно обрабатывает матч
	processor.On("Process", mock.Anything, match).Return(nil)

	pool.Start()

	// Ждём обработки
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 1
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// Проверяем, что матч был обработан
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

	// Возвращаем матч один раз, затем nil
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Первые две попытки падают, третья успешна
	processor.On("Process", mock.Anything, match).Return(errors.New("temporary error")).Twice()
	processor.On("Process", mock.Anything, match).Return(nil).Once()

	pool.Start()

	// Ждём обработки с retry
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 1
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// Последняя попытка успешна (всего 3 вызова Process)
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

	// Возвращаем 10 матчей, затем nil
	for i := 0; i < 10; i++ {
		queue.On("Dequeue", mock.Anything).Return(testMatch(), nil).Once()
	}
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Processor должен обработать все матчи
	processor.On("Process", mock.Anything, mock.AnythingOfType("*domain.Match")).Return(nil)

	pool.Start()

	// Ждём обработки всех матчей
	require.Eventually(t, func() bool {
		return processor.GetProcessedCount() >= 10
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// Все матчи должны быть обработаны
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

	// Готовим матч, обработка которого занимает время
	match := testMatch()
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Processor тратит 200мс на завершение
	processor.On("Process", mock.Anything, match).Run(func(args mock.Arguments) {
		time.Sleep(200 * time.Millisecond)
	}).Return(nil)

	pool.Start()

	// Ждём начала обработки
	time.Sleep(50 * time.Millisecond)

	// Останавливаем пул - должен дождаться завершения текущего матча
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Пул остановился штатно
	case <-time.After(5 * time.Second):
		t.Fatal("Pool did not stop in time")
	}

	// Проверяем, что матч был обработан
	assert.Equal(t, int32(1), processor.GetProcessedCount())
}

func TestPool_FailedMatchCounting(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 1 // Без retry

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	match := testMatch()

	// Возвращаем матч один раз, затем nil
	queue.On("Dequeue", mock.Anything).Return(match, nil).Once()
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Обработка всегда падает
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

	// Отменяем контекст
	pool.cancel()

	// Wait должен вернуться после завершения всех воркеров
	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Успех
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

	// Не вызываем Start(): выставляем состояние вручную, чтобы не плодить настоящих воркеров.
	pool.totalWorkers.Store(2)

	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(150), nil)

	pool.scale()

	// scale() должен был создать дополнительных воркеров: target = current(2) + 10 = 12
	// Ждём, когда spawn-горутины увеличат totalWorkers.
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

	// Моделируем пул с 12 воркерами, пустой очередью и 0 активных воркеров.
	// Вручную заполняем workerCancels заглушечными cancel-функциями, чтобы
	// scale-down было что отменять.
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

	// scale() при queueSize == 0 и activeWorkers*3 < currentWorkers
	// даёт target = current(12) - 2 = 10 (медленный scale-down, гистерезис).
	pool.workerMu.Lock()
	remaining := len(pool.workerCancels)
	pool.workerMu.Unlock()
	assert.Equal(t, 10, remaining)

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

	// Даже при пустой очереди scale не должен опуститься ниже min
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

	// Несколько scale-up не должны превышать max
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

	// Process возвращает ErrMatchNotFound - должен быть пропущен (без retry), зачтён как успех
	processor.On("Process", mock.Anything, match).Return(ErrMatchNotFound)

	pool.Start()

	// Ждём, пока матч будет получен и обработан
	require.Eventually(t, func() bool {
		// MatchNotFound трактуется как успех (возврат nil), поэтому matchesProcessed увеличивается
		return pool.GetStats().MatchesProcessed >= 1
	}, 5*time.Second, 10*time.Millisecond)

	pool.Stop()

	// НЕ должен считаться как failed
	assert.Equal(t, int64(0), pool.GetStats().MatchesFailed)
	// Process должен быть вызван только один раз (без retry)
	processor.AssertNumberOfCalls(t, "Process", 1)
}

func TestPool_PanicRecovery_Respawns(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 2
	cfg.MaxWorkers = 4
	cfg.RetryAttempts = 1

	queue := NewMockQueueManager()
	log := testLogger()
	m := testMetrics()

	// Считаем, сколько раз был вызван Process.
	var callCount atomic.Int32

	// Используем кастомный processor, который паникует при первом вызове, но успешен на последующих.
	processor := NewMockMatchProcessor()

	// Нужно возвращать матчи, чтобы воркеры реально вызывали Process и ловили панику.
	// Возвращаем достаточно матчей, чтобы и пересозданный воркер смог что-то взять.
	for i := 0; i < 20; i++ {
		queue.On("Dequeue", mock.Anything).Return(testMatch(), nil).Once()
	}
	queue.On("Dequeue", mock.Anything).Return(nil, nil)
	queue.On("GetTotalQueueSize", mock.Anything).Return(int64(0), nil)

	// Первый вызов паникует, последующие - успешные.
	processor.On("Process", mock.Anything, mock.AnythingOfType("*domain.Match")).Run(func(args mock.Arguments) {
		n := callCount.Add(1)
		if n == 1 {
			panic("test panic for recovery")
		}
	}).Return(nil)

	pool := NewPool(cfg, queue, processor, log, m)
	pool.Start()

	// После паники воркер должен быть пересоздан (через ~1 секунду),
	// и общее число воркеров должно вернуться хотя бы к MinWorkers.
	assert.Eventually(t, func() bool {
		return pool.GetStats().TotalWorkers >= cfg.MinWorkers
	}, 5*time.Second, 100*time.Millisecond)

	// Проверяем, что Process был вызван более одного раза (паника произошла,
	// и пересозданный воркер продолжил обработку).
	assert.Eventually(t, func() bool {
		return callCount.Load() > 1
	}, 5*time.Second, 100*time.Millisecond)

	pool.Stop()
}

func TestPool_ProcessWithRetry_ContextCancelled(t *testing.T) {
	cfg := testConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 1
	cfg.RetryAttempts = 3
	cfg.RetryDelay = 2 * time.Second // Длинная задержка, чтобы можно было отменить во время неё

	queue := NewMockQueueManager()
	processor := NewMockMatchProcessor()
	log := testLogger()
	m := testMetrics()

	pool := NewPool(cfg, queue, processor, log, m)

	match := testMatch()

	// Processor всегда падает, провоцируя retry с backoff
	processor.On("Process", mock.Anything, match).Return(errors.New("always fails"))

	// Вызываем processWithRetry напрямую с отменяемым контекстом.
	ctx, cancel := context.WithCancel(context.Background())

	// Отменяем контекст после небольшой задержки - во время retry backoff
	// между попыткой 1 (сразу) и попыткой 2 (после RetryDelay * 2 = 4s).
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := pool.processWithRetry(ctx, match)

	// Функция должна вернуть context.Canceled, потому что контекст был
	// отменён во время ожидания retry delay.
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
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

	// Возвращаем 5 матчей, затем nil
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
