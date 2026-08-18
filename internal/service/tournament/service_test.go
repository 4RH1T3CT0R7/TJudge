package tournament

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// моки репозиториев/очереди/лока. рукописные на testify/mock, в одном файле с тестами

type MockTournamentRepository struct{ mock.Mock }

func (m *MockTournamentRepository) Create(ctx context.Context, t *models.Tournament) error {
	return m.Called(ctx, t).Error(0)
}
func (m *MockTournamentRepository) Update(ctx context.Context, t *models.Tournament) error {
	return m.Called(ctx, t).Error(0)
}
func (m *MockTournamentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TournamentStatus) error {
	return m.Called(ctx, id, status).Error(0)
}
func (m *MockTournamentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockTournamentRepository) AddParticipant(ctx context.Context, p *models.TournamentParticipant) error {
	return m.Called(ctx, p).Error(0)
}

func (m *MockTournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).(*models.Tournament)
	return v, args.Error(1)
}

func (m *MockTournamentRepository) List(ctx context.Context, filter models.TournamentFilter) ([]*models.Tournament, error) {
	args := m.Called(ctx, filter)
	v, _ := args.Get(0).([]*models.Tournament)
	return v, args.Error(1)
}

func (m *MockTournamentRepository) GetParticipantsCount(ctx context.Context, id uuid.UUID) (int, error) {
	args := m.Called(ctx, id)
	return args.Int(0), args.Error(1)
}

func (m *MockTournamentRepository) GetTeamsCount(ctx context.Context, id uuid.UUID) (int, error) {
	args := m.Called(ctx, id)
	return args.Int(0), args.Error(1)
}

func (m *MockTournamentRepository) GetParticipants(ctx context.Context, id uuid.UUID) ([]*models.TournamentParticipant, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*models.TournamentParticipant)
	return v, args.Error(1)
}

func (m *MockTournamentRepository) GetLatestParticipants(ctx context.Context, id uuid.UUID) ([]*models.TournamentParticipant, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*models.TournamentParticipant)
	return v, args.Error(1)
}

func (m *MockTournamentRepository) GetLatestParticipantsGroupedByGame(ctx context.Context, id uuid.UUID) (map[string][]*models.TournamentParticipant, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).(map[string][]*models.TournamentParticipant)
	return v, args.Error(1)
}

func (m *MockTournamentRepository) GetLatestParticipantsByGame(ctx context.Context, id uuid.UUID, gameType string) ([]*models.TournamentParticipant, error) {
	args := m.Called(ctx, id, gameType)
	v, _ := args.Get(0).([]*models.TournamentParticipant)
	return v, args.Error(1)
}

func (m *MockTournamentRepository) GetLeaderboard(ctx context.Context, id uuid.UUID, limit int) ([]*models.LeaderboardEntry, error) {
	args := m.Called(ctx, id, limit)
	v, _ := args.Get(0).([]*models.LeaderboardEntry)
	return v, args.Error(1)
}

func (m *MockTournamentRepository) GetCrossGameLeaderboard(ctx context.Context, id uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*models.CrossGameLeaderboardEntry)
	return v, args.Error(1)
}

type MockMatchRepository struct{ mock.Mock }

func (m *MockMatchRepository) Create(ctx context.Context, match *models.Match) error {
	return m.Called(ctx, match).Error(0)
}
func (m *MockMatchRepository) CreateBatch(ctx context.Context, matches []*models.Match) error {
	return m.Called(ctx, matches).Error(0)
}
func (m *MockMatchRepository) DeleteBatch(ctx context.Context, ids []uuid.UUID) error {
	return m.Called(ctx, ids).Error(0)
}

func (m *MockMatchRepository) GetByTournamentID(ctx context.Context, id uuid.UUID, limit, offset int) ([]*models.Match, error) {
	args := m.Called(ctx, id, limit, offset)
	v, _ := args.Get(0).([]*models.Match)
	return v, args.Error(1)
}

func (m *MockMatchRepository) GetPendingByTournamentID(ctx context.Context, id uuid.UUID) ([]*models.Match, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*models.Match)
	return v, args.Error(1)
}

func (m *MockMatchRepository) GetPendingByTournamentAndGame(ctx context.Context, id uuid.UUID, gameType string) ([]*models.Match, error) {
	args := m.Called(ctx, id, gameType)
	v, _ := args.Get(0).([]*models.Match)
	return v, args.Error(1)
}

func (m *MockMatchRepository) ResetFailedMatches(ctx context.Context, id uuid.UUID) (int64, error) {
	args := m.Called(ctx, id)
	return int64(args.Int(0)), args.Error(1)
}

func (m *MockMatchRepository) GetMatchesByRounds(ctx context.Context, id uuid.UUID) ([]*models.MatchRound, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*models.MatchRound)
	return v, args.Error(1)
}

// эти два в интерфейсе есть, но в тестах сервиса не дёргаются
func (m *MockMatchRepository) GetNextRoundNumber(ctx context.Context, id uuid.UUID) (int, error) {
	args := m.Called(ctx, id)
	return args.Int(0), args.Error(1)
}

func (m *MockMatchRepository) GetNextRoundNumberByGame(ctx context.Context, id uuid.UUID, gameType string) (int, error) {
	args := m.Called(ctx, id, gameType)
	return args.Int(0), args.Error(1)
}

func (m *MockMatchRepository) GetPlayedProgramPairs(ctx context.Context, id uuid.UUID, gameType string) (map[string]struct{}, error) {
	args := m.Called(ctx, id, gameType)
	v, _ := args.Get(0).(map[string]struct{})
	return v, args.Error(1)
}

type MockQueueManager struct{ mock.Mock }

func (m *MockQueueManager) Enqueue(ctx context.Context, match *models.Match) error {
	return m.Called(ctx, match).Error(0)
}
func (m *MockQueueManager) EnqueueBatch(ctx context.Context, matches []*models.Match) error {
	return m.Called(ctx, matches).Error(0)
}

type MockDistributedLock struct{ mock.Mock }

