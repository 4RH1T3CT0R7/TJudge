package team

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- моки ---

type MockTeamRepository struct {
	mock.Mock
}

func (m *MockTeamRepository) Create(ctx context.Context, team *models.Team) error {
	return m.Called(ctx, team).Error(0)
}

func (m *MockTeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamRepository) GetByCode(ctx context.Context, code string) (*models.Team, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Team, error) {
	args := m.Called(ctx, tournamentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Team), args.Error(1)
}

func (m *MockTeamRepository) List(ctx context.Context, filter models.TeamFilter) ([]*models.Team, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Team), args.Error(1)
}

func (m *MockTeamRepository) Update(ctx context.Context, team *models.Team) error {
	return m.Called(ctx, team).Error(0)
}

func (m *MockTeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockTeamRepository) AddMember(ctx context.Context, member *models.TeamMember) error {
	return m.Called(ctx, member).Error(0)
}

func (m *MockTeamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	return m.Called(ctx, teamID, userID).Error(0)
}

func (m *MockTeamRepository) GetMembers(ctx context.Context, teamID uuid.UUID) ([]*models.TeamMember, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TeamMember), args.Error(1)
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

func (m *MockTeamRepository) GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, tournamentID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamRepository) GenerateUniqueCode(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockTeamRepository) GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*models.TeamWithMembers, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TeamWithMembers), args.Error(1)
}

func (m *MockTeamRepository) DisqualifyTeamFull(ctx context.Context, teamID, tournamentID uuid.UUID) (int64, int64, int64, error) {
	args := m.Called(ctx, teamID, tournamentID)
	return args.Get(0).(int64), args.Get(1).(int64), args.Get(2).(int64), args.Error(3)
}

func (m *MockTeamRepository) RestoreTeam(ctx context.Context, teamID uuid.UUID) error {
	return m.Called(ctx, teamID).Error(0)
}

func (m *MockTeamRepository) IsTeamDisqualified(ctx context.Context, teamID uuid.UUID) (bool, error) {
	args := m.Called(ctx, teamID)
	return args.Bool(0), args.Error(1)
}

type MockTournamentRepository struct {
	mock.Mock
}

func (m *MockTournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Tournament), args.Error(1)
}

// noopLock просто выполняет функцию без реального лока — для юнитов сойдёт
type noopLock struct{}

func (n *noopLock) WithLock(ctx context.Context, _ string, _ time.Duration, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newTestTeamService(t *testing.T) (*Service, *MockTeamRepository, *MockTournamentRepository) {
	teamRepo := new(MockTeamRepository)
	tournamentRepo := new(MockTournamentRepository)
	log, _ := logger.New("error", "json")
	return NewService(teamRepo, tournamentRepo, &noopLock{}, log), teamRepo, tournamentRepo
}

// --- CreateTeam ---

func TestService_CreateTeam_Success(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID, userID := uuid.New(), uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending}, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("GenerateUniqueCode", ctx).Return("ABC123", nil)
	teamRepo.On("Create", ctx, mock.AnythingOfType("*models.Team")).Return(nil)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*models.TeamMember")).Return(nil)

	team, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "My Team", UserID: userID})

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

// команду можно завести только в pending-турнире
func TestService_CreateTeam_TournamentNotPending(t *testing.T) {
	for _, status := range []models.TournamentStatus{models.TournamentActive, models.TournamentCompleted, models.TournamentCancelled} {
		t.Run(string(status), func(t *testing.T) {
			svc, _, tournamentRepo := newTestTeamService(t)
			ctx := context.Background()
			tID := uuid.New()

			tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: status}, nil)

			_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: uuid.New()})
			assert.Error(t, err)
		})
	}
}

func TestService_CreateTeam_UserAlreadyInTeam(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	tID, userID := uuid.New(), uuid.New()

	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending}, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(true, nil)

	_, err := svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: userID})
	assert.Error(t, err)
}

// --- JoinTeamByCode ---

func TestService_JoinTeamByCode_Success(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID, userID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "ABC123").Return(&models.Team{ID: teamID, TournamentID: tID, Code: "ABC123"}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending, MaxTeamSize: 5}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(false, nil)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*models.TeamMember")).Return(nil)

	result, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "ABC123", UserID: userID})

	require.NoError(t, err)
	assert.Equal(t, teamID, result.ID)
}

// recordingLock запоминает ключи взятых локов
type recordingLock struct{ keys []string }

func (l *recordingLock) WithLock(ctx context.Context, key string, _ time.Duration, fn func(ctx context.Context) error) error {
	l.keys = append(l.keys, key)
	return fn(ctx)
}

