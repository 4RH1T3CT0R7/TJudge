package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/team"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTeamService — заглушка TeamService на testify/mock
type MockTeamService struct {
	mock.Mock
}

func (m *MockTeamService) CreateTeam(ctx context.Context, req *team.CreateTeamRequest) (*models.Team, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) JoinTeamByCode(ctx context.Context, req *team.JoinTeamRequest) (*models.Team, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) LeaveTeam(ctx context.Context, teamID, userID uuid.UUID) error {
	return m.Called(ctx, teamID, userID).Error(0)
}

func (m *MockTeamService) RemoveMember(ctx context.Context, teamID, memberUserID, leaderID uuid.UUID) error {
	return m.Called(ctx, teamID, memberUserID, leaderID).Error(0)
}

func (m *MockTeamService) UpdateTeamName(ctx context.Context, teamID uuid.UUID, name string, leaderID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, teamID, name, leaderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) GetTeamByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) GetTeamByCode(ctx context.Context, code string) (*models.Team, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*models.TeamWithMembers, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TeamWithMembers), args.Error(1)
}

func (m *MockTeamService) GetTeamsByTournament(ctx context.Context, tournamentID uuid.UUID) ([]*models.Team, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Team), args.Error(1)
}

func (m *MockTeamService) GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, tournamentID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) GetInviteLink(ctx context.Context, teamID, leaderID uuid.UUID, baseURL string) (string, error) {
	args := m.Called(ctx, teamID, leaderID, baseURL)
	return args.String(0), args.Error(1)
}

func (m *MockTeamService) DeleteTeam(ctx context.Context, teamID uuid.UUID) error {
	return m.Called(ctx, teamID).Error(0)
}

func (m *MockTeamService) DisqualifyTeam(ctx context.Context, teamID uuid.UUID) (*team.DisqualifyResult, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*team.DisqualifyResult), args.Error(1)
}

func (m *MockTeamService) RestoreTeam(ctx context.Context, teamID uuid.UUID) error {
	return m.Called(ctx, teamID).Error(0)
}

func newTestTeamHandler() (*TeamHandler, *MockTeamService) {
	svc := new(MockTeamService)
	log, _ := logger.New("error", "json")
	return NewTeamHandler(svc, "http://localhost:8080", log), svc
}