// если мок не вернул ошибку - реально зовём fn, тк надо прогнать тело под локом
func (m *MockDistributedLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error {
	args := m.Called(ctx, key, ttl, fn)
	if args.Error(0) == nil {
		return fn(ctx)
	}
	return args.Error(0)
}

type MockGameRepository struct{ mock.Mock }

func (m *MockGameRepository) SetActiveGame(ctx context.Context, id, gameID uuid.UUID) error {
	return m.Called(ctx, id, gameID).Error(0)
}
func (m *MockGameRepository) ResetGameByType(ctx context.Context, id uuid.UUID, gameType string) error {
	return m.Called(ctx, id, gameType).Error(0)
}
func (m *MockGameRepository) UpdateAutoRoundLastRun(ctx context.Context, id, gameID uuid.UUID) error {
	return m.Called(ctx, id, gameID).Error(0)
}

func (m *MockGameRepository) GetTournamentGames(ctx context.Context, id uuid.UUID) ([]*models.TournamentGame, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*models.TournamentGame)
	return v, args.Error(1)
}

func (m *MockGameRepository) GetAutoRoundEnabledGames(ctx context.Context) ([]*models.AutoRoundGameInfo, error) {
	args := m.Called(ctx)
	v, _ := args.Get(0).([]*models.AutoRoundGameInfo)
	return v, args.Error(1)
}

func (m *MockGameRepository) HasNewProgramsSince(ctx context.Context, id uuid.UUID, gameType string, since time.Time) (bool, error) {
	args := m.Called(ctx, id, gameType, since)
	return args.Bool(0), args.Error(1)
}

func (m *MockGameRepository) HasActiveMatchesForGame(ctx context.Context, id uuid.UUID, gameType string) (bool, error) {
	args := m.Called(ctx, id, gameType)
	return args.Bool(0), args.Error(1)
}

type MockProgramRepository struct{ mock.Mock }

func (m *MockProgramRepository) GetByTournamentAndGame(ctx context.Context, id, gameID uuid.UUID) ([]*models.Program, error) {
	args := m.Called(ctx, id, gameID)
	v, _ := args.Get(0).([]*models.Program)
	return v, args.Error(1)
}

// setupTestRedisCache - кэш на miniredis, живёт в рамках теста
func setupTestRedisCache(t *testing.T) *cache.Cache {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return cache.NewFromClient(client)
}

// newTestService - сервис со всеми моками и реальным кэшем на miniredis
func newTestService(t *testing.T) (*Service, *MockTournamentRepository, *MockMatchRepository, *MockQueueManager, *MockDistributedLock, *MockGameRepository) {
	t.Helper()
	tournamentRepo := new(MockTournamentRepository)
	matchRepo := new(MockMatchRepository)
	queueManager := new(MockQueueManager)
	distributedLock := new(MockDistributedLock)
	gameRepo := new(MockGameRepository)

	testCache := setupTestRedisCache(t)
	t.Cleanup(func() { testCache.Close() })

	log, _ := logger.New("error", "json")
	service := NewService(
		tournamentRepo, matchRepo, queueManager, gameRepo,
		cache.NewTournamentCache(testCache), cache.NewLeaderboardCache(testCache),
		events.NoopNotifier{}, distributedLock, log,
	)
	return service, tournamentRepo, matchRepo, queueManager, distributedLock, gameRepo
}

// newTestSchedulingService - то же, но для планировщика матчей
func newTestSchedulingService(t *testing.T) (*SchedulingService, *MockTournamentRepository, *MockMatchRepository, *MockQueueManager, *MockDistributedLock, *MockGameRepository) {
	t.Helper()
	tournamentRepo := new(MockTournamentRepository)
	matchRepo := new(MockMatchRepository)
	queueManager := new(MockQueueManager)
	distributedLock := new(MockDistributedLock)
	gameRepo := new(MockGameRepository)

	log, _ := logger.New("error", "json")
	service := NewSchedulingService(tournamentRepo, matchRepo, queueManager, gameRepo, distributedLock, events.NoopNotifier{}, log)
	return service, tournamentRepo, matchRepo, queueManager, distributedLock, gameRepo
}

// матчеры аргументов WithLock - лок просто пропускает тело внутрь
func anyLock() []any {
	return []any{mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")}
}

func TestService_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentRepo.On("Create", ctx, mock.AnythingOfType("*models.Tournament")).Return(nil)

		result, err := service.Create(ctx, &CreateRequest{Name: "Test Tournament", GameType: "prisoners_dilemma"})
		require.NoError(t, err)
		assert.Equal(t, "Test Tournament", result.Name)
		assert.Equal(t, "prisoners_dilemma", result.GameType)
		assert.Equal(t, models.TournamentPending, result.Status)
		assert.Equal(t, 1, result.MaxTeamSize)
		assert.NotEqual(t, uuid.Nil, result.ID)
		assert.NotEmpty(t, result.Code)
	})

	t.Run("validation_error", func(t *testing.T) {
		service, _, _, _, _, _ := newTestService(t)

		result, err := service.Create(context.Background(), &CreateRequest{Name: "", GameType: "chess"})
		assert.Nil(t, result)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.Code)
	})

	t.Run("repo_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentRepo.On("Create", ctx, mock.AnythingOfType("*models.Tournament")).Return(fmt.Errorf("database connection lost"))

		result, err := service.Create(ctx, &CreateRequest{Name: "Test Tournament", GameType: "chess"})
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to create tournament")
		assert.Contains(t, err.Error(), "database connection lost")
	})

	t.Run("default_team_size", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournamentRepo.On("Create", ctx, mock.AnythingOfType("*models.Tournament")).Return(nil)

		// нулевой (и отрицательный) размер команды схлопывается в 1
		res, err := service.Create(ctx, &CreateRequest{Name: "A", GameType: "chess", MaxTeamSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, res.MaxTeamSize)
	})
}

