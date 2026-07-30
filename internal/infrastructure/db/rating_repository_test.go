//go:build integration

package db_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/rating"
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
	gameRepo       *db.GameRepository
	// айдишники для очистки
	ratingHistoryIDs []uuid.UUID
	participantIDs   []uuid.UUID
	programIDs       []uuid.UUID
	gameIDs          []uuid.UUID
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
		gameRepo:       db.NewGameRepository(database),
	}
	suite.Run(t, s)
}

func (s *RatingRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// порядок FK: rating_history -> tournament_participants -> programs -> tournaments -> games -> users
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
	for _, id := range s.gameIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM games WHERE id = $1", id)
	}
	for _, id := range s.userIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	}
	s.ratingHistoryIDs = nil
	s.participantIDs = nil
	s.programIDs = nil
	s.gameIDs = nil
	s.tournamentIDs = nil
	s.userIDs = nil
}

func (s *RatingRepositorySuite) createUser(suffix string) *domain.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

func (s *RatingRepositorySuite) createTournament(code string, creatorID uuid.UUID) *domain.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

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

// готовит юзера, турнир и прогу - типовой сетап для тестов рейтинга
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

	s.createRatingHistory(program.ID, tournament.ID, 1500, 1520, 20, nil)
	s.createRatingHistory(program.ID, tournament.ID, 1520, 1510, -10, nil)
	s.createRatingHistory(program.ID, tournament.ID, 1510, 1540, 30, nil)

	ctx := context.Background()
	history, err := s.repo.GetByProgramID(ctx, program.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), history, 3)

	// сортировка created_at DESC
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

	// история по обеим прогам
	s.createRatingHistory(program.ID, tournament.ID, 1500, 1520, 20, nil)
	s.createRatingHistory(program2.ID, tournament.ID, 1500, 1480, -20, nil)

	ctx := context.Background()
	history, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), history, 2)

	for _, h := range history {
		assert.Equal(s.T(), tournament.ID, h.TournamentID)
	}
}

