//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MatchRepositorySuite struct {
	suite.Suite
	database       *db.DB
	repo           *db.MatchRepository
	userRepo       *db.UserRepository
	tournamentRepo *db.TournamentRepository
	programRepo    *db.ProgramRepository
	// track created IDs for cleanup
	matchIDs      []uuid.UUID
	programIDs    []uuid.UUID
	tournamentIDs []uuid.UUID
	userIDs       []uuid.UUID
}

func TestMatchRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &MatchRepositorySuite{
		database:       database,
		repo:           db.NewMatchRepository(database),
		userRepo:       db.NewUserRepository(database),
		tournamentRepo: db.NewTournamentRepository(database),
		programRepo:    db.NewProgramRepository(database),
	}
	suite.Run(t, s)
}

func (s *MatchRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// Delete in FK order: matches -> programs -> tournaments -> users
	for _, id := range s.matchIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM matches WHERE id = $1", id)
	}
	for _, id := range s.programIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM programs WHERE id = $1", id)
	}
	for _, id := range s.tournamentIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM tournaments WHERE id = $1", id)
	}
	for _, id := range s.userIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	}
	s.matchIDs = nil
	s.programIDs = nil
	s.tournamentIDs = nil
	s.userIDs = nil
}

// createUser creates a test user and tracks it for cleanup.
func (s *MatchRepositorySuite) createUser(suffix string) *domain.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

// createTournament creates a test tournament and tracks it for cleanup.
func (s *MatchRepositorySuite) createTournament(code string, creatorID uuid.UUID) *domain.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

// createProgram creates a minimal test program and tracks it for cleanup.
func (s *MatchRepositorySuite) createProgram(userID uuid.UUID, name string) *domain.Program {
	ctx := context.Background()
	program := &domain.Program{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     name,
		GameType: "prisoners_dilemma",
		CodePath: "/tmp/test/" + name + ".py",
		Language: "python",
		Version:  1,
	}
	err := s.programRepo.Create(ctx, program)
	require.NoError(s.T(), err)
	s.programIDs = append(s.programIDs, program.ID)
	return program
}

// createMatch creates a test match and tracks it for cleanup.
func (s *MatchRepositorySuite) createMatch(tournamentID, program1ID, program2ID uuid.UUID, gameType string, status domain.MatchStatus, priority domain.MatchPriority, roundNumber int) *domain.Match {
	ctx := context.Background()
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     gameType,
		Status:       status,
		Priority:     priority,
		RoundNumber:  roundNumber,
		CreatedAt:    time.Now(),
	}
	err := s.repo.Create(ctx, match)
	require.NoError(s.T(), err)
	s.matchIDs = append(s.matchIDs, match.ID)
	return match
}

// setupMatchPrerequisites creates a user, tournament, and two programs for match tests.
func (s *MatchRepositorySuite) setupMatchPrerequisites(suffix string) (tournament *domain.Tournament, prog1, prog2 *domain.Program) {
	user := s.createUser("match_" + suffix)
	tournament = s.createTournament("TM"+suffix, user.ID)
	prog1 = s.createProgram(user.ID, "Bot1_"+suffix)
	prog2 = s.createProgram(user.ID, "Bot2_"+suffix)
	return
}

func (s *MatchRepositorySuite) TestCreate() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("crt")

	ctx := context.Background()
	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tournament.ID,
		Program1ID:   prog1.ID,
		Program2ID:   prog2.ID,
		GameType:     "prisoners_dilemma",
		Status:       domain.MatchPending,
		Priority:     domain.PriorityMedium,
		RoundNumber:  1,
		CreatedAt:    time.Now(),
	}

	err := s.repo.Create(ctx, match)
	require.NoError(s.T(), err)
	s.matchIDs = append(s.matchIDs, match.ID)

	// Verify by fetching
	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), match.ID, result.ID)
	assert.Equal(s.T(), match.TournamentID, result.TournamentID)
	assert.Equal(s.T(), match.Program1ID, result.Program1ID)
	assert.Equal(s.T(), match.Program2ID, result.Program2ID)
	assert.Equal(s.T(), domain.MatchPending, result.Status)
	assert.Equal(s.T(), domain.PriorityMedium, result.Priority)
	assert.Equal(s.T(), 1, result.RoundNumber)
}