func TestService_GetByID(t *testing.T) {
	t.Run("cache_hit", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Cached Tournament", GameType: "chess", Status: models.TournamentPending}

		// кладём в кэш заранее - до бд дело не дойдёт
		require.NoError(t, service.tournamentCache.Set(ctx, tournament))

		result, err := service.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "Cached Tournament", result.Name)
		tournamentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("cache_miss_repo_hit", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "DB Tournament", GameType: "chess", Status: models.TournamentActive}
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		result, err := service.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "DB Tournament", result.Name)

		// после промаха турнир должен осесть в кэше
		cached, err := service.tournamentCache.Get(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "DB Tournament", cached.Name)
	})

	t.Run("not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournamentRepo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

		result, err := service.GetByID(ctx, id)
		assert.Nil(t, result)
		assert.True(t, errors.IsNotFound(err))
	})
}

func TestService_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		tournaments := []*models.Tournament{
			{ID: uuid.New(), Name: "T1", GameType: "chess", Status: models.TournamentPending},
			{ID: uuid.New(), Name: "T2", GameType: "chess", Status: models.TournamentActive},
		}
		filter := models.TournamentFilter{Limit: 10}
		tournamentRepo.On("List", ctx, filter).Return(tournaments, nil)

		result, err := service.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "T1", result[0].Name)
	})

	t.Run("limit_clamped", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		// 0 -> дефолтный 50, больше 100 -> 100
		tournamentRepo.On("List", ctx, models.TournamentFilter{Limit: 50}).Return([]*models.Tournament{}, nil)
		tournamentRepo.On("List", ctx, models.TournamentFilter{Limit: 100}).Return([]*models.Tournament{}, nil)

		_, err := service.List(ctx, models.TournamentFilter{Limit: 0})
		require.NoError(t, err)
		_, err = service.List(ctx, models.TournamentFilter{Limit: 101})
		require.NoError(t, err)

		tournamentRepo.AssertCalled(t, "List", ctx, models.TournamentFilter{Limit: 50})
		tournamentRepo.AssertCalled(t, "List", ctx, models.TournamentFilter{Limit: 100})
	})

	t.Run("repo_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		filter := models.TournamentFilter{Limit: 10}
		tournamentRepo.On("List", ctx, filter).Return(([]*models.Tournament)(nil), fmt.Errorf("db error"))

		result, err := service.List(ctx, filter)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestService_Join(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		maxParticipants := 10
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentPending, MaxParticipants: &maxParticipants}

		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", ctx, id).Return(5, nil)
		tournamentRepo.On("AddParticipant", ctx, mock.AnythingOfType("*models.TournamentParticipant")).Return(nil)

		programID := uuid.New()
		err := service.Join(ctx, &JoinRequest{TournamentID: id, ProgramID: programID})
		require.NoError(t, err)

		// участник добавлен со стартовым рейтингом 1500
		tournamentRepo.AssertCalled(t, "AddParticipant", ctx, mock.MatchedBy(func(p *models.TournamentParticipant) bool {
			return p.TournamentID == id && p.ProgramID == programID && p.Rating == 1500
		}))
	})

	t.Run("tournament_not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

		err := service.Join(ctx, &JoinRequest{TournamentID: id, ProgramID: uuid.New()})
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("not_pending", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		err := service.Join(ctx, &JoinRequest{TournamentID: id, ProgramID: uuid.New()})
		assert.Equal(t, errors.ErrTournamentStarted, err)
	})

	t.Run("full", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		maxParticipants := 5
		tournament := &models.Tournament{ID: id, Name: "Full", GameType: "chess", Status: models.TournamentPending, MaxParticipants: &maxParticipants}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", ctx, id).Return(5, nil)

		err := service.Join(ctx, &JoinRequest{TournamentID: id, ProgramID: uuid.New()})
		assert.Equal(t, errors.ErrTournamentFull, err)
	})

	t.Run("no_max_participants", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Unlimited", GameType: "chess", Status: models.TournamentPending, MaxParticipants: nil}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("AddParticipant", ctx, mock.AnythingOfType("*models.TournamentParticipant")).Return(nil)

		err := service.Join(ctx, &JoinRequest{TournamentID: id, ProgramID: uuid.New()})
		require.NoError(t, err)

		// без лимита счётчик участников не дёргаем
		tournamentRepo.AssertNotCalled(t, "GetParticipantsCount", mock.Anything, mock.Anything)
	})

	t.Run("participants_count_error", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		maxParticipants := 10
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentPending, MaxParticipants: &maxParticipants}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("GetParticipantsCount", ctx, id).Return(0, fmt.Errorf("db connection lost"))

		err := service.Join(ctx, &JoinRequest{TournamentID: id, ProgramID: uuid.New()})
		assert.Contains(t, err.Error(), "failed to get participants count")

		// счёт не получем - участника не добавляем
		tournamentRepo.AssertNotCalled(t, "AddParticipant", mock.Anything, mock.Anything)
	})

	t.Run("lock_error", func(t *testing.T) {
		service, _, _, _, distributedLock, _ := newTestService(t)

		distributedLock.On("WithLock", anyLock()...).Return(errors.ErrConflict.WithMessage("lock already held"))

		err := service.Join(context.Background(), &JoinRequest{TournamentID: uuid.New(), ProgramID: uuid.New()})
		assert.Contains(t, err.Error(), "lock already held")
	})
}

