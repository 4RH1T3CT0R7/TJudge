package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockMatchRepository mocks the match repository
type MockMatchRepository struct {
	mock.Mock
}

func (m *MockMatchRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Match), args.Error(1)
}

func (m *MockMatchRepository) List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

func (m *MockMatchRepository) GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*db.MatchStatistics, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.MatchStatistics), args.Error(1)
}

func (m *MockMatchRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Match, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

// MockMatchCache mocks the match cache
type MockMatchCache struct {
	mock.Mock
}

func (m *MockMatchCache) Get(ctx context.Context, matchID uuid.UUID) (*domain.MatchResult, error) {
	args := m.Called(ctx, matchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MatchResult), args.Error(1)
}

func (m *MockMatchCache) Set(ctx context.Context, matchID uuid.UUID, result *domain.MatchResult) error {
	args := m.Called(ctx, matchID, result)
	return args.Error(0)
}

func (m *MockMatchCache) GetMatch(ctx context.Context, matchID uuid.UUID) (*domain.Match, error) {
	args := m.Called(ctx, matchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Match), args.Error(1)
}

func (m *MockMatchCache) SetMatch(ctx context.Context, match *domain.Match) error {
	args := m.Called(ctx, match)
	return args.Error(0)
}

func TestMatchHandler_Get(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get match from cache", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		matchID := uuid.New()
		cachedResult := &domain.MatchResult{
			MatchID: matchID,
			Score1:  2,
			Score2:  1,
			Winner:  1,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(cachedResult, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.MatchResult
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, cachedResult.MatchID, response.MatchID)

		mockCache.AssertExpectations(t)
		// Repository should not be called if cache hit
		mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("successfully get match from database on cache miss", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		matchID := uuid.New()
		dbMatch := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "chess",
			Status:       domain.MatchRunning,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(dbMatch, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, dbMatch.ID, response.ID)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/invalid-uuid", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("match not found", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		matchID := uuid.New()

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(nil, errors.ErrNotFound.WithMessage("match not found"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})
}

func TestMatchHandler_List(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully list matches", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		expectedMatches := []*domain.Match{
			{
				ID:           uuid.New(),
				TournamentID: uuid.New(),
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "chess",
				Status:       domain.MatchCompleted,
			},
			{
				ID:           uuid.New(),
				TournamentID: uuid.New(),
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "chess",
				Status:       domain.MatchPending,
			},
		}

		mockRepo.On("List", mock.Anything, mock.MatchedBy(func(filter domain.MatchFilter) bool {
			return filter.Limit == 50 && filter.Offset == 0
		})).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 2)

		mockRepo.AssertExpectations(t)
	})

	t.Run("list with tournament_id filter", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		tournamentID := uuid.New()
		expectedMatches := []*domain.Match{
			{
				ID:           uuid.New(),
				TournamentID: tournamentID,
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "chess",
				Status:       domain.MatchCompleted,
			},
		}

		mockRepo.On("List", mock.Anything, mock.MatchedBy(func(filter domain.MatchFilter) bool {
			return filter.TournamentID != nil && *filter.TournamentID == tournamentID
		})).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?tournament_id="+tournamentID.String(), nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("list with status filter", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		expectedMatches := []*domain.Match{
			{
				ID:           uuid.New(),
				TournamentID: uuid.New(),
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "chess",
				Status:       domain.MatchCompleted,
			},
		}

		mockRepo.On("List", mock.Anything, mock.MatchedBy(func(filter domain.MatchFilter) bool {
			return filter.Status == domain.MatchCompleted
		})).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?status=completed", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("list with pagination", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		expectedMatches := []*domain.Match{}

		mockRepo.On("List", mock.Anything, mock.MatchedBy(func(filter domain.MatchFilter) bool {
			return filter.Limit == 10 && filter.Offset == 20
		})).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?limit=10&offset=20", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("invalid tournament_id", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?tournament_id=invalid", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid program_id", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?program_id=invalid", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestMatchHandler_GetStatistics(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get statistics for all matches", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		expectedStats := &db.MatchStatistics{
			Total:     100,
			Completed: 80,
			Running:   15,
			Failed:    5,
			Pending:   0,
		}

		mockRepo.On("GetStatistics", mock.Anything, (*uuid.UUID)(nil)).Return(expectedStats, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/statistics", nil)
		w := httptest.NewRecorder()

		handler.GetStatistics(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response db.MatchStatistics
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, 100, response.Total)
		assert.Equal(t, 80, response.Completed)

		mockRepo.AssertExpectations(t)
	})

	t.Run("successfully get statistics for specific tournament", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		tournamentID := uuid.New()
		expectedStats := &db.MatchStatistics{
			Total:     20,
			Completed: 18,
			Running:   2,
			Failed:    0,
			Pending:   0,
		}

		mockRepo.On("GetStatistics", mock.Anything, &tournamentID).Return(expectedStats, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/statistics?tournament_id="+tournamentID.String(), nil)
		w := httptest.NewRecorder()

		handler.GetStatistics(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response db.MatchStatistics
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, 20, response.Total)

		mockRepo.AssertExpectations(t)
	})

	t.Run("invalid tournament_id", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/statistics?tournament_id=invalid", nil)
		w := httptest.NewRecorder()

		handler.GetStatistics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("database error", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		handler := NewMatchHandler(mockRepo, mockCache, log)

		mockRepo.On("GetStatistics", mock.Anything, (*uuid.UUID)(nil)).Return(nil, errors.ErrInternal.WithMessage("database error"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/statistics", nil)
		w := httptest.NewRecorder()

		handler.GetStatistics(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

// MockMatchQueueManager mocks the queue manager interface
type MockMatchQueueManager struct {
	mock.Mock
}

func (m *MockMatchQueueManager) GetStats(ctx context.Context) (*queue.QueueStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*queue.QueueStats), args.Error(1)
}

func (m *MockMatchQueueManager) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMatchQueueManager) PurgeInvalidMatches(ctx context.Context, validator func(matchID string) bool) (int64, error) {
	args := m.Called(ctx, validator)
	return args.Get(0).(int64), args.Error(1)
}

// MockMatchProgramLookup mocks the program lookup interface
type MockMatchProgramLookup struct {
	mock.Mock
}

func (m *MockMatchProgramLookup) GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Program), args.Error(1)
}

func TestMatchHandler_GetQueueStats(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandlerFull(mockRepo, mockCache, nil, mockQueue, log)

		expectedStats := &queue.QueueStats{
			High:   5,
			Medium: 10,
			Low:    3,
			Total:  18,
		}

		mockQueue.On("GetStats", mock.Anything).Return(expectedStats, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/queue/stats", nil)
		w := httptest.NewRecorder()

		handler.GetQueueStats(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response queue.QueueStats
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, int64(5), response.High)
		assert.Equal(t, int64(10), response.Medium)
		assert.Equal(t, int64(3), response.Low)
		assert.Equal(t, int64(18), response.Total)

		mockQueue.AssertExpectations(t)
	})

	t.Run("queue_manager_nil", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)

		handler := NewMatchHandler(mockRepo, mockCache, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/queue/stats", nil)
		w := httptest.NewRecorder()

		handler.GetQueueStats(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("service_error", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandlerFull(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("GetStats", mock.Anything).Return(nil, errors.ErrInternal.WithMessage("redis connection failed"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/queue/stats", nil)
		w := httptest.NewRecorder()

		handler.GetQueueStats(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockQueue.AssertExpectations(t)
	})
}

func TestMatchHandler_ClearQueue(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandlerFull(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("Clear", mock.Anything).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/clear", nil)
		w := httptest.NewRecorder()

		handler.ClearQueue(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "All queues cleared successfully", response["message"])

		mockQueue.AssertExpectations(t)
	})

	t.Run("queue_manager_nil", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)

		handler := NewMatchHandler(mockRepo, mockCache, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/clear", nil)
		w := httptest.NewRecorder()

		handler.ClearQueue(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("service_error", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandlerFull(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("Clear", mock.Anything).Return(errors.ErrInternal.WithMessage("failed to clear queues"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/clear", nil)
		w := httptest.NewRecorder()

		handler.ClearQueue(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockQueue.AssertExpectations(t)
	})
}

func TestMatchHandler_PurgeInvalidMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("success", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandlerFull(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("PurgeInvalidMatches", mock.Anything, mock.AnythingOfType("func(string) bool")).Return(int64(7), nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/purge", nil)
		w := httptest.NewRecorder()

		handler.PurgeInvalidMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "Invalid matches purged successfully", response["message"])
		assert.Equal(t, float64(7), response["purged_count"])

		mockQueue.AssertExpectations(t)
	})

	t.Run("queue_manager_nil", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)

		handler := NewMatchHandler(mockRepo, mockCache, log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/purge", nil)
		w := httptest.NewRecorder()

		handler.PurgeInvalidMatches(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("service_error", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandlerFull(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("PurgeInvalidMatches", mock.Anything, mock.AnythingOfType("func(string) bool")).Return(int64(0), errors.ErrInternal.WithMessage("purge failed"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/purge", nil)
		w := httptest.NewRecorder()

		handler.PurgeInvalidMatches(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockQueue.AssertExpectations(t)
	})

	t.Run("zero_purged", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockQueue := new(MockMatchQueueManager)

		handler := NewMatchHandlerFull(mockRepo, mockCache, nil, mockQueue, log)

		mockQueue.On("PurgeInvalidMatches", mock.Anything, mock.AnythingOfType("func(string) bool")).Return(int64(0), nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/queue/purge", nil)
		w := httptest.NewRecorder()

		handler.PurgeInvalidMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, float64(0), response["purged_count"])

		mockQueue.AssertExpectations(t)
	})
}

func TestMatchHandler_ErrorFiltering(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("no_error_message", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchCompleted,
			ErrorMessage: nil,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)

		userID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Nil(t, response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("empty_error_message", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		emptyErr := ""
		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			ErrorMessage: &emptyErr,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)

		userID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		// Empty error message is treated as no error - returned as-is
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, "", *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("admin_sees_full_error", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		errorMsg := "runtime error: index out of bounds at line 42"
		winner := 1
		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			Winner:       &winner,
			ErrorMessage: &errorMsg,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)

		adminID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, adminID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleAdmin)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, errorMsg, *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("owner_sees_own_error", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		ownerID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()
		errorMsg := "segfault in user code at line 15"
		winner := 1 // Program1 won, so Program2 failed

		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			Winner:       &winner,
			ErrorMessage: &errorMsg,
		}

		// The failed program is program2 (winner=1 means program1 won)
		failedProgram := &domain.Program{
			ID:     program2ID,
			UserID: ownerID,
			Name:   "my-bot",
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)
		mockProgramLookup.On("GetByID", mock.Anything, program2ID).Return(failedProgram, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, ownerID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, errorMsg, *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockProgramLookup.AssertExpectations(t)
	})

	t.Run("non_owner_sees_generic", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		programOwnerID := uuid.New()
		otherUserID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()
		errorMsg := "segfault in user code at line 15"
		winner := 1 // Program1 won, so Program2 failed

		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			Winner:       &winner,
			ErrorMessage: &errorMsg,
		}

		// The failed program is program2 (winner=1 means program1 won)
		// The owner of program2 is programOwnerID, but the requesting user is otherUserID
		failedProgram := &domain.Program{
			ID:     program2ID,
			UserID: programOwnerID,
			Name:   "opponent-bot",
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)
		mockProgramLookup.On("GetByID", mock.Anything, program2ID).Return(failedProgram, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, otherUserID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, "Программа оппонента завершилась с ошибкой", *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockProgramLookup.AssertExpectations(t)
	})

	t.Run("winner_2_program1_failed_owner_sees_error", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		ownerID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()
		errorMsg := "timeout exceeded"
		winner := 2 // Program2 won, so Program1 failed

		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			Winner:       &winner,
			ErrorMessage: &errorMsg,
		}

		// The failed program is program1 (winner=2 means program2 won)
		failedProgram := &domain.Program{
			ID:     program1ID,
			UserID: ownerID,
			Name:   "my-bot",
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)
		mockProgramLookup.On("GetByID", mock.Anything, program1ID).Return(failedProgram, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, ownerID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, errorMsg, *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockProgramLookup.AssertExpectations(t)
	})

	t.Run("no_winner_error_hidden", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		userID := uuid.New()
		errorMsg := "both programs crashed"

		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			Winner:       nil, // No winner - cannot determine failed program
			ErrorMessage: &errorMsg,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.ErrorMessage)
		// No winner means we cannot determine failed program, so error is hidden
		assert.Equal(t, "Ошибка выполнения матча", *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("program_lookup_error_hides_message", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)
		mockProgramLookup := new(MockMatchProgramLookup)

		handler := NewMatchHandlerWithProgramLookup(mockRepo, mockCache, mockProgramLookup, log)

		matchID := uuid.New()
		userID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()
		errorMsg := "internal error details"
		winner := 1 // Program1 won, so Program2 failed

		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			Winner:       &winner,
			ErrorMessage: &errorMsg,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)
		// Program lookup fails - error message should be hidden
		mockProgramLookup.On("GetByID", mock.Anything, program2ID).Return(nil, errors.ErrNotFound.WithMessage("program not found"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.ErrorMessage)
		assert.Equal(t, "Ошибка выполнения матча", *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockProgramLookup.AssertExpectations(t)
	})

	t.Run("no_program_lookup_returns_as_is", func(t *testing.T) {
		mockRepo := new(MockMatchRepository)
		mockCache := new(MockMatchCache)

		// Handler without program lookup - errors should be returned as-is
		handler := NewMatchHandler(mockRepo, mockCache, log)

		matchID := uuid.New()
		errorMsg := "detailed error message"
		winner := 1

		match := &domain.Match{
			ID:           matchID,
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchFailed,
			Winner:       &winner,
			ErrorMessage: &errorMsg,
		}

		mockCache.On("Get", mock.Anything, matchID).Return(nil, nil)
		mockRepo.On("GetByID", mock.Anything, matchID).Return(match, nil)

		userID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID.String(), nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", matchID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.RoleKey, domain.RoleUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Match
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.ErrorMessage)
		// Without program lookup, filterMatchError returns match as-is
		assert.Equal(t, errorMsg, *response.ErrorMessage)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})
}