func (s *MatchRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *MatchRepositorySuite) TestCreateBatch() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("batch")

	ctx := context.Background()
	matches := []*domain.Match{
		{
			ID:           uuid.New(),
			TournamentID: tournament.ID,
			Program1ID:   prog1.ID,
			Program2ID:   prog2.ID,
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchPending,
			Priority:     domain.PriorityHigh,
			RoundNumber:  1,
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			TournamentID: tournament.ID,
			Program1ID:   prog2.ID,
			Program2ID:   prog1.ID,
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchPending,
			Priority:     domain.PriorityLow,
			RoundNumber:  1,
			CreatedAt:    time.Now(),
		},
	}

	err := s.repo.CreateBatch(ctx, matches)
	require.NoError(s.T(), err)
	for _, m := range matches {
		s.matchIDs = append(s.matchIDs, m.ID)
	}

	// Verify both were created
	for _, m := range matches {
		result, err := s.repo.GetByID(ctx, m.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), m.ID, result.ID)
	}
}

func (s *MatchRepositorySuite) TestCreateBatch_Empty() {
	ctx := context.Background()

	err := s.repo.CreateBatch(ctx, []*domain.Match{})
	require.NoError(s.T(), err)
}

func (s *MatchRepositorySuite) TestGetByTournamentID() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("gettid")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 2)

	ctx := context.Background()
	matches, err := s.repo.GetByTournamentID(ctx, tournament.ID, 10, 0)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// Verify ordering: round_number DESC, created_at DESC
	assert.GreaterOrEqual(s.T(), matches[0].RoundNumber, matches[1].RoundNumber)
}

func (s *MatchRepositorySuite) TestGetByTournamentID_Pagination() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("getpag")

	for i := 0; i < 5; i++ {
		s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, i+1)
	}

	ctx := context.Background()

	// First page
	matches, err := s.repo.GetByTournamentID(ctx, tournament.ID, 3, 0)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 3)

	// Second page
	matches, err = s.repo.GetByTournamentID(ctx, tournament.ID, 3, 3)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)
}

func (s *MatchRepositorySuite) TestGetPendingByTournamentID() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("pndtid")

	// Create pending matches with different priorities
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityLow, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityHigh, 1)
	// Create a completed match that should NOT appear
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)

	ctx := context.Background()
	matches, err := s.repo.GetPendingByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// Verify ordering: high priority first
	assert.Equal(s.T(), domain.PriorityHigh, matches[0].Priority)
	assert.Equal(s.T(), domain.PriorityLow, matches[1].Priority)

	// Verify all are pending
	for _, m := range matches {
		assert.Equal(s.T(), domain.MatchPending, m.Status)
	}
}

func (s *MatchRepositorySuite) TestGetPendingByTournamentAndGame() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("pndgm")

	// Create pending matches for two different game types
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityHigh, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()
	matches, err := s.repo.GetPendingByTournamentAndGame(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	for _, m := range matches {
		assert.Equal(s.T(), "prisoners_dilemma", m.GameType)
		assert.Equal(s.T(), domain.MatchPending, m.Status)
	}

	// Check the other game type
	matches, err = s.repo.GetPendingByTournamentAndGame(ctx, tournament.ID, "tug_of_war")
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 1)
}

func (s *MatchRepositorySuite) TestUpdateStatus() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updst")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()

	// Update to running (should set started_at)
	err := s.repo.UpdateStatus(ctx, match.ID, domain.MatchRunning)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchRunning, result.Status)
	assert.NotNil(s.T(), result.StartedAt, "started_at should be set when status is running")
}

func (s *MatchRepositorySuite) TestUpdateStatus_ToCompleted() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updcm")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()

	err := s.repo.UpdateStatus(ctx, match.ID, domain.MatchCompleted)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchCompleted, result.Status)
	// started_at should not be set when going directly to completed
	assert.Nil(s.T(), result.StartedAt)
}

func (s *MatchRepositorySuite) TestUpdateStatus_NotFound() {
	ctx := context.Background()

	err := s.repo.UpdateStatus(ctx, uuid.New(), domain.MatchRunning)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *MatchRepositorySuite) TestUpdateResult_Success() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updrs")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchRunning, domain.PriorityMedium, 1)

	ctx := context.Background()
	result := &domain.MatchResult{
		MatchID: match.ID,
		Score1:  10,
		Score2:  5,
		Winner:  1,
	}

	err := s.repo.UpdateResult(ctx, match.ID, result)
	require.NoError(s.T(), err)

	fetched, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchCompleted, fetched.Status)
	assert.NotNil(s.T(), fetched.Score1)
	assert.Equal(s.T(), 10, *fetched.Score1)
	assert.NotNil(s.T(), fetched.Score2)
	assert.Equal(s.T(), 5, *fetched.Score2)
	assert.NotNil(s.T(), fetched.Winner)
	assert.Equal(s.T(), 1, *fetched.Winner)
	assert.NotNil(s.T(), fetched.CompletedAt)
	assert.Nil(s.T(), fetched.ErrorCode)
	assert.Nil(s.T(), fetched.ErrorMessage)
}

