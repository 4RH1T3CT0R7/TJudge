//go:build integration

package storage_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TeamRepositorySuite struct {
	suite.Suite
	database       *storage.DB
	repo           *storage.TeamRepository
	userRepo       *storage.UserRepository
	tournamentRepo *storage.TournamentRepository
}

func TestTeamRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &TeamRepositorySuite{
		database:       database,
		repo:           storage.NewTeamRepository(database),
		userRepo:       storage.NewUserRepository(database),
		tournamentRepo: storage.NewTournamentRepository(database),
	}
	suite.Run(t, s)
}

func (s *TeamRepositorySuite) TearDownTest() {
	ctx := context.Background()
	_, _ = s.database.ExecContext(ctx, "DELETE FROM team_members WHERE team_id IN (SELECT id FROM teams WHERE name LIKE 'Test Team%')")
	_, _ = s.database.ExecContext(ctx, "DELETE FROM teams WHERE name LIKE 'Test Team%'")
	_, _ = s.database.ExecContext(ctx, "DELETE FROM tournaments WHERE code LIKE 'TTEAM%'")
	_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE username LIKE 'testuser_team%'")
}

func (s *TeamRepositorySuite) createTeamUser(suffix string) *models.User {
	return createTestUser(s.T(), s.userRepo, "team_"+suffix)
}

func (s *TeamRepositorySuite) createTeamTournament(code string, creatorID uuid.UUID) *models.Tournament {
	return createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
}

func (s *TeamRepositorySuite) createTeam(name, code string, tournamentID, leaderID uuid.UUID) *models.Team {
	s.T().Helper()
	ctx := context.Background()

	team := &models.Team{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Name:         name,
		Code:         code,
		LeaderID:     leaderID,
	}

	err := s.repo.Create(ctx, team)
	require.NoError(s.T(), err)

	return team
}

func (s *TeamRepositorySuite) addMember(teamID, userID uuid.UUID) *models.TeamMember {
	s.T().Helper()
	ctx := context.Background()

	member := &models.TeamMember{
		ID:     uuid.New(),
		TeamID: teamID,
		UserID: userID,
	}

	err := s.repo.AddMember(ctx, member)
	require.NoError(s.T(), err)

	return member
}

// --- Create ---

func (s *TeamRepositorySuite) TestCreate() {
	user := s.createTeamUser("create1")
	tournament := s.createTeamTournament("TTEAM01", user.ID)

	ctx := context.Background()
	team := &models.Team{
		ID:           uuid.New(),
		TournamentID: tournament.ID,
		Name:         "Test Team Create",
		Code:         "CREA01",
		LeaderID:     user.ID,
	}

	err := s.repo.Create(ctx, team)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), team.CreatedAt)
	assert.NotZero(s.T(), team.UpdatedAt)
}

// --- GetByID ---

func (s *TeamRepositorySuite) TestGetByID() {
	user := s.createTeamUser("getid1")
	tournament := s.createTeamTournament("TTEAM02", user.ID)
	team := s.createTeam("Test Team GetByID", "GETID1", tournament.ID, user.ID)

	ctx := context.Background()
	result, err := s.repo.GetByID(ctx, team.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), team.ID, result.ID)
	assert.Equal(s.T(), team.TournamentID, result.TournamentID)
	assert.Equal(s.T(), team.Name, result.Name)
	assert.Equal(s.T(), team.Code, result.Code)
	assert.Equal(s.T(), team.LeaderID, result.LeaderID)
	assert.NotZero(s.T(), result.CreatedAt)
	assert.NotZero(s.T(), result.UpdatedAt)
}

func (s *TeamRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// --- GetByCode ---

func (s *TeamRepositorySuite) TestGetByCode() {
	user := s.createTeamUser("getcode1")
	tournament := s.createTeamTournament("TTEAM03", user.ID)
	team := s.createTeam("Test Team GetByCode", "GCOD01", tournament.ID, user.ID)

	ctx := context.Background()
	result, err := s.repo.GetByCode(ctx, team.Code)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), team.ID, result.ID)
	assert.Equal(s.T(), team.Name, result.Name)
	assert.Equal(s.T(), team.Code, result.Code)
}

