//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// TournamentLifecycleSuite is the integration test suite for full tournament lifecycle operations
type TournamentLifecycleSuite struct {
	suite.Suite
	db             *db.DB
	userRepo       *db.UserRepository
	tournamentRepo *db.TournamentRepository
	teamRepo       *db.TeamRepository
	gameRepo       *db.GameRepository
	programRepo    *db.ProgramRepository
	ctx            context.Context
}

func (s *TournamentLifecycleSuite) SetupSuite() {
	if os.Getenv("RUN_INTEGRATION") != "true" {
		s.T().Skip("Skipping integration tests (set RUN_INTEGRATION=true)")
	}

	s.ctx = context.Background()

	host := getEnv("DB_HOST", "localhost")
	port := getEnvInt("DB_PORT", 5432)
	user := getEnv("DB_USER", "tjudge")
	password := getEnv("DB_PASSWORD", "secret")
	dbName := getEnv("DB_NAME", "tjudge_test")

	log, _ := logger.New("debug", "json")
	m := metrics.New()

	var err error
	s.db, err = db.New(&config.DatabaseConfig{
		Host:           host,
		Port:           port,
		User:           user,
		Password:       password,
		Name:           dbName,
		SSLMode:        "disable",
		MaxConnections: 10,
		MaxIdle:        5,
		MaxLifetime:    5 * time.Minute,
	}, log, m)
	require.NoError(s.T(), err)

	s.userRepo = db.NewUserRepository(s.db)
	s.tournamentRepo = db.NewTournamentRepository(s.db)
	s.teamRepo = db.NewTeamRepository(s.db)
	s.gameRepo = db.NewGameRepository(s.db)
	s.programRepo = db.NewProgramRepository(s.db)
}

func (s *TournamentLifecycleSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *TournamentLifecycleSuite) SetupTest() {
	s.cleanupTestData()
}

func (s *TournamentLifecycleSuite) cleanupTestData() {
	// Clean up in reverse order of foreign key dependencies
	s.db.ExecContext(s.ctx, "DELETE FROM tournament_participants WHERE tournament_id IN (SELECT id FROM tournaments WHERE name LIKE 'lifecycle_test_%')")
	s.db.ExecContext(s.ctx, "DELETE FROM programs WHERE code_path LIKE 'lifecycle_test_%'")
	s.db.ExecContext(s.ctx, "DELETE FROM team_members WHERE team_id IN (SELECT id FROM teams WHERE name LIKE 'lifecycle_test_%')")
	s.db.ExecContext(s.ctx, "DELETE FROM tournament_games WHERE tournament_id IN (SELECT id FROM tournaments WHERE name LIKE 'lifecycle_test_%')")
	s.db.ExecContext(s.ctx, "DELETE FROM teams WHERE name LIKE 'lifecycle_test_%'")
	s.db.ExecContext(s.ctx, "DELETE FROM tournaments WHERE name LIKE 'lifecycle_test_%'")
	s.db.ExecContext(s.ctx, "DELETE FROM games WHERE name LIKE 'lifecycle_test_%'")
	s.db.ExecContext(s.ctx, "DELETE FROM users WHERE username LIKE 'lifecycle_test_%'")
}

// =============================================================================
// Helper methods for creating test entities
// =============================================================================

func (s *TournamentLifecycleSuite) createTestUser(suffix string) *domain.User {
	s.T().Helper()
	user := &domain.User{
		ID:           uuid.New(),
		Username:     fmt.Sprintf("lifecycle_test_user_%s_%s", suffix, uuid.New().String()[:8]),
		Email:        fmt.Sprintf("lifecycle_%s_%s@test.com", suffix, uuid.New().String()[:8]),
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)
	return user
}

func (s *TournamentLifecycleSuite) createTestTournament(suffix string, creatorID *uuid.UUID) *domain.Tournament {
	s.T().Helper()
	maxParticipants := 32
	tournament := &domain.Tournament{
		ID:              uuid.New(),
		Name:            fmt.Sprintf("lifecycle_test_tournament_%s", suffix),
		Code:            uuid.New().String()[:6],
		Description:     "Integration test tournament",
		GameType:        "lifecycle_test_game",
		Status:          domain.TournamentPending,
		MaxParticipants: &maxParticipants,
		MaxTeamSize:     4,
		IsPermanent:     false,
		CreatorID:       creatorID,
	}
	err := s.tournamentRepo.Create(s.ctx, tournament)
	require.NoError(s.T(), err)
	return tournament
}