func (s *MatchRepositorySuite) TestUpdateResult_WithError() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updre")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchRunning, domain.PriorityMedium, 1)

	ctx := context.Background()
	result := &domain.MatchResult{
		MatchID:      match.ID,
		Score1:       0,
		Score2:       0,
		Winner:       0,
		ErrorCode:    1,
		ErrorMessage: "timeout exceeded",
	}

	err := s.repo.UpdateResult(ctx, match.ID, result)
	require.NoError(s.T(), err)

	fetched, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchFailed, fetched.Status)
	assert.NotNil(s.T(), fetched.ErrorCode)
	assert.Equal(s.T(), 1, *fetched.ErrorCode)
	assert.NotNil(s.T(), fetched.ErrorMessage)
	assert.Equal(s.T(), "timeout exceeded", *fetched.ErrorMessage)
	assert.NotNil(s.T(), fetched.CompletedAt)
}

func (s *MatchRepositorySuite) TestResetFailedMatches() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("rstfld")

	// Create failed matches
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchFailed, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchFailed, domain.PriorityMedium, 1)
	// Create a pending match that should NOT be affected
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()
	affected, err := s.repo.ResetFailedMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), affected)

	// Verify all matches are now pending
	matches, err := s.repo.GetByTournamentID(ctx, tournament.ID, 10, 0)
	require.NoError(s.T(), err)
	for _, m := range matches {
		assert.Equal(s.T(), domain.MatchPending, m.Status)
	}
}

func (s *MatchRepositorySuite) TestResetFailedMatches_NoFailed() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("rstnf")
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()
	affected, err := s.repo.ResetFailedMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), affected)
}

func (s *MatchRepositorySuite) TestGetNextRoundNumber() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("nxtrnd")

	ctx := context.Background()

	// No matches yet - should return 1
	nextRound, err := s.repo.GetNextRoundNumber(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, nextRound)

	// Create matches in round 1
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	nextRound, err = s.repo.GetNextRoundNumber(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, nextRound)

	// Create a match in round 3 (skipping round 2)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 3)

	nextRound, err = s.repo.GetNextRoundNumber(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 4, nextRound)
}

func (s *MatchRepositorySuite) TestGetNextRoundNumberByGame() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("nxtrng")

	ctx := context.Background()

	// No matches for this game type
	nextRound, err := s.repo.GetNextRoundNumberByGame(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, nextRound)

	// Create matches for different game types
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 2)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", domain.MatchPending, domain.PriorityMedium, 1)

	nextRound, err = s.repo.GetNextRoundNumberByGame(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, nextRound)

	nextRound, err = s.repo.GetNextRoundNumberByGame(ctx, tournament.ID, "tug_of_war")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, nextRound)
}

func (s *MatchRepositorySuite) TestGetMatchesByRounds() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("mbrnd")

	// Create matches across multiple rounds and game types
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", domain.MatchCompleted, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 2)

	ctx := context.Background()
	rounds, err := s.repo.GetMatchesByRounds(ctx, tournament.ID)
	require.NoError(s.T(), err)

	// Should have 3 groups: (round 1, prisoners_dilemma), (round 1, tug_of_war), (round 2, prisoners_dilemma)
	assert.Len(s.T(), rounds, 3)

	// Verify each round has its matches populated
	for _, round := range rounds {
		assert.NotEmpty(s.T(), round.Matches, "round %d/%s should have matches", round.RoundNumber, round.GameType)
		assert.Equal(s.T(), round.TotalMatches, len(round.Matches))
	}
}

func (s *MatchRepositorySuite) TestGetStatistics() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("stats")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchFailed, domain.PriorityMedium, 1)

	ctx := context.Background()
	stats, err := s.repo.GetStatistics(ctx, &tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, stats.Total)
	assert.Equal(s.T(), 1, stats.Pending)
	assert.Equal(s.T(), 1, stats.Completed)
	assert.Equal(s.T(), 1, stats.Failed)
	assert.Equal(s.T(), 0, stats.Running)
}

