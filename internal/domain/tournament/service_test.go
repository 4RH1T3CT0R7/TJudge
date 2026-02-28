package tournament

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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
	args := m.Called(ctx, key, ttl, fn)

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

// MockProgramRepository mocks ProgramRepository for ScheduleNewProgramMatches tests
type MockProgramRepository struct {
	mock.Mock
}

func (m *MockProgramRepository) GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error) {
	args := m.Called(ctx, tournamentID, gameID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Program), args.Error(1)
}

// setupTestRedisCache creates a test Redis cache backed by miniredis
func setupTestRedisCache(t *testing.T) *cache.Cache {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return cache.NewFromClient(client)
}

// newTestService creates a Service with all mocks and a real cache from miniredis.
// Returns the service and all mocks for assertion.
func newTestService(t *testing.T) (
	*Service,
	*MockTournamentRepository,
	*MockMatchRepository,
	*MockQueueManager,
	*MockBroadcaster,
	*MockDistributedLock,
	*MockGameRepository,
) {
	t.Helper()

	tournamentRepo := new(MockTournamentRepository)
	matchRepo := new(MockMatchRepository)
	queueManager := new(MockQueueManager)
	broadcaster := new(MockBroadcaster)
	distributedLock := new(MockDistributedLock)
	gameRepo := new(MockGameRepository)

	testCache := setupTestRedisCache(t)
	t.Cleanup(func() { testCache.Close() })

	tournamentCache := cache.NewTournamentCache(testCache)
	leaderboardCache := cache.NewLeaderboardCache(testCache)

	log, _ := logger.New("error", "json")

	service := NewService(
		tournamentRepo,
		matchRepo,
		queueManager,
		gameRepo,
		tournamentCache,
		leaderboardCache,
		broadcaster,
		distributedLock,
		log,
	)

	return service, tournamentRepo, matchRepo, queueManager, broadcaster, distributedLock, gameRepo
}

// -----------------------------------------------------------------------------
// TestService_Create
// -----------------------------------------------------------------------------

func TestService_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentRepo.On("Create", ctx, mock.AnythingOfType("*domain.Tournament")).Return(nil)

		req := &CreateRequest{
			Name:     "Test Tournament",
			GameType: "prisoners_dilemma",
		}

		result, err := service.Create(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Test Tournament", result.Name)
		assert.Equal(t, "prisoners_dilemma", result.GameType)
		assert.Equal(t, domain.TournamentPending, result.Status)
		assert.Equal(t, 1, result.MaxTeamSize)
		assert.NotEqual(t, uuid.Nil, result.ID)
		assert.NotEmpty(t, result.Code)

		tournamentRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*domain.Tournament"))
	})

	t.Run("validation_error", func(t *testing.T) {
		service, _, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		req := &CreateRequest{
			Name:     "",
			GameType: "chess",
		}

		result, err := service.Create(ctx, req)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.Code)
	})

	t.Run("repo_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentRepo.On("Create", ctx, mock.AnythingOfType("*domain.Tournament")).
			Return(fmt.Errorf("database connection lost"))

		req := &CreateRequest{
			Name:     "Test Tournament",
			GameType: "chess",
		}

		result, err := service.Create(ctx, req)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create tournament")
		assert.Contains(t, err.Error(), "database connection lost")
	})

	t.Run("default_max_team_size", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentRepo.On("Create", ctx, mock.AnythingOfType("*domain.Tournament")).Return(nil)

		req := &CreateRequest{
			Name:        "Test Tournament",
			GameType:    "chess",
			MaxTeamSize: 0,
		}

		result, err := service.Create(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, result.MaxTeamSize)

		// Also check negative MaxTeamSize
		req2 := &CreateRequest{
			Name:        "Test Tournament 2",
			GameType:    "chess",
			MaxTeamSize: -5,
		}

		result2, err := service.Create(ctx, req2)
		require.NoError(t, err)
		assert.Equal(t, 1, result2.MaxTeamSize)
	})
}

// -----------------------------------------------------------------------------
// TestService_GetByID
// -----------------------------------------------------------------------------

func TestService_GetByID(t *testing.T) {
	t.Run("cache_hit", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Cached Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		// Pre-populate cache by calling Create flow (Set is called internally)
		err := service.tournamentCache.Set(ctx, tournament)
		require.NoError(t, err)

		result, err := service.GetByID(ctx, tournamentID)
		require.NoError(t, err)
		assert.Equal(t, tournamentID, result.ID)
		assert.Equal(t, "Cached Tournament", result.Name)

		// Repo should NOT be called -- served from cache
		tournamentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("cache_miss_repo_hit", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "DB Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		result, err := service.GetByID(ctx, tournamentID)
		require.NoError(t, err)
		assert.Equal(t, tournamentID, result.ID)
		assert.Equal(t, "DB Tournament", result.Name)

		tournamentRepo.AssertCalled(t, "GetByID", ctx, tournamentID)

		// Verify it was cached -- a second call should not hit repo again
		tournamentRepo2 := new(MockTournamentRepository)
		// We cannot swap repos on the service, but we can verify cache is populated
		cached, err := service.tournamentCache.Get(ctx, tournamentID)
		require.NoError(t, err)
		assert.NotNil(t, cached)
		assert.Equal(t, "DB Tournament", cached.Name)
		_ = tournamentRepo2
	})

	t.Run("not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(nil, errors.ErrNotFound)

		result, err := service.GetByID(ctx, tournamentID)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	})
}