func (s *TournamentLifecycleSuite) createTestGame(suffix string) *domain.Game {
	s.T().Helper()
	game := &domain.Game{
		ID:          uuid.New(),
		Name:        fmt.Sprintf("lifecycle_test_%s_%s", suffix, uuid.New().String()[:8]),
		DisplayName: fmt.Sprintf("Lifecycle Test Game %s", suffix),
		Rules:       "Test rules for integration testing",
	}
	err := s.gameRepo.Create(s.ctx, game)
	require.NoError(s.T(), err)
	return game
}

func (s *TournamentLifecycleSuite) createTestTeam(suffix string, tournamentID, leaderID uuid.UUID) *domain.Team {
	s.T().Helper()
	team := &domain.Team{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Name:         fmt.Sprintf("lifecycle_test_team_%s_%s", suffix, uuid.New().String()[:8]),
		Code:         uuid.New().String()[:6],
		LeaderID:     leaderID,
	}
	err := s.teamRepo.Create(s.ctx, team)
	require.NoError(s.T(), err)
	return team
}

func (s *TournamentLifecycleSuite) createTestProgram(user *domain.User, team *domain.Team, tournament *domain.Tournament, game *domain.Game, suffix string) *domain.Program {
	s.T().Helper()
	program := &domain.Program{
		ID:           uuid.New(),
		UserID:       user.ID,
		TeamID:       &team.ID,
		TournamentID: &tournament.ID,
		GameID:       &game.ID,
		Name:         fmt.Sprintf("Program %s", suffix),
		GameType:     game.Name,
		CodePath:     fmt.Sprintf("lifecycle_test_%s", suffix),
		Language:     "python",
		Version:      1,
	}
	err := s.programRepo.Create(s.ctx, program)
	require.NoError(s.T(), err)
	return program
}

// =============================================================================
// Test: Create Tournament
// =============================================================================

func (s *TournamentLifecycleSuite) TestTournamentLifecycle_CreateTournament() {
	creator := s.createTestUser("creator")
	tournament := s.createTestTournament("create", &creator.ID)

	// Verify the tournament is persisted and retrievable
	found, err := s.tournamentRepo.GetByID(s.ctx, tournament.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), tournament.ID, found.ID)
	assert.Equal(s.T(), tournament.Name, found.Name)
	assert.Equal(s.T(), tournament.Code, found.Code)
	assert.Equal(s.T(), tournament.Description, found.Description)
	assert.Equal(s.T(), tournament.GameType, found.GameType)
	assert.Equal(s.T(), domain.TournamentPending, found.Status)
	assert.Equal(s.T(), *tournament.MaxParticipants, *found.MaxParticipants)
	assert.Equal(s.T(), tournament.MaxTeamSize, found.MaxTeamSize)
	assert.Equal(s.T(), tournament.IsPermanent, found.IsPermanent)
	assert.Equal(s.T(), creator.ID, *found.CreatorID)
	assert.NotZero(s.T(), found.CreatedAt)
	assert.NotZero(s.T(), found.UpdatedAt)
	assert.Equal(s.T(), 0, found.Version)
}

// =============================================================================
// Test: Add Game to Tournament
// =============================================================================

func (s *TournamentLifecycleSuite) TestTournamentLifecycle_AddGame() {
	creator := s.createTestUser("addgame_creator")
	tournament := s.createTestTournament("addgame", &creator.ID)
	game := s.createTestGame("addgame")

	// Link game to tournament
	err := s.gameRepo.AddToTournament(s.ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	// Verify the game is linked to the tournament
	games, err := s.gameRepo.GetByTournamentID(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), games, 1)
	assert.Equal(s.T(), game.ID, games[0].ID)
	assert.Equal(s.T(), game.Name, games[0].Name)
	assert.Equal(s.T(), game.DisplayName, games[0].DisplayName)

	// Verify tournament_game record
	tg, err := s.gameRepo.GetTournamentGame(s.ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), tournament.ID, tg.TournamentID)
	assert.Equal(s.T(), game.ID, tg.GameID)
	assert.False(s.T(), tg.IsActive)
	assert.False(s.T(), tg.RoundCompleted)
	assert.Equal(s.T(), 0, tg.CurrentRound)
}

