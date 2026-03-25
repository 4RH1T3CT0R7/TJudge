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

func (m *MockTournamentService) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MatchRound), args.Error(1)
}

// MockSchedulingService mocks the scheduling service
type MockSchedulingService struct {
	mock.Mock
}

func (m *MockSchedulingService) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockSchedulingService) RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	args := m.Called(ctx, tournamentID)
	return args.Int(0), args.Error(1)
}

func (m *MockSchedulingService) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	args := m.Called(ctx, tournamentID, gameType)
	return args.Int(0), args.Error(1)
}

func TestTournamentHandler_Create(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully create tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedTournament.ID, response.ID)
		assert.Equal(t, expectedTournament.Name, response.Name)

		mockService.AssertExpectations(t)
	})

	t.Run("validation error - empty name", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedTournament.ID, response.ID)

		mockService.AssertExpectations(t)
	})

	t.Run("tournament not found", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)

		mockService.AssertExpectations(t)
	})

	t.Run("list with filters", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)
		assert.Equal(t, 1800, response[0].Rating)

		mockService.AssertExpectations(t)
	})

	t.Run("get leaderboard with custom limit", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

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

func TestTournamentHandler_Complete(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully complete tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Complete", mock.Anything, tournamentID).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/complete", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Complete(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "completed", response["status"])

		mockService.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/invalid-uuid/complete", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Complete(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("tournament not active", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Complete", mock.Anything, tournamentID).Return(errors.ErrConflict.WithMessage("tournament is not active"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/complete", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Complete(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_Delete(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully delete tournament", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Delete", mock.Anything, tournamentID).Return(nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tournaments/"+tournamentID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())

		mockService.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tournaments/invalid-uuid", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("active tournament cannot be deleted", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("Delete", mock.Anything, tournamentID).Return(errors.ErrConflict.WithMessage("cannot delete active tournament"))

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tournaments/"+tournamentID.String(), nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_RunAllMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully run all matches", func(t *testing.T) {
		mockService := new(MockTournamentService)
		mockScheduling := new(MockSchedulingService)
		handler := NewTournamentHandler(mockService, mockScheduling, log)

		tournamentID := uuid.New()

		mockScheduling.On("RunAllMatches", mock.Anything, tournamentID).Return(15, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/run-matches", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RunAllMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "started", response["status"])
		assert.Equal(t, float64(15), response["enqueued"])

		mockScheduling.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/invalid-uuid/run-matches", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RunAllMatches(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("tournament not active", func(t *testing.T) {
		mockService := new(MockTournamentService)
		mockScheduling := new(MockSchedulingService)
		handler := NewTournamentHandler(mockService, mockScheduling, log)

		tournamentID := uuid.New()

		mockScheduling.On("RunAllMatches", mock.Anything, tournamentID).Return(0, errors.ErrConflict.WithMessage("tournament is not active"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/run-matches", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RunAllMatches(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockScheduling.AssertExpectations(t)
	})
}

func TestTournamentHandler_RunGameMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully run game matches", func(t *testing.T) {
		mockService := new(MockTournamentService)
		mockScheduling := new(MockSchedulingService)
		handler := NewTournamentHandler(mockService, mockScheduling, log)

		tournamentID := uuid.New()

		mockScheduling.On("RunGameMatches", mock.Anything, tournamentID, "prisoners_dilemma").Return(8, nil)

		body, _ := json.Marshal(map[string]string{"game_type": "prisoners_dilemma"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/games/run-matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RunGameMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "started", response["status"])
		assert.Equal(t, "prisoners_dilemma", response["game_type"])
		assert.Equal(t, float64(8), response["enqueued"])

		mockScheduling.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		body, _ := json.Marshal(map[string]string{"game_type": "prisoners_dilemma"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/invalid-uuid/games/run-matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RunGameMatches(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing game_type in body", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/games/run-matches", bytes.NewBuffer([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RunGameMatches(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty game_type", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		body, _ := json.Marshal(map[string]string{"game_type": ""})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/games/run-matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RunGameMatches(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTournamentHandler_RetryFailedMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully retry failed matches", func(t *testing.T) {
		mockService := new(MockTournamentService)
		mockScheduling := new(MockSchedulingService)
		handler := NewTournamentHandler(mockService, mockScheduling, log)

		tournamentID := uuid.New()

		mockScheduling.On("RetryFailedMatches", mock.Anything, tournamentID).Return(3, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/retry-matches", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RetryFailedMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, "retried", response["status"])
		assert.Equal(t, float64(3), response["enqueued"])

		mockScheduling.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/invalid-uuid/retry-matches", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.RetryFailedMatches(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTournamentHandler_GetCrossGameLeaderboard(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get cross-game leaderboard", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedEntries := []*domain.CrossGameLeaderboardEntry{
			{
				Rank:        1,
				TeamName:    "Team Alpha",
				ProgramID:   uuid.New(),
				ProgramName: "AlphaBot",
				TotalRating: 3600,
				TotalWins:   20,
				TotalLosses: 5,
				TotalGames:  25,
			},
			{
				Rank:        2,
				TeamName:    "Team Beta",
				ProgramID:   uuid.New(),
				ProgramName: "BetaBot",
				TotalRating: 3200,
				TotalWins:   15,
				TotalLosses: 10,
				TotalGames:  25,
			},
		}

		mockService.On("GetCrossGameLeaderboard", mock.Anything, tournamentID).Return(expectedEntries, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/cross-game-leaderboard", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetCrossGameLeaderboard(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*domain.CrossGameLeaderboardEntry
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)
		assert.Equal(t, 3600, response[0].TotalRating)
		assert.Equal(t, "Team Alpha", response[0].TeamName)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/invalid-uuid/cross-game-leaderboard", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetCrossGameLeaderboard(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		mockService.On("GetCrossGameLeaderboard", mock.Anything, tournamentID).Return(nil, errors.ErrInternal.WithMessage("database error"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/cross-game-leaderboard", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetCrossGameLeaderboard(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestTournamentHandler_GetMatches(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get matches with defaults", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedMatches := []*domain.Match{
			{
				ID:           uuid.New(),
				TournamentID: tournamentID,
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "prisoners_dilemma",
				Status:       domain.MatchCompleted,
			},
			{
				ID:           uuid.New(),
				TournamentID: tournamentID,
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "prisoners_dilemma",
				Status:       domain.MatchPending,
			},
		}

		mockService.On("GetMatches", mock.Anything, tournamentID, 50, 0).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/matches", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*domain.Match
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)

		mockService.AssertExpectations(t)
	})

	t.Run("with pagination parameters", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedMatches := []*domain.Match{
			{
				ID:           uuid.New(),
				TournamentID: tournamentID,
				Program1ID:   uuid.New(),
				Program2ID:   uuid.New(),
				GameType:     "tug_of_war",
				Status:       domain.MatchCompleted,
			},
		}

		mockService.On("GetMatches", mock.Anything, tournamentID, 10, 20).Return(expectedMatches, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/matches?limit=10&offset=20", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetMatches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*domain.Match
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 1)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/invalid-uuid/matches", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetMatches(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTournamentHandler_GetMatchesByRounds(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("successfully get matches by rounds", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		expectedRounds := []*domain.MatchRound{
			{
				RoundNumber:    1,
				GameType:       "prisoners_dilemma",
				TotalMatches:   6,
				CompletedCount: 6,
				PendingCount:   0,
				RunningCount:   0,
				FailedCount:    0,
				Matches: []*domain.Match{
					{
						ID:           uuid.New(),
						TournamentID: tournamentID,
						RoundNumber:  1,
						Status:       domain.MatchCompleted,
					},
				},
			},
			{
				RoundNumber:    2,
				GameType:       "prisoners_dilemma",
				TotalMatches:   6,
				CompletedCount: 3,
				PendingCount:   2,
				RunningCount:   1,
				FailedCount:    0,
				Matches: []*domain.Match{
					{
						ID:           uuid.New(),
						TournamentID: tournamentID,
						RoundNumber:  2,
						Status:       domain.MatchRunning,
					},
				},
			},
		}

		mockService.On("GetMatchesByRounds", mock.Anything, tournamentID).Return(expectedRounds, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/"+tournamentID.String()+"/matches/rounds", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetMatchesByRounds(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []*domain.MatchRound
		decodeJSONData(t, w.Body, &response)
		assert.Len(t, response, 2)
		assert.Equal(t, 1, response[0].RoundNumber)
		assert.Equal(t, 6, response[0].TotalMatches)
		assert.Equal(t, 2, response[1].RoundNumber)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments/invalid-uuid/matches/rounds", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetMatchesByRounds(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTournamentHandler_CreateMatch(t *testing.T) {
	log, _ := logger.New("error", "json")

	t.Run("success with explicit priority", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()

		expectedMatch := &domain.Match{
			ID:           uuid.New(),
			TournamentID: tournamentID,
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			Priority:     domain.PriorityHigh,
		}

		mockService.On("CreateMatch", mock.Anything, tournamentID, program1ID, program2ID, domain.PriorityHigh).Return(expectedMatch, nil)

		body, _ := json.Marshal(map[string]interface{}{
			"program1_id": program1ID,
			"program2_id": program2ID,
			"priority":    "high",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.CreateMatch(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response domain.Match
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedMatch.ID, response.ID)
		assert.Equal(t, expectedMatch.TournamentID, response.TournamentID)
		assert.Equal(t, expectedMatch.Program1ID, response.Program1ID)
		assert.Equal(t, expectedMatch.Program2ID, response.Program2ID)

		mockService.AssertExpectations(t)
	})

	t.Run("success with default priority", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()

		expectedMatch := &domain.Match{
			ID:           uuid.New(),
			TournamentID: tournamentID,
			Program1ID:   program1ID,
			Program2ID:   program2ID,
			Priority:     domain.PriorityMedium,
		}

		mockService.On("CreateMatch", mock.Anything, tournamentID, program1ID, program2ID, domain.PriorityMedium).Return(expectedMatch, nil)

		body, _ := json.Marshal(map[string]interface{}{
			"program1_id": program1ID,
			"program2_id": program2ID,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.CreateMatch(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response domain.Match
		decodeJSONData(t, w.Body, &response)
		assert.Equal(t, expectedMatch.ID, response.ID)
		assert.Equal(t, domain.PriorityMedium, response.Priority)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid tournament UUID", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		body, _ := json.Marshal(map[string]interface{}{
			"program1_id": uuid.New(),
			"program2_id": uuid.New(),
			"priority":    "high",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/not-a-uuid/matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "not-a-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.CreateMatch(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/matches", bytes.NewBuffer([]byte("{invalid json")))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.CreateMatch(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockTournamentService)
		handler := NewTournamentHandler(mockService, new(MockSchedulingService), log)

		tournamentID := uuid.New()
		program1ID := uuid.New()
		program2ID := uuid.New()

		mockService.On("CreateMatch", mock.Anything, tournamentID, program1ID, program2ID, domain.PriorityMedium).Return(nil, errors.ErrNotFound.WithMessage("tournament not found"))

		body, _ := json.Marshal(map[string]interface{}{
			"program1_id": program1ID,
			"program2_id": program2ID,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments/"+tournamentID.String()+"/matches", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tournamentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.CreateMatch(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockService.AssertExpectations(t)
	})
}