func (s *MatchRepositorySuite) TestHasStartedMatches() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("hasst")

	ctx := context.Background()

	// No started matches
	has, err := s.repo.HasStartedMatches(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// Add pending match (should not count as started)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	has, err = s.repo.HasStartedMatches(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// Add completed match
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	has, err = s.repo.HasStartedMatches(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.True(s.T(), has)
}

func (s *MatchRepositorySuite) TestHasAnyRunningMatches() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("hasrn")

	ctx := context.Background()

	// No matches at all
	has, err := s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// Add completed match (should not count)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	has, err = s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// Add pending match
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	has, err = s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), has)
}

func (s *MatchRepositorySuite) TestGetActiveGameType() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("actgm")

	ctx := context.Background()

	// No active matches
	gameType, err := s.repo.GetActiveGameType(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), gameType)

	// Add pending match
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	gameType, err = s.repo.GetActiveGameType(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "prisoners_dilemma", gameType)
}

func (s *MatchRepositorySuite) TestDeleteMatchesForGame() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("delgm")

	m1 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	m3 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()
	affected, err := s.repo.DeleteMatchesForGame(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), affected)

	// Remove deleted match IDs from tracking
	s.matchIDs = []uuid.UUID{m3.ID}
	_ = m1 // already deleted via DeleteMatchesForGame

	// tug_of_war match should still exist
	result, err := s.repo.GetByID(ctx, m3.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "tug_of_war", result.GameType)
}

func (s *MatchRepositorySuite) TestList_WithFilters() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("listf")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()

	// Filter by status
	matches, err := s.repo.List(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
		Status:       domain.MatchPending,
		Limit:        10,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// Filter by game type
	matches, err = s.repo.List(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
		GameType:     "tug_of_war",
		Limit:        10,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 1)

	// Filter by program
	matches, err = s.repo.List(ctx, domain.MatchFilter{
		ProgramID: &prog1.ID,
		Limit:     10,
	})
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(matches), 3)
}

func (s *MatchRepositorySuite) TestGetPending() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("getpd")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityLow, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityHigh, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)

	ctx := context.Background()
	matches, err := s.repo.GetPending(ctx, 10)
	require.NoError(s.T(), err)

	// Should have at least 2 pending matches (may have more from other tests)
	assert.GreaterOrEqual(s.T(), len(matches), 2)

	// Verify they are all pending
	for _, m := range matches {
		assert.Equal(s.T(), domain.MatchPending, m.Status)
	}
}

// --- GetByID_Success, GetStuckRunning, BatchUpdateStatus, BatchUpdateResults, ListWithCursor ---

func (s *MatchRepositorySuite) TestGetByID_Success() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("gbids")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityHigh, 3)

	ctx := context.Background()
	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), match.ID, result.ID)
	assert.Equal(s.T(), tournament.ID, result.TournamentID)
	assert.Equal(s.T(), prog1.ID, result.Program1ID)
	assert.Equal(s.T(), prog2.ID, result.Program2ID)
	assert.Equal(s.T(), "prisoners_dilemma", result.GameType)
	assert.Equal(s.T(), domain.MatchPending, result.Status)
	assert.Equal(s.T(), domain.PriorityHigh, result.Priority)
	assert.Equal(s.T(), 3, result.RoundNumber)
	assert.Nil(s.T(), result.Score1)
	assert.Nil(s.T(), result.Score2)
	assert.Nil(s.T(), result.Winner)
	assert.Nil(s.T(), result.ErrorCode)
	assert.Nil(s.T(), result.ErrorMessage)
	assert.Nil(s.T(), result.StartedAt)
	assert.Nil(s.T(), result.CompletedAt)
	assert.NotZero(s.T(), result.CreatedAt)
}

func (s *MatchRepositorySuite) TestGetStuckRunning() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("stuck")

	ctx := context.Background()

	// Create a running match with started_at set far in the past
	stuckMatch := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	// Set status to running with an old started_at
	oldTime := time.Now().Add(-2 * time.Hour)
	_, err := s.database.ExecContext(ctx,
		"UPDATE matches SET status = $2, started_at = $3 WHERE id = $1",
		stuckMatch.ID, domain.MatchRunning, oldTime)
	require.NoError(s.T(), err)

	// Create a recently-started running match (should NOT be stuck)
	recentMatch := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	_, err = s.database.ExecContext(ctx,
		"UPDATE matches SET status = $2, started_at = NOW() WHERE id = $1",
		recentMatch.ID, domain.MatchRunning)
	require.NoError(s.T(), err)

	// Create a pending match (should NOT be returned)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	// Get stuck matches with 1-hour threshold
	stuckMatches, err := s.repo.GetStuckRunning(ctx, 1*time.Hour, 10)
	require.NoError(s.T(), err)

	// Should find at least the stuck match
	var foundStuck bool
	var foundRecent bool
	for _, m := range stuckMatches {
		if m.ID == stuckMatch.ID {
			foundStuck = true
		}
		if m.ID == recentMatch.ID {
			foundRecent = true
		}
	}
	assert.True(s.T(), foundStuck, "should find the stuck match")
	assert.False(s.T(), foundRecent, "should NOT find the recently started match")
}

