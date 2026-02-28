//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RatingRepositorySuite struct {
	suite.Suite
	database       *db.DB
	repo           *db.RatingRepository
	userRepo       *db.UserRepository
	tournamentRepo *db.TournamentRepository
	programRepo    *db.ProgramRepository
	// track created IDs for cleanup
	ratingHistoryIDs []uuid.UUID
	participantIDs   []uuid.UUID
	programIDs       []uuid.UUID
	tournamentIDs    []uuid.UUID
	userIDs          []uuid.UUID
}

func TestRatingRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &RatingRepositorySuite{
		database:       database,
		repo:           db.NewRatingRepository(database),
		userRepo:       db.NewUserRepository(database),
		tournamentRepo: db.NewTournamentRepository(database),
		programRepo:    db.NewProgramRepository(database),
	}
	suite.Run(t, s)
}

func (s *RatingRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// Delete in FK order: rating_history -> tournament_participants -> programs -> tournaments -> users
	for _, id := range s.ratingHistoryIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM rating_history WHERE id = $1", id)
	}
	for _, id := range s.participantIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM tournament_participants WHERE id = $1", id)
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
	s.ratingHistoryIDs = nil
	s.participantIDs = nil
	s.programIDs = nil
	s.tournamentIDs = nil
	s.userIDs = nil
}

// createUser creates a test user and tracks it for cleanup.
func (s *RatingRepositorySuite) createUser(suffix string) *domain.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

// createTournament creates a test tournament and tracks it for cleanup.
func (s *RatingRepositorySuite) createTournament(code string, creatorID uuid.UUID) *domain.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

// createProgram creates a minimal test program and tracks it for cleanup.
func (s *RatingRepositorySuite) createProgram(userID uuid.UUID, name string) *domain.Program {
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

// addParticipant adds a program as a tournament participant and tracks it for cleanup.
func (s *RatingRepositorySuite) addParticipant(tournamentID, programID uuid.UUID, rating int) *domain.TournamentParticipant {
	ctx := context.Background()
	participant := &domain.TournamentParticipant{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		ProgramID:    programID,
		Rating:       rating,
	}
	err := s.tournamentRepo.AddParticipant(ctx, participant)
	require.NoError(s.T(), err)
	s.participantIDs = append(s.participantIDs, participant.ID)
	return participant
}

// createRatingHistory creates a rating history entry and tracks it for cleanup.
func (s *RatingRepositorySuite) createRatingHistory(programID, tournamentID uuid.UUID, oldRating, newRating, change int, matchID *uuid.UUID) *domain.RatingHistory {
	ctx := context.Background()
	history := &domain.RatingHistory{
		ID:           uuid.New(),
		ProgramID:    programID,
		TournamentID: tournamentID,
		OldRating:    oldRating,
		NewRating:    newRating,
		Change:       change,
		MatchID:      matchID,
		CreatedAt:    time.Now(),
	}
	err := s.repo.Create(ctx, history)
	require.NoError(s.T(), err)
	s.ratingHistoryIDs = append(s.ratingHistoryIDs, history.ID)
	return history
}

// setupRatingPrerequisites creates a user, tournament, and program for rating tests.
func (s *RatingRepositorySuite) setupRatingPrerequisites(suffix string) (tournament *domain.Tournament, program *domain.Program) {
	user := s.createUser("rating_" + suffix)
	tournament = s.createTournament("TR"+suffix, user.ID)
	program = s.createProgram(user.ID, "RatingBot_"+suffix)
	return
}

func (s *RatingRepositorySuite) TestCreate() {
	tournament, program := s.setupRatingPrerequisites("crt")

	ctx := context.Background()
	history := &domain.RatingHistory{
		ID:           uuid.New(),
		ProgramID:    program.ID,
		TournamentID: tournament.ID,
		OldRating:    1500,
		NewRating:    1520,
		Change:       20,
		CreatedAt:    time.Now(),
	}

	err := s.repo.Create(ctx, history)
	require.NoError(s.T(), err)
	s.ratingHistoryIDs = append(s.ratingHistoryIDs, history.ID)
}

func (s *RatingRepositorySuite) TestCreate_WithMatchID() {
	tournament, program := s.setupRatingPrerequisites("crtm")

	matchID := uuid.New()
	s.createRatingHistory(program.ID, tournament.ID, 1500, 1520, 20, &matchID)
}

func (s *RatingRepositorySuite) TestGetByProgramID() {
	tournament, program := s.setupRatingPrerequisites("gbpid")

	// Create multiple rating history entries
	s.createRatingHistory(program.ID, tournament.ID, 1500, 1520, 20, nil)
	s.createRatingHistory(program.ID, tournament.ID, 1520, 1510, -10, nil)
	s.createRatingHistory(program.ID, tournament.ID, 1510, 1540, 30, nil)

	ctx := context.Background()
	history, err := s.repo.GetByProgramID(ctx, program.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), history, 3)

	// Verify ordering by created_at DESC
	for i := 0; i < len(history)-1; i++ {
		assert.True(s.T(), !history[i].CreatedAt.Before(history[i+1].CreatedAt),
			"history should be ordered by created_at DESC")
	}
}