// лимит команды держится локом на команду, а create и join одного пользователя в
// турнире - общим ключом; лок только на пользователя пропускал параллельные вступления
func TestService_TeamMembershipLockKeys(t *testing.T) {
	teamRepo := new(MockTeamRepository)
	tournamentRepo := new(MockTournamentRepository)
	log, _ := logger.New("error", "json")
	lock := &recordingLock{}
	svc := NewService(teamRepo, tournamentRepo, lock, log)
	ctx := context.Background()
	teamID, tID, userID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "ABC123").Return(&models.Team{ID: teamID, TournamentID: tID, Code: "ABC123"}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending, MaxTeamSize: 5}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(true, nil)

	_, _ = svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "ABC123", UserID: userID})
	_, _ = svc.CreateTeam(ctx, &CreateTeamRequest{TournamentID: tID, Name: "T", UserID: userID})

	require.Len(t, lock.keys, 3)
	assert.Equal(t, "team:join:"+teamID.String(), lock.keys[1])
	assert.Equal(t, lock.keys[0], lock.keys[2], "create и join одного пользователя должны делить лок")
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

	teamRepo.On("GetByCode", ctx, "CODE").Return(&models.Team{TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentActive}, nil)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: uuid.New()})
	assert.Error(t, err)
}

func TestService_JoinTeamByCode_TeamFull(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&models.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending, MaxTeamSize: 3}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(3, nil)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "full")
}

// MaxTeamSize 0 значит без лимита, тк проверки размера тогда вообще нет
func TestService_JoinTeamByCode_UnlimitedTeamSize(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID, userID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByCode", ctx, "CODE").Return(&models.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending, MaxTeamSize: 0}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(100, nil)
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

	teamRepo.On("GetByCode", ctx, "CODE").Return(&models.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending, MaxTeamSize: 5}, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("IsUserInAnyTeamInTournament", ctx, tID, userID).Return(true, nil)

	_, err := svc.JoinTeamByCode(ctx, &JoinTeamRequest{Code: "CODE", UserID: userID})
	assert.Error(t, err)
}

// --- LeaveTeam ---

func TestService_LeaveTeam_RegularMember(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, memberID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID}, nil)
	tournamentRepo.On("GetByID", ctx, uuid.Nil).Return(&models.Tournament{Status: models.TournamentActive}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, memberID).Return(true, nil)
	teamRepo.On("RemoveMember", ctx, teamID, memberID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, memberID)
	assert.NoError(t, err)
	teamRepo.AssertExpectations(t)
}

// уходит лидер, а он последний — команда удаляется
func TestService_LeaveTeam_LeaderLastMember(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, tID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID, TournamentID: tID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(1, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending}, nil)
	teamRepo.On("Delete", ctx, teamID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)
	assert.NoError(t, err)
	teamRepo.AssertExpectations(t)
}

// тот же случай, но турнир активный — сносить команду нельзя
func TestService_LeaveTeam_LeaderLastMember_ActiveTournament(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, tID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID, TournamentID: tID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(1, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentActive}, nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete team during active tournament")
	teamRepo.AssertExpectations(t)
}

// елси уходит лидер, а в команде есть ещё люди — лидерство переходит первому не-лидеру
func TestService_LeaveTeam_LeaderTransfersLeadership(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, member1ID, member2ID := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	team := &models.Team{ID: teamID, LeaderID: leaderID}
	teamRepo.On("GetByID", ctx, teamID).Return(team, nil)
	tournamentRepo.On("GetByID", ctx, uuid.Nil).Return(&models.Tournament{Status: models.TournamentPending}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(3, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return([]*models.TeamMember{
		{TeamID: teamID, UserID: leaderID},
		{TeamID: teamID, UserID: member1ID},
		{TeamID: teamID, UserID: member2ID},
	}, nil)
	teamRepo.On("Update", ctx, mock.Anything).Return(nil)
	teamRepo.On("RemoveMember", ctx, teamID, leaderID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	require.NoError(t, err)
	assert.Equal(t, member1ID, team.LeaderID)
	teamRepo.AssertExpectations(t)
}

func TestService_LeaveTeam_UserNotInTeam(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, userID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: uuid.New()}, nil)
	tournamentRepo.On("GetByID", ctx, uuid.Nil).Return(&models.Tournament{Status: models.TournamentPending}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, userID).Return(false, nil)

	err := svc.LeaveTeam(ctx, teamID, userID)
	assert.Error(t, err)
}

// завершённый турнир: состав команды заморожен, иначе уход последнего участника
// удалил бы команду из итоговых таблиц
func TestService_LeaveTeam_CompletedTournament(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, tID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentCompleted}, nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	appErr := errors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, 409, appErr.Code)
	teamRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	teamRepo.AssertNotCalled(t, "RemoveMember", mock.Anything, mock.Anything, mock.Anything)
}

// гонка: GetMemberCount вернул 2, а GetMembers отдал только лидера
// (остальные вышли между вызовами). по факту команда пустая — удаляется
func TestService_LeaveTeam_OnlyLeaderLeft(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, tID := uuid.New(), uuid.New(), uuid.New()

	team := &models.Team{ID: teamID, LeaderID: leaderID, TournamentID: tID}
	teamRepo.On("GetByID", ctx, teamID).Return(team, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, leaderID).Return(true, nil)
	teamRepo.On("GetMemberCount", ctx, teamID).Return(2, nil)
	teamRepo.On("GetMembers", ctx, teamID).Return([]*models.TeamMember{{UserID: leaderID}}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending}, nil)
	teamRepo.On("Delete", ctx, teamID).Return(nil)

	err := svc.LeaveTeam(ctx, teamID, leaderID)

	assert.NoError(t, err)
	teamRepo.AssertNotCalled(t, "RemoveMember")
	teamRepo.AssertCalled(t, "Delete", ctx, teamID)
	teamRepo.AssertExpectations(t)
}

