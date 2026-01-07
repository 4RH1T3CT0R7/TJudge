//go:build benchmark
// +build benchmark

package benchmark

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
)

var (
	queueOnce    sync.Once
	queueManager *queue.QueueManager
	redisCache   *cache.Cache
)

func setupQueue(b *testing.B) {
	queueOnce.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}

		log, err := logger.New("error", "json")
		if err != nil {
			b.Fatalf("Failed to create logger: %v", err)
		}

		m := metrics.New()

		redisCache, err = cache.New(&cfg.Redis, log, m)
		if err != nil {
			b.Fatalf("Failed to connect to Redis: %v", err)
		}

		queueManager = queue.NewQueueManager(redisCache, log, m)
	})
}

// BenchmarkQueueEnqueue measures enqueue performance
func BenchmarkQueueEnqueue(b *testing.B) {
	setupQueue(b)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		match := &domain.Match{
			ID:           uuid.New(),
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "tictactoe",
			Status:       domain.MatchPending,
			Priority:     domain.PriorityMedium,
			CreatedAt:    time.Now(),
		}
		queueManager.Enqueue(ctx, match)
	}
}

// BenchmarkQueueEnqueueParallel measures parallel enqueue performance
func BenchmarkQueueEnqueueParallel(b *testing.B) {
	setupQueue(b)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			match := &domain.Match{
				ID:           uuid.New(),
				TournamentID: uuid.New(),
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "tictactoe",
				Status:       domain.MatchPending,
				Priority:     domain.PriorityMedium,
				CreatedAt:    time.Now(),
			}
			queueManager.Enqueue(ctx, match)
		}
	})
}

// BenchmarkQueueDequeue measures dequeue performance
func BenchmarkQueueDequeue(b *testing.B) {
	setupQueue(b)

	ctx := context.Background()

	// Pre-populate queue
	for i := 0; i < 1000; i++ {
		match := &domain.Match{
			ID:           uuid.New(),
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "tictactoe",
			Status:       domain.MatchPending,
			Priority:     domain.PriorityMedium,
			CreatedAt:    time.Now(),
		}
		queueManager.Enqueue(ctx, match)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queueManager.Dequeue(ctx)
	}
}

// BenchmarkQueueEnqueueDequeue measures combined enqueue/dequeue
func BenchmarkQueueEnqueueDequeue(b *testing.B) {
	setupQueue(b)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		match := &domain.Match{
			ID:           uuid.New(),
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "tictactoe",
			Status:       domain.MatchPending,
			Priority:     domain.PriorityMedium,
			CreatedAt:    time.Now(),
		}
		queueManager.Enqueue(ctx, match)
		queueManager.Dequeue(ctx)
	}
}

// BenchmarkQueueSize measures queue size retrieval
func BenchmarkQueueSize(b *testing.B) {
	setupQueue(b)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			queueManager.GetTotalQueueSize(ctx)
		}
	})
}

// BenchmarkQueuePriorities measures priority queue operations
func BenchmarkQueuePriorities(b *testing.B) {
	setupQueue(b)

	ctx := context.Background()
	priorities := []domain.MatchPriority{
		domain.PriorityHigh,
		domain.PriorityMedium,
		domain.PriorityLow,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		match := &domain.Match{
			ID:           uuid.New(),
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "tictactoe",
			Status:       domain.MatchPending,
			Priority:     priorities[i%len(priorities)],
			CreatedAt:    time.Now(),
		}
		queueManager.Enqueue(ctx, match)
	}
}