func (s *TeamRepositorySuite) TestGetByCode_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByCode(ctx, "ZZZZZZ")
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// --- GetByTournamentID ---

func (s *TeamRepositorySuite) TestGetByTournamentID() {
	user := s.createTeamUser("gettid1")
	tournament := s.createTeamTournament("TTEAM04", user.ID)

	team1 := s.createTeam("Test Team ByTID 1", "BTID01", tournament.ID, user.ID)
	team2 := s.createTeam("Test Team ByTID 2", "BTID02", tournament.ID, user.ID)

	ctx := context.Background()
	teams, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)

	assert.Len(s.T(), teams, 2)

	ids := make(map[uuid.UUID]bool)
	for _, t := range teams {
		ids[t.ID] = true
	}
	assert.True(s.T(), ids[team1.ID])
	assert.True(s.T(), ids[team2.ID])
}

func (s *TeamRepositorySuite) TestGetByTournamentID_Empty() {
	ctx := context.Background()

	teams, err := s.repo.GetByTournamentID(ctx, uuid.New())
	require.NoError(s.T(), err)

	assert.Empty(s.T(), teams)
}

// --- List ---

func (s *TeamRepositorySuite) TestList_FilterByTournament() {
	user := s.createTeamUser("list1")
	tournament := s.createTeamTournament("TTEAM05", user.ID)

	s.createTeam("Test Team List 1", "LIST01", tournament.ID, user.ID)
	s.createTeam("Test Team List 2", "LIST02", tournament.ID, user.ID)

	ctx := context.Background()
	filter := models.TeamFilter{
		TournamentID: &tournament.ID,
		Limit:        10,
	}
	teams, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	assert.Len(s.T(), teams, 2)
	for _, t := range teams {
		assert.Equal(s.T(), tournament.ID, t.TournamentID)
	}
}

func (s *TeamRepositorySuite) TestList_FilterByLeader() {
	user1 := s.createTeamUser("listldr1")
	user2 := s.createTeamUser("listldr2")
	tournament := s.createTeamTournament("TTEAM06", user1.ID)

	s.createTeam("Test Team Leader1", "LLDR01", tournament.ID, user1.ID)
	s.createTeam("Test Team Leader2", "LLDR02", tournament.ID, user2.ID)

	ctx := context.Background()
	filter := models.TeamFilter{
		LeaderID: &user1.ID,
		Limit:    10,
	}
	teams, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	assert.GreaterOrEqual(s.T(), len(teams), 1)
	for _, t := range teams {
		assert.Equal(s.T(), user1.ID, t.LeaderID)
	}
}

func (s *TeamRepositorySuite) TestList_LimitOffset() {
	user := s.createTeamUser("listpg1")
	tournament := s.createTeamTournament("TTEAM07", user.ID)

	s.createTeam("Test Team Page 1", "PAGE01", tournament.ID, user.ID)
	s.createTeam("Test Team Page 2", "PAGE02", tournament.ID, user.ID)
	s.createTeam("Test Team Page 3", "PAGE03", tournament.ID, user.ID)

	ctx := context.Background()
	filter := models.TeamFilter{
		TournamentID: &tournament.ID,
		Limit:        2,
		Offset:       0,
	}
	teams, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)
	assert.Len(s.T(), teams, 2)

	filter.Offset = 2
	teams, err = s.repo.List(ctx, filter)
	require.NoError(s.T(), err)
	assert.Len(s.T(), teams, 1)
}

// --- Update ---

func (s *TeamRepositorySuite) TestUpdate() {
	user1 := s.createTeamUser("upd1")
	user2 := s.createTeamUser("upd2")
	tournament := s.createTeamTournament("TTEAM08", user1.ID)
	team := s.createTeam("Test Team Update", "UPDT01", tournament.ID, user1.ID)

	ctx := context.Background()
	team.Name = "Test Team Updated Name"
	team.LeaderID = user2.ID

	err := s.repo.Update(ctx, team)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, team.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Test Team Updated Name", result.Name)
	assert.Equal(s.T(), user2.ID, result.LeaderID)
	assert.True(s.T(), result.UpdatedAt.After(result.CreatedAt) || result.UpdatedAt.Equal(result.CreatedAt))
}

