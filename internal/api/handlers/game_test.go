package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/game"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGameService is a mock for GameService interface
type MockGameService struct {
	mock.Mock
}

func (m *MockGameService) Create(ctx context.Context, req *game.CreateRequest) (*domain.Game, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameService) GetByName(ctx context.Context, name string) (*domain.Game, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameService) List(ctx context.Context, filter domain.GameFilter) ([]*domain.Game, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameService) Update(ctx context.Context, id uuid.UUID, req *game.UpdateRequest) (*domain.Game, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameService) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGameService) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Game, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameService) AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	args := m.Called(ctx, tournamentID, gameID)
	return args.Error(0)
}

func (m *MockGameService) RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	args := m.Called(ctx, tournamentID, gameID)
	return args.Error(0)
}

// MockGameTournamentRepository is a mock for GameTournamentRepository
type MockGameTournamentRepository struct {
	mock.Mock
}

func (m *MockGameTournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tournament), args.Error(1)
}

func newTestGameHandler(t *testing.T) (*GameHandler, *MockGameService) {
	t.Helper()
	svc := new(MockGameService)
	log, _ := logger.New("error", "json")
	return NewGameHandler(svc, log), svc
}

func newTestGameHandlerWithTournamentRepo(t *testing.T) (*GameHandler, *MockGameService, *MockGameTournamentRepository) {
	t.Helper()
	svc := new(MockGameService)
	tournamentRepo := new(MockGameTournamentRepository)
	log, _ := logger.New("error", "json")
	handler := NewGameHandlerWithRepos(svc, nil, nil, tournamentRepo, log)
	return handler, svc, tournamentRepo
}

// --- Create ---

func TestGameHandler_Create_Success(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("Create", mock.Anything, mock.AnythingOfType("*game.CreateRequest")).
		Return(&domain.Game{ID: gameID, Name: "chess", DisplayName: "Chess"}, nil)

	body, _ := json.Marshal(game.CreateRequest{Name: "chess", DisplayName: "Chess"})
	req := httptest.NewRequest("POST", "/api/v1/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var result domain.Game
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, gameID, result.ID)
	assert.Equal(t, "chess", result.Name)
	svc.AssertExpectations(t)
}