// -----------------------------------------------------------------------------
// TestService_List
// -----------------------------------------------------------------------------

func TestService_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournaments := []*domain.Tournament{
			{ID: uuid.New(), Name: "T1", GameType: "chess", Status: domain.TournamentPending},
			{ID: uuid.New(), Name: "T2", GameType: "chess", Status: domain.TournamentActive},
		}

		filter := domain.TournamentFilter{Limit: 10}
		tournamentRepo.On("List", ctx, filter).Return(tournaments, nil)

		result, err := service.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "T1", result[0].Name)
		assert.Equal(t, "T2", result[1].Name)
	})

	t.Run("default_limit", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournaments := []*domain.Tournament{}

		// When Limit=0, it should be clamped to 50
		expectedFilter := domain.TournamentFilter{Limit: 50}
		tournamentRepo.On("List", ctx, expectedFilter).Return(tournaments, nil)

		result, err := service.List(ctx, domain.TournamentFilter{Limit: 0})
		require.NoError(t, err)
		assert.Empty(t, result)

		tournamentRepo.AssertCalled(t, "List", ctx, expectedFilter)
	})

	t.Run("max_limit", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournaments := []*domain.Tournament{}

		// When Limit=101, it should be clamped to 100
		expectedFilter := domain.TournamentFilter{Limit: 100}
		tournamentRepo.On("List", ctx, expectedFilter).Return(tournaments, nil)

		result, err := service.List(ctx, domain.TournamentFilter{Limit: 101})
		require.NoError(t, err)
		assert.Empty(t, result)

		tournamentRepo.AssertCalled(t, "List", ctx, expectedFilter)
	})

	t.Run("repo_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		filter := domain.TournamentFilter{Limit: 10}
		tournamentRepo.On("List", ctx, filter).Return(([]*domain.Tournament)(nil), fmt.Errorf("db error"))

		result, err := service.List(ctx, filter)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

// -----------------------------------------------------------------------------
// TestService_Join
// -----------------------------------------------------------------------------

func TestService_Join(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		maxParticipants := 10
		tournament := &domain.Tournament{
			ID:              tournamentID,
			Name:            "Test Tournament",
			GameType:        "chess",
			Status:          domain.TournamentPending,
			MaxParticipants: &maxParticipants,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", ctx, tournamentID).Return(5, nil)
		tournamentRepo.On("AddParticipant", ctx, mock.AnythingOfType("*domain.TournamentParticipant")).Return(nil)

		programID := uuid.New()
		req := &JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    programID,
		}

		err := service.Join(ctx, req)
		require.NoError(t, err)

		tournamentRepo.AssertCalled(t, "AddParticipant", ctx, mock.MatchedBy(func(p *domain.TournamentParticipant) bool {
			return p.TournamentID == tournamentID &&
				p.ProgramID == programID &&
				p.Rating == 1500
		}))
	})

	t.Run("tournament_not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(nil, errors.ErrNotFound)

		req := &JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		err := service.Join(ctx, req)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("tournament_not_pending", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		req := &JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		err := service.Join(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, errors.ErrTournamentStarted, err)
	})

	t.Run("tournament_full", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		maxParticipants := 5
		tournament := &domain.Tournament{
			ID:              tournamentID,
			Name:            "Full Tournament",
			GameType:        "chess",
			Status:          domain.TournamentPending,
			MaxParticipants: &maxParticipants,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", ctx, tournamentID).Return(5, nil)

		req := &JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		err := service.Join(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, errors.ErrTournamentFull, err)
	})

	t.Run("no_max_participants", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:              tournamentID,
			Name:            "Unlimited Tournament",
			GameType:        "chess",
			Status:          domain.TournamentPending,
			MaxParticipants: nil, // no max
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		tournamentRepo.On("AddParticipant", ctx, mock.AnythingOfType("*domain.TournamentParticipant")).Return(nil)

		req := &JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		err := service.Join(ctx, req)
		require.NoError(t, err)

		// GetParticipantsCount should NOT be called when MaxParticipants is nil
		tournamentRepo.AssertNotCalled(t, "GetParticipantsCount", mock.Anything, mock.Anything)
	})

	t.Run("lock_error", func(t *testing.T) {
		service, _, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(errors.ErrConflict.WithMessage("lock already held"))

		req := &JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		err := service.Join(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lock already held")
	})
}

// -----------------------------------------------------------------------------
// TestService_Start
// -----------------------------------------------------------------------------

