package team

import (
	"context"
	"fmt"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockTeamRepository struct {
	mock.Mock
}

func (m *MockTeamRepository) Create(ctx context.Context, team *domain.Team) error {
	return m.Called(ctx, team).Error(0)
}

func (m *MockTeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Team, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Team), args.Error(1)
}

func (m *MockTeamRepository) GetByCode(ctx context.Context, code string) (*domain.Team, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Team), args.Error(1)
}

func (m *MockTeamRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Team, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Team), args.Error(1)
}

func (m *MockTeamRepository) List(ctx context.Context, filter domain.TeamFilter) ([]*domain.Team, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Team), args.Error(1)
}

func (m *MockTeamRepository) Update(ctx context.Context, team *domain.Team) error {
	return m.Called(ctx, team).Error(0)
}

func (m *MockTeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockTeamRepository) AddMember(ctx context.Context, member *domain.TeamMember) error {
	return m.Called(ctx, member).Error(0)
}

func (m *MockTeamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	return m.Called(ctx, teamID, userID).Error(0)
}

func (m *MockTeamRepository) GetMembers(ctx context.Context, teamID uuid.UUID) ([]*domain.TeamMember, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TeamMember), args.Error(1)
}

func (m *MockTeamRepository) GetMemberCount(ctx context.Context, teamID uuid.UUID) (int, error) {
	args := m.Called(ctx, teamID)
	return args.Int(0), args.Error(1)
}

func (m *MockTeamRepository) IsUserInTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTeamRepository) IsUserInAnyTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, tournamentID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTeamRepository) GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*domain.Team, error) {
	args := m.Called(ctx, tournamentID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Team), args.Error(1)
}

func (m *MockTeamRepository) GenerateUniqueCode(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockTeamRepository) GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*domain.TeamWithMembers, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TeamWithMembers), args.Error(1)
}

type MockTournamentRepository struct {
	mock.Mock
}

func (m *MockTournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tournament), args.Error(1)
}

func newTestTeamService(t *testing.T) (*Service, *MockTeamRepository, *MockTournamentRepository) {
	teamRepo := new(MockTeamRepository)
	tournamentRepo := new(MockTournamentRepository)
	log, _ := logger.New("error", "json")
	return NewService(teamRepo, tournamentRepo, log), teamRepo, tournamentRepo
}

// --- CreateTeam ---

func TestService_CreateTeam_Success(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID := uuid.New()
	userID := uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{
		ID: tID, Status: domain.TournamentPending,
	}, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("GenerateUniqueCode", ctx).Return("ABC123", nil)
	teamRepo.On("Create", ctx, mock.AnythingOfType("*domain.Team")).Return(nil)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*domain.TeamMember")).Return(nil)

	team, err := svc.CreateTeam(ctx, &CreateTeamRequest{
		TournamentID: tID, Name: "My Team", UserID: userID,
	})

	require.NoError(t, err)
	assert.Equal(t, "My Team", team.Name)
	assert.Equal(t, userID, team.LeaderID)
	teamRepo.AssertExpectations(t)
	tournamentRepo.AssertExpectations(t)
}

func TestService_CreateTeam_TournamentNotFound(t *testing.T) {
	svc, _, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID := uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(nil, errors.ErrNotFound)

	_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: uuid.New()})
	assert.Error(t, err)
}

func TestService_CreateTeam_TournamentNotPending(t *testing.T) {
	for _, status := range []domain.TournamentStatus{domain.TournamentActive, domain.TournamentCompleted, domain.TournamentCancelled} {
		t.Run(string(status), func(t *testing.T) {
			svc, _, tournamentRepo := newTestTeamService(t)
			ctx := context.Background()
			tID := uuid.New()

			tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: status}, nil)

			_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: uuid.New()})
			assert.Error(t, err)
		})
	}
}

func TestService_CreateTeam_UserAlreadyInTeam(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID, userID := uuid.New(), uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(true, nil)

	_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: userID})
	assert.Error(t, err)
}

func TestService_CreateTeam_CodeGenerationError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID, userID := uuid.New(), uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("GenerateUniqueCode", ctx).Return("", errors.ErrInternal)

	_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: userID})
	assert.Error(t, err)
}

func TestService_CreateTeam_CreateError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID, userID := uuid.New(), uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("GenerateUniqueCode", ctx).Return("CODE", nil)
	teamRepo.On("Create", ctx, mock.Anything).Return(errors.ErrInternal)

	_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: userID})
	assert.Error(t, err)
}

func TestService_CreateTeam_AddMemberError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID, userID := uuid.New(), uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("GenerateUniqueCode", ctx).Return("CODE", nil)
	teamRepo.On("Create", ctx, mock.Anything).Return(nil)
	teamRepo.On("AddMember", ctx, mock.Anything).Return(errors.ErrInternal)

	_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: userID})
	assert.Error(t, err)
}

