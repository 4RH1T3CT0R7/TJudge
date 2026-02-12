//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// QueueProcessingSuite tests the match queue with real Redis.
type QueueProcessingSuite struct {
	suite.Suite
	cache *cache.Cache
	qm    *queue.QueueManager
	ctx   context.Context
}

func (s *QueueProcessingSuite) SetupSuite() {
	if os.Getenv("RUN_INTEGRATION") != "true" {
		s.T().Skip("Skipping integration tests (set RUN_INTEGRATION=true)")
	}

	s.ctx = context.Background()

	host := getEnv("REDIS_HOST", "localhost")
	port := getEnvInt("REDIS_PORT", 6379)
	password := getEnv("REDIS_PASSWORD", "")

	log, _ := logger.New("debug", "json")
	m := metrics.New()

	var err error
	s.cache, err = cache.New(&config.RedisConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       1, // Use DB 1 for tests
		PoolSize: 10,
	}, log, m)
	require.NoError(s.T(), err)

	s.qm = queue.NewQueueManager(s.cache, log, m)
}

func (s *QueueProcessingSuite) TearDownSuite() {
	if s.cache != nil {
		s.cache.Close()
	}
}

func (s *QueueProcessingSuite) SetupTest() {
	// Clear all queues before each test
	err := s.qm.Clear(s.ctx)
	require.NoError(s.T(), err)
}

// =============================================================================
// Queue Processing Tests
// =============================================================================

func (s *QueueProcessingSuite) TestQueueProcessing_EnqueueDequeue() {
	matchID := uuid.New()
	tournamentID := uuid.New()
	program1ID := uuid.New()
	program2ID := uuid.New()

	match := &domain.Match{
		ID:           matchID,
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     "prisoners_dilemma",
		Status:       domain.MatchPending,
		Priority:     domain.PriorityMedium,
	}

	// Enqueue the match
	err := s.qm.Enqueue(s.ctx, match)
	require.NoError(s.T(), err)

	// Dequeue the match
	dequeued, err := s.qm.Dequeue(s.ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), dequeued)

	// Verify match data is preserved
	assert.Equal(s.T(), matchID, dequeued.ID)
	assert.Equal(s.T(), tournamentID, dequeued.TournamentID)
	assert.Equal(s.T(), program1ID, dequeued.Program1ID)
	assert.Equal(s.T(), program2ID, dequeued.Program2ID)
	assert.Equal(s.T(), "prisoners_dilemma", dequeued.GameType)
	assert.Equal(s.T(), domain.MatchPending, dequeued.Status)
	assert.Equal(s.T(), domain.PriorityMedium, dequeued.Priority)
}

func (s *QueueProcessingSuite) TestQueueProcessing_PriorityOrdering() {
	// Enqueue matches with different priorities in mixed order
	lowMatch := &domain.Match{
		ID:       uuid.New(),
		GameType: "low_priority_game",
		Status:   domain.MatchPending,
		Priority: domain.PriorityLow,
	}
	mediumMatch := &domain.Match{
		ID:       uuid.New(),
		GameType: "medium_priority_game",
		Status:   domain.MatchPending,
		Priority: domain.PriorityMedium,
	}
	highMatch := &domain.Match{
		ID:       uuid.New(),
		GameType: "high_priority_game",
		Status:   domain.MatchPending,
		Priority: domain.PriorityHigh,
	}

	// Enqueue in order: low, medium, high
	err := s.qm.Enqueue(s.ctx, lowMatch)
	require.NoError(s.T(), err)
	err = s.qm.Enqueue(s.ctx, mediumMatch)
	require.NoError(s.T(), err)
	err = s.qm.Enqueue(s.ctx, highMatch)
	require.NoError(s.T(), err)

	// Dequeue should return high priority first
	first, err := s.qm.Dequeue(s.ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), first)
	assert.Equal(s.T(), highMatch.ID, first.ID, "first dequeued match should be high priority")
	assert.Equal(s.T(), domain.PriorityHigh, first.Priority)

	// Then medium
	second, err := s.qm.Dequeue(s.ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), second)
	assert.Equal(s.T(), mediumMatch.ID, second.ID, "second dequeued match should be medium priority")
	assert.Equal(s.T(), domain.PriorityMedium, second.Priority)

	// Then low
	third, err := s.qm.Dequeue(s.ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), third)
	assert.Equal(s.T(), lowMatch.ID, third.ID, "third dequeued match should be low priority")
	assert.Equal(s.T(), domain.PriorityLow, third.Priority)
}