func TestService_Start(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, broadcaster, distributedLock, gameRepo := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		// Start calls tournamentRepo.GetByID directly (bypassing cache)
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", mock.Anything, tournamentID).Return(3, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tournament")).Return(nil)
		gameRepo.On("GetTournamentGames", mock.Anything, tournamentID).Return([]*domain.TournamentGame{}, nil)
		broadcaster.On("Broadcast", tournamentID, "tournament_update", mock.Anything).Return()

		err := service.Start(ctx, tournamentID)
		require.NoError(t, err)

		tournamentRepo.AssertCalled(t, "Update", mock.Anything, mock.MatchedBy(func(t *domain.Tournament) bool {
			return t.Status == domain.TournamentActive && t.StartTime != nil
		}))
		broadcaster.AssertCalled(t, "Broadcast", tournamentID, "tournament_update", mock.Anything)
	})

	t.Run("not_pending", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)

		err := service.Start(ctx, tournamentID)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "already started")
	})

	t.Run("too_few_participants", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", mock.Anything, tournamentID).Return(1, nil)

		err := service.Start(ctx, tournamentID)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Contains(t, appErr.Message, "at least 2 participants")
	})

	t.Run("repo_update_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", mock.Anything, tournamentID).Return(5, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tournament")).
			Return(fmt.Errorf("db write error"))

		err := service.Start(ctx, tournamentID)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Contains(t, appErr.Message, "failed to update tournament status")
	})

	t.Run("activates_first_game", func(t *testing.T) {
		service, tournamentRepo, _, _, broadcaster, distributedLock, gameRepo := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Multi-Game Tournament",
			GameType: "multi",
			Status:   domain.TournamentPending,
		}

		games := []*domain.TournamentGame{
			{TournamentID: tournamentID, GameID: gameID, IsActive: false},
			{TournamentID: tournamentID, GameID: uuid.New(), IsActive: false},
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", mock.Anything, tournamentID).Return(4, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tournament")).Return(nil)
		gameRepo.On("GetTournamentGames", mock.Anything, tournamentID).Return(games, nil)
		gameRepo.On("SetActiveGame", mock.Anything, tournamentID, gameID).Return(nil)
		broadcaster.On("Broadcast", tournamentID, "tournament_update", mock.Anything).Return()

		err := service.Start(ctx, tournamentID)
		require.NoError(t, err)

		gameRepo.AssertCalled(t, "GetTournamentGames", mock.Anything, tournamentID)
		gameRepo.AssertCalled(t, "SetActiveGame", mock.Anything, tournamentID, gameID)
	})
}

// -----------------------------------------------------------------------------
// TestService_Complete
// -----------------------------------------------------------------------------

func TestService_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, broadcaster, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		// Complete now calls tournamentRepo.GetByID directly (bypassing cache)
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tournament")).Return(nil)
		broadcaster.On("Broadcast", tournamentID, "tournament_update", mock.Anything).Return()

		err := service.Complete(ctx, tournamentID)
		require.NoError(t, err)

		tournamentRepo.AssertCalled(t, "Update", mock.Anything, mock.MatchedBy(func(t *domain.Tournament) bool {
			return t.Status == domain.TournamentCompleted && t.EndTime != nil
		}))
		broadcaster.AssertCalled(t, "Broadcast", tournamentID, "tournament_update", mock.Anything)
	})

	t.Run("not_active", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Pending Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, tournamentID).Return(tournament, nil)

		err := service.Complete(ctx, tournamentID)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "not active")
	})

	t.Run("lock_error", func(t *testing.T) {
		service, _, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(fmt.Errorf("redis connection lost"))

		err := service.Complete(ctx, tournamentID)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "could not complete tournament")
	})
}

// -----------------------------------------------------------------------------
// TestService_Delete
// -----------------------------------------------------------------------------

func TestService_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Pending Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		tournamentRepo.On("Delete", ctx, tournamentID).Return(nil)

		err := service.Delete(ctx, tournamentID)
		require.NoError(t, err)

		tournamentRepo.AssertCalled(t, "Delete", ctx, tournamentID)
	})

	t.Run("active_tournament", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		err := service.Delete(ctx, tournamentID)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "cannot delete active tournament")
	})

	t.Run("not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(nil, errors.ErrNotFound)

		err := service.Delete(ctx, tournamentID)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	})
}

// -----------------------------------------------------------------------------
// TestService_GetLeaderboard
// -----------------------------------------------------------------------------