// =============================================================================
// Test: Register Team
// =============================================================================

func (s *TournamentLifecycleSuite) TestTournamentLifecycle_RegisterTeam() {
	creator := s.createTestUser("regteam_creator")
	tournament := s.createTestTournament("regteam", &creator.ID)

	leader := s.createTestUser("regteam_leader")
	member := s.createTestUser("regteam_member")

	// Create team
	team := s.createTestTeam("regteam", tournament.ID, leader.ID)

	// Add leader and member to team
	leaderMember := &domain.TeamMember{
		ID:     uuid.New(),
		TeamID: team.ID,
		UserID: leader.ID,
	}
	err := s.teamRepo.AddMember(s.ctx, leaderMember)
	require.NoError(s.T(), err)

	teamMember := &domain.TeamMember{
		ID:     uuid.New(),
		TeamID: team.ID,
		UserID: member.ID,
	}
	err = s.teamRepo.AddMember(s.ctx, teamMember)
	require.NoError(s.T(), err)

	// Verify team is in the tournament
	teams, err := s.teamRepo.GetByTournamentID(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), teams, 1)
	assert.Equal(s.T(), team.ID, teams[0].ID)
	assert.Equal(s.T(), tournament.ID, teams[0].TournamentID)

	// Verify team members
	members, err := s.teamRepo.GetMembers(s.ctx, team.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), members, 2)

	// Verify membership check
	isLeaderInTeam, err := s.teamRepo.IsUserInTeam(s.ctx, team.ID, leader.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), isLeaderInTeam)

	isMemberInTeam, err := s.teamRepo.IsUserInTeam(s.ctx, team.ID, member.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), isMemberInTeam)

	// Verify user is in tournament team
	isInTournament, err := s.teamRepo.IsUserInAnyTeamInTournament(s.ctx, tournament.ID, leader.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), isInTournament)

	// Verify member count
	count, err := s.teamRepo.GetMemberCount(s.ctx, team.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)
}

// =============================================================================
// Test: Upload Program
// =============================================================================

func (s *TournamentLifecycleSuite) TestTournamentLifecycle_UploadProgram() {
	creator := s.createTestUser("upload_creator")
	tournament := s.createTestTournament("upload", &creator.ID)
	game := s.createTestGame("upload")

	// Link game to tournament
	err := s.gameRepo.AddToTournament(s.ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	// Create team
	leader := s.createTestUser("upload_leader")
	team := s.createTestTeam("upload", tournament.ID, leader.ID)
	leaderMember := &domain.TeamMember{
		ID:     uuid.New(),
		TeamID: team.ID,
		UserID: leader.ID,
	}
	err = s.teamRepo.AddMember(s.ctx, leaderMember)
	require.NoError(s.T(), err)

	// Create program
	program := s.createTestProgram(leader, team, tournament, game, "upload")

	// Verify program is persisted
	found, err := s.programRepo.GetByID(s.ctx, program.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), program.ID, found.ID)
	assert.Equal(s.T(), leader.ID, found.UserID)
	assert.Equal(s.T(), &team.ID, found.TeamID)
	assert.Equal(s.T(), &tournament.ID, found.TournamentID)
	assert.Equal(s.T(), &game.ID, found.GameID)
	assert.Equal(s.T(), game.Name, found.GameType)
	assert.Equal(s.T(), "python", found.Language)
	assert.Equal(s.T(), 1, found.Version)

	// Verify program is linked to user
	programs, err := s.programRepo.GetByUserID(s.ctx, leader.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), programs, 1)
	assert.Equal(s.T(), program.ID, programs[0].ID)

	// Verify ownership
	isOwner, err := s.programRepo.CheckOwnership(s.ctx, program.ID, leader.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), isOwner)

	notOwner, err := s.programRepo.CheckOwnership(s.ctx, program.ID, creator.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), notOwner)
}