func (s *RatingRepositorySuite) TestGetByProgramID_Empty() {
	ctx := context.Background()

	history, err := s.repo.GetByProgramID(ctx, uuid.New())
	require.NoError(s.T(), err)
	assert.Empty(s.T(), history)
}

func (s *RatingRepositorySuite) TestGetByTournamentID() {
	tournament, program := s.setupRatingPrerequisites("gbtid")
	user2 := s.createUser("rating_gbtid2")
	program2 := s.createProgram(user2.ID, "RatingBot_gbtid2")

	// Create history for both programs
	s.createRatingHistory(program.ID, tournament.ID, 1500, 1520, 20, nil)
	s.createRatingHistory(program2.ID, tournament.ID, 1500, 1480, -20, nil)

	ctx := context.Background()
	history, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), history, 2)

	// Verify all entries belong to this tournament
	for _, h := range history {
		assert.Equal(s.T(), tournament.ID, h.TournamentID)
	}
}

func (s *RatingRepositorySuite) TestUpdateParticipantRating() {
	tournament, program := s.setupRatingPrerequisites("updrt")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()
	// Delta-based: +100 from 1500 = 1600
	err := s.repo.UpdateParticipantRating(ctx, tournament.ID, program.ID, 100)
	require.NoError(s.T(), err)

	// Verify the rating was updated
	rating, err := s.repo.GetParticipantRating(ctx, tournament.ID, program.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1600, rating)
}

func (s *RatingRepositorySuite) TestUpdateParticipantRating_NotFound() {
	ctx := context.Background()

	err := s.repo.UpdateParticipantRating(ctx, uuid.New(), uuid.New(), 100)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *RatingRepositorySuite) TestUpdateParticipantStats_Win() {
	tournament, program := s.setupRatingPrerequisites("sttw")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()

	// Record a win
	err := s.repo.UpdateParticipantStats(ctx, tournament.ID, program.ID, true, false)
	require.NoError(s.T(), err)

	// Record another win
	err = s.repo.UpdateParticipantStats(ctx, tournament.ID, program.ID, true, false)
	require.NoError(s.T(), err)

	// Verify stats via direct query (no getter for full stats in the repo)
	var wins, losses, draws int
	err = s.database.QueryRowContext(ctx,
		"SELECT wins, losses, draws FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program.ID,
	).Scan(&wins, &losses, &draws)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, wins)
	assert.Equal(s.T(), 0, losses)
	assert.Equal(s.T(), 0, draws)
}