func TestService_Start(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, gameRepo := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentPending}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		// Start читает турнир прямо из бд, мимо кэша
		tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)
		tournamentRepo.On("GetTeamsCount", mock.Anything, id).Return(3, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Tournament")).Return(nil)
		gameRepo.On("GetTournamentGames", mock.Anything, id).Return([]*models.TournamentGame{}, nil)

		err := service.Start(ctx, id)
		require.NoError(t, err)
		tournamentRepo.AssertCalled(t, "Update", mock.Anything, mock.MatchedBy(func(tt *models.Tournament) bool {
			return tt.Status == models.TournamentActive && tt.StartTime != nil
		}))
	})

	t.Run("not_pending", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)

		err := service.Start(ctx, id)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "already started")
	})

	t.Run("too_few_teams", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentPending}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)
		tournamentRepo.On("GetTeamsCount", mock.Anything, id).Return(1, nil)

		err := service.Start(ctx, id)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Contains(t, appErr.Message, "минимум 2 команды")
	})

	t.Run("update_error", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentPending}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)
		tournamentRepo.On("GetTeamsCount", mock.Anything, id).Return(5, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Tournament")).Return(fmt.Errorf("db write error"))

		err := service.Start(ctx, id)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Contains(t, appErr.Message, "failed to update tournament status")
	})

	t.Run("activates_first_game", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, gameRepo := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		gameID := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Multi", GameType: "multi", Status: models.TournamentPending}
		games := []*models.TournamentGame{
			{TournamentID: id, GameID: gameID, IsActive: false},
			{TournamentID: id, GameID: uuid.New(), IsActive: false},
		}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)
		tournamentRepo.On("GetTeamsCount", mock.Anything, id).Return(4, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Tournament")).Return(nil)
		gameRepo.On("GetTournamentGames", mock.Anything, id).Return(games, nil)
		gameRepo.On("SetActiveGame", mock.Anything, id, gameID).Return(nil)

		err := service.Start(ctx, id)
		require.NoError(t, err)

		// активируется именно первая игра из списка
		gameRepo.AssertCalled(t, "SetActiveGame", mock.Anything, id, gameID)
	})
}

func TestService_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)
		tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Tournament")).Return(nil)

		err := service.Complete(ctx, id)
		require.NoError(t, err)
		tournamentRepo.AssertCalled(t, "Update", mock.Anything, mock.MatchedBy(func(tt *models.Tournament) bool {
			return tt.Status == models.TournamentCompleted && tt.EndTime != nil
		}))
	})

	t.Run("not_active", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Pending", GameType: "chess", Status: models.TournamentPending}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)

		err := service.Complete(ctx, id)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "not active")
	})

	t.Run("lock_error", func(t *testing.T) {
		service, _, _, _, distributedLock, _ := newTestService(t)

		// не-AppError из лока заворачивается в ErrConflict
		distributedLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("redis connection lost"))

		err := service.Complete(context.Background(), uuid.New())
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "could not complete tournament")
	})
}

func TestService_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Pending", GameType: "chess", Status: models.TournamentPending}
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("Delete", ctx, id).Return(nil)

		err := service.Delete(ctx, id)
		require.NoError(t, err)
		tournamentRepo.AssertCalled(t, "Delete", ctx, id)
	})

	t.Run("active_tournament", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		err := service.Delete(ctx, id)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "cannot delete active tournament")
	})

	t.Run("not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournamentRepo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

		err := service.Delete(ctx, id)
		assert.True(t, errors.IsNotFound(err))
	})
}

func TestService_GetLeaderboard(t *testing.T) {
	t.Run("queries_db_for_full_data", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		programID := uuid.New()

		// в кэше только частичные данные (id + рейтинг), полный лидерборд всё равно из бд
		require.NoError(t, service.leaderboardCache.UpdateRating(ctx, id, programID, 1800))

		entries := []*models.LeaderboardEntry{
			{Rank: 1, ProgramID: programID, ProgramName: "bot-v1", Rating: 1800, Wins: 5, Losses: 2, TotalGames: 7},
		}
		tournamentRepo.On("GetLeaderboard", ctx, id, 10).Return(entries, nil)

		result, err := service.GetLeaderboard(ctx, id, 10)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, programID, result[0].ProgramID)
		assert.Equal(t, 1800, result[0].Rating)
		assert.Equal(t, "bot-v1", result[0].ProgramName)
		assert.Equal(t, 5, result[0].Wins)
		tournamentRepo.AssertCalled(t, "GetLeaderboard", ctx, id, 10)
	})

	t.Run("repo_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournamentRepo.On("GetLeaderboard", ctx, id, 10).Return(nil, fmt.Errorf("db error"))

		result, err := service.GetLeaderboard(ctx, id, 10)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestService_GetCrossGameLeaderboard(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		entries := []*models.CrossGameLeaderboardEntry{
			{Rank: 1, TeamName: "Team Alpha", TotalRating: 3000, TotalWins: 10},
			{Rank: 2, TeamName: "Team Beta", TotalRating: 2500, TotalWins: 7},
		}
		tournamentRepo.On("GetCrossGameLeaderboard", ctx, id).Return(entries, nil)

		result, err := service.GetCrossGameLeaderboard(ctx, id)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "Team Alpha", result[0].TeamName)
		assert.Equal(t, 3000, result[0].TotalRating)
	})

	t.Run("db_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournamentRepo.On("GetCrossGameLeaderboard", ctx, id).Return(nil, fmt.Errorf("db connection error"))

		entries, err := service.GetCrossGameLeaderboard(ctx, id)
		assert.Nil(t, entries)
		assert.Error(t, err)
	})
}

func TestService_CreateMatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "prisoners_dilemma", Status: models.TournamentActive}
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		matchRepo.On("Create", ctx, mock.AnythingOfType("*models.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*models.Match")).Return(nil)

		result, err := service.CreateMatch(ctx, id, program1ID, program2ID, models.PriorityMedium)
		require.NoError(t, err)
		assert.Equal(t, id, result.TournamentID)
		assert.Equal(t, program1ID, result.Program1ID)
		assert.Equal(t, program2ID, result.Program2ID)
		assert.Equal(t, "prisoners_dilemma", result.GameType)
		assert.Equal(t, models.MatchPending, result.Status)
		assert.Equal(t, models.PriorityMedium, result.Priority)

		matchRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*models.Match"))
		queueManager.AssertCalled(t, "Enqueue", ctx, mock.AnythingOfType("*models.Match"))
	})

	t.Run("tournament_not_found", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		tournamentRepo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

		result, err := service.CreateMatch(ctx, id, uuid.New(), uuid.New(), models.PriorityMedium)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get tournament")
	})

	t.Run("validation_error", func(t *testing.T) {
		service, tournamentRepo, _, _, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		sameID := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentActive}
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		// одинаковые проги - матч сам с собой, валидация ругается
		result, err := service.CreateMatch(ctx, id, sameID, sameID, models.PriorityMedium)
		assert.Nil(t, result)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.Code)
	})

	t.Run("queue_error_non_fatal", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, _, _ := newTestService(t)
		ctx := context.Background()

		id := uuid.New()
		program1ID := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentActive}
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		matchRepo.On("Create", ctx, mock.AnythingOfType("*models.Match")).Return(nil)
		queueManager.On("Enqueue", ctx, mock.AnythingOfType("*models.Match")).Return(fmt.Errorf("queue unavailable"))

		// матч уже в бд, ошибка очереди не критична
		result, err := service.CreateMatch(ctx, id, program1ID, uuid.New(), models.PriorityMedium)
		require.NoError(t, err)
		assert.Equal(t, program1ID, result.Program1ID)
	})
}

