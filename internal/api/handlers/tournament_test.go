package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/tournament"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTournamentService mocks the tournament service
type MockTournamentService struct {
	mock.Mock
}

func (m *MockTournamentService) Create(ctx context.Context, req *tournament.CreateRequest) (*domain.Tournament, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tournament), args.Error(1)
}

func (m *MockTournamentService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tournament), args.Error(1)
}

func (m *MockTournamentService) List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tournament), args.Error(1)
}

func (m *MockTournamentService) Join(ctx context.Context, req *tournament.JoinRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockTournamentService) Start(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

func (m *MockTournamentService) Complete(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

func (m *MockTournamentService) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	args := m.Called(ctx, tournamentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LeaderboardEntry), args.Error(1)
}

func (m *MockTournamentService) CreateMatch(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID, priority domain.MatchPriority) (*domain.Match, error) {
	args := m.Called(ctx, tournamentID, program1ID, program2ID, priority)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Match), args.Error(1)
}

func (m *MockTournamentService) GetMatches(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error) {
	args := m.Called(ctx, tournamentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

func (m *MockTournamentService) Delete(ctx context.Context, tournamentID uuid.UUID) error {
	args := m.Called(ctx, tournamentID)
	return args.Error(0)
}

func (m *MockTournamentService) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CrossGameLeaderboardEntry), args.Error(1)
}

func (m *MockTournamentService) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockTournamentService) RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockTournamentService) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MatchRound), args.Error(1)
}

func (m *MockTournamentService) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	args := m.Called(ctx, tournamentID, gameType)
	return args.Int(0), args.Error(1)
}

func TestTournamentHandler_Create(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully create tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		maxParticipants := 10
		reqBody := tournament.CreateRequest{
			Name:            "Test Tournament",
			GameType:        "chess",
			MaxParticipants: &maxParticipants,
		}

		expectedTournament := &domain.Tournament{
			ID:              uuid.New(),
			Name:            reqBody.Name,
			GameType:        reqBody.GameType,
			Status:          domain.TournamentPending,
			MaxParticipants: &maxParticipants,
		}

		mockService.On("Create", mock.Anything, &reqBody).Return(expectedTournament, nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response domain.Tournament
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, expectedTournament.ID, response.ID)
		assert.Equal(t, expectedTournament.Name, response.Name)

		mockService.AssertExpectations(t)
	})

	t.Run("validation error - empty name", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		reqBody := tournament.CreateRequest{
			Name:     "", // Invalid
			GameType: "chess",
		}

		mockService.On("Create", mock.Anything, &reqBody).Return(nil, errors.ErrValidation.WithMessage("name is required"))

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Create(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Get(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()
		expectedTournament := &domain.Tournament{
			ID:       tournamentID,
			Name:     "Test Tournament",
			GameType: "chess",
			Status:   domain.TournamentActive,
		}

		mockService.On("GetByID", mock.Anything, tournamentID).Return(expectedTournament, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String(), nil)

		// Set up Chi URL params
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.Tournament
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, expectedTournament.ID, response.ID)

		mockService.AssertExpectations(t)
	})

	t.Run("tournament not found", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()

		mockService.On("GetByID", mock.Anything, tournamentID).Return(nil, errors.ErrNotFound.WithMessage("tournament not found"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid UUID format", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/invalid-uuid", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Get(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTournamentHandler_List(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully list tournaments", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		expectedTournaments := []*domain.Tournament{
			{
				ID:       uuid.New(),
				Name:     "Tournament 1",
				GameType: "chess",
				Status:   domain.TournamentActive,
			},
			{
				ID:       uuid.New(),
				Name:     "Tournament 2",
				GameType: "chess",
				Status:   domain.TournamentPending,
			},
		}

		mockService.On("List", mock.Anything, mock.AnythingOfType("domain.TournamentFilter")).Return(expectedTournaments, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*domain.Tournament
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 2)

		mockService.AssertExpectations(t)
	})

	t.Run("list with filters", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		expectedTournaments := []*domain.Tournament{
			{
				ID:       uuid.New(),
				Name:     "Tournament 1",
				GameType: "chess",
				Status:   domain.TournamentActive,
			},
		}

		mockService.On("List", mock.Anything, mock.MatchedBy(func(filter domain.TournamentFilter) bool {
			return filter.Status == domain.TournamentActive && filter.GameType == "chess"
		})).Return(expectedTournaments, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments?status=active&game_type=chess", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Join(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully join tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()
		reqBody := tournament.JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		mockService.On("Join", mock.Anything, &reqBody).Return(nil)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/join", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Join(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("tournament already started", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()
		reqBody := tournament.JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		mockService.On("Join", mock.Anything, &reqBody).Return(errors.ErrTournamentStarted)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/join", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Join(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("tournament full", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()
		reqBody := tournament.JoinRequest{
			TournamentID: tournamentID,
			ProgramID:    uuid.New(),
		}

		mockService.On("Join", mock.Anything, &reqBody).Return(errors.ErrTournamentFull)

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/join", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Join(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Start(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully start tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()

		mockService.On("Start", mock.Anything, tournamentID).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/start", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Start(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("tournament already started", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()

		mockService.On("Start", mock.Anything, tournamentID).Return(errors.ErrConflict.WithMessage("tournament already started"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/start", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Start(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("insufficient participants", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()

		mockService.On("Start", mock.Anything, tournamentID).Return(errors.ErrValidation.WithMessage("needs at least 2 participants"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/start", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Start(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_GetLeaderboard(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get leaderboard", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()
		expectedLeaderboard := []*domain.LeaderboardEntry{
			{
				ProgramID: uuid.New(),
				Rating:    1800,
				Wins:      10,
				Losses:    2,
				Draws:     1,
			},
			{
				ProgramID: uuid.New(),
				Rating:    1700,
				Wins:      8,
				Losses:    4,
				Draws:     1,
			},
		}

		mockService.On("GetLeaderboard", mock.Anything, tournamentID, 100).Return(expectedLeaderboard, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/leaderboard", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetLeaderboard(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*domain.LeaderboardEntry
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
		assert.Equal(t, 1800, response[0].Rating)

		mockService.AssertExpectations(t)
	})

	t.Run("get leaderboard with custom limit", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, log)

		tournamentID := uuid.New()
		expectedLeaderboard := []*domain.LeaderboardEntry{
			{
				ProgramID: uuid.New(),
				Rating:    1800,
			},
		}

		mockService.On("GetLeaderboard", mock.Anything, tournamentID, 10).Return(expectedLeaderboard, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/leaderboard?limit=10", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetLeaderboard(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})
}
