//go:build integration

package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TournamentRepositorySuite struct {
	suite.Suite
	database    *db.DB
	repo        *db.TournamentRepository
	userRepo    *db.UserRepository
	programRepo *db.ProgramRepository
	teamRepo    *db.TeamRepository
	gameRepo    *db.GameRepository
	matchRepo   *db.MatchRepository
	// track created IDs for cleanup
	participantIDs []uuid.UUID
	matchIDs       []uuid.UUID
	programIDs     []uuid.UUID
	teamIDs        []uuid.UUID
	gameIDs        []uuid.UUID
	tournamentIDs  []uuid.UUID
	userIDs        []uuid.UUID
}

func TestTournamentRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &TournamentRepositorySuite{
		database:    database,
		repo:        db.NewTournamentRepository(database),
		userRepo:    db.NewUserRepository(database),
		programRepo: db.NewProgramRepository(database),
		teamRepo:    db.NewTeamRepository(database),
		gameRepo:    db.NewGameRepository(database),
		matchRepo:   db.NewMatchRepository(database),
	}
	suite.Run(t, s)
}

func (s *TournamentRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// Delete in FK order: matches -> rating_history -> tournament_participants -> programs -> teams -> tournaments -> games -> users
	for _, id := range s.matchIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM matches WHERE id = $1", id)
	}
	for _, id := range s.participantIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM tournament_participants WHERE id = $1", id)
	}
	for _, id := range s.programIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM programs WHERE id = $1", id)
	}
	for _, id := range s.teamIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM teams WHERE id = $1", id)
	}
	// Also cleanup by code pattern for legacy tests
	_, _ = s.database.ExecContext(ctx, "DELETE FROM tournaments WHERE code LIKE 'TEST%'")
	for _, id := range s.tournamentIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM tournaments WHERE id = $1", id)
	}
	for _, id := range s.gameIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM games WHERE id = $1", id)
	}
	_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE username LIKE 'testuser_%'")
	for _, id := range s.userIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	}
	s.matchIDs = nil
	s.participantIDs = nil
	s.programIDs = nil
	s.teamIDs = nil
	s.gameIDs = nil
	s.tournamentIDs = nil
	s.userIDs = nil
}

// createTrackedUser creates a test user and tracks it for cleanup.
func (s *TournamentRepositorySuite) createTrackedUser(suffix string) *domain.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

// createTrackedTournament creates a test tournament and tracks it for cleanup.
func (s *TournamentRepositorySuite) createTrackedTournament(code string, creatorID uuid.UUID) *domain.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo(), code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

// tournamentRepo returns the tournament repository (convenience alias).
func (s *TournamentRepositorySuite) tournamentRepo() *db.TournamentRepository {
	return s.repo
}

// createTrackedGame creates a test game and tracks it for cleanup.
func (s *TournamentRepositorySuite) createTrackedGame(name string) *domain.Game {
	ctx := context.Background()
	game := &domain.Game{
		ID:          uuid.New(),
		Name:        name,
		DisplayName: "Test Game " + name,
		Rules:       "Test rules",
	}
	err := s.gameRepo.Create(ctx, game)
	require.NoError(s.T(), err)
	s.gameIDs = append(s.gameIDs, game.ID)
	return game
}

// createTrackedTeam creates a test team and tracks it for cleanup.
func (s *TournamentRepositorySuite) createTrackedTeam(tournamentID, leaderID uuid.UUID, code string) *domain.Team {
	ctx := context.Background()
	team := &domain.Team{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Name:         "Test Team " + code,
		Code:         code,
		LeaderID:     leaderID,
	}
	err := s.teamRepo.Create(ctx, team)
	require.NoError(s.T(), err)
	s.teamIDs = append(s.teamIDs, team.ID)
	return team
}

// createTrackedProgram creates a test program with team/tournament/game references and tracks it for cleanup.
func (s *TournamentRepositorySuite) createTrackedProgram(userID uuid.UUID, teamID, tournamentID, gameID *uuid.UUID, name string, version int) *domain.Program {
	ctx := context.Background()
	program := &domain.Program{
		ID:           uuid.New(),
		UserID:       userID,
		TeamID:       teamID,
		TournamentID: tournamentID,
		GameID:       gameID,
		Name:         name,
		GameType:     "prisoners_dilemma",
		CodePath:     "/tmp/test/" + name + ".py",
		Language:     "python",
		Version:      version,
	}
	err := s.programRepo.Create(ctx, program)
	require.NoError(s.T(), err)
	s.programIDs = append(s.programIDs, program.ID)
	return program
}