func TestService_GetLeaderboard(t *testing.T) {
	t.Run("always_queries_db_for_complete_data", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		programID := uuid.New()

		// Pre-populate leaderboard cache (partial data: only ProgramID + Rating)
		err := service.leaderboardCache.UpdateRating(ctx, tournamentID, programID, 1800)
		require.NoError(t, err)

		// DB returns complete data including ProgramName, Wins, Losses, etc.
		entries := []*domain.LeaderboardEntry{
			{Rank: 1, ProgramID: programID, ProgramName: "bot-v1", Rating: 1800, Wins: 5, Losses: 2, TotalGames: 7},
		}
		tournamentRepo.On("GetLeaderboard", ctx, tournamentID, 10).Return(entries, nil)

		result, err := service.GetLeaderboard(ctx, tournamentID, 10)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, programID, result[0].ProgramID)
		assert.Equal(t, 1800, result[0].Rating)
		assert.Equal(t, "bot-v1", result[0].ProgramName)
		assert.Equal(t, 5, result[0].Wins)

		// DB is always called to get complete leaderboard data
		tournamentRepo.AssertCalled(t, "GetLeaderboard", ctx, tournamentID, 10)
	})

	t.Run("cache_miss_repo", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		programID := uuid.New()
		entries := []*domain.LeaderboardEntry{
			{Rank: 1, ProgramID: programID, Rating: 1700, Wins: 5, Losses: 2},
		}

		tournamentRepo.On("GetLeaderboard", ctx, tournamentID, 10).Return(entries, nil)

		result, err := service.GetLeaderboard(ctx, tournamentID, 10)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, 1700, result[0].Rating)

		tournamentRepo.AssertCalled(t, "GetLeaderboard", ctx, tournamentID, 10)
	})

	t.Run("repo_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		tournamentRepo.On("GetLeaderboard", ctx, tournamentID, 10).Return(nil, fmt.Errorf("db error"))

		result, err := service.GetLeaderboard(ctx, tournamentID, 10)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

// -----------------------------------------------------------------------------
// TestService_CreateMatch
// -----------------------------------------------------------------------------

func TestService_CreateMatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()

		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "prisoners_dilemma",
			Status:   domain.TournamentActive,
		}

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		matchRepo.On("Create", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)

		result, err := service.CreateMatch(ctx, tournamentID, program1ID, program2ID, domain.PriorityMedium)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tournamentID, result.TournamentID)
		assert.Equal(t, program1ID, result.Program1ID)
		assert.Equal(t, program2ID, result.Program2ID)
		assert.Equal(t, "prisoners_dilemma", result.GameType)
		assert.Equal(t, domain.MatchPending, result.Status)
		assert.Equal(t, domain.PriorityMedium, result.Priority)

		matchRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*domain.Match"))
		queueManager.AssertCalled(t, "Enqueue", ctx, mock.AnythingOfType("*domain.Match"))
	})

	t.Run("tournament_not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(nil, errors.ErrNotFound)

		result, err := service.CreateMatch(ctx, tournamentID, uuid.New(), uuid.New(), domain.PriorityMedium)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get tournament")
	})

	t.Run("validation_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		sameID := uuid.New()

		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		// Same program IDs should trigger validation error
		result, err := service.CreateMatch(ctx, tournamentID, sameID, sameID, domain.PriorityMedium)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.Code)
	})

	t.Run("queue_error_non_fatal", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()

		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		matchRepo.On("Create", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).
			Return(fmt.Errorf("queue unavailable"))

		result, err := service.CreateMatch(ctx, tournamentID, program1ID, program2ID, domain.PriorityMedium)
		// Match should still be created despite queue error
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, program1ID, result.Program1ID)
	})
}

// -----------------------------------------------------------------------------
// TestService_RunAllMatches
// -----------------------------------------------------------------------------

func TestService_RunAllMatches(t *testing.T) {
	t.Run("with_existing_pending", func(t *testing.T) {
		service, _, matchRepo, queueManager, _, distLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		pendingMatches := []*domain.Match{
			{ID: uuid.New(), TournamentID: tournamentID, Status: domain.MatchPending},
			{ID: uuid.New(), TournamentID: tournamentID, Status: domain.MatchPending},
		}

		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentID", ctx, tournamentID).Return(pendingMatches, nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)

		count, err := service.RunAllMatches(ctx, tournamentID)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		queueManager.AssertNumberOfCalls(t, "Enqueue", 2)
	})

	t.Run("generate_new_round", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, _, distLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		program1ID := uuid.New()
		program2ID := uuid.New()
		participants := []*domain.TournamentParticipant{
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: program1ID, Rating: 1500},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: program2ID, Rating: 1500},
		}

		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		// No pending matches -- trigger new round generation
		matchRepo.On("GetPendingByTournamentID", ctx, tournamentID).Return([]*domain.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetLatestParticipantsGroupedByGame", ctx, tournamentID).Return(map[string][]*domain.TournamentParticipant{
			"chess": participants,
		}, nil)
		matchRepo.On("GetNextRoundNumberByGame", ctx, tournamentID, "chess").Return(1, nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*domain.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)

		count, err := service.RunAllMatches(ctx, tournamentID)
		require.NoError(t, err)
		// 2 participants, each plays against the other in both directions = 2 matches
		assert.Equal(t, 2, count)

		matchRepo.AssertCalled(t, "CreateBatch", ctx, mock.MatchedBy(func(matches []*domain.Match) bool {
			return len(matches) == 2
		}))
	})

	t.Run("not_active", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, _, distLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Pending Tournament",
			GameType: "chess",
			Status:   domain.TournamentPending,
		}

		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentID", ctx, tournamentID).Return([]*domain.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		count, err := service.RunAllMatches(ctx, tournamentID)
		assert.Error(t, err)
		assert.Equal(t, 0, count)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "not active")
	})

	t.Run("no_participants", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, _, distLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentID", ctx, tournamentID).Return([]*domain.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetLatestParticipantsGroupedByGame", ctx, tournamentID).Return(map[string][]*domain.TournamentParticipant{}, nil)

		count, err := service.RunAllMatches(ctx, tournamentID)
		assert.Error(t, err)
		assert.Equal(t, 0, count)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.Code)
		assert.Contains(t, appErr.Message, "at least 2 participants")
	})
}