func (s *QueueProcessingSuite) TestQueueProcessing_QueueSize() {
	// Enqueue multiple matches across different priorities
	highMatches := 3
	mediumMatches := 5
	lowMatches := 2

	for i := 0; i < highMatches; i++ {
		err := s.qm.Enqueue(s.ctx, &domain.Match{
			ID:       uuid.New(),
			GameType: "size_test",
			Status:   domain.MatchPending,
			Priority: domain.PriorityHigh,
		})
		require.NoError(s.T(), err)
	}

	for i := 0; i < mediumMatches; i++ {
		err := s.qm.Enqueue(s.ctx, &domain.Match{
			ID:       uuid.New(),
			GameType: "size_test",
			Status:   domain.MatchPending,
			Priority: domain.PriorityMedium,
		})
		require.NoError(s.T(), err)
	}

	for i := 0; i < lowMatches; i++ {
		err := s.qm.Enqueue(s.ctx, &domain.Match{
			ID:       uuid.New(),
			GameType: "size_test",
			Status:   domain.MatchPending,
			Priority: domain.PriorityLow,
		})
		require.NoError(s.T(), err)
	}

	// Verify individual queue sizes
	highSize, err := s.qm.GetQueueSize(s.ctx, domain.PriorityHigh)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(highMatches), highSize)

	mediumSize, err := s.qm.GetQueueSize(s.ctx, domain.PriorityMedium)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(mediumMatches), mediumSize)

	lowSize, err := s.qm.GetQueueSize(s.ctx, domain.PriorityLow)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(lowMatches), lowSize)

	// Verify total queue size
	totalSize, err := s.qm.GetTotalQueueSize(s.ctx)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(highMatches+mediumMatches+lowMatches), totalSize)

	// Dequeue one and verify size decreases
	_, err = s.qm.Dequeue(s.ctx)
	require.NoError(s.T(), err)

	highSizeAfter, err := s.qm.GetQueueSize(s.ctx, domain.PriorityHigh)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(highMatches-1), highSizeAfter)
}

func (s *QueueProcessingSuite) TestQueueProcessing_EmptyDequeue() {
	// Dequeue from empty queue should return nil (BRPop with 1s timeout)
	start := time.Now()
	match, err := s.qm.Dequeue(s.ctx)
	elapsed := time.Since(start)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), match, "dequeue from empty queue should return nil")
	// BRPop blocks for 1 second when the queue is empty
	assert.GreaterOrEqual(s.T(), elapsed, 900*time.Millisecond,
		"dequeue should block for approximately 1 second on empty queue")
}

func (s *QueueProcessingSuite) TestQueueProcessing_ConcurrentEnqueueDequeue() {
	const numProducers = 5
	const matchesPerProducer = 10
	totalMatches := numProducers * matchesPerProducer

	// Track enqueued match IDs
	enqueuedIDs := make([]uuid.UUID, totalMatches)
	for i := 0; i < totalMatches; i++ {
		enqueuedIDs[i] = uuid.New()
	}

	var wg sync.WaitGroup

	// Start producers
	wg.Add(numProducers)
	for p := 0; p < numProducers; p++ {
		go func(producerIdx int) {
			defer wg.Done()
			for m := 0; m < matchesPerProducer; m++ {
				idx := producerIdx*matchesPerProducer + m
				match := &domain.Match{
					ID:       enqueuedIDs[idx],
					GameType: "concurrent_test",
					Status:   domain.MatchPending,
					Priority: domain.PriorityMedium,
				}
				err := s.qm.Enqueue(s.ctx, match)
				assert.NoError(s.T(), err)
			}
		}(p)
	}

	// Wait for all producers to finish
	wg.Wait()

	// Verify total queue size
	totalSize, err := s.qm.GetTotalQueueSize(s.ctx)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(totalMatches), totalSize)

	// Start consumers concurrently
	var dequeuedCount atomic.Int64
	dequeuedIDs := make(chan uuid.UUID, totalMatches)

	const numConsumers = 3
	wg.Add(numConsumers)
	for c := 0; c < numConsumers; c++ {
		go func() {
			defer wg.Done()
			for {
				match, err := s.qm.Dequeue(s.ctx)
				if err != nil {
					return
				}
				if match == nil {
					// Queue is empty (BRPop timed out)
					return
				}
				dequeuedIDs <- match.ID
				dequeuedCount.Add(1)
			}
		}()
	}

	wg.Wait()
	close(dequeuedIDs)

	// Collect all dequeued IDs
	dequeuedSet := make(map[uuid.UUID]bool)
	for id := range dequeuedIDs {
		dequeuedSet[id] = true
	}

	// All enqueued matches should have been dequeued exactly once
	assert.Equal(s.T(), int64(totalMatches), dequeuedCount.Load(),
		"all enqueued matches should be dequeued")

	for _, id := range enqueuedIDs {
		assert.True(s.T(), dequeuedSet[id],
			"enqueued match %s should have been dequeued", id)
	}
}

func TestQueueProcessingSuite(t *testing.T) {
	suite.Run(t, new(QueueProcessingSuite))
}