// --- JoinTeamByCode ---

func TestService_JoinTeamByCode_Success(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID, userID := uuid.New(), uuid.New(), uuid.New()

	team := &domain.Team{ID: teamID, TournamentID: tID, Code: "ABC123"}
	teamRepo.On("GetByCode", ctx, "ABC123").Return(team, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending, MaxTeamSize: 5}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*domain.TeamMember")).Return(nil)

	result, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "ABC123", UserID: userID})
	require.NoError(t, err)
	assert.Equal(t, teamID, result.ID)
}

func TestService_JoinTeamByCode_CodeNotFound(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()

	teamRepo.On("GetByCode", ctx, "INVALID").Return(nil, errors.ErrNotFound)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "INVALID", UserID: uuid.New()})
	assert.Error(t, err)
}

func TestService_JoinTeamByCode_TournamentNotPending(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID := uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&domain.Team{TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentActive}, nil)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: uuid.New()})
	assert.Error(t, err)
}

func TestService_JoinTeamByCode_TeamFull(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&domain.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending, MaxTeamSize: 3}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(3, nil) // Already full

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: uuid.New()})
	assert.Error(t, err)
}

func TestService_JoinTeamByCode_UnlimitedTeamSize(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID, userID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&domain.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending, MaxTeamSize: 0}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(100, nil) // Even 100 is fine
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("AddMember", ctx, mock.Anything).Return(nil)

	result, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: userID})
	require.NoError(t, err)
	assert.Equal(t, teamID, result.ID)
}

func TestService_JoinTeamByCode_UserAlreadyInTeam(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID, userID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&domain.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending, MaxTeamSize: 5}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(true, nil)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: userID})
	assert.Error(t, err)
}

// --- LeaveTeam ---

func TestService_LeaveTeam_RegularMember(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	memberID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, memberID).Return(true, nil)
	teamRepo.On("RemoveMember", ctx, teamID, memberID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, memberID)
	assert.NoError(t, err)
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_LeaderLastMember(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	tID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID, TournamentID: tID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(1, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("Delete", ctx, teamID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)
	assert.NoError(t, err)
	teamRepo.AssertExpectations(t)
	tournamentRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_LeaderLastMember_ActiveTournament(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	tID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID, TournamentID: tID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(1, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentActive}, nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete team during active tournament")
	teamRepo.AssertExpectations(t)
	tournamentRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_LeaderWithOthers(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	otherID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return([]*domain.TeamMember{
		{UserID: leaderID},
		{UserID: otherID},
	}, nil)
	teamRepo.On("Update", ctx, mock.MatchedBy(func(t *domain.Team) bool {
		return t.LeaderID == otherID // Leadership transferred
	})).Return(nil)
	teamRepo.On("RemoveMember", ctx, teamID, leaderID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)
	assert.NoError(t, err)
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_UserNotInTeam(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, userID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: uuid.New()}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, userID).Return(false, nil)

	err := svc.LeaveTeam(ctx, teamID, userID)
	assert.Error(t, err)
}

func TestService_LeaveTeam_TeamNotFound(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(nil, errors.ErrNotFound)

	err := svc.LeaveTeam(ctx, teamID, uuid.New())
	assert.Error(t, err)
}

// --- RemoveMember ---

func TestService_RemoveMember_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, memberID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, memberID).Return(true, nil)
	teamRepo.On("RemoveMember", ctx, teamID, memberID).Return(nil)

	err := svc.RemoveMember(ctx, teamID, memberID, leaderID)
	assert.NoError(t, err)
}

func TestService_RemoveMember_NotLeader(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, notLeader := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)

	err := svc.RemoveMember(ctx, teamID, uuid.New(), notLeader)
	assert.Error(t, err)
}

func TestService_RemoveMember_SelfRemoval(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)

	err := svc.RemoveMember(ctx, teamID, leaderID, leaderID) // Trying to remove self
	assert.Error(t, err)
}

func TestService_RemoveMember_TargetNotInTeam(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, memberID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, memberID).Return(false, nil)

	err := svc.RemoveMember(ctx, teamID, memberID, leaderID)
	assert.Error(t, err)
}

// --- UpdateTeamName ---

func TestService_UpdateTeamName_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID, Name: "Old"}, nil)
	teamRepo.On("Update", ctx, mock.MatchedBy(func(t *domain.Team) bool {
		return t.Name == "New Name"
	})).Return(nil)

	team, err := svc.UpdateTeamName(ctx, teamID, "New Name", leaderID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", team.Name)
}

func TestService_UpdateTeamName_NotLeader(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: uuid.New()}, nil)

	_, err := svc.UpdateTeamName(ctx, teamID, "New", uuid.New())
	assert.Error(t, err)
}