func (s *RatingRepositorySuite) TestUpdateParticipantStats_Loss() {
	tournament, program := s.setupRatingPrerequisites("sttl")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()

	err := s.repo.UpdateParticipantStats(ctx, tournament.ID, program.ID, false, false)
	require.NoError(s.T(), err)

	var wins, losses, draws int
	err = s.database.QueryRowContext(ctx,
		"SELECT wins, losses, draws FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program.ID,
	).Scan(&wins, &losses, &draws)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, wins)
	assert.Equal(s.T(), 1, losses)
	assert.Equal(s.T(), 0, draws)
}

func (s *RatingRepositorySuite) TestUpdateParticipantStats_Draw() {
	tournament, program := s.setupRatingPrerequisites("sttd")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()

	err := s.repo.UpdateParticipantStats(ctx, tournament.ID, program.ID, false, true)
	require.NoError(s.T(), err)

	var wins, losses, draws int
	err = s.database.QueryRowContext(ctx,
		"SELECT wins, losses, draws FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program.ID,
	).Scan(&wins, &losses, &draws)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, wins)
	assert.Equal(s.T(), 0, losses)
	assert.Equal(s.T(), 1, draws)
}

func (s *RatingRepositorySuite) TestUpdateParticipantStats_NotFound() {
	ctx := context.Background()

	err := s.repo.UpdateParticipantStats(ctx, uuid.New(), uuid.New(), true, false)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *RatingRepositorySuite) TestGetParticipantRating() {
	tournament, program := s.setupRatingPrerequisites("getpr")
	s.addParticipant(tournament.ID, program.ID, 1750)

	ctx := context.Background()
	rating, err := s.repo.GetParticipantRating(ctx, tournament.ID, program.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1750, rating)
}

func (s *RatingRepositorySuite) TestGetParticipantRating_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetParticipantRating(ctx, uuid.New(), uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *RatingRepositorySuite) TestGetParticipantRatings() {
	tournament, program1 := s.setupRatingPrerequisites("gprts")
	user2 := s.createUser("rating_gprts2")
	program2 := s.createProgram(user2.ID, "RatingBot_gprts2")

	s.addParticipant(tournament.ID, program1.ID, 1500)
	s.addParticipant(tournament.ID, program2.ID, 1600)

	ctx := context.Background()
	rating1, rating2, err := s.repo.GetParticipantRatings(ctx, tournament.ID, program1.ID, program2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1500, rating1)
	assert.Equal(s.T(), 1600, rating2)
}

func (s *RatingRepositorySuite) TestGetParticipantRatings_Program1NotFound() {
	tournament, program := s.setupRatingPrerequisites("gpr1n")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()
	_, _, err := s.repo.GetParticipantRatings(ctx, tournament.ID, uuid.New(), program.ID)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *RatingRepositorySuite) TestGetParticipantRatings_Program2NotFound() {
	tournament, program := s.setupRatingPrerequisites("gpr2n")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()
	_, _, err := s.repo.GetParticipantRatings(ctx, tournament.ID, program.ID, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *RatingRepositorySuite) TestRatingHistoryFields() {
	tournament, program := s.setupRatingPrerequisites("flds")
	matchID := uuid.New()

	history := s.createRatingHistory(program.ID, tournament.ID, 1500, 1530, 30, &matchID)

	ctx := context.Background()
	results, err := s.repo.GetByProgramID(ctx, program.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), results, 1)

	result := results[0]
	assert.Equal(s.T(), history.ID, result.ID)
	assert.Equal(s.T(), program.ID, result.ProgramID)
	assert.Equal(s.T(), tournament.ID, result.TournamentID)
	assert.Equal(s.T(), 1500, result.OldRating)
	assert.Equal(s.T(), 1530, result.NewRating)
	assert.Equal(s.T(), 30, result.Change)
	assert.NotNil(s.T(), result.MatchID)
	assert.Equal(s.T(), matchID, *result.MatchID)
	assert.NotZero(s.T(), result.CreatedAt)
}
