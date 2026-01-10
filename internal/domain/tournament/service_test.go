package tournament

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations
type MockTournamentRepository struct {
	mock.Mock
}

func (m *MockTournamentRepository) Create(ctx context.Context, tournament *domain.Tournament) error {
	args := m.Called(ctx, tournament)
	return args.Error(0)
}

func (m *MockTournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tournament), args.Error(1)
}

func (m *MockTournamentRepository) List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*domain.Tournament), args.Error(1)
}

func (m *MockTournamentRepository) Update(ctx context.Context, tournament *domain.Tournament) error {
	args := m.Called(ctx, tournament)
	return args.Error(0)
}

func (m *MockTournamentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TournamentStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTournamentRepository) GetParticipantsCount(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockTournamentRepository) GetParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TournamentParticipant), args.Error(1)
}

func (m *MockTournamentRepository) AddParticipant(ctx context.Context, participant *domain.TournamentParticipant) error {
	args := m.Called(ctx, participant)
	return args.Error(0)
}

func (m *MockTournamentRepository) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	args := m.Called(ctx, tournamentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LeaderboardEntry), args.Error(1)
}

func (m *MockTournamentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTournamentRepository) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CrossGameLeaderboardEntry), args.Error(1)
}

func (m *MockTournamentRepository) GetLatestParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TournamentParticipant), args.Error(1)
}

func (m *MockTournamentRepository) GetLatestParticipantsGroupedByGame(ctx context.Context, tournamentID uuid.UUID) (map[string][]*domain.TournamentParticipant, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string][]*domain.TournamentParticipant), args.Error(1)
}

func (m *MockTournamentRepository) GetLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.TournamentParticipant, error) {
	args := m.Called(ctx, tournamentID, gameType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TournamentParticipant), args.Error(1)
}

type MockMatchRepository struct {
	mock.Mock
}

func (m *MockMatchRepository) Create(ctx context.Context, match *domain.Match) error {
	args := m.Called(ctx, match)
	return args.Error(0)
}

func (m *MockMatchRepository) CreateBatch(ctx context.Context, matches []*domain.Match) error {
	args := m.Called(ctx, matches)
	return args.Error(0)
}

func (m *MockMatchRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error) {
	args := m.Called(ctx, tournamentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

func (m *MockMatchRepository) GetPendingByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Match, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

func (m *MockMatchRepository) GetFailedByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Match, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

func (m *MockMatchRepository) ResetFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int64, error) {
	args := m.Called(ctx, tournamentID)
	return int64(args.Int(0)), args.Error(1)
}

func (m *MockMatchRepository) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MatchRound), args.Error(1)
}

func (m *MockMatchRepository) GetNextRoundNumber(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockMatchRepository) GetNextRoundNumberByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	args := m.Called(ctx, tournamentID, gameType)
	return args.Int(0), args.Error(1)
}

func (m *MockMatchRepository) GetPendingByTournamentAndGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.Match, error) {
	args := m.Called(ctx, tournamentID, gameType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

type MockQueueManager struct {
	mock.Mock
}

func (m *MockQueueManager) Enqueue(ctx context.Context, match *domain.Match) error {
	args := m.Called(ctx, match)
	return args.Error(0)
}

type MockBroadcaster struct {
	mock.Mock
}

func (m *MockBroadcaster) Broadcast(tournamentID uuid.UUID, messageType string, payload interface{}) {
	m.Called(tournamentID, messageType, payload)
}

type MockDistributedLock struct {
	mock.Mock
}

func (m *MockDistributedLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error {
	args := m.Called(ctx, key, ttl, mock.AnythingOfType("func(context.Context) error"))

	// Actually call the function to simulate real behavior
	if args.Error(0) == nil {
		return fn(ctx)
	}
	return args.Error(0)
}

type MockGameRepository struct {
	mock.Mock
}

func (m *MockGameRepository) GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGame, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TournamentGame), args.Error(1)
}

func (m *MockGameRepository) SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	args := m.Called(ctx, tournamentID, gameID)
	return args.Error(0)
}

// TestConcurrentJoin tests that concurrent join operations don't exceed max participants
func TestConcurrentJoin(t *testing.T) {
	t.Run("prevents exceeding max participants with distributed lock", func(t *testing.T) {
		tournamentRepo := new(MockTournamentRepository)
		matchRepo := new(MockMatchRepository)
		queueManager := new(MockQueueManager)
		broadcaster := new(MockBroadcaster)

		// Use real cache for integration test
		// For unit test, we'll simulate with a counter
		var participantCount int64
		maxParticipants := 10

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:              tournamentID,
			Name:            "Test Tournament",
			GameType:        "chess",
			Status:          domain.TournamentPending,
			MaxParticipants: &maxParticipants,
		}

		// Mock tournament retrieval
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)

		// Mock participants count - uses atomic counter
		tournamentRepo.On("GetParticipantsCount", mock.Anything, tournamentID).Return(
			func(ctx context.Context, id uuid.UUID) int {
				return int(atomic.LoadInt64(&participantCount))
			},
			func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
		)

		// Mock add participant - increments counter
		tournamentRepo.On("AddParticipant", mock.Anything, mock.AnythingOfType("*domain.TournamentParticipant")).Return(
			func(ctx context.Context, p *domain.TournamentParticipant) error {
				count := atomic.AddInt64(&participantCount, 1)
				if count > int64(maxParticipants) {
					atomic.AddInt64(&participantCount, -1)
					return errors.ErrTournamentFull
				}
				return nil
			},
		)

		// Create service with real distributed lock using test cache
		testCache := setupTestRedisCache(t)
		defer testCache.Close()

		tournamentCache := cache.NewTournamentCache(testCache)
		leaderboardCache := cache.NewLeaderboardCache(testCache)
		distributedLock := cache.NewDistributedLock(testCache)

		log, _ := logger.New("error", "json")

		service := NewService(
			tournamentRepo,
			matchRepo,
			queueManager,
			nil, // gameRepo not needed for join test
			tournamentCache,
			leaderboardCache,
			broadcaster,
			distributedLock,
			log,
		)

		// Try to join with more goroutines than max participants
		var wg sync.WaitGroup
		successCount := int64(0)
		errorCount := int64(0)
		concurrentJoins := 20

		for i := 0; i < concurrentJoins; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				req := &JoinRequest{
					TournamentID: tournamentID,
					ProgramID:    uuid.New(),
				}

				err := service.Join(context.Background(), req)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
					if err != errors.ErrTournamentFull {
						t.Logf("Unexpected error: %v", err)
					}
				}
			}(i)
		}

		wg.Wait()

		// Exactly maxParticipants should succeed
		assert.Equal(t, int64(maxParticipants), successCount, "expected exactly max participants to join")
		assert.Equal(t, int64(concurrentJoins-maxParticipants), errorCount, "expected remaining joins to fail")
		assert.Equal(t, int64(maxParticipants), participantCount, "participant count should equal max")
	})
}