func TestGameHandler_Create_InvalidJSON(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("POST", "/api/v1/games", bytes.NewReader([]byte("invalid")))
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_Create_ValidationError(t *testing.T) {
	handler, svc := newTestGameHandler(t)

	svc.On("Create", mock.Anything, mock.Anything).
		Return(nil, errors.ErrValidation.WithMessage("invalid name"))

	body, _ := json.Marshal(game.CreateRequest{Name: "INVALID"})
	req := httptest.NewRequest("POST", "/api/v1/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	svc.AssertExpectations(t)
}

func TestGameHandler_Create_Conflict(t *testing.T) {
	handler, svc := newTestGameHandler(t)

	svc.On("Create", mock.Anything, mock.Anything).
		Return(nil, errors.ErrConflict.WithMessage("game already exists"))

	body, _ := json.Marshal(game.CreateRequest{Name: "chess"})
	req := httptest.NewRequest("POST", "/api/v1/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	svc.AssertExpectations(t)
}

// --- List ---

func TestGameHandler_List_NoFilters(t *testing.T) {
	handler, svc := newTestGameHandler(t)

	games := []*domain.Game{
		{ID: uuid.New(), Name: "chess"},
		{ID: uuid.New(), Name: "tictactoe"},
	}
	svc.On("List", mock.Anything, domain.GameFilter{Limit: 50, Offset: 0}).
		Return(games, nil)

	req := httptest.NewRequest("GET", "/api/v1/games", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []*domain.Game
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Len(t, result, 2)
	svc.AssertExpectations(t)
}

func TestGameHandler_List_WithPagination(t *testing.T) {
	handler, svc := newTestGameHandler(t)

	svc.On("List", mock.Anything, domain.GameFilter{Limit: 10, Offset: 20}).
		Return([]*domain.Game{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/games?limit=10&offset=20", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGameHandler_List_WithNameFilter(t *testing.T) {
	handler, svc := newTestGameHandler(t)

	svc.On("List", mock.Anything, domain.GameFilter{Name: "chess", Limit: 50, Offset: 0}).
		Return([]*domain.Game{{ID: uuid.New(), Name: "chess"}}, nil)

	req := httptest.NewRequest("GET", "/api/v1/games?name=chess", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGameHandler_List_ServiceError(t *testing.T) {
	handler, svc := newTestGameHandler(t)

	svc.On("List", mock.Anything, mock.Anything).
		Return(nil, errors.ErrInternal)

	req := httptest.NewRequest("GET", "/api/v1/games", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	svc.AssertExpectations(t)
}

// --- Get ---

func TestGameHandler_Get_Success(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("GetByID", mock.Anything, gameID).
		Return(&domain.Game{ID: gameID, Name: "chess"}, nil)

	req := httptest.NewRequest("GET", "/api/v1/games/"+gameID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result domain.Game
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, gameID, result.ID)
	svc.AssertExpectations(t)
}

func TestGameHandler_Get_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/games/not-a-uuid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_Get_NotFound(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("GetByID", mock.Anything, gameID).
		Return(nil, errors.ErrNotFound)

	req := httptest.NewRequest("GET", "/api/v1/games/"+gameID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertExpectations(t)
}

// --- GetByName ---

func TestGameHandler_GetByName_Success(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("GetByName", mock.Anything, "chess").
		Return(&domain.Game{ID: gameID, Name: "chess"}, nil)

	req := httptest.NewRequest("GET", "/api/v1/games/name/chess", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "chess")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetByName(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGameHandler_GetByName_EmptyName(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/games/name/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetByName(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_GetByName_NotFound(t *testing.T) {
	handler, svc := newTestGameHandler(t)

	svc.On("GetByName", mock.Anything, "nonexistent").
		Return(nil, errors.ErrNotFound)

	req := httptest.NewRequest("GET", "/api/v1/games/name/nonexistent", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetByName(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertExpectations(t)
}

// --- Update ---

func TestGameHandler_Update_Success(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("Update", mock.Anything, gameID, mock.AnythingOfType("*game.UpdateRequest")).
		Return(&domain.Game{ID: gameID, Name: "chess", DisplayName: "Chess Updated"}, nil)

	body, _ := json.Marshal(game.UpdateRequest{DisplayName: "Chess Updated"})
	req := httptest.NewRequest("PUT", "/api/v1/games/"+gameID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Update(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result domain.Game
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, "Chess Updated", result.DisplayName)
	svc.AssertExpectations(t)
}

func TestGameHandler_Update_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	body, _ := json.Marshal(game.UpdateRequest{DisplayName: "Test"})
	req := httptest.NewRequest("PUT", "/api/v1/games/bad-uuid", bytes.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Update(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_Update_InvalidJSON(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	gameID := uuid.New()

	req := httptest.NewRequest("PUT", "/api/v1/games/"+gameID.String(), bytes.NewReader([]byte("invalid")))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Update(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_Update_NotFound(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("Update", mock.Anything, gameID, mock.Anything).
		Return(nil, errors.ErrNotFound)

	body, _ := json.Marshal(game.UpdateRequest{DisplayName: "Test"})
	req := httptest.NewRequest("PUT", "/api/v1/games/"+gameID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Update(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertExpectations(t)
}

// --- Delete ---

func TestGameHandler_Delete_Success(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("Delete", mock.Anything, gameID).Return(nil)

	req := httptest.NewRequest("DELETE", "/api/v1/games/"+gameID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	svc.AssertExpectations(t)
}

func TestGameHandler_Delete_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("DELETE", "/api/v1/games/bad", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_Delete_NotFound(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	gameID := uuid.New()

	svc.On("Delete", mock.Anything, gameID).Return(errors.ErrNotFound)

	req := httptest.NewRequest("DELETE", "/api/v1/games/"+gameID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertExpectations(t)
}

// --- GetTournamentGames ---

func TestGameHandler_GetTournamentGames_Success(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	tournamentID := uuid.New()

	games := []*domain.Game{
		{ID: uuid.New(), Name: "chess"},
	}
	svc.On("GetByTournamentID", mock.Anything, tournamentID).Return(games, nil)

	req := httptest.NewRequest("GET", "/api/v1/tournaments/"+tournamentID.String()+"/games", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetTournamentGames(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []*domain.Game
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Len(t, result, 1)
	svc.AssertExpectations(t)
}

func TestGameHandler_GetTournamentGames_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/tournaments/invalid/games", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetTournamentGames(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- AddGameToTournament ---

func TestGameHandler_AddGameToTournament_AdminSuccess(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	tournamentID := uuid.New()
	gameID := uuid.New()
	userID := uuid.New()

	svc.On("AddToTournament", mock.Anything, tournamentID, gameID).Return(nil)

	body, _ := json.Marshal(AddGameToTournamentRequest{GameID: gameID})
	req := httptest.NewRequest("POST", "/api/v1/tournaments/"+tournamentID.String()+"/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.RoleKey, "admin")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddGameToTournament(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	svc.AssertExpectations(t)
}

func TestGameHandler_AddGameToTournament_CreatorSuccess(t *testing.T) {
	handler, svc, tournamentRepo := newTestGameHandlerWithTournamentRepo(t)
	tournamentID := uuid.New()
	gameID := uuid.New()
	userID := uuid.New()

	tournamentRepo.On("GetByID", mock.Anything, tournamentID).
		Return(&domain.Tournament{ID: tournamentID, CreatorID: &userID}, nil)
	svc.On("AddToTournament", mock.Anything, tournamentID, gameID).Return(nil)

	body, _ := json.Marshal(AddGameToTournamentRequest{GameID: gameID})
	req := httptest.NewRequest("POST", "/api/v1/tournaments/"+tournamentID.String()+"/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.RoleKey, "user")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddGameToTournament(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	svc.AssertExpectations(t)
	tournamentRepo.AssertExpectations(t)
}

func TestGameHandler_AddGameToTournament_Forbidden(t *testing.T) {
	handler, _, tournamentRepo := newTestGameHandlerWithTournamentRepo(t)
	tournamentID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()

	tournamentRepo.On("GetByID", mock.Anything, tournamentID).
		Return(&domain.Tournament{ID: tournamentID, CreatorID: &otherUserID}, nil)

	body, _ := json.Marshal(AddGameToTournamentRequest{GameID: uuid.New()})
	req := httptest.NewRequest("POST", "/api/v1/tournaments/"+tournamentID.String()+"/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.RoleKey, "user")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddGameToTournament(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	tournamentRepo.AssertExpectations(t)
}

func TestGameHandler_AddGameToTournament_MissingUserID(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()

	body, _ := json.Marshal(AddGameToTournamentRequest{GameID: uuid.New()})
	req := httptest.NewRequest("POST", "/api/v1/tournaments/"+tournamentID.String()+"/games", bytes.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.AddGameToTournament(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGameHandler_AddGameToTournament_InvalidTournamentUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("POST", "/api/v1/tournaments/invalid/games", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.AddGameToTournament(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- RemoveGameFromTournament ---

func TestGameHandler_RemoveGameFromTournament_Success(t *testing.T) {
	handler, svc := newTestGameHandler(t)
	tournamentID := uuid.New()
	gameID := uuid.New()

	svc.On("RemoveFromTournament", mock.Anything, tournamentID, gameID).Return(nil)

	req := httptest.NewRequest("DELETE", "/api/v1/tournaments/"+tournamentID.String()+"/games/"+gameID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.RemoveGameFromTournament(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	svc.AssertExpectations(t)
}

func TestGameHandler_RemoveGameFromTournament_InvalidTournamentUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("DELETE", "/api/v1/tournaments/invalid/games/"+uuid.New().String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	rctx.URLParams.Add("gameId", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.RemoveGameFromTournament(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_RemoveGameFromTournament_InvalidGameUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()

	req := httptest.NewRequest("DELETE", "/api/v1/tournaments/"+tournamentID.String()+"/games/bad", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.RemoveGameFromTournament(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- GetGameLeaderboard ---

func TestGameHandler_GetGameLeaderboard_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetGameLeaderboard(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "leaderboard repository not configured")
}

func TestGameHandler_GetGameLeaderboard_InvalidTournamentUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	rctx.URLParams.Add("gameId", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetGameLeaderboard(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_GetGameLeaderboard_InvalidGameUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	rctx.URLParams.Add("gameId", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetGameLeaderboard(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- GetGameMatches ---

func TestGameHandler_GetGameMatches_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetGameMatches(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "match repository not configured")
}

// --- GetGamePrograms ---

func TestGameHandler_GetGamePrograms_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetGamePrograms(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "program repository not configured")
}

func TestGameHandler_GetGamePrograms_InvalidTournamentUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad")
	rctx.URLParams.Add("gameId", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetGamePrograms(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_GetGamePrograms_InvalidGameUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	rctx.URLParams.Add("gameId", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetGamePrograms(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- GetTournamentGamesWithStatus ---

func TestGameHandler_GetTournamentGamesWithStatus_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetTournamentGamesWithStatus(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGameHandler_GetTournamentGamesWithStatus_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetTournamentGamesWithStatus(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- MarkGameRoundCompleted ---

func TestGameHandler_MarkGameRoundCompleted_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.MarkGameRoundCompleted(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- SetActiveGame ---

func TestGameHandler_SetActiveGame_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.SetActiveGame(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGameHandler_SetActiveGame_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.SetActiveGame(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- DeactivateAllGames ---

func TestGameHandler_DeactivateAllGames_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.DeactivateAllGames(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGameHandler_DeactivateAllGames_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.DeactivateAllGames(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- GetActiveGame ---

func TestGameHandler_GetActiveGame_NoRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetActiveGame(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGameHandler_GetActiveGame_InvalidUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.GetActiveGame(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- ResetGameRound ---

func TestGameHandler_ResetGameRound_NoTournamentGameStatusRepo(t *testing.T) {
	handler, _ := newTestGameHandler(t)
	tournamentID := uuid.New()
	gameID := uuid.New()

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tournamentID.String())
	rctx.URLParams.Add("gameId", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.ResetGameRound(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "tournament game status repository not configured")
}

func TestGameHandler_ResetGameRound_InvalidTournamentUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad")
	rctx.URLParams.Add("gameId", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.ResetGameRound(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGameHandler_ResetGameRound_InvalidGameUUID(t *testing.T) {
	handler, _ := newTestGameHandler(t)

	req := httptest.NewRequest("POST", "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	rctx.URLParams.Add("gameId", "bad")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	handler.ResetGameRound(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