func (s *TeamRepositorySuite) TestUpdate_NotFound() {
	ctx := context.Background()

	team := &models.Team{
		ID:       uuid.New(),
		Name:     "Test Team Nonexistent",
		LeaderID: uuid.New(),
	}

	err := s.repo.Update(ctx, team)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// --- Delete ---

func (s *TeamRepositorySuite) TestDelete() {
	user := s.createTeamUser("del1")
	tournament := s.createTeamTournament("TTEAM09", user.ID)
	team := s.createTeam("Test Team Delete", "DELE01", tournament.ID, user.ID)

	ctx := context.Background()
	err := s.repo.Delete(ctx, team.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetByID(ctx, team.ID)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *TeamRepositorySuite) TestDelete_NotFound() {
	ctx := context.Background()

	err := s.repo.Delete(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// --- AddMember ---

func (s *TeamRepositorySuite) TestAddMember() {
	leader := s.createTeamUser("addm_ldr")
	member := s.createTeamUser("addm_usr")
	tournament := s.createTeamTournament("TTEAM10", leader.ID)
	team := s.createTeam("Test Team AddMember", "ADMB01", tournament.ID, leader.ID)

	ctx := context.Background()
	tm := &models.TeamMember{
		ID:     uuid.New(),
		TeamID: team.ID,
		UserID: member.ID,
	}

	err := s.repo.AddMember(ctx, tm)
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), tm.JoinedAt)
}

func (s *TeamRepositorySuite) TestAddMember_Duplicate() {
	leader := s.createTeamUser("addmdup_ldr")
	member := s.createTeamUser("addmdup_usr")
	tournament := s.createTeamTournament("TTEAM11", leader.ID)
	team := s.createTeam("Test Team AddMemberDup", "ADUP01", tournament.ID, leader.ID)

	s.addMember(team.ID, member.ID)

	ctx := context.Background()
	duplicate := &models.TeamMember{
		ID:     uuid.New(),
		TeamID: team.ID,
		UserID: member.ID,
	}

	err := s.repo.AddMember(ctx, duplicate)
	assert.Error(s.T(), err)
}

// --- RemoveMember ---

func (s *TeamRepositorySuite) TestRemoveMember() {
	leader := s.createTeamUser("rmm_ldr")
	member := s.createTeamUser("rmm_usr")
	tournament := s.createTeamTournament("TTEAM12", leader.ID)
	team := s.createTeam("Test Team RemoveMember", "RMMB01", tournament.ID, leader.ID)

	s.addMember(team.ID, member.ID)

	ctx := context.Background()
	err := s.repo.RemoveMember(ctx, team.ID, member.ID)
	require.NoError(s.T(), err)

	// участника больше нет в списке
	members, err := s.repo.GetMembers(ctx, team.ID)
	require.NoError(s.T(), err)
	for _, m := range members {
		assert.NotEqual(s.T(), member.ID, m.UserID)
	}
}

func (s *TeamRepositorySuite) TestRemoveMember_NotFound() {
	ctx := context.Background()

	err := s.repo.RemoveMember(ctx, uuid.New(), uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// --- GetMembers ---

func (s *TeamRepositorySuite) TestGetMembers() {
	leader := s.createTeamUser("gm_ldr")
	user1 := s.createTeamUser("gm_usr1")
	user2 := s.createTeamUser("gm_usr2")
	tournament := s.createTeamTournament("TTEAM13", leader.ID)
	team := s.createTeam("Test Team GetMembers", "GMEM01", tournament.ID, leader.ID)

	m1 := s.addMember(team.ID, user1.ID)
	m2 := s.addMember(team.ID, user2.ID)

	ctx := context.Background()
	members, err := s.repo.GetMembers(ctx, team.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), members, 2)

	userIDs := make(map[uuid.UUID]bool)
	for _, m := range members {
		userIDs[m.UserID] = true
		assert.Equal(s.T(), team.ID, m.TeamID)
		assert.NotZero(s.T(), m.JoinedAt)
	}
	assert.True(s.T(), userIDs[m1.UserID])
	assert.True(s.T(), userIDs[m2.UserID])
}

func (s *TeamRepositorySuite) TestGetMembers_Empty() {
	leader := s.createTeamUser("gme_ldr")
	tournament := s.createTeamTournament("TTEAM14", leader.ID)
	team := s.createTeam("Test Team GetMembersEmpty", "GMEM02", tournament.ID, leader.ID)

	ctx := context.Background()
	members, err := s.repo.GetMembers(ctx, team.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), members)
}

// --- GetMemberCount ---

func (s *TeamRepositorySuite) TestGetMemberCount() {
	leader := s.createTeamUser("mc_ldr")
	user1 := s.createTeamUser("mc_usr1")
	user2 := s.createTeamUser("mc_usr2")
	tournament := s.createTeamTournament("TTEAM15", leader.ID)
	team := s.createTeam("Test Team MemberCount", "MCNT01", tournament.ID, leader.ID)

	s.addMember(team.ID, user1.ID)
	s.addMember(team.ID, user2.ID)

	ctx := context.Background()
	count, err := s.repo.GetMemberCount(ctx, team.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)
}

func (s *TeamRepositorySuite) TestGetMemberCount_Zero() {
	leader := s.createTeamUser("mcz_ldr")
	tournament := s.createTeamTournament("TTEAM16", leader.ID)
	team := s.createTeam("Test Team MemberCountZero", "MCNT02", tournament.ID, leader.ID)

	ctx := context.Background()
	count, err := s.repo.GetMemberCount(ctx, team.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, count)
}

// --- IsUserInTeam ---

func (s *TeamRepositorySuite) TestIsUserInTeam_True() {
	leader := s.createTeamUser("iut_ldr")
	member := s.createTeamUser("iut_usr")
	tournament := s.createTeamTournament("TTEAM17", leader.ID)
	team := s.createTeam("Test Team IsUserInTeam", "IUIT01", tournament.ID, leader.ID)

	s.addMember(team.ID, member.ID)

	ctx := context.Background()
	exists, err := s.repo.IsUserInTeam(ctx, team.ID, member.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *TeamRepositorySuite) TestIsUserInTeam_False() {
	leader := s.createTeamUser("iutf_ldr")
	tournament := s.createTeamTournament("TTEAM18", leader.ID)
	team := s.createTeam("Test Team IsUserInTeamFalse", "IUIT02", tournament.ID, leader.ID)

	ctx := context.Background()
	exists, err := s.repo.IsUserInTeam(ctx, team.ID, uuid.New())
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

// --- IsUserInAnyTeamInTournament ---

func (s *TeamRepositorySuite) TestIsUserInAnyTeamInTournament_True() {
	leader := s.createTeamUser("iuatt_ldr")
	member := s.createTeamUser("iuatt_usr")
	tournament := s.createTeamTournament("TTEAM19", leader.ID)
	team := s.createTeam("Test Team IsUserAnyTeam", "IUAT01", tournament.ID, leader.ID)

	s.addMember(team.ID, member.ID)

	ctx := context.Background()
	exists, err := s.repo.IsUserInAnyTeamInTournament(ctx, tournament.ID, member.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *TeamRepositorySuite) TestIsUserInAnyTeamInTournament_False() {
	leader := s.createTeamUser("iuatf_ldr")
	tournament := s.createTeamTournament("TTEAM20", leader.ID)
	s.createTeam("Test Team IsUserAnyTeamF", "IUAT02", tournament.ID, leader.ID)

	ctx := context.Background()
	exists, err := s.repo.IsUserInAnyTeamInTournament(ctx, tournament.ID, uuid.New())
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

// --- GetUserTeamInTournament ---

func (s *TeamRepositorySuite) TestGetUserTeamInTournament() {
	leader := s.createTeamUser("gutt_ldr")
	member := s.createTeamUser("gutt_usr")
	tournament := s.createTeamTournament("TTEAM21", leader.ID)
	team := s.createTeam("Test Team GetUserTeam", "GUTT01", tournament.ID, leader.ID)

	s.addMember(team.ID, member.ID)

	ctx := context.Background()
	result, err := s.repo.GetUserTeamInTournament(ctx, tournament.ID, member.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), team.ID, result.ID)
	assert.Equal(s.T(), team.Name, result.Name)
	assert.Equal(s.T(), tournament.ID, result.TournamentID)
}

func (s *TeamRepositorySuite) TestGetUserTeamInTournament_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetUserTeamInTournament(ctx, uuid.New(), uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// --- GenerateUniqueCode ---

func (s *TeamRepositorySuite) TestGenerateUniqueCode() {
	ctx := context.Background()

	code, err := s.repo.GenerateUniqueCode(ctx)
	require.NoError(s.T(), err)

	assert.Len(s.T(), code, 8)
	for _, c := range code {
		assert.Contains(s.T(), "ABCDEFGHJKLMNPQRSTUVWXYZ23456789", string(c))
	}
}

func (s *TeamRepositorySuite) TestGenerateUniqueCode_Uniqueness() {
	ctx := context.Background()

	codes := make(map[string]bool)
	for i := 0; i < 10; i++ {
		code, err := s.repo.GenerateUniqueCode(ctx)
		require.NoError(s.T(), err)
		codes[code] = true
	}

	// все коды должны быть разными
	assert.Len(s.T(), codes, 10)
}

// --- GetTeamWithMembers ---

func (s *TeamRepositorySuite) TestGetTeamWithMembers() {
	leader := s.createTeamUser("gtwm_ldr")
	user1 := s.createTeamUser("gtwm_usr1")
	user2 := s.createTeamUser("gtwm_usr2")
	tournament := s.createTeamTournament("TTEAM22", leader.ID)
	team := s.createTeam("Test Team WithMembers", "GTWM01", tournament.ID, leader.ID)

	s.addMember(team.ID, user1.ID)
	s.addMember(team.ID, user2.ID)

	ctx := context.Background()
	result, err := s.repo.GetTeamWithMembers(ctx, team.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), team.ID, result.ID)
	assert.Equal(s.T(), team.Name, result.Name)
	assert.Len(s.T(), result.Members, 2)

	usernames := make(map[string]bool)
	for _, m := range result.Members {
		usernames[m.Username] = true
		assert.NotZero(s.T(), m.ID)
		assert.NotEmpty(s.T(), m.Email)
	}
	assert.True(s.T(), usernames[user1.Username])
	assert.True(s.T(), usernames[user2.Username])
}

func (s *TeamRepositorySuite) TestGetTeamWithMembers_NoMembers() {
	leader := s.createTeamUser("gtwmn_ldr")
	tournament := s.createTeamTournament("TTEAM23", leader.ID)
	team := s.createTeam("Test Team WithMembersNone", "GTWM02", tournament.ID, leader.ID)

	ctx := context.Background()
	result, err := s.repo.GetTeamWithMembers(ctx, team.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), team.ID, result.ID)
	assert.Empty(s.T(), result.Members)
}

func (s *TeamRepositorySuite) TestGetTeamWithMembers_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetTeamWithMembers(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// дисквалификация убирает и завершённые матчи, и форфейты (failed): иначе
// технические победы над командой остаются у соперников в лидербордах
func (s *TeamRepositorySuite) TestDisqualifyTeamFull_RemovesPlayedMatches() {
	ctx := context.Background()
	user := s.createTeamUser("dq1")
	opponent := s.createTeamUser("dq2")
	tournament := s.createTeamTournament("TTEAMDQ", user.ID)
	team := s.createTeam("Test Team DQ", "TDQ001", tournament.ID, user.ID)
	other := s.createTeam("Test Team DQ Other", "TDQ002", tournament.ID, opponent.ID)

	programRepo := storage.NewProgramRepository(s.database)
	matchRepo := storage.NewMatchRepository(s.database)
	newProgram := func(owner uuid.UUID, teamID uuid.UUID, name string) *models.Program {
		p := &models.Program{
			ID: uuid.New(), UserID: owner, TeamID: &teamID, TournamentID: &tournament.ID,
			Name: name, GameType: "dilemma", CodePath: "/tmp/" + name, Language: "python", Version: 1,
		}
		require.NoError(s.T(), programRepo.Create(ctx, p))
		s.T().Cleanup(func() { _, _ = s.database.ExecContext(ctx, "DELETE FROM programs WHERE id = $1", p.ID) })
		return p
	}
	dq := newProgram(user.ID, team.ID, "dq_bot")
	opp := newProgram(opponent.ID, other.ID, "opp_bot")

	statuses := []models.MatchStatus{models.MatchCompleted, models.MatchFailed, models.MatchPending, models.MatchRunning}
	ids := make(map[models.MatchStatus]uuid.UUID)
	for _, st := range statuses {
		m := &models.Match{
			ID: uuid.New(), TournamentID: tournament.ID, Program1ID: dq.ID, Program2ID: opp.ID,
			GameType: "dilemma", Status: st, Priority: models.PriorityMedium, RoundNumber: 1, CreatedAt: time.Now(),
		}
		require.NoError(s.T(), matchRepo.Create(ctx, m))
		ids[st] = m.ID
		s.T().Cleanup(func() { _, _ = s.database.ExecContext(ctx, "DELETE FROM matches WHERE id = $1", m.ID) })
	}

	deleted, cancelled, _, err := s.repo.DisqualifyTeamFull(ctx, team.ID, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), deleted)
	assert.Equal(s.T(), int64(2), cancelled)

	for _, st := range []models.MatchStatus{models.MatchCompleted, models.MatchFailed} {
		_, err := matchRepo.GetByID(ctx, ids[st])
		assert.True(s.T(), errors.IsNotFound(err), "матч %s должен быть удалён", st)
	}
	for _, st := range []models.MatchStatus{models.MatchPending, models.MatchRunning} {
		m, err := matchRepo.GetByID(ctx, ids[st])
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.MatchCancelled, m.Status)
	}
}

// ELO соперника освобождается от матчей с дисквалифицированной командой:
// рейтинг участника и поздние точки истории сдвигаются на удалённую дельту
func (s *TeamRepositorySuite) TestDisqualifyTeamFull_RevertsOpponentRating() {
	ctx := context.Background()
	user := s.createTeamUser("dqr1")
	opponent := s.createTeamUser("dqr2")
	third := s.createTeamUser("dqr3")
	tournament := s.createTeamTournament("TTEAMDQR", user.ID)
	team := s.createTeam("Test Team DQR", "TDQR01", tournament.ID, user.ID)
	oppTeam := s.createTeam("Test Team DQR Opp", "TDQR02", tournament.ID, opponent.ID)
	thirdTeam := s.createTeam("Test Team DQR Third", "TDQR03", tournament.ID, third.ID)

	programRepo := storage.NewProgramRepository(s.database)
	matchRepo := storage.NewMatchRepository(s.database)
	newProgram := func(owner, teamID uuid.UUID, name string, rating int) *models.Program {
		p := &models.Program{
			ID: uuid.New(), UserID: owner, TeamID: &teamID, TournamentID: &tournament.ID,
			Name: name, GameType: "dilemma", CodePath: "/tmp/" + name, Language: "python", Version: 1,
		}
		require.NoError(s.T(), programRepo.Create(ctx, p))
		require.NoError(s.T(), s.tournamentRepo.AddParticipant(ctx, &models.TournamentParticipant{
			ID: uuid.New(), TournamentID: tournament.ID, ProgramID: p.ID, Rating: rating,
		}))
		s.T().Cleanup(func() {
			_, _ = s.database.ExecContext(ctx, "DELETE FROM tournament_participants WHERE program_id = $1", p.ID)
			_, _ = s.database.ExecContext(ctx, "DELETE FROM programs WHERE id = $1", p.ID)
		})
		return p
	}
	dq := newProgram(user.ID, team.ID, "dqr_bot", 1484)
	opp := newProgram(opponent.ID, oppTeam.ID, "dqr_opp", 1530)
	thr := newProgram(third.ID, thirdTeam.ID, "dqr_third", 1486)

	// матч с командой (+16 сопернику), затем матч с третьей командой (+14)
	start := time.Now().Add(-time.Hour)
	played := func(p1, p2 *models.Program, at time.Time, history map[uuid.UUID][2]int) {
		m := &models.Match{
			ID: uuid.New(), TournamentID: tournament.ID, Program1ID: p1.ID, Program2ID: p2.ID,
			GameType: "dilemma", Status: models.MatchCompleted, Priority: models.PriorityMedium, RoundNumber: 1, CreatedAt: at,
		}
		require.NoError(s.T(), matchRepo.Create(ctx, m))
		s.T().Cleanup(func() {
			_, _ = s.database.ExecContext(ctx, "DELETE FROM rating_history WHERE match_id = $1", m.ID)
			_, _ = s.database.ExecContext(ctx, "DELETE FROM matches WHERE id = $1", m.ID)
		})
		for pid, r := range history {
			_, err := s.database.ExecContext(ctx, `
				INSERT INTO rating_history (id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				uuid.New(), pid, tournament.ID, r[0], r[1], r[1]-r[0], m.ID, at)
			require.NoError(s.T(), err)
		}
	}
	played(dq, opp, start, map[uuid.UUID][2]int{dq.ID: {1500, 1484}, opp.ID: {1500, 1516}})
	played(opp, thr, start.Add(time.Minute), map[uuid.UUID][2]int{opp.ID: {1516, 1530}, thr.ID: {1500, 1486}})

	_, _, _, err := s.repo.DisqualifyTeamFull(ctx, team.ID, tournament.ID)
	require.NoError(s.T(), err)

	rating := func(pid uuid.UUID) int {
		var r int
		require.NoError(s.T(), s.database.QueryRowContext(ctx,
			"SELECT rating FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
			tournament.ID, pid).Scan(&r))
		return r
	}
	assert.Equal(s.T(), 1514, rating(opp.ID))
	assert.Equal(s.T(), 1486, rating(thr.ID))

	var oldR, newR int
	require.NoError(s.T(), s.database.QueryRowContext(ctx,
		"SELECT old_rating, new_rating FROM rating_history WHERE program_id = $1", opp.ID).Scan(&oldR, &newR))
	assert.Equal(s.T(), 1500, oldR)
	assert.Equal(s.T(), 1514, newR)
}

// дисквалификация во время применения рейтинга матча с командой: ждёт его
// коммита и откатывает и его дельту, без взаимной блокировки на участниках
func (s *TeamRepositorySuite) TestDisqualifyTeamFull_WaitsForRatingApply() {
	ctx := context.Background()
	user := s.createTeamUser("dqw1")
	opponent := s.createTeamUser("dqw2")
	tournament := s.createTeamTournament("TTEAMDQW", user.ID)
	team := s.createTeam("Test Team DQW", "TDQW01", tournament.ID, user.ID)
	oppTeam := s.createTeam("Test Team DQW Opp", "TDQW02", tournament.ID, opponent.ID)

	programRepo := storage.NewProgramRepository(s.database)
	matchRepo := storage.NewMatchRepository(s.database)
	newProgram := func(owner, teamID uuid.UUID, name string, rating int) *models.Program {
		p := &models.Program{
			ID: uuid.New(), UserID: owner, TeamID: &teamID, TournamentID: &tournament.ID,
			Name: name, GameType: "dilemma", CodePath: "/tmp/" + name, Language: "python", Version: 1,
		}
		require.NoError(s.T(), programRepo.Create(ctx, p))
		_, err := s.database.ExecContext(ctx,
			"INSERT INTO tournament_participants (tournament_id, program_id, rating) VALUES ($1, $2, $3)",
			tournament.ID, p.ID, rating)
		require.NoError(s.T(), err)
		s.T().Cleanup(func() {
			_, _ = s.database.ExecContext(ctx, "DELETE FROM tournament_participants WHERE program_id = $1", p.ID)
			_, _ = s.database.ExecContext(ctx, "DELETE FROM programs WHERE id = $1", p.ID)
		})
		return p
	}
	dq := newProgram(user.ID, team.ID, "dqw_bot", 1484)
	opp := newProgram(opponent.ID, oppTeam.ID, "dqw_opp", 1516)

	start := time.Now().Add(-time.Hour)
	newMatch := func(at time.Time) uuid.UUID {
		m := &models.Match{
			ID: uuid.New(), TournamentID: tournament.ID, Program1ID: dq.ID, Program2ID: opp.ID,
			GameType: "dilemma", Status: models.MatchCompleted, Priority: models.PriorityMedium, RoundNumber: 1, CreatedAt: at,
		}
		require.NoError(s.T(), matchRepo.Create(ctx, m))
		s.T().Cleanup(func() {
			_, _ = s.database.ExecContext(ctx, "DELETE FROM rating_history WHERE match_id = $1", m.ID)
			_, _ = s.database.ExecContext(ctx, "DELETE FROM matches WHERE id = $1", m.ID)
		})
		return m.ID
	}
	insertHistory := func(q interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}, matchID, pid uuid.UUID, oldR, newR int, at time.Time) {
		_, err := q.ExecContext(ctx, `
			INSERT INTO rating_history (id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			uuid.New(), pid, tournament.ID, oldR, newR, newR-oldR, matchID, at)
		require.NoError(s.T(), err)
	}

	// первый матч уже применён: соперник +16
	first := newMatch(start)
	insertHistory(s.database, first, dq.ID, 1500, 1484, start)
	insertHistory(s.database, first, opp.ID, 1500, 1516, start)

	// второй применяется прямо сейчас: шаги ApplyMatchResult в том же порядке блокировок
	second := newMatch(start.Add(time.Minute))
	tx, err := s.database.BeginTx(ctx, nil)
	require.NoError(s.T(), err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, "SELECT 1 FROM matches WHERE id = $1 FOR KEY SHARE", second)
	require.NoError(s.T(), err)

	done := make(chan error, 1)
	go func() {
		_, _, _, err := s.repo.DisqualifyTeamFull(ctx, team.ID, tournament.ID)
		done <- err
	}()
	require.Eventually(s.T(), func() bool {
		var waiting int
		_ = s.database.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(&waiting)
		return waiting > 0
	}, 5*time.Second, 20*time.Millisecond)

	_, err = tx.ExecContext(ctx, `
		SELECT rating FROM tournament_participants
		WHERE tournament_id = $1 AND program_id IN ($2, $3)
		ORDER BY program_id FOR UPDATE`, tournament.ID, dq.ID, opp.ID)
	require.NoError(s.T(), err)
	at := start.Add(time.Minute)
	insertHistory(tx, second, dq.ID, 1484, 1470, at)
	insertHistory(tx, second, opp.ID, 1516, 1530, at)
	_, err = tx.ExecContext(ctx,
		"UPDATE tournament_participants SET rating = rating + 14 WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, opp.ID)
	require.NoError(s.T(), err)
	require.NoError(s.T(), tx.Commit())

	select {
	case err := <-done:
		require.NoError(s.T(), err)
	case <-time.After(10 * time.Second):
		s.T().Fatal("дисквалификация не завершилась")
	}

	var rating, history int
	require.NoError(s.T(), s.database.QueryRowContext(ctx,
		"SELECT rating FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, opp.ID).Scan(&rating))
	assert.Equal(s.T(), 1500, rating)
	require.NoError(s.T(), s.database.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rating_history WHERE program_id = $1", opp.ID).Scan(&history))
	assert.Zero(s.T(), history)
}