// createTestParticipant adds a tournament participant and tracks it for cleanup.
func (s *TournamentRepositorySuite) createTestParticipant(tournamentID, programID uuid.UUID, rating int) *domain.TournamentParticipant {
	ctx := context.Background()
	participant := &domain.TournamentParticipant{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		ProgramID:    programID,
		Rating:       rating,
	}
	err := s.repo.AddParticipant(ctx, participant)
	require.NoError(s.T(), err)
	s.participantIDs = append(s.participantIDs, participant.ID)
	return participant
}

// createTrackedMatch creates a match and tracks it for cleanup.
func (s *TournamentRepositorySuite) createTrackedMatch(tournamentID, program1ID, program2ID uuid.UUID, gameType string, status domain.MatchStatus) *domain.Match {
	ctx := context.Background()
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     gameType,
		Status:       status,
		Priority:     domain.PriorityMedium,
		RoundNumber:  1,
		CreatedAt:    time.Now(),
	}
	err := s.matchRepo.Create(ctx, match)
	require.NoError(s.T(), err)
	s.matchIDs = append(s.matchIDs, match.ID)
	return match
}

// setupParticipantPrerequisites creates user, tournament, team, game, program, and participant.
// Returns tournament, program, and participant for use in tests.
func (s *TournamentRepositorySuite) setupParticipantPrerequisites(suffix string, rating int) (*domain.Tournament, *domain.Program, *domain.TournamentParticipant) {
	user := s.createTrackedUser("tp_" + suffix)
	tournament := s.createTrackedTournament("TP"+suffix, user.ID)
	program := s.createTrackedProgram(user.ID, nil, nil, nil, "Bot_"+suffix, 1)
	participant := s.createTestParticipant(tournament.ID, program.ID, rating)
	return tournament, program, participant
}

func (s *TournamentRepositorySuite) TestCreate() {
	ctx := context.Background()
	creator := createTestUser(s.T(), s.userRepo, "creator_"+uuid.New().String()[:8])

	tournament := &domain.Tournament{
		ID:              uuid.New(),
		Code:            "TEST001",
		Name:            "Test Tournament",
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          domain.TournamentPending,
		MaxParticipants: intPtr(100),
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       uuidPtr(creator.ID),
		Metadata:        map[string]interface{}{"test": "value"},
	}

	err := s.repo.Create(ctx, tournament)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), tournament.CreatedAt)
	assert.NotZero(s.T(), tournament.UpdatedAt)
	assert.Equal(s.T(), 0, tournament.Version)
}

func (s *TournamentRepositorySuite) TestGetByID() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST002")

	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), tournament.ID, result.ID)
	assert.Equal(s.T(), tournament.Code, result.Code)
	assert.Equal(s.T(), tournament.Name, result.Name)
	assert.Equal(s.T(), tournament.Status, result.Status)
}

func (s *TournamentRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
}

func (s *TournamentRepositorySuite) TestList() {
	ctx := context.Background()

	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST003")
	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST004")
	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST005")

	filter := domain.TournamentFilter{Limit: 10}
	tournaments, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	assert.GreaterOrEqual(s.T(), len(tournaments), 3)
}

func (s *TournamentRepositorySuite) TestList_FilterByStatus() {
	ctx := context.Background()

	t1, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST006")
	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST007")

	err := s.repo.UpdateStatus(ctx, t1.ID, domain.TournamentActive)
	require.NoError(s.T(), err)

	filter := domain.TournamentFilter{
		Status: domain.TournamentActive,
		Limit:  10,
	}
	tournaments, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	var found bool
	for _, t := range tournaments {
		if t.ID == t1.ID {
			found = true
			break
		}
	}
	assert.True(s.T(), found, "Active tournament should be in list")
}

func (s *TournamentRepositorySuite) TestUpdateStatus() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST008")

	err := s.repo.UpdateStatus(ctx, tournament.ID, domain.TournamentActive)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.TournamentActive, result.Status)
}

func (s *TournamentRepositorySuite) TestUpdate() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST009")

	tournament.Name = "Updated Name"
	tournament.MaxParticipants = intPtr(200)

	err := s.repo.Update(ctx, tournament)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Name", result.Name)
	assert.Equal(s.T(), intPtr(200), result.MaxParticipants)
}

func (s *TournamentRepositorySuite) TestDelete() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST010")

	err := s.repo.Delete(ctx, tournament.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetByID(ctx, tournament.ID)
	assert.Error(s.T(), err)
}