// =============================================================================
// Test: Full Flow
// =============================================================================

func (s *TournamentLifecycleSuite) TestTournamentLifecycle_FullFlow() {
	// Step 1: Create tournament
	admin := s.createTestUser("full_admin")
	tournament := s.createTestTournament("fullflow", &admin.ID)

	// Step 2: Add games to tournament
	game1 := s.createTestGame("fullflow_pd")
	game2 := s.createTestGame("fullflow_tow")

	err := s.gameRepo.AddToTournament(s.ctx, tournament.ID, game1.ID)
	require.NoError(s.T(), err)
	err = s.gameRepo.AddToTournament(s.ctx, tournament.ID, game2.ID)
	require.NoError(s.T(), err)

	// Verify both games are linked
	games, err := s.gameRepo.GetByTournamentID(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), games, 2)

	// Step 3: Create users and teams
	leader1 := s.createTestUser("full_leader1")
	leader2 := s.createTestUser("full_leader2")
	leader3 := s.createTestUser("full_leader3")

	team1 := s.createTestTeam("fullflow_alpha", tournament.ID, leader1.ID)
	team2 := s.createTestTeam("fullflow_beta", tournament.ID, leader2.ID)
	team3 := s.createTestTeam("fullflow_gamma", tournament.ID, leader3.ID)

	// Add leaders as team members
	for _, pair := range []struct {
		teamID   uuid.UUID
		leaderID uuid.UUID
	}{
		{team1.ID, leader1.ID},
		{team2.ID, leader2.ID},
		{team3.ID, leader3.ID},
	} {
		err = s.teamRepo.AddMember(s.ctx, &domain.TeamMember{
			ID:     uuid.New(),
			TeamID: pair.teamID,
			UserID: pair.leaderID,
		})
		require.NoError(s.T(), err)
	}

	// Verify all teams in tournament
	teams, err := s.teamRepo.GetByTournamentID(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), teams, 3)

	// Step 4: Upload programs for each team for each game
	programs := make([]*domain.Program, 0, 6)
	for i, teamInfo := range []struct {
		leader *domain.User
		team   *domain.Team
	}{
		{leader1, team1},
		{leader2, team2},
		{leader3, team3},
	} {
		for j, game := range []*domain.Game{game1, game2} {
			prog := s.createTestProgram(
				teamInfo.leader,
				teamInfo.team,
				tournament,
				game,
				fmt.Sprintf("full_t%d_g%d", i+1, j+1),
			)
			programs = append(programs, prog)

			// Add as tournament participant
			participant := &domain.TournamentParticipant{
				ID:           uuid.New(),
				TournamentID: tournament.ID,
				ProgramID:    prog.ID,
				Rating:       1500,
			}
			err = s.tournamentRepo.AddParticipant(s.ctx, participant)
			require.NoError(s.T(), err)
		}
	}

	// Verify all participants
	participants, err := s.tournamentRepo.GetParticipants(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants, 6) // 3 teams * 2 games

	// Step 5: Activate tournament
	err = s.tournamentRepo.UpdateStatus(s.ctx, tournament.ID, domain.TournamentActive)
	require.NoError(s.T(), err)

	// Verify tournament is active
	updatedTournament, err := s.tournamentRepo.GetByID(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.TournamentActive, updatedTournament.Status)

	// Step 6: Verify participants count
	count, err := s.tournamentRepo.GetParticipantsCount(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 6, count)

	// Step 7: Verify tournament games state
	tournamentGames, err := s.gameRepo.GetTournamentGames(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), tournamentGames, 2)
	for _, tg := range tournamentGames {
		assert.False(s.T(), tg.RoundCompleted)
		assert.Equal(s.T(), 0, tg.CurrentRound)
	}

	// Step 8: Set active game and verify
	err = s.gameRepo.SetActiveGame(s.ctx, tournament.ID, game1.ID)
	require.NoError(s.T(), err)

	activeTG, err := s.gameRepo.GetActiveGame(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), game1.ID, activeTG.GameID)
	assert.True(s.T(), activeTG.IsActive)

	// Verify game2 is not active
	isGame2Active, err := s.gameRepo.IsGameActive(s.ctx, tournament.ID, game2.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), isGame2Active)

	// Step 9: Verify team with members
	teamWithMembers, err := s.teamRepo.GetTeamWithMembers(s.ctx, team1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), team1.ID, teamWithMembers.ID)
	assert.Len(s.T(), teamWithMembers.Members, 1)
	assert.Equal(s.T(), leader1.ID, teamWithMembers.Members[0].ID)

	// Step 10: Verify user team in tournament lookup
	foundTeam, err := s.teamRepo.GetUserTeamInTournament(s.ctx, tournament.ID, leader2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), team2.ID, foundTeam.ID)

	// Step 11: Verify programs by tournament and game
	game1Programs, err := s.programRepo.GetByTournamentAndGame(s.ctx, tournament.ID, game1.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), game1Programs, 3) // 3 teams, 1 program per team for game1
}