func TestService_GetMatches(t *testing.T) {
	service, _, matchRepo, _, _, _ := newTestService(t)
	ctx := context.Background()

	id := uuid.New()
	expected := []*models.Match{
		{ID: uuid.New(), TournamentID: id, Status: models.MatchCompleted},
		{ID: uuid.New(), TournamentID: id, Status: models.MatchPending},
	}
	matchRepo.On("GetByTournamentID", ctx, id, 10, 0).Return(expected, nil)

	result, err := service.GetMatches(ctx, id, 10, 0)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, expected[0].ID, result[0].ID)
}

func TestService_GetMatchesByRounds(t *testing.T) {
	service, _, matchRepo, _, _, _ := newTestService(t)
	ctx := context.Background()

	id := uuid.New()
	expected := []*models.MatchRound{
		{RoundNumber: 1, GameType: "chess", TotalMatches: 6, CompletedCount: 6},
		{RoundNumber: 2, GameType: "chess", TotalMatches: 6, CompletedCount: 3, PendingCount: 3},
	}
	matchRepo.On("GetMatchesByRounds", ctx, id).Return(expected, nil)

	result, err := service.GetMatchesByRounds(ctx, id)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 1, result[0].RoundNumber)
	assert.Equal(t, 2, result[1].RoundNumber)
}

func TestService_RunAllMatches(t *testing.T) {
	t.Run("with_existing_pending", func(t *testing.T) {
		service, _, matchRepo, queueManager, distLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		pending := []*models.Match{
			{ID: uuid.New(), TournamentID: id, Status: models.MatchPending},
			{ID: uuid.New(), TournamentID: id, Status: models.MatchPending},
		}
		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentID", ctx, id).Return(pending, nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)

		count, err := service.RunAllMatches(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		queueManager.AssertNumberOfCalls(t, "EnqueueBatch", 1)
	})

	t.Run("generate_new_round", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, distLock, gameRepo := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		participants := []*models.TournamentParticipant{
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New(), Rating: 1500},
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New(), Rating: 1500},
		}
		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		// pending пусто -> сброс + генерация нового раунда
		matchRepo.On("GetPendingByTournamentID", ctx, id).Return([]*models.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("GetLatestParticipantsGroupedByGame", ctx, id).Return(map[string][]*models.TournamentParticipant{"chess": participants}, nil)
		gameRepo.On("ResetGameByType", ctx, id, "chess").Return(nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)

		count, err := service.RunAllMatches(ctx, id)
		require.NoError(t, err)
		// 2 участника, обе ориентации = 2 матча
		assert.Equal(t, 2, count)
		gameRepo.AssertCalled(t, "ResetGameByType", ctx, id, "chess")
		matchRepo.AssertCalled(t, "CreateBatch", ctx, mock.MatchedBy(func(matches []*models.Match) bool { return len(matches) == 2 }))
	})

	t.Run("not_active", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, distLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Pending", GameType: "chess", Status: models.TournamentPending}
		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentID", ctx, id).Return([]*models.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		count, err := service.RunAllMatches(ctx, id)
		assert.Equal(t, 0, count)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "not active")
	})

	t.Run("no_participants", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, distLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentID", ctx, id).Return([]*models.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("GetLatestParticipantsGroupedByGame", ctx, id).Return(map[string][]*models.TournamentParticipant{}, nil)

		count, err := service.RunAllMatches(ctx, id)
		assert.Equal(t, 0, count)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.Code)
		assert.Contains(t, appErr.Message, "at least 2 participants")
	})

	t.Run("enqueue_error_rolls_back_created", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, distLock, gameRepo := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentActive}
		participants := map[string][]*models.TournamentParticipant{
			"chess": {
				{ID: uuid.New(), ProgramID: uuid.New(), TournamentID: id},
				{ID: uuid.New(), ProgramID: uuid.New(), TournamentID: id},
			},
		}
		distLock.On("WithLock", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentID", ctx, id).Return([]*models.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("GetLatestParticipantsGroupedByGame", ctx, id).Return(participants, nil)
		gameRepo.On("ResetGameByType", ctx, id, "chess").Return(nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(fmt.Errorf("redis pipeline error"))
		// матчи из этого вызова обязаны откатиться
		matchRepo.On("DeleteBatch", ctx, mock.AnythingOfType("[]uuid.UUID")).Return(nil)

		count, err := service.RunAllMatches(ctx, id)
		assert.Error(t, err)
		assert.Equal(t, 0, count)
		matchRepo.AssertCalled(t, "DeleteBatch", ctx, mock.AnythingOfType("[]uuid.UUID"))
	})
}