func withUserID(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- Create ---

func TestTeamHandler_Create_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	teamID := uuid.New()
	tID := uuid.New()

	body, _ := json.Marshal(CreateTeamRequest{TournamentID: tID, Name: "My Team"})
	req := httptest.NewRequest("POST", "/api/v1/teams", bytes.NewReader(body))
	req = withUserID(req, userID)

	// имя и турнир из тела, автора берём из контекста
	svc.On("CreateTeam", mock.Anything, mock.MatchedBy(func(r *team.CreateTeamRequest) bool {
		return r.UserID == userID && r.Name == "My Team"
	})).Return(&models.Team{ID: teamID, Name: "My Team"}, nil)

	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestTeamHandler_Create_MissingUserID(t *testing.T) {
	h, _ := newTestTeamHandler()

	body, _ := json.Marshal(CreateTeamRequest{Name: "Test"})
	req := httptest.NewRequest("POST", "/api/v1/teams", bytes.NewReader(body))
	// user id в контекст не кладём

	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestTeamHandler_Create_Conflict(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()

	body, _ := json.Marshal(CreateTeamRequest{TournamentID: uuid.New(), Name: "Team"})
	req := httptest.NewRequest("POST", "/api/v1/teams", bytes.NewReader(body))
	req = withUserID(req, userID)

	// юзер уже в команде — сервис отдаёт conflict
	svc.On("CreateTeam", mock.Anything, mock.Anything).Return(nil, errors.ErrConflict.WithMessage("user already in a team"))

	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

// --- JoinByCode ---

func TestTeamHandler_JoinByCode_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()

	body, _ := json.Marshal(JoinByCodeRequest{Code: "ABC123"})
	req := httptest.NewRequest("POST", "/api/v1/teams/join", bytes.NewReader(body))
	req = withUserID(req, userID)

	svc.On("JoinTeamByCode", mock.Anything, mock.Anything).Return(&models.Team{ID: uuid.New()}, nil)

	rr := httptest.NewRecorder()
	h.JoinByCode(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTeamHandler_JoinByCode_NotFound(t *testing.T) {
	h, svc := newTestTeamHandler()

	body, _ := json.Marshal(JoinByCodeRequest{Code: "XXX"})
	req := httptest.NewRequest("POST", "/api/v1/teams/join", bytes.NewReader(body))
	req = withUserID(req, uuid.New())

	// невалидный инвайт-код — команды с таким кодом нет
	svc.On("JoinTeamByCode", mock.Anything, mock.Anything).Return(nil, errors.ErrNotFound)

	rr := httptest.NewRecorder()
	h.JoinByCode(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// --- Get ---

func TestTeamHandler_Get_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	teamID := uuid.New()

	req := httptest.NewRequest("GET", "/api/v1/teams/"+teamID.String(), nil)
	req = withChiParam(req, "id", teamID.String())

	svc.On("GetTeamWithMembers", mock.Anything, teamID).Return(&models.TeamWithMembers{
		Team: models.Team{ID: teamID, Name: "Test"},
	}, nil)

	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- UpdateName ---

func TestTeamHandler_UpdateName_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	teamID := uuid.New()

	body, _ := json.Marshal(UpdateNameRequest{Name: "New Name"})
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = withUserID(req, userID)
	req = withChiParam(req, "id", teamID.String())

	svc.On("UpdateTeamName", mock.Anything, teamID, "New Name", userID).Return(&models.Team{Name: "New Name"}, nil)

	rr := httptest.NewRecorder()
	h.UpdateName(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTeamHandler_UpdateName_Forbidden(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	teamID := uuid.New()

	body, _ := json.Marshal(UpdateNameRequest{Name: "New"})
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = withUserID(req, userID)
	req = withChiParam(req, "id", teamID.String())

	// переименовать может только лидер
	svc.On("UpdateTeamName", mock.Anything, teamID, "New", userID).Return(nil, errors.ErrForbidden)

	rr := httptest.NewRecorder()
	h.UpdateName(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// --- Leave ---

func TestTeamHandler_Leave_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	teamID := uuid.New()

	req := httptest.NewRequest("POST", "/", nil)
	req = withUserID(req, userID)
	req = withChiParam(req, "id", teamID.String())

	svc.On("LeaveTeam", mock.Anything, teamID, userID).Return(nil)

	rr := httptest.NewRecorder()
	h.Leave(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestTeamHandler_Leave_MissingUserID(t *testing.T) {
	h, _ := newTestTeamHandler()

	req := httptest.NewRequest("POST", "/", nil)
	req = withChiParam(req, "id", uuid.New().String())

	rr := httptest.NewRecorder()
	h.Leave(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// --- RemoveMember ---

func TestTeamHandler_RemoveMember_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	leaderID := uuid.New()
	teamID := uuid.New()
	memberID := uuid.New()

	req := httptest.NewRequest("DELETE", "/", nil)
	req = withUserID(req, leaderID)

	// исключающий — лидер, id команды и участника едут в урле
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", teamID.String())
	rctx.URLParams.Add("userId", memberID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	svc.On("RemoveMember", mock.Anything, teamID, memberID, leaderID).Return(nil)

	rr := httptest.NewRecorder()
	h.RemoveMember(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestTeamHandler_RemoveMember_Forbidden(t *testing.T) {
	h, svc := newTestTeamHandler()
	leaderID := uuid.New()
	teamID := uuid.New()
	memberID := uuid.New()

	req := httptest.NewRequest("DELETE", "/", nil)
	req = withUserID(req, leaderID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", teamID.String())
	rctx.URLParams.Add("userId", memberID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// исключает не лидер — сервис отдаёт forbidden
	svc.On("RemoveMember", mock.Anything, teamID, memberID, leaderID).Return(errors.ErrForbidden)

	rr := httptest.NewRecorder()
	h.RemoveMember(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// --- GetInviteLink ---

func TestTeamHandler_GetInviteLink_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	teamID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withUserID(req, userID)
	req = withChiParam(req, "id", teamID.String())

	svc.On("GetInviteLink", mock.Anything, teamID, userID, "http://localhost:8080").Return("http://localhost:8080/join/CODE", nil)
	svc.On("GetTeamByID", mock.Anything, teamID).Return(&models.Team{ID: teamID, Code: "CODE"}, nil)

	rr := httptest.NewRecorder()
	h.GetInviteLink(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "CODE")
}

func TestTeamHandler_GetInviteLink_Forbidden(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	teamID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withUserID(req, userID)
	req = withChiParam(req, "id", teamID.String())

	// ссылку видит только лидер
	svc.On("GetInviteLink", mock.Anything, teamID, userID, "http://localhost:8080").Return("", errors.ErrForbidden)

	rr := httptest.NewRecorder()
	h.GetInviteLink(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// --- GetTournamentTeams ---

func TestTeamHandler_GetTournamentTeams_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	tID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withChiParam(req, "id", tID.String())

	svc.On("GetTeamsByTournament", mock.Anything, tID).Return([]*models.Team{{Name: "A"}}, nil)

	rr := httptest.NewRecorder()
	h.GetTournamentTeams(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- GetMyTeam ---

func TestTeamHandler_GetMyTeam_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	tID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withUserID(req, userID)
	req = withChiParam(req, "id", tID.String())

	svc.On("GetUserTeamInTournament", mock.Anything, tID, userID).Return(&models.Team{Name: "My"}, nil)

	rr := httptest.NewRecorder()
	h.GetMyTeam(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTeamHandler_GetMyTeam_NoTeam(t *testing.T) {
	h, svc := newTestTeamHandler()
	userID := uuid.New()
	tID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withUserID(req, userID)
	req = withChiParam(req, "id", tID.String())

	svc.On("GetUserTeamInTournament", mock.Anything, tID, userID).Return(nil, errors.ErrNotFound)

	rr := httptest.NewRecorder()
	h.GetMyTeam(rr, req)

	// команды нет — это не ошибка, отдаём null
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"data":null`)
}

// --- Delete ---

func TestTeamHandler_Delete_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	teamID := uuid.New()

	req := httptest.NewRequest("DELETE", "/", nil)
	req = withChiParam(req, "id", teamID.String())

	svc.On("DeleteTeam", mock.Anything, teamID).Return(nil)

	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// --- GetMembers ---

func TestTeamHandler_GetMembers_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	teamID := uuid.New()
	userID := uuid.New()

	req := httptest.NewRequest("GET", "/", nil)
	req = withChiParam(req, "id", teamID.String())

	svc.On("GetTeamWithMembers", mock.Anything, teamID).Return(&models.TeamWithMembers{
		Team: models.Team{ID: teamID, Name: "Test"},
		Members: []models.User{
			{ID: userID, Username: "alice"},
		},
	}, nil)

	rr := httptest.NewRecorder()
	h.GetMembers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "alice")
}

// --- Disqualify ---

func TestTeamHandler_Disqualify_Success(t *testing.T) {
	h, svc := newTestTeamHandler()
	adminID := uuid.New()
	teamID := uuid.New()

	req := httptest.NewRequest("POST", "/", nil)
	req = withUserID(req, adminID)
	req = withChiParam(req, "id", teamID.String())

	// доступ к ручке уже отфильтрован RBAC-мидлварей, сервис просто гасит команду
	svc.On("DisqualifyTeam", mock.Anything, teamID).Return(&team.DisqualifyResult{}, nil)

	rr := httptest.NewRecorder()
	h.Disqualify(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