// -----------------------------------------------------------------------------
// TestService_RunGameMatches
// -----------------------------------------------------------------------------

func TestService_RunGameMatches(t *testing.T) {
	t.Run("with_existing_pending", func(t *testing.T) {
		service, _, matchRepo, queueManager, _, distLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameType := "prisoners_dilemma"

		pendingMatches := []*domain.Match{
			{ID: uuid.New(), TournamentID: tournamentID, GameType: gameType, Status: domain.MatchPending},
			{ID: uuid.New(), TournamentID: tournamentID, GameType: gameType, Status: domain.MatchPending},
			{ID: uuid.New(), TournamentID: tournamentID, GameType: gameType, Status: domain.MatchPending},
		}

		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentAndGame", ctx, tournamentID, gameType).Return(pendingMatches, nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)

		count, err := service.RunGameMatches(ctx, tournamentID, gameType)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		queueManager.AssertNumberOfCalls(t, "Enqueue", 3)
	})

	t.Run("generate_new_round", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, _, distLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameType := "prisoners_dilemma"
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: gameType,
			Status:   domain.TournamentActive,
		}

		program1ID := uuid.New()
		program2ID := uuid.New()
		program3ID := uuid.New()
		participants := []*domain.TournamentParticipant{
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: program1ID},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: program2ID},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: program3ID},
		}

		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentAndGame", ctx, tournamentID, gameType).Return([]*domain.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)
		tournamentRepo.On("GetLatestParticipantsByGame", ctx, tournamentID, gameType).Return(participants, nil)
		matchRepo.On("GetNextRoundNumberByGame", ctx, tournamentID, gameType).Return(2, nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*domain.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)

		count, err := service.RunGameMatches(ctx, tournamentID, gameType)
		require.NoError(t, err)
		// 3 participants: AB, BA, AC, CA, BC, CB = 6 matches
		assert.Equal(t, 6, count)
	})

	t.Run("not_active", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, _, distLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameType := "chess"
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Completed Tournament",
			GameType: gameType,
			Status:   domain.TournamentCompleted,
		}

		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentAndGame", ctx, tournamentID, gameType).Return([]*domain.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		count, err := service.RunGameMatches(ctx, tournamentID, gameType)
		assert.Error(t, err)
		assert.Equal(t, 0, count)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
	})
}

// -----------------------------------------------------------------------------
// TestService_RetryFailedMatches
// -----------------------------------------------------------------------------

func TestService_RetryFailedMatches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, _, matchRepo, queueManager, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		matchRepo.On("ResetFailedMatches", ctx, tournamentID).Return(3, nil)

		pendingMatches := []*domain.Match{
			{ID: uuid.New(), TournamentID: tournamentID, Status: domain.MatchPending},
			{ID: uuid.New(), TournamentID: tournamentID, Status: domain.MatchPending},
			{ID: uuid.New(), TournamentID: tournamentID, Status: domain.MatchPending},
		}

		matchRepo.On("GetPendingByTournamentID", ctx, tournamentID).Return(pendingMatches, nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)

		count, err := service.RetryFailedMatches(ctx, tournamentID)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		queueManager.AssertNumberOfCalls(t, "Enqueue", 3)
	})

	t.Run("no_failed_matches", func(t *testing.T) {
		service, _, matchRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()

		matchRepo.On("ResetFailedMatches", ctx, tournamentID).Return(0, nil)

		count, err := service.RetryFailedMatches(ctx, tournamentID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		// GetPendingByTournamentID should NOT be called when resetCount is 0
		matchRepo.AssertNotCalled(t, "GetPendingByTournamentID", mock.Anything, mock.Anything)
	})
}

// -----------------------------------------------------------------------------
// TestService_generateRoundRobinMatches
// -----------------------------------------------------------------------------

func TestService_generateRoundRobinMatches(t *testing.T) {
	t.Run("two_participants", func(t *testing.T) {
		service, _, _, _, _, _, _ := newTestService(t)

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		p1 := uuid.New()
		p2 := uuid.New()
		participants := []*domain.TournamentParticipant{
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: p1},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: p2},
		}

		matches, err := service.generateRoundRobinMatches(tournament, participants, 1)
		require.NoError(t, err)
		// 2 participants: AB, BA = 2 matches
		assert.Len(t, matches, 2)

		// Verify both directions exist
		hasAB := false
		hasBA := false
		for _, m := range matches {
			if m.Program1ID == p1 && m.Program2ID == p2 {
				hasAB = true
			}
			if m.Program1ID == p2 && m.Program2ID == p1 {
				hasBA = true
			}
		}
		assert.True(t, hasAB, "expected match p1 vs p2")
		assert.True(t, hasBA, "expected match p2 vs p1")
	})

	t.Run("three_participants", func(t *testing.T) {
		service, _, _, _, _, _, _ := newTestService(t)

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		participants := []*domain.TournamentParticipant{
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: uuid.New()},
		}

		matches, err := service.generateRoundRobinMatches(tournament, participants, 1)
		require.NoError(t, err)
		// 3 participants: AB, AC, BA, BC, CA, CB = 6 matches (n*(n-1))
		assert.Len(t, matches, 6)

		// Verify all matches have correct fields
		for _, m := range matches {
			assert.Equal(t, tournamentID, m.TournamentID)
			assert.Equal(t, "chess", m.GameType)
			assert.Equal(t, domain.MatchPending, m.Status)
			assert.Equal(t, domain.PriorityMedium, m.Priority)
			assert.Equal(t, 1, m.RoundNumber)
			assert.NotEqual(t, m.Program1ID, m.Program2ID)
		}
	})

	t.Run("empty_participants", func(t *testing.T) {
		service, _, _, _, _, _, _ := newTestService(t)

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		var participants []*domain.TournamentParticipant

		matches, err := service.generateRoundRobinMatches(tournament, participants, 1)
		require.NoError(t, err)
		assert.Len(t, matches, 0)
	})
}