// --- RemoveMember ---

func TestService_RemoveMember_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, memberID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, memberID).Return(true, nil)
	teamRepo.On("RemoveMember", ctx, teamID, memberID).Return(nil)

	err := svc.RemoveMember(ctx, teamID, memberID, leaderID)
	assert.NoError(t, err)
}

func TestService_RemoveMember_NotLeader(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, notLeader := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID}, nil)

	err := svc.RemoveMember(ctx, teamID, uuid.New(), notLeader)
	assert.Error(t, err)
}

// себя через remove не удалить — для этого есть leave
func TestService_RemoveMember_SelfRemoval(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID}, nil)

	err := svc.RemoveMember(ctx, teamID, leaderID, leaderID)
	assert.Error(t, err)
}

func TestService_RemoveMember_TargetNotInTeam(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID, memberID := uuid.New(), uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID}, nil)
	teamRepo.On("IsUserInTeam", ctx, teamID, memberID).Return(false, nil)

	err := svc.RemoveMember(ctx, teamID, memberID, leaderID)
	assert.Error(t, err)
}

// --- UpdateTeamName ---

func TestService_UpdateTeamName_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID, Name: "Old"}, nil)
	teamRepo.On("Update", ctx, mock.MatchedBy(func(t *models.Team) bool {
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

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: uuid.New()}, nil)

	_, err := svc.UpdateTeamName(ctx, teamID, "New", uuid.New())
	assert.Error(t, err)
}

// --- GetInviteLink ---

func TestService_GetInviteLink_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, leaderID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: leaderID, Code: "ABC123"}, nil)

	link, err := svc.GetInviteLink(ctx, teamID, leaderID, "http://localhost:8080")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/join/ABC123", link)
}

func TestService_GetInviteLink_NotLeader(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, LeaderID: uuid.New()}, nil)

	_, err := svc.GetInviteLink(ctx, teamID, uuid.New(), "http://localhost")
	assert.Error(t, err)
}

// --- GetTeamByID (простой проброс в репозиторий) ---

func TestService_GetTeamByID_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	id := uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(&models.Team{ID: id}, nil)

	team, err := svc.GetTeamByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, team.ID)
}

// --- DeleteTeam ---

func TestService_DeleteTeam_Success(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	id, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(&models.Team{ID: id, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending}, nil)
	teamRepo.On("Delete", ctx, id).Return(nil)

	err := svc.DeleteTeam(ctx, id)
	assert.NoError(t, err)
}

func TestService_DeleteTeam_ActiveTournament(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	id, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, id).Return(&models.Team{ID: id, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentActive}, nil)

	err := svc.DeleteTeam(ctx, id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete team from active or completed tournament")
}

// --- Disqualify / Restore ---

func TestService_DisqualifyTeam_Success(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, TournamentID: tID, IsDisqualified: false}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentActive}, nil)
	// вся чистка идёт одной транзакцией в репозитории, тут просто отдаются счётчики
	teamRepo.On("DisqualifyTeamFull", ctx, teamID, tID).Return(int64(5), int64(3), int64(2), nil)

	res, err := svc.DisqualifyTeam(ctx, teamID)

	require.NoError(t, err)
	assert.Equal(t, int64(5), res.MatchesDeleted)
	assert.Equal(t, int64(3), res.MatchesCancelled)
	assert.Equal(t, int64(2), res.RatingHistoryReset)
	teamRepo.AssertExpectations(t)
}

func TestService_DisqualifyTeam_AlreadyDisqualified(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, IsDisqualified: true}, nil)

	res, err := svc.DisqualifyTeam(ctx, teamID)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "already disqualified")
}

// дисквалить можно только в активном турнире
func TestService_DisqualifyTeam_NotActiveTournament(t *testing.T) {
	svc, teamRepo, tournamentRepo := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, TournamentID: tID}, nil)
	tournamentRepo.On("GetByID", ctx, tID).Return(&models.Tournament{ID: tID, Status: models.TournamentPending}, nil)

	res, err := svc.DisqualifyTeam(ctx, teamID)
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestService_RestoreTeam_Success(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID, tID := uuid.New(), uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, TournamentID: tID, IsDisqualified: true}, nil)
	teamRepo.On("RestoreTeam", ctx, teamID).Return(nil)

	err := svc.RestoreTeam(ctx, teamID)
	assert.NoError(t, err)
	teamRepo.AssertExpectations(t)
}

// восстанавливать не дисквалифицированную команду нечего
func TestService_RestoreTeam_NotDisqualified(t *testing.T) {
	svc, teamRepo, _ := newTestTeamService(t)
	ctx := context.Background()
	teamID := uuid.New()

	teamRepo.On("GetByID", ctx, teamID).Return(&models.Team{ID: teamID, IsDisqualified: false}, nil)

	err := svc.RestoreTeam(ctx, teamID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not disqualified")
}