func TestService_RunGameMatches(t *testing.T) {
	t.Run("with_existing_pending", func(t *testing.T) {
		service, _, matchRepo, queueManager, distLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		gameType := "prisoners_dilemma"
		pending := []*models.Match{
			{ID: uuid.New(), TournamentID: id, GameType: gameType, Status: models.MatchPending},
			{ID: uuid.New(), TournamentID: id, GameType: gameType, Status: models.MatchPending},
			{ID: uuid.New(), TournamentID: id, GameType: gameType, Status: models.MatchPending},
		}
		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentAndGame", ctx, id, gameType).Return(pending, nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)

		count, err := service.RunGameMatches(ctx, id, gameType)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("generate_new_round", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, distLock, gameRepo := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		gameType := "prisoners_dilemma"
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: gameType, Status: models.TournamentActive}
		participants := []*models.TournamentParticipant{
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New()},
		}
		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentAndGame", ctx, id, gameType).Return([]*models.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)
		tournamentRepo.On("GetLatestParticipantsByGame", ctx, id, gameType).Return(participants, nil)
		gameRepo.On("ResetGameByType", ctx, id, gameType).Return(nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)

		count, err := service.RunGameMatches(ctx, id, gameType)
		require.NoError(t, err)
		// 3 участника: AB, BA, AC, CA, BC, CB = 6 матчей
		assert.Equal(t, 6, count)
	})

	t.Run("not_active", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, distLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		gameType := "chess"
		tournament := &models.Tournament{ID: id, Name: "Completed", GameType: gameType, Status: models.TournamentCompleted}
		distLock.On("WithLock", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("func(context.Context) error")).Return(nil)
		matchRepo.On("GetPendingByTournamentAndGame", ctx, id, gameType).Return([]*models.Match{}, nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		count, err := service.RunGameMatches(ctx, id, gameType)
		assert.Equal(t, 0, count)
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
	})
}

func TestService_RetryFailedMatches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, _, matchRepo, queueManager, _, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		matchRepo.On("ResetFailedMatches", ctx, id).Return(3, nil)
		pending := []*models.Match{
			{ID: uuid.New(), TournamentID: id, Status: models.MatchPending},
			{ID: uuid.New(), TournamentID: id, Status: models.MatchPending},
			{ID: uuid.New(), TournamentID: id, Status: models.MatchPending},
		}
		matchRepo.On("GetPendingByTournamentID", ctx, id).Return(pending, nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)

		count, err := service.RetryFailedMatches(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("no_failed_matches", func(t *testing.T) {
		service, _, matchRepo, _, _, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		matchRepo.On("ResetFailedMatches", ctx, id).Return(0, nil)

		count, err := service.RetryFailedMatches(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		// нечего перезапускать - в очередь не лезем
		matchRepo.AssertNotCalled(t, "GetPendingByTournamentID", mock.Anything, mock.Anything)
	})

	t.Run("reset_error", func(t *testing.T) {
		service, _, matchRepo, _, _, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		matchRepo.On("ResetFailedMatches", ctx, id).Return(0, fmt.Errorf("db error"))

		count, err := service.RetryFailedMatches(ctx, id)
		assert.Error(t, err)
		assert.Equal(t, 0, count)
	})
}

// round-robin: каждый с каждым в обе стороны (AB и BA), сам с собой не играет
func TestService_generateRoundRobinMatchesForGame(t *testing.T) {
	id := uuid.New()
	tournament := &models.Tournament{ID: id, Name: "Test", GameType: "chess", Status: models.TournamentActive}

	t.Run("two_participants_both_directions", func(t *testing.T) {
		service, _, _, _, _, _ := newTestSchedulingService(t)

		p1 := uuid.New()
		p2 := uuid.New()
		participants := []*models.TournamentParticipant{
			{ID: uuid.New(), TournamentID: id, ProgramID: p1},
			{ID: uuid.New(), TournamentID: id, ProgramID: p2},
		}

		matches, err := service.generateRoundRobinMatchesForGame(tournament, participants, "chess", 1, models.PriorityMedium, nil)
		require.NoError(t, err)
		assert.Len(t, matches, 2)

		hasAB, hasBA := false, false
		for _, m := range matches {
			if m.Program1ID == p1 && m.Program2ID == p2 {
				hasAB = true
			}
			if m.Program1ID == p2 && m.Program2ID == p1 {
				hasBA = true
			}
		}
		assert.True(t, hasAB, "нет матча p1 vs p2")
		assert.True(t, hasBA, "нет матча p2 vs p1")
	})

	t.Run("three_participants_fields", func(t *testing.T) {
		service, _, _, _, _, _ := newTestSchedulingService(t)

		participants := []*models.TournamentParticipant{
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New()},
			{ID: uuid.New(), TournamentID: id, ProgramID: uuid.New()},
		}

		matches, err := service.generateRoundRobinMatchesForGame(tournament, participants, "chess", 1, models.PriorityMedium, nil)
		require.NoError(t, err)
		// n*(n-1) = 3*2 = 6
		assert.Len(t, matches, 6)

		// заодно проверяем поля сгенерированных матчей
		seen := make(map[[2]uuid.UUID]bool)
		for _, m := range matches {
			assert.Equal(t, id, m.TournamentID)
			assert.Equal(t, "chess", m.GameType)
			assert.Equal(t, models.MatchPending, m.Status)
			assert.Equal(t, models.PriorityMedium, m.Priority)
			assert.Equal(t, 1, m.RoundNumber)
			assert.NotEqual(t, m.Program1ID, m.Program2ID)
			key := [2]uuid.UUID{m.Program1ID, m.Program2ID}
			assert.False(t, seen[key], "дубль пары")
			seen[key] = true
		}
	})

	t.Run("skips_played_pairs", func(t *testing.T) {
		service, _, _, _, _, _ := newTestSchedulingService(t)

		p1, p2, p3 := uuid.New(), uuid.New(), uuid.New()
		participants := []*models.TournamentParticipant{
			{ID: uuid.New(), TournamentID: id, ProgramID: p1},
			{ID: uuid.New(), TournamentID: id, ProgramID: p2},
			{ID: uuid.New(), TournamentID: id, ProgramID: p3},
		}
		// p1 vs p2 и p2 vs p1 уже сыграны
		played := map[string]struct{}{
			p1.String() + "|" + p2.String(): {},
			p2.String() + "|" + p1.String(): {},
		}

		matches, err := service.generateRoundRobinMatchesForGame(tournament, participants, "chess", 2, models.PriorityMedium, played)
		require.NoError(t, err)
		// 6 пар всего минус 2 сыгранных = 4
		assert.Len(t, matches, 4)
		for _, m := range matches {
			_, isPlayed := played[m.Program1ID.String()+"|"+m.Program2ID.String()]
			assert.False(t, isPlayed, "сыгранная пара попала в матчи")
		}

		// а елси все пары уже сыграны - на выходе пусто
		allPlayed := map[string]struct{}{}
		for i := range participants {
			for j := range participants {
				if i != j {
					allPlayed[participants[i].ProgramID.String()+"|"+participants[j].ProgramID.String()] = struct{}{}
				}
			}
		}
		empty, err := service.generateRoundRobinMatchesForGame(tournament, participants, "chess", 3, models.PriorityMedium, allPlayed)
		require.NoError(t, err)
		assert.Len(t, empty, 0)
	})
}