// --- GetInviteLink ---

func TestService_GetInviteLink_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID, Code: "ABC123"}, nil)

	link, err := svc.GetInviteLink(ctx, teamID, leaderID, "http://localhost:8080")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/join/ABC123", link)
}

func TestService_GetInviteLink_NotLeader(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: uuid.New()}, nil)

	_, err := svc.GetInviteLink(ctx, teamID, uuid.New(), "http://localhost")
	assert.Error(t, err)
}

// --- Simple proxy methods ---

func TestService_GetTeamByID_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()

	expected := &domain.Team{ID: id}
	teamRepo.On("GetByID", ctx, id).Return(expected, nil)

	team, err := svc.GetTeamByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, team.ID)
}

func TestService_GetTeamByID_Error(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

	_, err := svc.GetTeamByID(ctx, id)
	assert.Error(t, err)
}

func TestService_GetTeamByCode_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()

	expected := &domain.Team{Code: "ABC"}
	teamRepo.On("GetByCode", ctx, "ABC").Return(expected, nil)

	team, err := svc.GetTeamByCode(ctx, "ABC")
	require.NoError(t, err)
	assert.Equal(t, "ABC", team.Code)
}

func TestService_GetTeamWithMembers_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()

	expected := &domain.TeamWithMembers{Team: domain.Team{ID: id}}
	teamRepo.On("GetTeamWithMembers", ctx, id).Return(expected, nil)

	result, err := svc.GetTeamWithMembers(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, result.ID)
}

func TestService_GetTeamsByTournament_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	tID := uuid.New()

	expected := []*domain.Team{{Name: "A"}, {Name: "B"}}
	teamRepo.On("GetByTournamentID", ctx, tID).Return(expected, nil)

	teams, err := svc.GetTeamsByTournament(ctx, tID)
	require.NoError(t, err)
	assert.Len(t, teams, 2)
}

func TestService_DeleteTeam_Success(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()
	tID := uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(&domain.Team{ID: id, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("Delete", ctx, id).Return(nil)

	err := svc.DeleteTeam(ctx, id)
	assert.NoError(t, err)
}

func TestService_DeleteTeam_ActiveTournament(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()
	tID := uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(&domain.Team{ID: id, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentActive}, nil)

	err := svc.DeleteTeam(ctx, id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete team from active or completed tournament")
}

func TestService_DeleteTeam_CompletedTournament(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()
	tID := uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(&domain.Team{ID: id, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentCompleted}, nil)

	err := svc.DeleteTeam(ctx, id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete team from active or completed tournament")
}

func TestService_DeleteTeam_TeamNotFound(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(nil, errors.ErrNotFound)

	err := svc.DeleteTeam(ctx, id)
	assert.Error(t, err)
}

func TestService_DeleteTeam_DeleteError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()
	tID := uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(&domain.Team{ID: id, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("Delete", ctx, id).Return(errors.ErrNotFound)

	err := svc.DeleteTeam(ctx, id)
	assert.Error(t, err)
}

// --- Additional edge cases ---

func TestService_JoinTeamByCode_MaxTeamSize1(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&domain.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending, MaxTeamSize: 1}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(1, nil) // Already at limit

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "full")
}

func TestService_LeaveTeam_LeaderWithMultipleMembers_TransfersLeadership(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	member1ID := uuid.New()
	member2ID := uuid.New()

	team := &domain.Team{ID: teamID, LeaderID: leaderID}

	teamRepo.On("GetByID", ctx, teamID).Return(team, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(3, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return([]*domain.TeamMember{
		{TeamID: teamID, UserID: leaderID},
		{TeamID: teamID, UserID: member1ID},
		{TeamID: teamID, UserID: member2ID},
	}, nil)
	teamRepo.On("Update", ctx, mock.Anything).Return(nil)
	teamRepo.On("RemoveMember", ctx, teamID, leaderID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	require.NoError(t, err)
	// Leadership should transfer to first non-leader member
	assert.Equal(t, member1ID, team.LeaderID)
	teamRepo.AssertExpectations(t)
}

func TestService_JoinTeamByCode_AddMemberError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID, userID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&domain.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending, MaxTeamSize: 5}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("AddMember", ctx, mock.Anything).Return(errors.ErrInternal)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: userID})
	assert.Error(t, err)
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_LeaderWithOthers_GetMembersError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	tID := uuid.New()

	team := &domain.Team{ID: teamID, TournamentID: tID, LeaderID: leaderID}
	teamRepo.On("GetByID", ctx, teamID).Return(team, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(3, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending}, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return(nil, errors.ErrInternal)

	err := svc.LeaveTeam(ctx, teamID, leaderID)
	assert.Error(t, err)
	teamRepo.AssertExpectations(t)
}

func TestService_RemoveMember_RemoveMemberError(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	memberID := uuid.New()
	tID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, TournamentID: tID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, memberID).Return(true, nil)
	teamRepo.On("RemoveMember", ctx, teamID, memberID).Return(errors.ErrInternal)

	err := svc.RemoveMember(ctx, teamID, memberID, leaderID)
	assert.Error(t, err)
	teamRepo.AssertExpectations(t)
}

func TestService_UpdateTeamName_UpdateError(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("Update", ctx, mock.Anything).Return(errors.ErrInternal)

	result, err := svc.UpdateTeamName(ctx, teamID, "New Name", leaderID)
	assert.Error(t, err)
	assert.Nil(t, result)
	teamRepo.AssertExpectations(t)
}

func TestService_GetInviteLink_TeamNotFound(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(nil, errors.ErrNotFound)

	link, err := svc.GetInviteLink(ctx, teamID, uuid.New(), "https://example.com")
	assert.Error(t, err)
	assert.Empty(t, link)
}

func TestService_JoinTeamByCode_TournamentError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID := uuid.New()
	teamID := uuid.New()

	teamRepo.On("GetByCode", ctx, "MYCODE").Return(&domain.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(nil, errors.ErrInternal)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "MYCODE", UserID: uuid.New()})
	assert.Error(t, err)
}