func (s *MatchRepositorySuite) TestBatchUpdateStatus() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("batus")

	// Create 3 pending matches
	m1 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	m2 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	m3 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()
	matchIDs := []uuid.UUID{m1.ID, m2.ID, m3.ID}

	// Batch update to completed
	err := s.repo.BatchUpdateStatus(ctx, matchIDs, domain.MatchCompleted)
	require.NoError(s.T(), err)

	// Verify all 3 are now completed
	for _, id := range matchIDs {
		result, err := s.repo.GetByID(ctx, id)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), domain.MatchCompleted, result.Status)
	}
}

func (s *MatchRepositorySuite) TestBatchUpdateResults() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("batur")

	// Create 3 running matches
	m1 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchRunning, domain.PriorityMedium, 1)
	m2 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchRunning, domain.PriorityMedium, 1)
	m3 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchRunning, domain.PriorityMedium, 1)

	ctx := context.Background()

	results := map[uuid.UUID]*domain.MatchResult{
		m1.ID: {MatchID: m1.ID, Score1: 10, Score2: 5, Winner: 1},
		m2.ID: {MatchID: m2.ID, Score1: 3, Score2: 3, Winner: 0},
		m3.ID: {MatchID: m3.ID, Score1: 0, Score2: 0, Winner: 0, ErrorCode: 1, ErrorMessage: "timeout"},
	}

	err := s.repo.BatchUpdateResults(ctx, results)
	require.NoError(s.T(), err)

	// Verify m1: completed with scores
	r1, err := s.repo.GetByID(ctx, m1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchCompleted, r1.Status)
	require.NotNil(s.T(), r1.Score1)
	assert.Equal(s.T(), 10, *r1.Score1)
	require.NotNil(s.T(), r1.Score2)
	assert.Equal(s.T(), 5, *r1.Score2)
	require.NotNil(s.T(), r1.Winner)
	assert.Equal(s.T(), 1, *r1.Winner)
	assert.NotNil(s.T(), r1.CompletedAt)

	// Verify m2: completed draw
	r2, err := s.repo.GetByID(ctx, m2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchCompleted, r2.Status)
	require.NotNil(s.T(), r2.Winner)
	assert.Equal(s.T(), 0, *r2.Winner)

	// Verify m3: failed with error
	r3, err := s.repo.GetByID(ctx, m3.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchFailed, r3.Status)
	require.NotNil(s.T(), r3.ErrorCode)
	assert.Equal(s.T(), 1, *r3.ErrorCode)
	require.NotNil(s.T(), r3.ErrorMessage)
	assert.Equal(s.T(), "timeout", *r3.ErrorMessage)
}

func (s *MatchRepositorySuite) TestListWithCursor() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("lstcr")

	// Create 5 matches with slight time gaps to ensure distinct created_at values
	for i := 0; i < 5; i++ {
		s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	}

	ctx := context.Background()

	// First page: get first 2
	first := 2
	pageReq := &pagination.PageRequest{First: &first}
	matches, hasMore, err := s.repo.ListWithCursor(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
	}, pageReq)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)
	assert.True(s.T(), hasMore, "should have more pages with 5 total items and limit 2")

	// Use the created_at of the last result as cursor for the next page
	lastMatch := matches[len(matches)-1]
	cursor := pagination.NewTimestampCursor(lastMatch.CreatedAt)
	cursorStr, err := cursor.Encode()
	require.NoError(s.T(), err)

	// Second page
	pageReq2 := &pagination.PageRequest{First: &first, After: &cursorStr}
	matches2, hasMore2, err := s.repo.ListWithCursor(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
	}, pageReq2)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches2, 2)
	assert.True(s.T(), hasMore2, "should have one more page")

	// Verify no overlap between pages
	for _, m1 := range matches {
		for _, m2 := range matches2 {
			assert.NotEqual(s.T(), m1.ID, m2.ID, "pages should not overlap")
		}
	}
}