// -----------------------------------------------------------------------------
// TestService_GetMatches
// -----------------------------------------------------------------------------

func TestService_GetMatches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, _, matchRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		expected := []*domain.Match{
			{ID: uuid.New(), TournamentID: tournamentID, Status: domain.MatchCompleted},
			{ID: uuid.New(), TournamentID: tournamentID, Status: domain.MatchPending},
		}

		matchRepo.On("GetByTournamentID", ctx, tournamentID, 10, 0).Return(expected, nil)

		result, err := service.GetMatches(ctx, tournamentID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, expected[0].ID, result[0].ID)
		assert.Equal(t, expected[1].ID, result[1].ID)
	})
}

// -----------------------------------------------------------------------------
// TestService_GetMatchesByRounds
// -----------------------------------------------------------------------------

func TestService_GetMatchesByRounds(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, _, matchRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		expected := []*domain.MatchRound{
			{RoundNumber: 1, GameType: "chess", TotalMatches: 6, CompletedCount: 6},
			{RoundNumber: 2, GameType: "chess", TotalMatches: 6, CompletedCount: 3, PendingCount: 3},
		}

		matchRepo.On("GetMatchesByRounds", ctx, tournamentID).Return(expected, nil)

		result, err := service.GetMatchesByRounds(ctx, tournamentID)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, 1, result[0].RoundNumber)
		assert.Equal(t, 2, result[1].RoundNumber)
	})
}

// -----------------------------------------------------------------------------
// TestService_ScheduleNewProgramMatches
// -----------------------------------------------------------------------------

func TestService_ScheduleNewProgramMatches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, broadcaster, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameID := uuid.New()
		newProgramID := uuid.New()
		teamID := uuid.New()
		otherTeamID := uuid.New()

		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		otherProgram := &domain.Program{
			ID:       uuid.New(),
			Name:     "Other Bot",
			GameType: "chess",
			TeamID:   &otherTeamID,
		}

		// Include the new program itself, which should be skipped
		programs := []*domain.Program{
			{ID: newProgramID, Name: "New Bot", GameType: "chess", TeamID: &teamID},
			otherProgram,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		programRepo := new(MockProgramRepository)
		programRepo.On("GetByTournamentAndGame", ctx, tournamentID, gameID).Return(programs, nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*domain.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)
		broadcaster.On("Broadcast", tournamentID, "matches_created", mock.Anything).Return()

		req := &ScheduleNewProgramMatchesRequest{
			TournamentID: tournamentID,
			GameID:       gameID,
			NewProgramID: newProgramID,
			TeamID:       teamID,
		}

		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		require.NoError(t, err)

		matchRepo.AssertCalled(t, "CreateBatch", ctx, mock.MatchedBy(func(matches []*domain.Match) bool {
			// Should create 2 bidirectional matches: newProgram vs otherProgram and otherProgram vs newProgram
			if len(matches) != 2 {
				return false
			}
			hasForward := matches[0].Program1ID == newProgramID && matches[0].Program2ID == otherProgram.ID
			hasReverse := matches[1].Program1ID == otherProgram.ID && matches[1].Program2ID == newProgramID
			return hasForward && hasReverse
		}))
		broadcaster.AssertCalled(t, "Broadcast", tournamentID, "matches_created", mock.Anything)
	})

	t.Run("completed_tournament", func(t *testing.T) {
		service, tournamentRepo, _, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameID := uuid.New()

		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Completed Tournament",
			GameType: "chess",
			Status:   domain.TournamentCompleted,
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		programRepo := new(MockProgramRepository)

		req := &ScheduleNewProgramMatchesRequest{
			TournamentID: tournamentID,
			GameID:       gameID,
			NewProgramID: uuid.New(),
			TeamID:       uuid.New(),
		}

		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		assert.Error(t, err)
		assert.True(t, errors.IsAppError(err))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "cannot schedule matches for completed tournament")
	})

	t.Run("no_other_programs", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameID := uuid.New()
		newProgramID := uuid.New()
		teamID := uuid.New()

		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		// Only the new program exists, no opponents
		programs := []*domain.Program{
			{ID: newProgramID, Name: "New Bot", GameType: "chess", TeamID: &teamID},
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		programRepo := new(MockProgramRepository)
		programRepo.On("GetByTournamentAndGame", ctx, tournamentID, gameID).Return(programs, nil)

		req := &ScheduleNewProgramMatchesRequest{
			TournamentID: tournamentID,
			GameID:       gameID,
			NewProgramID: newProgramID,
			TeamID:       teamID,
		}

		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		require.NoError(t, err)

		// No matches should be created
		matchRepo.AssertNotCalled(t, "CreateBatch", mock.Anything, mock.Anything)
	})
}