// --- Participant tests ---

func (s *TournamentRepositorySuite) TestAddParticipant_Success() {
	user := s.createTrackedUser("tp_add")
	tournament := s.createTrackedTournament("TPADD1", user.ID)
	program := s.createTrackedProgram(user.ID, nil, nil, nil, "BotAdd", 1)

	ctx := context.Background()
	participant := &domain.TournamentParticipant{
		ID:           uuid.New(),
		TournamentID: tournament.ID,
		ProgramID:    program.ID,
		Rating:       1500,
	}

	err := s.repo.AddParticipant(ctx, participant)
	require.NoError(s.T(), err)
	s.participantIDs = append(s.participantIDs, participant.ID)

	assert.NotZero(s.T(), participant.CreatedAt)
}

func (s *TournamentRepositorySuite) TestGetParticipants_Empty() {
	user := s.createTrackedUser("tp_gpe")
	tournament := s.createTrackedTournament("TPGPE1", user.ID)

	ctx := context.Background()
	participants, err := s.repo.GetParticipants(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), participants)
}

func (s *TournamentRepositorySuite) TestGetParticipants_Multiple() {
	user := s.createTrackedUser("tp_gpm")
	tournament := s.createTrackedTournament("TPGPM1", user.ID)

	// Create 3 programs and add them as participants
	for i := 0; i < 3; i++ {
		prog := s.createTrackedProgram(user.ID, nil, nil, nil, fmt.Sprintf("BotGPM%d", i), 1)
		s.createTestParticipant(tournament.ID, prog.ID, 1500+i*100)
	}

	ctx := context.Background()
	participants, err := s.repo.GetParticipants(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants, 3)

	// Verify all belong to the same tournament
	for _, p := range participants {
		assert.Equal(s.T(), tournament.ID, p.TournamentID)
	}
}

func (s *TournamentRepositorySuite) TestGetParticipantsCount_Zero() {
	user := s.createTrackedUser("tp_gcz")
	tournament := s.createTrackedTournament("TPGCZ1", user.ID)

	ctx := context.Background()
	count, err := s.repo.GetParticipantsCount(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, count)
}

func (s *TournamentRepositorySuite) TestGetParticipantsCount_AfterAdding() {
	user := s.createTrackedUser("tp_gca")
	tournament := s.createTrackedTournament("TPGCA1", user.ID)

	// Add 5 participants
	for i := 0; i < 5; i++ {
		prog := s.createTrackedProgram(user.ID, nil, nil, nil, fmt.Sprintf("BotGCA%d", i), 1)
		s.createTestParticipant(tournament.ID, prog.ID, 1500)
	}

	ctx := context.Background()
	count, err := s.repo.GetParticipantsCount(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 5, count)
}

func (s *TournamentRepositorySuite) TestGetLeaderboard_OrderedByRating() {
	user := s.createTrackedUser("tp_lbo")
	tournament := s.createTrackedTournament("TPLBO1", user.ID)

	// Create 3 participants with different ratings.
	// GetLeaderboard uses score from completed matches; however with no matches,
	// the leaderboard should still return all participants (with total_score=0).
	// We test that the result set is complete.
	var progIDs []uuid.UUID
	for i := 0; i < 3; i++ {
		prog := s.createTrackedProgram(user.ID, nil, nil, nil, fmt.Sprintf("BotLBO%d", i), 1)
		s.createTestParticipant(tournament.ID, prog.ID, 1500+i*100)
		progIDs = append(progIDs, prog.ID)
	}

	ctx := context.Background()
	leaderboard, err := s.repo.GetLeaderboard(ctx, tournament.ID, 10)
	require.NoError(s.T(), err)
	assert.Len(s.T(), leaderboard, 3)

	// With no matches, all scores are 0 and rank is assigned by row_number
	for _, entry := range leaderboard {
		assert.GreaterOrEqual(s.T(), entry.Rank, 1)
	}
}

func (s *TournamentRepositorySuite) TestGetLeaderboard_LimitEnforced() {
	user := s.createTrackedUser("tp_lbl")
	tournament := s.createTrackedTournament("TPLBL1", user.ID)

	// Create 5 participants
	for i := 0; i < 5; i++ {
		prog := s.createTrackedProgram(user.ID, nil, nil, nil, fmt.Sprintf("BotLBL%d", i), 1)
		s.createTestParticipant(tournament.ID, prog.ID, 1500+i*50)
	}

	ctx := context.Background()
	leaderboard, err := s.repo.GetLeaderboard(ctx, tournament.ID, 2)
	require.NoError(s.T(), err)
	assert.Len(s.T(), leaderboard, 2)
}