func TestService_ScheduleNewProgramMatches(t *testing.T) {
	t.Run("success_skips_own_team", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, distributedLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		gameID := uuid.New()
		teamID := uuid.New()
		otherTeamID := uuid.New()
		newProgramID := uuid.New()
		sameTeamProgramID := uuid.New()
		otherTeamProgramID := uuid.New()

		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		// новая прога, сокомандник и чужая прога. матчи только против чужой
		programs := []*models.Program{
			{ID: newProgramID, Name: "New Bot", GameType: "chess", TeamID: &teamID},
			{ID: sameTeamProgramID, Name: "Teammate Bot", GameType: "chess", TeamID: &teamID},
			{ID: otherTeamProgramID, Name: "Opponent Bot", GameType: "chess", TeamID: &otherTeamID},
		}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		programRepo := new(MockProgramRepository)
		programRepo.On("GetByTournamentAndGame", ctx, id, gameID).Return(programs, nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)

		req := &ScheduleNewProgramMatchesRequest{TournamentID: id, GameID: gameID, NewProgramID: newProgramID, TeamID: teamID}
		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		require.NoError(t, err)

		// ровно 2 матча против чужой проги (обе стороны), сокомандник исключён
		matchRepo.AssertCalled(t, "CreateBatch", ctx, mock.MatchedBy(func(matches []*models.Match) bool {
			if len(matches) != 2 {
				return false
			}
			hasForward, hasReverse := false, false
			for _, m := range matches {
				if m.Program1ID == sameTeamProgramID || m.Program2ID == sameTeamProgramID {
					return false
				}
				if m.Program1ID == newProgramID && m.Program2ID == otherTeamProgramID {
					hasForward = true
				}
				if m.Program1ID == otherTeamProgramID && m.Program2ID == newProgramID {
					hasReverse = true
				}
			}
			return hasForward && hasReverse
		}))
		queueManager.AssertNumberOfCalls(t, "EnqueueBatch", 1)
	})

	t.Run("completed_tournament", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Completed", GameType: "chess", Status: models.TournamentCompleted}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		req := &ScheduleNewProgramMatchesRequest{TournamentID: id, GameID: uuid.New(), NewProgramID: uuid.New(), TeamID: uuid.New()}
		err := service.ScheduleNewProgramMatches(ctx, req, new(MockProgramRepository))
		appErr := errors.GetAppError(err)
		require.NotNil(t, appErr)
		assert.Equal(t, 409, appErr.Code)
		assert.Contains(t, appErr.Message, "cannot schedule matches for completed tournament")
	})

	t.Run("no_opponents", func(t *testing.T) {
		service, tournamentRepo, matchRepo, _, distributedLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		gameID := uuid.New()
		newProgramID := uuid.New()
		teamID := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		// в турнире только новая прога, играть не с кем
		programs := []*models.Program{{ID: newProgramID, Name: "New Bot", GameType: "chess", TeamID: &teamID}}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		programRepo := new(MockProgramRepository)
		programRepo.On("GetByTournamentAndGame", ctx, id, gameID).Return(programs, nil)

		req := &ScheduleNewProgramMatchesRequest{TournamentID: id, GameID: gameID, NewProgramID: newProgramID, TeamID: teamID}
		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		require.NoError(t, err)
		matchRepo.AssertNotCalled(t, "CreateBatch", mock.Anything, mock.Anything)
	})

	t.Run("get_programs_error", func(t *testing.T) {
		service, tournamentRepo, _, _, distributedLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		gameID := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		programRepo := new(MockProgramRepository)
		programRepo.On("GetByTournamentAndGame", ctx, id, gameID).Return(nil, fmt.Errorf("program db error"))

		req := &ScheduleNewProgramMatchesRequest{TournamentID: id, GameID: gameID, NewProgramID: uuid.New(), TeamID: uuid.New()}
		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		assert.Contains(t, err.Error(), "failed to get programs")
		assert.Contains(t, err.Error(), "program db error")
	})

	t.Run("enqueue_error_rolls_back", func(t *testing.T) {
		service, tournamentRepo, matchRepo, queueManager, distributedLock, _ := newTestSchedulingService(t)
		ctx := context.Background()

		id := uuid.New()
		gameID := uuid.New()
		teamID := uuid.New()
		otherTeamID := uuid.New()
		newProgramID := uuid.New()
		tournament := &models.Tournament{ID: id, Name: "Active", GameType: "chess", Status: models.TournamentActive}
		programs := []*models.Program{
			{ID: newProgramID, Name: "New Bot", GameType: "chess", TeamID: &teamID},
			{ID: uuid.New(), Name: "Opponent Bot", GameType: "chess", TeamID: &otherTeamID},
		}
		distributedLock.On("WithLock", anyLock()...).Return(nil)
		tournamentRepo.On("GetByID", ctx, id).Return(tournament, nil)

		programRepo := new(MockProgramRepository)
		programRepo.On("GetByTournamentAndGame", ctx, id, gameID).Return(programs, nil)
		matchRepo.On("CreateBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(nil)
		queueManager.On("EnqueueBatch", ctx, mock.AnythingOfType("[]*models.Match")).Return(fmt.Errorf("redis pipeline error"))
		// при ошибке enqueue матчи откатываются через DeleteBatch
		matchRepo.On("DeleteBatch", ctx, mock.AnythingOfType("[]uuid.UUID")).Return(nil)

		req := &ScheduleNewProgramMatchesRequest{TournamentID: id, GameID: gameID, NewProgramID: newProgramID, TeamID: teamID}
		err := service.ScheduleNewProgramMatches(ctx, req, programRepo)
		assert.Contains(t, err.Error(), "failed to enqueue matches")
		assert.Contains(t, err.Error(), "redis pipeline error")
		matchRepo.AssertCalled(t, "DeleteBatch", ctx, mock.AnythingOfType("[]uuid.UUID"))
	})
}