// -----------------------------------------------------------------------------
// TestService_GetCrossGameLeaderboard
// -----------------------------------------------------------------------------

func TestService_GetCrossGameLeaderboard(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		entries := []*domain.CrossGameLeaderboardEntry{
			{Rank: 1, TeamName: "Team Alpha", TotalRating: 3000, TotalWins: 10},
			{Rank: 2, TeamName: "Team Beta", TotalRating: 2500, TotalWins: 7},
		}

		tournamentRepo.On("GetCrossGameLeaderboard", ctx, tournamentID).Return(entries, nil)

		result, err := service.GetCrossGameLeaderboard(ctx, tournamentID)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "Team Alpha", result[0].TeamName)
		assert.Equal(t, 3000, result[0].TotalRating)
		assert.Equal(t, "Team Beta", result[1].TeamName)
	})
}

// -----------------------------------------------------------------------------
// Concurrent tests (using real distributed lock via miniredis)
// -----------------------------------------------------------------------------

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

		// Mock participants count - uses atomic counter via Run callback
		getCountCall := tournamentRepo.On("GetParticipantsCount", mock.Anything, tournamentID)
		getCountCall.Run(func(args mock.Arguments) {
			count := int(atomic.LoadInt64(&participantCount))
			getCountCall.ReturnArguments = mock.Arguments{count, nil}
		}).Return(0, nil) // default return, overridden by Run

		// Mock add participant - increments counter via Run callback
		addParticipantCall := tournamentRepo.On("AddParticipant", mock.Anything, mock.AnythingOfType("*domain.TournamentParticipant"))
		addParticipantCall.Run(func(args mock.Arguments) {
			count := atomic.AddInt64(&participantCount, 1)
			if count > int64(maxParticipants) {
				atomic.AddInt64(&participantCount, -1)
				addParticipantCall.ReturnArguments = mock.Arguments{errors.ErrTournamentFull}
			} else {
				addParticipantCall.ReturnArguments = mock.Arguments{nil}
			}
		}).Return(nil) // default return, overridden by Run

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

		// No more than maxParticipants should succeed (lock contention may cause some to fail)
		assert.LessOrEqual(t, successCount, int64(maxParticipants), "should not exceed max participants")
		assert.Equal(t, successCount, participantCount, "participant count should match successful joins")
		assert.Equal(t, int64(concurrentJoins), successCount+errorCount, "all joins should complete")
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

		// Mock GetByID - changes status after first start via Run callback
		getByIDCall := tournamentRepo.On("GetByID", mock.Anything, tournamentID)
		getByIDCall.Run(func(args mock.Arguments) {
			if atomic.LoadInt64(&startCount) > 0 {
				// Return already started tournament
				copy := *tournament
				copy.Status = domain.TournamentActive
				getByIDCall.ReturnArguments = mock.Arguments{&copy, nil}
			} else {
				getByIDCall.ReturnArguments = mock.Arguments{tournament, nil}
			}
		}).Return(tournament, nil)

		tournamentRepo.On("GetParticipants", mock.Anything, tournamentID).Return(participants, nil)
		tournamentRepo.On("GetParticipantsCount", mock.Anything, tournamentID).Return(3, nil)

		matchRepo.On("CreateBatch", mock.Anything, mock.AnythingOfType("[]*domain.Match")).Return(nil)

		queueManager.On("Enqueue", mock.Anything, mock.AnythingOfType("*domain.Match")).Return(nil)

		// Mock Update - increments counter via Run callback
		updateCall := tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tournament"))
		updateCall.Run(func(args mock.Arguments) {
			atomic.AddInt64(&startCount, 1)
			updateCall.ReturnArguments = mock.Arguments{nil}
		}).Return(nil)

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

// -----------------------------------------------------------------------------
// TestService_generateRoundRobinMatches_EdgeCases
// -----------------------------------------------------------------------------

