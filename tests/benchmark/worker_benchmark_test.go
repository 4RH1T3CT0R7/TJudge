//go:build benchmark
// +build benchmark

package benchmark

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/worker"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

var (
	benchMetrics     *metrics.Metrics
	benchMetricsOnce sync.Once
)

// MockBenchQueueManager mocks the QueueManager interface for benchmarks
type MockBenchQueueManager struct {
	mock.Mock
	mu           sync.Mutex
	matches      chan *domain.Match
	matchCount   int32
	dequeueCount atomic.Int64
}

func NewMockBenchQueueManager(matchCount int) *MockBenchQueueManager {
	m := &MockBenchQueueManager{
		matches:    make(chan *domain.Match, matchCount),
		matchCount: int32(matchCount),
	}

	// Pre-fill with matches
	for i := 0; i < matchCount; i++ {
		m.matches <- &domain.Match{
			ID:       uuid.New(),
			Priority: domain.PriorityMedium,
			Status:   domain.MatchPending,
			GameType: "tictactoe",
		}
	}

	return m
}

func (m *MockBenchQueueManager) Dequeue(ctx context.Context) (*domain.Match, error) {
	m.dequeueCount.Add(1)
	select {
	case match := <-m.matches:
		return match, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, nil
	}
}

func (m *MockBenchQueueManager) GetTotalQueueSize(ctx context.Context) (int64, error) {
	return int64(len(m.matches)), nil
}

// MockBenchMatchProcessor mocks the MatchProcessor for benchmarks
type MockBenchMatchProcessor struct {
	processedCount atomic.Int64
	processingTime time.Duration
}

func NewMockBenchMatchProcessor(processingTime time.Duration) *MockBenchMatchProcessor {
	return &MockBenchMatchProcessor{
		processingTime: processingTime,
	}
}

func (m *MockBenchMatchProcessor) Process(ctx context.Context, match *domain.Match) error {
	// Simulate processing time
	if m.processingTime > 0 {
		time.Sleep(m.processingTime)
	}
	m.processedCount.Add(1)
	return nil
}

func (m *MockBenchMatchProcessor) GetProcessedCount() int64 {
	return m.processedCount.Load()
}

func benchMetricsInstance() *metrics.Metrics {
	benchMetricsOnce.Do(func() {
		benchMetrics = metrics.New()
	})
	return benchMetrics
}

func benchLogger() *logger.Logger {
	log, _ := logger.New("error", "json")
	return log
}

// BenchmarkWorkerPool_ThroughputSmall tests throughput with small worker pool
func BenchmarkWorkerPool_ThroughputSmall(b *testing.B) {
	cfg := config.WorkerConfig{
		MinWorkers:    2,
		MaxWorkers:    4,
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		queue := NewMockBenchQueueManager(100)
		processor := NewMockBenchMatchProcessor(0)
		log := benchLogger()
		m := benchMetricsInstance()

		pool := worker.NewPool(cfg, queue, processor, log, m)

		b.StartTimer()

		pool.Start()

		// Wait until all matches processed or timeout
		deadline := time.Now().Add(5 * time.Second)
		for processor.GetProcessedCount() < 100 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}

		pool.Stop()
		pool.Wait()
	}
}

// BenchmarkWorkerPool_ThroughputMedium tests throughput with medium worker pool
func BenchmarkWorkerPool_ThroughputMedium(b *testing.B) {
	cfg := config.WorkerConfig{
		MinWorkers:    4,
		MaxWorkers:    8,
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		queue := NewMockBenchQueueManager(500)
		processor := NewMockBenchMatchProcessor(0)
		log := benchLogger()
		m := benchMetricsInstance()

		pool := worker.NewPool(cfg, queue, processor, log, m)

		b.StartTimer()

		pool.Start()

		// Wait until all matches processed or timeout
		deadline := time.Now().Add(10 * time.Second)
		for processor.GetProcessedCount() < 500 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}

		pool.Stop()
		pool.Wait()
	}
}

// BenchmarkWorkerPool_ThroughputLarge tests throughput with large worker pool
func BenchmarkWorkerPool_ThroughputLarge(b *testing.B) {
	cfg := config.WorkerConfig{
		MinWorkers:    8,
		MaxWorkers:    16,
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		queue := NewMockBenchQueueManager(1000)
		processor := NewMockBenchMatchProcessor(0)
		log := benchLogger()
		m := benchMetricsInstance()

		pool := worker.NewPool(cfg, queue, processor, log, m)

		b.StartTimer()

		pool.Start()

		// Wait until all matches processed or timeout
		deadline := time.Now().Add(15 * time.Second)
		for processor.GetProcessedCount() < 1000 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}

		pool.Stop()
		pool.Wait()
	}
}

// BenchmarkWorkerPool_ProcessingLatency measures processing latency
func BenchmarkWorkerPool_ProcessingLatency(b *testing.B) {
	cfg := config.WorkerConfig{
		MinWorkers:    4,
		MaxWorkers:    4,
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	// Simulate 10ms processing time
	processingTime := 10 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		queue := NewMockBenchQueueManager(50)
		processor := NewMockBenchMatchProcessor(processingTime)
		log := benchLogger()
		m := benchMetricsInstance()

		pool := worker.NewPool(cfg, queue, processor, log, m)

		b.StartTimer()

		pool.Start()

		// Wait until all matches processed or timeout
		deadline := time.Now().Add(10 * time.Second)
		for processor.GetProcessedCount() < 50 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}

		pool.Stop()
		pool.Wait()
	}
}

// BenchmarkWorkerPool_ScaleUp tests autoscaling performance
func BenchmarkWorkerPool_ScaleUp(b *testing.B) {
	cfg := config.WorkerConfig{
		MinWorkers:    1,
		MaxWorkers:    16,
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		queue := NewMockBenchQueueManager(200)
		processor := NewMockBenchMatchProcessor(5 * time.Millisecond)
		log := benchLogger()
		m := benchMetricsInstance()

		pool := worker.NewPool(cfg, queue, processor, log, m)

		b.StartTimer()

		pool.Start()

		// Wait until all matches processed or timeout
		deadline := time.Now().Add(10 * time.Second)
		for processor.GetProcessedCount() < 200 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}

		pool.Stop()
		pool.Wait()
	}
}

// BenchmarkMatchCreation measures match object creation performance
func BenchmarkMatchCreation(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = &domain.Match{
				ID:           uuid.New(),
				TournamentID: uuid.New(),
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "tictactoe",
				Status:       domain.MatchPending,
				Priority:     domain.PriorityMedium,
				CreatedAt:    time.Now(),
			}
		}
	})
}

// BenchmarkUUIDGeneration measures UUID generation performance
func BenchmarkUUIDGeneration(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = uuid.New()
		}
	})
}