func TestService_JoinTeamByCode_GetMemberCountError(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID := uuid.New()
	teamID := uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE01").Return(&domain.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&domain.Tournament{ID: tID, Status: domain.TournamentPending, MaxTeamSize: 5}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(0, errors.ErrInternal)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE01", UserID: uuid.New()})
	assert.Error(t, err)
}

func TestService_GetUserTeamInTournament_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	tID, userID := uuid.New(), uuid.New()
	team := &domain.Team{ID: uuid.New(), TournamentID: tID}

	teamRepo.On("GetUserTeamInTournament", ctx, tID, userID).Return(team, nil)

	result, err := svc.GetUserTeamInTournament(ctx, tID, userID)
	require.NoError(t, err)
	assert.Equal(t, team.ID, result.ID)
}

// --- LeaveTeam error edge cases ---

func TestService_LeaveTeam_GetMemberCountError(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(0, fmt.Errorf("db error"))

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get member count")
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_GetMembersError(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return(nil, fmt.Errorf("db error"))

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get team members")
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_UpdateLeadershipError(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	otherID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return([]*domain.TeamMember{
		{UserID: leaderID},
		{UserID: otherID},
	}, nil)
	teamRepo.On("Update", ctx, mock.MatchedBy(func(t *domain.Team) bool {
		return t.LeaderID == otherID
	})).Return(fmt.Errorf("db error"))

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to transfer leadership")
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_GetMembersReturnsOnlyLeader(t *testing.T) {
	// Race condition: GetMemberCount returns 2 but GetMembers only returns
	// the leader (other members left between the two calls). The team should
	// be deleted since there are effectively no other members.
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	tournamentID := uuid.New()

	team := &domain.Team{ID: teamID, LeaderID: leaderID, TournamentID: tournamentID}
	teamRepo.On("GetByID", ctx, teamID).Return(team, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return([]*domain.TeamMember{
		{UserID: leaderID},
	}, nil)
	tournamentRepo.On("GetByID", ctx, tournamentID).Return(&domain.Tournament{
		ID:     tournamentID,
		Status: domain.TournamentPending,
	}, nil)
	teamRepo.On("Delete", ctx, teamID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	assert.NoError(t, err)
	teamRepo.AssertNotCalled(t, "RemoveMember")
	teamRepo.AssertNotCalled(t, "Update")
	teamRepo.AssertCalled(t, "Delete", ctx, teamID)
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_GetMembersReturnsOnlyLeader_ActiveTournament(t *testing.T) {
	// Same race condition, but tournament is active — team cannot be deleted.
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	tournamentID := uuid.New()

	team := &domain.Team{ID: teamID, LeaderID: leaderID, TournamentID: tournamentID}
	teamRepo.On("GetByID", ctx, teamID).Return(team, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return([]*domain.TeamMember{
		{UserID: leaderID},
	}, nil)
	tournamentRepo.On("GetByID", ctx, tournamentID).Return(&domain.Tournament{
		ID:     tournamentID,
		Status: domain.TournamentActive,
	}, nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete team during active tournament")
	teamRepo.AssertNotCalled(t, "Delete")
	teamRepo.AssertNotCalled(t, "RemoveMember")
}

func TestService_RemoveMember_IsUserInTeamError(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()
	leaderID := uuid.New()
	targetID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&domain.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, targetID).Return(false, fmt.Errorf("db error"))

	err := svc.RemoveMember(ctx, teamID, targetID, leaderID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check user in team")
	teamRepo.AssertExpectations(t)
}
