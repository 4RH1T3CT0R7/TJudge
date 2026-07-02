package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRatingHistoryRepository struct {
	mock.Mock
}

func (m *MockRatingHistoryRepository) GetByProgramAndTournament(ctx context.Context, programID, tournamentID uuid.UUID, limit int) ([]*domain.RatingHistory, error) {
	args := m.Called(ctx, programID, tournamentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RatingHistory), args.Error(1)
}

func newTestRatingHistoryHandler(t *testing.T) (*RatingHistoryHandler, *MockRatingHistoryRepository) {
	t.Helper()
	repo := new(MockRatingHistoryRepository)
	log, _ := logger.New("error", "json")
	return NewRatingHistoryHandler(repo, log), repo
}

func ratingHistoryRequest(tournamentID, programID string) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID)
	rctx.URLParams.Add("programId", programID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestRatingHistoryHandler_Success(t *testing.T) {
	handler, repo := newTestRatingHistoryHandler(t)
	tournamentID := uuid.New()
	programID := uuid.New()

	repo.On("GetByProgramAndTournament", mock.Anything, programID, tournamentID, 200).
		Return([]*domain.RatingHistory{
			{ProgramID: programID, TournamentID: tournamentID, OldRating: 1500, NewRating: 1516, Change: 16},
			{ProgramID: programID, TournamentID: tournamentID, OldRating: 1516, NewRating: 1508, Change: -8},
		}, nil)

	rr := httptest.NewRecorder()
	handler.GetProgramRatingHistory(rr, ratingHistoryRequest(tournamentID.String(), programID.String()))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []*domain.RatingHistory
	decodeJSONData(t, rr.Body, &result)
	assert.Len(t, result, 2)
	assert.Equal(t, 16, result[0].Change)
	repo.AssertExpectations(t)
}

func TestRatingHistoryHandler_InvalidUUIDs(t *testing.T) {
	handler, _ := newTestRatingHistoryHandler(t)

	rr := httptest.NewRecorder()
	handler.GetProgramRatingHistory(rr, ratingHistoryRequest("not-a-uuid", uuid.New().String()))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	handler.GetProgramRatingHistory(rr, ratingHistoryRequest(uuid.New().String(), "not-a-uuid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRatingHistoryHandler_RepoError(t *testing.T) {
	handler, repo := newTestRatingHistoryHandler(t)
	tournamentID := uuid.New()
	programID := uuid.New()

	repo.On("GetByProgramAndTournament", mock.Anything, programID, tournamentID, 200).
		Return(nil, assert.AnError)

	rr := httptest.NewRecorder()
	handler.GetProgramRatingHistory(rr, ratingHistoryRequest(tournamentID.String(), programID.String()))

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	repo.AssertExpectations(t)
}

// --- GetHeadToHead (живёт в GameRoundHandler, мок расширен в game_test.go) ---

func TestGameHandler_GetHeadToHead_Success(t *testing.T) {
	handler, svc, leaderboardRepo, _, _, _ := newGameHandlerWithAllRepos(t)
	tournamentID := uuid.New()
	gameID := uuid.New()
	teamA, teamB := uuid.New(), uuid.New()

	svc.On("GetByID", mock.Anything, gameID).
		Return(&domain.Game{ID: gameID, Name: "dilemma"}, nil)
	leaderboardRepo.On("GetHeadToHead", mock.Anything, tournamentID, "dilemma").
		Return([]*domain.HeadToHeadCell{
			{TeamID: teamA, TeamName: "alpha", OpponentID: teamB, OpponentName: "beta", Wins: 2, Losses: 1, Draws: 1},
		}, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetHeadToHead(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []*domain.HeadToHeadCell
	decodeJSONData(t, rr.Body, &result)
	assert.Len(t, result, 1)
	assert.Equal(t, "alpha", result[0].TeamName)
	assert.Equal(t, 2, result[0].Wins)
	svc.AssertExpectations(t)
	leaderboardRepo.AssertExpectations(t)
}

func TestGameHandler_GetHeadToHead_GameNotFound(t *testing.T) {
	handler, svc, _, _, _, _ := newGameHandlerWithAllRepos(t)
	gameID := uuid.New()

	svc.On("GetByID", mock.Anything, gameID).Return(nil, errors.ErrNotFound)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetHeadToHead(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertExpectations(t)
}