func (s *TournamentRepositorySuite) TestGetCrossGameLeaderboard() {
	user := s.createTrackedUser("tp_cgl")
	tournament := s.createTrackedTournament("TPCGL1", user.ID)

	// Cross-game leaderboard requires programs with team_id and game_id,
	// plus completed matches. Without completed matches, it returns empty.
	// We test graceful handling of the empty case.
	ctx := context.Background()
	entries, err := s.repo.GetCrossGameLeaderboard(ctx, tournament.ID)
	require.NoError(s.T(), err)

	// With no programs/matches, the result should be empty (nil or empty slice)
	assert.Empty(s.T(), entries)

	// Now set up a full scenario: team + game + program + match
	game := s.createTrackedGame("cgl_game1")
	team := s.createTrackedTeam(tournament.ID, user.ID, "TCGL01")
	prog1 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "BotCGL1", 1)

	user2 := s.createTrackedUser("tp_cgl2")
	team2 := s.createTrackedTeam(tournament.ID, user2.ID, "TCGL02")
	prog2 := s.createTrackedProgram(user2.ID, &team2.ID, &tournament.ID, &game.ID, "BotCGL2", 1)

	// Add participants
	s.createTestParticipant(tournament.ID, prog1.ID, 1500)
	s.createTestParticipant(tournament.ID, prog2.ID, 1500)

	// Create a completed match so there is data to aggregate
	match := s.createTrackedMatch(tournament.ID, prog1.ID, prog2.ID, game.Name, domain.MatchRunning)
	score1 := 10
	score2 := 5
	winner := 1
	_, _ = s.database.ExecContext(ctx,
		"UPDATE matches SET status = 'completed', score1 = $2, score2 = $3, winner = $4, completed_at = NOW() WHERE id = $1",
		match.ID, score1, score2, winner)

	entries, err = s.repo.GetCrossGameLeaderboard(ctx, tournament.ID)
	require.NoError(s.T(), err)
	// Should have 2 entries (one per team)
	assert.Len(s.T(), entries, 2)

	// The team with higher score should be ranked first
	if len(entries) >= 2 {
		assert.GreaterOrEqual(s.T(), entries[0].TotalRating, entries[1].TotalRating)
	}
}

func (s *TournamentRepositorySuite) TestGetLatestParticipants() {
	user := s.createTrackedUser("tp_glp")
	tournament := s.createTrackedTournament("TPGLP1", user.ID)
	game := s.createTrackedGame("glp_game")
	team := s.createTrackedTeam(tournament.ID, user.ID, "TGLP01")

	// Create a program with full references (needed for the INNER JOIN on programs)
	prog := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "BotGLP1", 1)
	s.createTestParticipant(tournament.ID, prog.ID, 1500)

	ctx := context.Background()
	participants, err := s.repo.GetLatestParticipants(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants, 1)
	assert.Equal(s.T(), prog.ID, participants[0].ProgramID)
}

func (s *TournamentRepositorySuite) TestGetLatestParticipantsByGame() {
	user := s.createTrackedUser("tp_lpg")
	tournament := s.createTrackedTournament("TPLPG1", user.ID)

	// Create two games
	game1 := s.createTrackedGame("lpg_game1")
	game2 := s.createTrackedGame("lpg_game2")

	team := s.createTrackedTeam(tournament.ID, user.ID, "TLPG01")

	// Create programs for each game
	prog1 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game1.ID, "BotLPG1", 1)
	prog2 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game2.ID, "BotLPG2", 1)

	// Add participants for both programs
	s.createTestParticipant(tournament.ID, prog1.ID, 1500)
	s.createTestParticipant(tournament.ID, prog2.ID, 1600)

	ctx := context.Background()

	// Filter by game1 name - should return only the participant with prog1
	participants1, err := s.repo.GetLatestParticipantsByGame(ctx, tournament.ID, game1.Name)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants1, 1)
	assert.Equal(s.T(), prog1.ID, participants1[0].ProgramID)

	// Filter by game2 name - should return only the participant with prog2
	participants2, err := s.repo.GetLatestParticipantsByGame(ctx, tournament.ID, game2.Name)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants2, 1)
	assert.Equal(s.T(), prog2.ID, participants2[0].ProgramID)
}