// TestConcurrentStart tests that only one Start operation succeeds
func TestConcurrentStart(t *testing.T) {
	t.Run("prevents multiple concurrent starts with distributed lock", func(t *testing.T) {
		tournamentRepo := new(MockTournamentRepository)
		matchRepo := new(MockMatchRepository)
		queueManager := new(MockQueueManager)
		broadcaster := new(MockBroadcaster)

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		participants := []*domain.TournamentParticipant{
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: uuid.New()},
		}

		var startCount int64

		// Mock GetByID - changes status after first start
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(
			func(ctx context.Context, id uuid.UUID) *domain.Tournament {
				if atomic.LoadInt64(&startCount) > 0 {
					// Return already started tournament
					t := *tournament
					t.Status = domain.TournamentActive
					return &t
				}
				return tournament
			},
			func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
		)

		tournamentRepo.On("GetParticipants", mock.Anything, tournamentID).Return(participants, nil)

		matchRepo.On("CreateBatch", mock.Anything, mock.AnythingOfType("[]*domain.Match")).Return(nil)

		queueManager.On("Enqueue", mock.Anything, mock.AnythingOfType("*domain.Match")).Return(nil)

		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tournament")).Return(
			func(ctx context.Context, t *domain.Tournament) error {
				atomic.AddInt64(&startCount, 1)
				return nil
			},
		)

		broadcaster.On("Broadcast", tournamentID, "tournament_update", mock.Anything).Return()

		// Create service with real distributed lock
		testCache := setupTestRedisCache(t)
		defer testCache.Close()

		tournamentCache := cache.NewTournamentCache(testCache)
		leaderboardCache := cache.NewLeaderboardCache(testCache)
		distributedLock := cache.NewDistributedLock(testCache)

		log, _ := logger.New("error", "json")

		service := NewService(
			tournamentRepo,
			matchRepo,
			queueManager,
			nil, // gameRepo not needed for concurrent start test
			tournamentCache,
			leaderboardCache,
			broadcaster,
			distributedLock,
			log,
		)

		// Try to start tournament concurrently
		var wg sync.WaitGroup
		successCount := int64(0)
		errorCount := int64(0)
		concurrentStarts := 5

		for i := 0; i < concurrentStarts; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				err := service.Start(context.Background(), tournamentID)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
			}()
		}

		wg.Wait()

		// Only one Start should succeed
		assert.Equal(t, int64(1), successCount, "expected exactly one start to succeed")
		assert.Equal(t, int64(concurrentStarts-1), errorCount, "expected remaining starts to fail")
		assert.Equal(t, int64(1), startCount, "tournament should be started exactly once")
	})
}

// TestRaceConditionInJoin tests for race conditions without distributed lock
func TestRaceConditionInJoin(t *testing.T) {
	t.Run("detects race condition when lock fails", func(t *testing.T) {
		tournamentRepo := new(MockTournamentRepository)
		matchRepo := new(MockMatchRepository)
		queueManager := new(MockQueueManager)
		broadcaster := new(MockBroadcaster)
		distributedLock := new(MockDistributedLock)

		tournamentID := uuid.New()
		maxParticipants := 5
		tournament := &domain.Tournament{
			ID:              tournamentID,
			Name:            "Test Tournament",
			GameType:        "chess",
			Status:          domain.TournamentPending,
			MaxParticipants: &maxParticipants,
		}

		// Simulate lock failure
		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(errors.ErrConflict.WithMessage("lock already held"))

		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)

		testCache := setupTestRedisCache(t)
		defer testCache.Close()

		tournamentCache := cache.NewTournamentCache(testCache)
		leaderboardCache := cache.NewLeaderboardCache(testCache)

		log, _ := logger.New("error", "json")

		service := NewService(
			tournamentRepo,
			matchRepo,
			queueManager,
			nil, // gameRepo not needed for lock failure test
			tournamentCache,
			leaderboardCache,
			broadcaster,
			distributedLock,
			log,
		)

		req := &JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		err := service.Join(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lock already held")
	})
}

// setupTestRedisCache creates a test Redis cache
// For integration tests, use real Redis or testcontainers
func setupTestRedisCache(t *testing.T) *cache.Cache {
	// Skip if no Redis available
	t.Skip("Implement setupTestRedisCache with real Redis or testcontainers")
	return nil
}