func TestGenerateCode(t *testing.T) {
	t.Run("length_and_charset", func(t *testing.T) {
		// без похожих символов I,O,l,0,1 - чтобы не путать при вводе
		for range 100 {
			code := generateCode()
			assert.Len(t, code, 6)
			for _, ch := range code {
				assert.NotContains(t, "IOl01", string(ch))
			}
		}
	})

	t.Run("codes_unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 50 {
			code := generateCode()
			assert.False(t, seen[code], "код-дубль: %s", code)
			seen[code] = true
		}
	})
}

// -----------------------------------------------------------------------------
// конкурентные тесты - тут реальный лок через miniredis, не мок
// -----------------------------------------------------------------------------

// конкурентный join не должен пробить лимит участников
func TestConcurrentJoin(t *testing.T) {
	tournamentRepo := new(MockTournamentRepository)

	var participantCount int64
	maxParticipants := 10

	id := uuid.New()
	tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentPending, MaxParticipants: &maxParticipants}
	tournamentRepo.On("GetByID", mock.Anything, id).Return(tournament, nil)

	// счётчик участников через atomic
	getCountCall := tournamentRepo.On("GetParticipantsCount", mock.Anything, id)
	getCountCall.Run(func(args mock.Arguments) {
		getCountCall.ReturnArguments = mock.Arguments{int(atomic.LoadInt64(&participantCount)), nil}
	}).Return(0, nil)

	// добавление участника инкрементит счётчик, но не выше лимита
	addCall := tournamentRepo.On("AddParticipant", mock.Anything, mock.AnythingOfType("*models.TournamentParticipant"))
	addCall.Run(func(args mock.Arguments) {
		count := atomic.AddInt64(&participantCount, 1)
		if count > int64(maxParticipants) {
			atomic.AddInt64(&participantCount, -1)
			addCall.ReturnArguments = mock.Arguments{errors.ErrTournamentFull}
		} else {
			addCall.ReturnArguments = mock.Arguments{nil}
		}
	}).Return(nil)

	testCache := setupTestRedisCache(t)
	defer testCache.Close()

	log, _ := logger.New("error", "json")
	service := NewService(
		tournamentRepo, new(MockMatchRepository), new(MockQueueManager), nil,
		cache.NewTournamentCache(testCache), cache.NewLeaderboardCache(testCache),
		events.NoopNotifier{}, cache.NewDistributedLock(testCache), log,
	)

	var wg sync.WaitGroup
	successCount := int64(0)
	errorCount := int64(0)
	concurrentJoins := 20

	for range concurrentJoins {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &JoinRequest{TournamentID: id, ProgramID: uuid.New()}
			if err := service.Join(context.Background(), req); err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errorCount, 1)
			}
		}()
	}
	wg.Wait()

	assert.LessOrEqual(t, successCount, int64(maxParticipants), "лимит участников пробит")
	assert.Equal(t, successCount, participantCount)
	assert.Equal(t, int64(concurrentJoins), successCount+errorCount, "все join должны завершиться")
}

// стартануть турнир дважды нельзя даже конкурентно
func TestConcurrentStart(t *testing.T) {
	tournamentRepo := new(MockTournamentRepository)

	id := uuid.New()
	tournament := &models.Tournament{ID: id, Name: "T", GameType: "chess", Status: models.TournamentPending}

	var startCount int64

	// после первого старта GetByID отдаёт уже активный турнир
	getByIDCall := tournamentRepo.On("GetByID", mock.Anything, id)
	getByIDCall.Run(func(args mock.Arguments) {
		if atomic.LoadInt64(&startCount) > 0 {
			cp := *tournament
			cp.Status = models.TournamentActive
			getByIDCall.ReturnArguments = mock.Arguments{&cp, nil}
		} else {
			getByIDCall.ReturnArguments = mock.Arguments{tournament, nil}
		}
	}).Return(tournament, nil)

	tournamentRepo.On("GetTeamsCount", mock.Anything, id).Return(3, nil)

	updateCall := tournamentRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Tournament"))
	updateCall.Run(func(args mock.Arguments) {
		atomic.AddInt64(&startCount, 1)
		updateCall.ReturnArguments = mock.Arguments{nil}
	}).Return(nil)

	testCache := setupTestRedisCache(t)
	defer testCache.Close()

	log, _ := logger.New("error", "json")
	service := NewService(
		tournamentRepo, new(MockMatchRepository), new(MockQueueManager), nil,
		cache.NewTournamentCache(testCache), cache.NewLeaderboardCache(testCache),
		events.NoopNotifier{}, cache.NewDistributedLock(testCache), log,
	)

	var wg sync.WaitGroup
	successCount := int64(0)
	errorCount := int64(0)
	concurrentStarts := 5

	for range concurrentStarts {
		wg.Go(func() {
			if err := service.Start(context.Background(), id); err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errorCount, 1)
			}
		})
	}
	wg.Wait()

	assert.Equal(t, int64(1), successCount, "стартовать должен ровно один")
	assert.Equal(t, int64(concurrentStarts-1), errorCount)
	assert.Equal(t, int64(1), startCount, "турнир стартанул ровно раз")
}
