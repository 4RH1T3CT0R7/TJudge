package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
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