// =============================================================================
// Test: Concurrent Registrations
// =============================================================================

func (s *TournamentLifecycleSuite) TestTournamentLifecycle_ConcurrentRegistrations() {
	admin := s.createTestUser("concurrent_admin")
	tournament := s.createTestTournament("concurrent", &admin.ID)
	game := s.createTestGame("concurrent")

	err := s.gameRepo.AddToTournament(s.ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	const numTeams = 10
	var wg sync.WaitGroup
	errs := make(chan error, numTeams*3) // team creation + member + participant

	// Create users first (sequentially to avoid conflicts)
	leaders := make([]*domain.User, numTeams)
	for i := 0; i < numTeams; i++ {
		leaders[i] = s.createTestUser(fmt.Sprintf("concurrent_%d", i))
	}

	// Create teams, members, programs, and participants concurrently
	for i := 0; i < numTeams; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			leader := leaders[idx]

			// Create team
			team := &domain.Team{
				ID:           uuid.New(),
				TournamentID: tournament.ID,
				Name:         fmt.Sprintf("lifecycle_test_team_concurrent_%d_%s", idx, uuid.New().String()[:8]),
				Code:         uuid.New().String()[:6],
				LeaderID:     leader.ID,
			}
			if err := s.teamRepo.Create(s.ctx, team); err != nil {
				errs <- fmt.Errorf("team create %d: %w", idx, err)
				return
			}

			// Add member
			member := &domain.TeamMember{
				ID:     uuid.New(),
				TeamID: team.ID,
				UserID: leader.ID,
			}
			if err := s.teamRepo.AddMember(s.ctx, member); err != nil {
				errs <- fmt.Errorf("member add %d: %w", idx, err)
				return
			}

			// Create program
			program := &domain.Program{
				ID:           uuid.New(),
				UserID:       leader.ID,
				TeamID:       &team.ID,
				TournamentID: &tournament.ID,
				GameID:       &game.ID,
				Name:         fmt.Sprintf("Concurrent Program %d", idx),
				GameType:     game.Name,
				CodePath:     fmt.Sprintf("lifecycle_test_concurrent_%d", idx),
				Language:     "python",
				Version:      1,
			}
			if err := s.programRepo.Create(s.ctx, program); err != nil {
				errs <- fmt.Errorf("program create %d: %w", idx, err)
				return
			}

			// Add as participant
			participant := &domain.TournamentParticipant{
				ID:           uuid.New(),
				TournamentID: tournament.ID,
				ProgramID:    program.ID,
				Rating:       1500,
			}
			if err := s.tournamentRepo.AddParticipant(s.ctx, participant); err != nil {
				errs <- fmt.Errorf("participant add %d: %w", idx, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	// Collect all errors
	for err := range errs {
		s.T().Errorf("concurrent operation failed: %v", err)
	}

	// Verify all teams were created
	teams, err := s.teamRepo.GetByTournamentID(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), teams, numTeams)

	// Verify all participants were added
	participants, err := s.tournamentRepo.GetParticipants(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants, numTeams)

	// Verify participant count
	count, err := s.tournamentRepo.GetParticipantsCount(s.ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), numTeams, count)
}

// =============================================================================
// Suite Runner
// =============================================================================

func TestTournamentLifecycleSuite(t *testing.T) {
	suite.Run(t, new(TournamentLifecycleSuite))
}