func TestService_generateRoundRobinMatches_EdgeCases(t *testing.T) {
	t.Run("one_participant_generates_zero_matches", func(t *testing.T) {
		service, _, _, _, _, _, _ := newTestService(t)

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Solo Tournament",
			GameType: "prisoners_dilemma",
			Status:   domain.TournamentActive,
		}

		participants := []*domain.TournamentParticipant{
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: uuid.New(), Rating: 1500},
		}

		matches, err := service.generateRoundRobinMatches(tournament, participants, 1)
		require.NoError(t, err)
		assert.Len(t, matches, 0, "1 participant cannot play against anyone, expected 0 matches")
	})

	t.Run("four_participants_generates_12_matches", func(t *testing.T) {
		service, _, _, _, _, _, _ := newTestService(t)

		tournamentID := uuid.New()
		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Four Player Tournament",
			GameType: "prisoners_dilemma",
			Status:   domain.TournamentActive,
		}

		p1 := uuid.New()
		p2 := uuid.New()
		p3 := uuid.New()
		p4 := uuid.New()
		participants := []*domain.TournamentParticipant{
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: p1, Rating: 1500},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: p2, Rating: 1500},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: p3, Rating: 1500},
			{ID: uuid.New(), TournamentID: tournamentID, ProgramID: p4, Rating: 1500},
		}

		matches, err := service.generateRoundRobinMatches(tournament, participants, 1)
		require.NoError(t, err)
		// Bidirectional round-robin: n*(n-1) = 4*3 = 12 matches
		// Each pair plays in both directions (AB and BA)
		assert.Len(t, matches, 12, "4 participants should generate 4*3=12 bidirectional matches")

		// Build a set of all (Program1ID, Program2ID) pairs to verify uniqueness
		type pair struct{ a, b uuid.UUID }
		seen := make(map[pair]bool)
		programIDs := []uuid.UUID{p1, p2, p3, p4}

		for _, m := range matches {
			assert.Equal(t, tournamentID, m.TournamentID)
			assert.Equal(t, "prisoners_dilemma", m.GameType)
			assert.Equal(t, domain.MatchPending, m.Status)
			assert.Equal(t, 1, m.RoundNumber)
			assert.NotEqual(t, m.Program1ID, m.Program2ID, "no self-play allowed")

			p := pair{m.Program1ID, m.Program2ID}
			assert.False(t, seen[p], "duplicate match pair detected: %v vs %v", m.Program1ID, m.Program2ID)
			seen[p] = true
		}

		// Verify every ordered pair exists (both directions for each unordered pair)
		for i := 0; i < len(programIDs); i++ {
			for j := 0; j < len(programIDs); j++ {
				if i == j {
					continue
				}
				p := pair{programIDs[i], programIDs[j]}
				assert.True(t, seen[p], "missing match: %v vs %v", programIDs[i], programIDs[j])
			}
		}
	})
}

// -----------------------------------------------------------------------------
// TestService_ScheduleNewProgramMatches_SkipsSameTeam
// -----------------------------------------------------------------------------

func TestService_ScheduleNewProgramMatches_SkipsSameTeam(t *testing.T) {
	t.Run("skips_programs_from_same_team", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, broadcaster, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		tournamentID := uuid.New()
		gameID := uuid.New()
		teamID := uuid.New()
		otherTeamID := uuid.New()
		newProgramID := uuid.New()
		sameTeamProgramID := uuid.New()
		otherTeamProgramID := uuid.New()

		tournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Active Tournament",
			GameType: "prisoners_dilemma",
			Status:   domain.TournamentActive,
		}

		// Three programs: one is the new program, one from the same team, one from a different team.
		// Only the program from the different team should produce matches.
		programs := []*domain.Program{
			{ID: newProgramID, Name: "New Bot", GameType: "prisoners_dilemma", TeamID: &teamID},
			{ID: sameTeamProgramID, Name: "Teammate Bot", GameType: "prisoners_dilemma", TeamID: &teamID},
			{ID: otherTeamProgramID, Name: "Opponent Bot", GameType: "prisoners_dilemma", TeamID: &otherTeamID},
		}

		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).
			Return(nil)
		tournamentRepo.On("GetByID", ctx, tournamentID).Return(tournament, nil)

		programRepo := new(MockProgramRepository)
		programRepo.On("GetByTournamentAndGame", ctx, tournamentID, gameID).Return(programs, nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*domain.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*domain.Match")).Return(nil)
		broadcaster.On("Broadcast", tournamentID, "matches_created", mock.Anything).Return()

		req := &ScheduleNewProgramMatchesRequest{
			TournamentID: tournamentID,
			GameID:       gameID,
			NewProgramID: newProgramID,
			TeamID:       teamID,
		}

		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		require.NoError(t, err)

		// Exactly 2 matches: newProgram vs otherTeamProgram (forward and reverse)
		// The same-team program must be excluded entirely.
		matchRepo.AssertCalled(t, "CreateBatch", ctx, mock.MatchedBy(func(matches []*domain.Match) bool {
			if len(matches) != 2 {
				return false
			}

			hasForward := false
			hasReverse := false
			for _, m := range matches {
				// No match should involve the same-team program
				assert.NotEqual(t, sameTeamProgramID, m.Program1ID, "same-team program must not appear as Program1")
				assert.NotEqual(t, sameTeamProgramID, m.Program2ID, "same-team program must not appear as Program2")

				if m.Program1ID == newProgramID && m.Program2ID == otherTeamProgramID {
					hasForward = true
				}
				if m.Program1ID == otherTeamProgramID && m.Program2ID == newProgramID {
					hasReverse = true
				}
			}
			return hasForward && hasReverse
		}))

		// Verify enqueue was called exactly 2 times (one per match)
		queueManager.AssertNumberOfCalls(t, "Enqueue", 2)
		broadcaster.AssertCalled(t, "Broadcast", tournamentID, "matches_created", mock.Anything)
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