func (s *RatingRepositorySuite) TestUpdateParticipantRating() {
	tournament, program := s.setupRatingPrerequisites("updrt")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()
	// дельта-апдейт: +100 от 1500 = 1600
	err := s.repo.UpdateParticipantRating(ctx, tournament.ID, program.ID, 100)
	require.NoError(s.T(), err)

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

	// две победы подряд
	err := s.repo.UpdateParticipantStats(ctx, tournament.ID, program.ID, true, false)
	require.NoError(s.T(), err)
	err = s.repo.UpdateParticipantStats(ctx, tournament.ID, program.ID, true, false)
	require.NoError(s.T(), err)

	// геттера полной статы в репо нет, читаем напрямую
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

func (s *RatingRepositorySuite) createGame(name string) *domain.Game {
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

func (s *RatingRepositorySuite) createProgramWithGame(userID uuid.UUID, gameID *uuid.UUID, name string) *domain.Program {
	ctx := context.Background()
	program := &domain.Program{
		ID:       uuid.New(),
		UserID:   userID,
		GameID:   gameID,
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

// ProcessMatchResultAtomic должен в одной транзакции обновить рейтинг+стату
// обоим участникам. проверяем zero-sum: сколько один выиграл, столько другой потерял.
func (s *RatingRepositorySuite) TestProcessMatchResultAtomic_Success() {
	tournament, program1 := s.setupRatingPrerequisites("pmat1")
	user2 := s.createUser("rating_pmat2")
	program2 := s.createProgram(user2.ID, "RatingBot_pmat2")

	s.addParticipant(tournament.ID, program1.ID, 1500)
	s.addParticipant(tournament.ID, program2.ID, 1500)

	ctx := context.Background()

	matchID := uuid.New()
	now := time.Now()

	// program1 победил: +32, program2 проиграл: -32
	update1 := &rating.ParticipantUpdate{
		ProgramID:    program1.ID,
		TournamentID: tournament.ID,
		History: &domain.RatingHistory{
			ID:           uuid.New(),
			ProgramID:    program1.ID,
			TournamentID: tournament.ID,
			OldRating:    1500,
			NewRating:    1532,
			Change:       32,
			MatchID:      &matchID,
			CreatedAt:    now,
		},
		RatingDelta: 32,
		Won:         true,
		Draw:        false,
	}
	update2 := &rating.ParticipantUpdate{
		ProgramID:    program2.ID,
		TournamentID: tournament.ID,
		History: &domain.RatingHistory{
			ID:           uuid.New(),
			ProgramID:    program2.ID,
			TournamentID: tournament.ID,
			OldRating:    1500,
			NewRating:    1468,
			Change:       -32,
			MatchID:      &matchID,
			CreatedAt:    now,
		},
		RatingDelta: -32,
		Won:         false,
		Draw:        false,
	}
	s.ratingHistoryIDs = append(s.ratingHistoryIDs, update1.History.ID, update2.History.ID)

	err := s.repo.ProcessMatchResultAtomic(ctx, update1, update2)
	require.NoError(s.T(), err)

	r1, err := s.repo.GetParticipantRating(ctx, tournament.ID, program1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1532, r1)

	r2, err := s.repo.GetParticipantRating(ctx, tournament.ID, program2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1468, r2)

	// zero-sum: суммарное изменение = 0
	assert.Equal(s.T(), 0, (r1-1500)+(r2-1500))

	// стата: program1 - 1 победа, program2 - 1 поражение
	var wins1, losses1, wins2, losses2 int
	err = s.database.QueryRowContext(ctx,
		"SELECT wins, losses FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program1.ID).Scan(&wins1, &losses1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, wins1)
	assert.Equal(s.T(), 0, losses1)

	err = s.database.QueryRowContext(ctx,
		"SELECT wins, losses FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program2.ID).Scan(&wins2, &losses2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, wins2)
	assert.Equal(s.T(), 1, losses2)
}

func (s *RatingRepositorySuite) TestProcessMatchResultAtomic_Draw() {
	tournament, program1 := s.setupRatingPrerequisites("pmdrw")
	user2 := s.createUser("rating_pmdrw2")
	program2 := s.createProgram(user2.ID, "RatingBot_pmdrw2")

	s.addParticipant(tournament.ID, program1.ID, 1500)
	s.addParticipant(tournament.ID, program2.ID, 1500)

	ctx := context.Background()

	matchID := uuid.New()
	now := time.Now()

	// ничья: у обоих дельта 0
	update1 := &rating.ParticipantUpdate{
		ProgramID:    program1.ID,
		TournamentID: tournament.ID,
		History: &domain.RatingHistory{
			ID:           uuid.New(),
			ProgramID:    program1.ID,
			TournamentID: tournament.ID,
			OldRating:    1500,
			NewRating:    1500,
			Change:       0,
			MatchID:      &matchID,
			CreatedAt:    now,
		},
		RatingDelta: 0,
		Won:         false,
		Draw:        true,
	}
	update2 := &rating.ParticipantUpdate{
		ProgramID:    program2.ID,
		TournamentID: tournament.ID,
		History: &domain.RatingHistory{
			ID:           uuid.New(),
			ProgramID:    program2.ID,
			TournamentID: tournament.ID,
			OldRating:    1500,
			NewRating:    1500,
			Change:       0,
			MatchID:      &matchID,
			CreatedAt:    now,
		},
		RatingDelta: 0,
		Won:         false,
		Draw:        true,
	}
	s.ratingHistoryIDs = append(s.ratingHistoryIDs, update1.History.ID, update2.History.ID)

	err := s.repo.ProcessMatchResultAtomic(ctx, update1, update2)
	require.NoError(s.T(), err)

	// рейтинги не поменялись
	r1, err := s.repo.GetParticipantRating(ctx, tournament.ID, program1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1500, r1)

	r2, err := s.repo.GetParticipantRating(ctx, tournament.ID, program2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1500, r2)

	// у обоих по одной ничьей
	var draws1, draws2 int
	err = s.database.QueryRowContext(ctx,
		"SELECT draws FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program1.ID).Scan(&draws1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, draws1)

	err = s.database.QueryRowContext(ctx,
		"SELECT draws FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program2.ID).Scan(&draws2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, draws2)
}

func (s *RatingRepositorySuite) TestUpdateParticipantRatingAndStats_Win() {
	tournament, program := s.setupRatingPrerequisites("upras")
	s.addParticipant(tournament.ID, program.ID, 1500)

	ctx := context.Background()

	err := s.repo.UpdateParticipantRatingAndStats(ctx, tournament.ID, program.ID, 50, true, false)
	require.NoError(s.T(), err)

	// рейтинг и стата обновились одной операцией
	rating, err := s.repo.GetParticipantRating(ctx, tournament.ID, program.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1550, rating)

	var wins, losses, draws int
	err = s.database.QueryRowContext(ctx,
		"SELECT wins, losses, draws FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, program.ID).Scan(&wins, &losses, &draws)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, wins)
	assert.Equal(s.T(), 0, losses)
	assert.Equal(s.T(), 0, draws)
}

func (s *RatingRepositorySuite) TestResetParticipantsForGame() {
	user := s.createUser("rating_rstg")
	tournament := s.createTournament("TRRST1", user.ID)
	game := s.createGame("rstg_game")

	// проги привязаны к игре
	prog1 := s.createProgramWithGame(user.ID, &game.ID, "RstBot1")
	prog2 := s.createProgramWithGame(user.ID, &game.ID, "RstBot2")

	s.addParticipant(tournament.ID, prog1.ID, 1500)
	s.addParticipant(tournament.ID, prog2.ID, 1500)

	ctx := context.Background()

	// уводим рейтинг и стату от дефолтов
	err := s.repo.UpdateParticipantRatingAndStats(ctx, tournament.ID, prog1.ID, 200, true, false)
	require.NoError(s.T(), err)
	err = s.repo.UpdateParticipantRatingAndStats(ctx, tournament.ID, prog2.ID, -100, false, false)
	require.NoError(s.T(), err)

	// перед сбросом значения не дефолтные
	r1, err := s.repo.GetParticipantRating(ctx, tournament.ID, prog1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1700, r1)

	// сброс всех участников этой игры
	affected, err := s.repo.ResetParticipantsForGame(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), affected)

	// вернулись к дефолтам: rating=1500, wins=0, losses=0, draws=0
	r1After, err := s.repo.GetParticipantRating(ctx, tournament.ID, prog1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1500, r1After)

	r2After, err := s.repo.GetParticipantRating(ctx, tournament.ID, prog2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1500, r2After)

	var wins, losses, draws int
	err = s.database.QueryRowContext(ctx,
		"SELECT wins, losses, draws FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournament.ID, prog1.ID).Scan(&wins, &losses, &draws)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, wins)
	assert.Equal(s.T(), 0, losses)
	assert.Equal(s.T(), 0, draws)
}

func (s *RatingRepositorySuite) TestResetParticipantsForGame_Empty() {
	user := s.createUser("rating_rste")
	tournament := s.createTournament("TRRSE1", user.ID)
	game := s.createGame("rste_game")

	ctx := context.Background()

	// участников этой игры нет - 0 затронуто, но без ошибки
	affected, err := s.repo.ResetParticipantsForGame(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), affected)
}

// регрессия на concurrent-deltas: N goroutine, каждая делает +1 через delta-based UPDATE.
// правильный результат: rating = baseline + N (никаких потерянных обновлений).
func (s *RatingRepositorySuite) TestUpdateParticipantRating_ConcurrentDeltas() {
	tournament, program := s.setupRatingPrerequisites("crace")
	baseline := 1500
	s.addParticipant(tournament.ID, program.ID, baseline)

	ctx := context.Background()
	const n = 100

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := s.repo.UpdateParticipantRating(ctx, tournament.ID, program.ID, 1); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(s.T(), err)
	}

	final, err := s.repo.GetParticipantRating(ctx, tournament.ID, program.ID)
	require.NoError(s.T(), err)
	// каждый из N параллельных UPDATE добавил +1, итого +N.
	// если БД не сериализует корректно, получим меньше baseline+N (lost update).
	assert.Equal(s.T(), baseline+n, final,
		"concurrent delta-based UPDATE must not lose updates (MVCC row-lock invariant)")
}
