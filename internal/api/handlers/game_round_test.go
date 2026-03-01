package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

type MockGameRoundLookupService struct {
	mock.Mock
}

func (m *MockGameRoundLookupService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

type MockGameMatchRepo struct {
	mock.Mock
}

func (m *MockGameMatchRepo) List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Match), args.Error(1)
}

type MockTournamentGameStatusRepo struct {
	mock.Mock
}

func (m *MockTournamentGameStatusRepo) GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGame, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TournamentGame), args.Error(1)
}

func (m *MockTournamentGameStatusRepo) GetTournamentGamesWithDetails(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGameWithDetails, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TournamentGameWithDetails), args.Error(1)
}

func (m *MockTournamentGameStatusRepo) MarkRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	return m.Called(ctx, tournamentID, gameID).Error(0)
}

func (m *MockTournamentGameStatusRepo) SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	return m.Called(ctx, tournamentID, gameID).Error(0)
}

func (m *MockTournamentGameStatusRepo) GetActiveGame(ctx context.Context, tournamentID uuid.UUID) (*domain.TournamentGame, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TournamentGame), args.Error(1)
}

func (m *MockTournamentGameStatusRepo) ResetGameRound(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	return m.Called(ctx, tournamentID, gameID).Error(0)
}

func (m *MockTournamentGameStatusRepo) ResetGameRoundFull(ctx context.Context, tournamentID, gameID uuid.UUID, gameType string) (int64, int64, int64, error) {
	args := m.Called(ctx, tournamentID, gameID, gameType)
	return args.Get(0).(int64), args.Get(1).(int64), args.Get(2).(int64), args.Error(3)
}

func (m *MockTournamentGameStatusRepo) DeactivateAllGames(ctx context.Context, tournamentID uuid.UUID) error {
	return m.Called(ctx, tournamentID).Error(0)
}

// --- Helpers ---

func newTestGameRoundHandler(
	gameSvc *MockGameRoundLookupService,
	matchRepo *MockGameMatchRepo,
	statusRepo *MockTournamentGameStatusRepo,
) *GameRoundHandler {
	log, _ := logger.New("error", "json")

	var mr GameMatchRepository
	if matchRepo != nil {
		mr = matchRepo
	}

	var sr TournamentGameStatusRepository
	if statusRepo != nil {
		sr = statusRepo
	}

	return NewGameRoundHandler(gameSvc, nil, mr, nil, sr, events.NoopBus{}, log)
}

func withTwoChiParams(r *http.Request, key1, val1, key2, val2 string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key1, val1)
	rctx.URLParams.Add(key2, val2)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// --- GetGameMatches ---

func TestGameRoundHandler_GetGameMatches_Success(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	matchRepo := new(MockGameMatchRepo)
	h := newTestGameRoundHandler(gameSvc, matchRepo, nil)

	tournamentID := uuid.New()
	gameID := uuid.New()
	matchID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withTwoChiParams(req, "id", tournamentID.String(), "gameId", gameID.String())

	gameSvc.On("GetByID", mock.Anything, gameID).Return(&domain.Game{
		ID:   gameID,
		Name: "prisoners_dilemma",
	}, nil)

	matchRepo.On("List", mock.Anything, mock.MatchedBy(func(f domain.MatchFilter) bool {
		return f.TournamentID != nil && *f.TournamentID == tournamentID && f.GameType == "prisoners_dilemma"
	})).Return([]*domain.Match{
		{ID: matchID, TournamentID: tournamentID, GameType: "prisoners_dilemma"},
	}, nil)

	rr := httptest.NewRecorder()
	h.GetGameMatches(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), matchID.String())
}

func TestGameRoundHandler_GetGameMatches_InvalidTournamentID(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	matchRepo := new(MockGameMatchRepo)
	h := newTestGameRoundHandler(gameSvc, matchRepo, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req = withTwoChiParams(req, "id", "bad", "gameId", uuid.New().String())

	rr := httptest.NewRecorder()
	h.GetGameMatches(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameRoundHandler_GetGameMatches_InvalidGameID(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	matchRepo := new(MockGameMatchRepo)
	h := newTestGameRoundHandler(gameSvc, matchRepo, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req = withTwoChiParams(req, "id", uuid.New().String(), "gameId", "bad")

	rr := httptest.NewRecorder()
	h.GetGameMatches(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameRoundHandler_GetGameMatches_GameNotFound(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	matchRepo := new(MockGameMatchRepo)
	h := newTestGameRoundHandler(gameSvc, matchRepo, nil)

	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withTwoChiParams(req, "id", tournamentID.String(), "gameId", gameID.String())

	gameSvc.On("GetByID", mock.Anything, gameID).Return(nil, errors.ErrNotFound)

	rr := httptest.NewRecorder()
	h.GetGameMatches(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGameRoundHandler_GetGameMatches_NilMatchRepo(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	h := newTestGameRoundHandler(gameSvc, nil, nil)

	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withTwoChiParams(req, "id", tournamentID.String(), "gameId", gameID.String())

	rr := httptest.NewRecorder()
	h.GetGameMatches(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- GetActiveGame ---

func TestGameRoundHandler_GetActiveGame_Success(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	statusRepo := new(MockTournamentGameStatusRepo)
	h := newTestGameRoundHandler(gameSvc, nil, statusRepo)

	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	statusRepo.On("GetActiveGame", mock.Anything, tournamentID).Return(&domain.TournamentGame{
		TournamentID: tournamentID,
		GameID:       gameID,
		IsActive:     true,
		CurrentRound: 1,
	}, nil)

	gameSvc.On("GetByID", mock.Anything, gameID).Return(&domain.Game{
		ID:          gameID,
		Name:        "tug_of_war",
		DisplayName: "Tug of War",
	}, nil)

	rr := httptest.NewRecorder()
	h.GetActiveGame(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "tug_of_war")
	assert.Contains(t, rr.Body.String(), "Tug of War")
}

func TestGameRoundHandler_GetActiveGame_NoActiveGame(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	statusRepo := new(MockTournamentGameStatusRepo)
	h := newTestGameRoundHandler(gameSvc, nil, statusRepo)

	tournamentID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	statusRepo.On("GetActiveGame", mock.Anything, tournamentID).Return(nil, errors.ErrNotFound)

	rr := httptest.NewRecorder()
	h.GetActiveGame(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "null")
}

func TestGameRoundHandler_GetActiveGame_InvalidID(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	statusRepo := new(MockTournamentGameStatusRepo)
	h := newTestGameRoundHandler(gameSvc, nil, statusRepo)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad-uuid")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.GetActiveGame(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameRoundHandler_GetActiveGame_NilStatusRepo(t *testing.T) {
	gameSvc := new(MockGameRoundLookupService)
	h := newTestGameRoundHandler(gameSvc, nil, nil)

	tournamentID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.GetActiveGame(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
