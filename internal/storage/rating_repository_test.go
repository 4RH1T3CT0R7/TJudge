//go:build integration

package storage_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/rating"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RatingRepositorySuite struct {
	suite.Suite
	database       *storage.DB
	repo           *storage.RatingRepository
	matchRepo      *storage.MatchRepository
	userRepo       *storage.UserRepository
	tournamentRepo *storage.TournamentRepository
	programRepo    *storage.ProgramRepository
	gameRepo       *storage.GameRepository
	// айдишники для очистки
	ratingHistoryIDs []uuid.UUID
	participantIDs   []uuid.UUID
	matchIDs         []uuid.UUID
	programIDs       []uuid.UUID
	gameIDs          []uuid.UUID
	tournamentIDs    []uuid.UUID
	userIDs          []uuid.UUID
}

func TestRatingRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &RatingRepositorySuite{
		database:       database,
		repo:           storage.NewRatingRepository(database),
		matchRepo:      storage.NewMatchRepository(database),
		userRepo:       storage.NewUserRepository(database),
		tournamentRepo: storage.NewTournamentRepository(database),
		programRepo:    storage.NewProgramRepository(database),
		gameRepo:       storage.NewGameRepository(database),
	}
	suite.Run(t, s)
}

func (s *RatingRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// порядок FK: rating_history -> tournament_participants -> programs -> tournaments -> games -> users
	for _, id := range s.ratingHistoryIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM rating_history WHERE id = $1", id)
	}
	for _, id := range s.matchIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM rating_history WHERE match_id = $1", id)
		_, _ = s.database.ExecContext(ctx, "DELETE FROM match_outbox WHERE match_id = $1", id)
		_, _ = s.database.ExecContext(ctx, "DELETE FROM matches WHERE id = $1", id)
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
	s.matchIDs = nil
	s.participantIDs = nil
	s.programIDs = nil
	s.gameIDs = nil
	s.tournamentIDs = nil
	s.userIDs = nil
}

func (s *RatingRepositorySuite) createUser(suffix string) *models.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

func (s *RatingRepositorySuite) createTournament(code string, creatorID uuid.UUID) *models.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

func (s *RatingRepositorySuite) createProgram(userID uuid.UUID, name string) *models.Program {
	ctx := context.Background()
	program := &models.Program{
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

func (s *RatingRepositorySuite) addParticipant(tournamentID, programID uuid.UUID, rating int) *models.TournamentParticipant {
	ctx := context.Background()
	participant := &models.TournamentParticipant{
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

func (s *RatingRepositorySuite) createRatingHistory(programID, tournamentID uuid.UUID, oldRating, newRating, change int, matchID *uuid.UUID) *models.RatingHistory {
	ctx := context.Background()
	history := &models.RatingHistory{
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
func (s *RatingRepositorySuite) setupRatingPrerequisites(suffix string) (tournament *models.Tournament, program *models.Program) {
	user := s.createUser("rating_" + suffix)
	tournament = s.createTournament("TR"+suffix, user.ID)
	program = s.createProgram(user.ID, "RatingBot_"+suffix)
	return
}

func (s *RatingRepositorySuite) TestCreate() {
	tournament, program := s.setupRatingPrerequisites("crt")

	ctx := context.Background()
	history := &models.RatingHistory{
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

	// геттера полной статы в репо нет, чтение напрямую
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

func (s *RatingRepositorySuite) createGame(name string) *models.Game {
	ctx := context.Background()
	game := &models.Game{
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

func (s *RatingRepositorySuite) createProgramWithGame(userID uuid.UUID, gameID *uuid.UUID, name string) *models.Program {
	ctx := context.Background()
	program := &models.Program{
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

// completedMatch создаёт матч так же, как его проводит воркер: running,
// затем результат вместе с outbox-задачей рейтинга
func (s *RatingRepositorySuite) completedMatch(tournamentID, program1ID, program2ID uuid.UUID, winner int) *models.Match {
	ctx := context.Background()
	match := &models.Match{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     "prisoners_dilemma",
		Status:       models.MatchRunning,
		Priority:     models.PriorityMedium,
		RoundNumber:  1,
		CreatedAt:    time.Now(),
	}
	require.NoError(s.T(), s.matchRepo.Create(ctx, match))
	s.matchIDs = append(s.matchIDs, match.ID)
	require.NoError(s.T(), s.matchRepo.UpdateResultWithOutbox(ctx, match.ID,
		&models.MatchResult{MatchID: match.ID, Score1: 3, Score2: 1, Winner: winner}))
	match.Status = models.MatchCompleted
	match.Winner = &winner
	return match
}

func (s *RatingRepositorySuite) ratingService() *rating.Service {
	log, _ := logger.New("error", "json")
	return rating.NewService(s.repo, events.NoopNotifier{}, log)
}

func (s *RatingRepositorySuite) stats(tournamentID, programID uuid.UUID) (ratingValue, wins, losses int) {
	err := s.database.QueryRowContext(context.Background(),
		"SELECT rating, wins, losses FROM tournament_participants WHERE tournament_id = $1 AND program_id = $2",
		tournamentID, programID).Scan(&ratingValue, &wins, &losses)
	require.NoError(s.T(), err)
	return
}

// рейтинг за матч применяется один раз: повторный вызов (fast path после
// диспетчера или наоборот) ничего не меняет, outbox-задача закрыта
func (s *RatingRepositorySuite) TestApplyMatchResult_Idempotent() {
	tournament, program1 := s.setupRatingPrerequisites("apid")
	user2 := s.createUser("rating_apid2")
	program2 := s.createProgram(user2.ID, "RatingBot_apid2")
	s.addParticipant(tournament.ID, program1.ID, 1500)
	s.addParticipant(tournament.ID, program2.ID, 1500)

	ctx := context.Background()
	svc := s.ratingService()
	match := s.completedMatch(tournament.ID, program1.ID, program2.ID, 1)

	require.NoError(s.T(), svc.ProcessMatchResult(ctx, match))
	require.NoError(s.T(), svc.ProcessMatchResult(ctx, match))

	r1, wins1, _ := s.stats(tournament.ID, program1.ID)
	r2, _, losses2 := s.stats(tournament.ID, program2.ID)
	assert.Equal(s.T(), 1516, r1)
	assert.Equal(s.T(), 1484, r2)
	assert.Equal(s.T(), 1, wins1)
	assert.Equal(s.T(), 1, losses2)

	var historyRows int
	require.NoError(s.T(), s.database.GetContext(ctx, &historyRows,
		"SELECT COUNT(*) FROM rating_history WHERE match_id = $1", match.ID))
	assert.Equal(s.T(), 2, historyRows)

	var outboxStatus string
	require.NoError(s.T(), s.database.GetContext(ctx, &outboxStatus,
		"SELECT status FROM match_outbox WHERE match_id = $1", match.ID))
	assert.Equal(s.T(), "done", outboxStatus)
}

// матч удалили (сброс раунда) до применения рейтинга - дельта не ложится
func (s *RatingRepositorySuite) TestApplyMatchResult_MatchDeleted() {
	tournament, program1 := s.setupRatingPrerequisites("apdel")
	user2 := s.createUser("rating_apdel2")
	program2 := s.createProgram(user2.ID, "RatingBot_apdel2")
	s.addParticipant(tournament.ID, program1.ID, 1500)
	s.addParticipant(tournament.ID, program2.ID, 1500)

	ctx := context.Background()
	match := s.completedMatch(tournament.ID, program1.ID, program2.ID, 1)
	_, err := s.database.ExecContext(ctx, "DELETE FROM matches WHERE id = $1", match.ID)
	require.NoError(s.T(), err)

	require.NoError(s.T(), s.ratingService().ProcessMatchResult(ctx, match))

	r1, wins1, _ := s.stats(tournament.ID, program1.ID)
	assert.Equal(s.T(), 1500, r1)
	assert.Equal(s.T(), 0, wins1)
}

// история есть, а outbox pending: так оставлял старый воркер, писавший рейтинг
// до закрытия задачи. повторно рейтинг не применяется
func (s *RatingRepositorySuite) TestApplyMatchResult_HistoryWithPendingOutbox() {
	tournament, program1 := s.setupRatingPrerequisites("aphist")
	user2 := s.createUser("rating_aphist2")
	program2 := s.createProgram(user2.ID, "RatingBot_aphist2")
	s.addParticipant(tournament.ID, program1.ID, 1516)
	s.addParticipant(tournament.ID, program2.ID, 1484)

	ctx := context.Background()
	match := s.completedMatch(tournament.ID, program1.ID, program2.ID, 1)
	s.createRatingHistory(program1.ID, tournament.ID, 1500, 1516, 16, &match.ID)
	s.createRatingHistory(program2.ID, tournament.ID, 1500, 1484, -16, &match.ID)

	require.NoError(s.T(), s.ratingService().ProcessMatchResult(ctx, match))

	r1, wins1, _ := s.stats(tournament.ID, program1.ID)
	assert.Equal(s.T(), 1516, r1)
	assert.Equal(s.T(), 0, wins1)

	var historyRows int
	require.NoError(s.T(), s.database.GetContext(ctx, &historyRows,
		"SELECT COUNT(*) FROM rating_history WHERE match_id = $1", match.ID))
	assert.Equal(s.T(), 2, historyRows)
}

// fast path и диспетчер одновременно по одному матчу: применяется ровно один раз
func (s *RatingRepositorySuite) TestApplyMatchResult_ConcurrentSameMatch() {
	tournament, program1 := s.setupRatingPrerequisites("apcc")
	user2 := s.createUser("rating_apcc2")
	program2 := s.createProgram(user2.ID, "RatingBot_apcc2")
	s.addParticipant(tournament.ID, program1.ID, 1500)
	s.addParticipant(tournament.ID, program2.ID, 1500)

	ctx := context.Background()
	svc := s.ratingService()
	match := s.completedMatch(tournament.ID, program1.ID, program2.ID, 2)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			errs <- svc.ProcessMatchResult(ctx, match)
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(s.T(), err)
	}

	r1, _, losses1 := s.stats(tournament.ID, program1.ID)
	assert.Equal(s.T(), 1484, r1)
	assert.Equal(s.T(), 1, losses1)

	var historyRows int
	require.NoError(s.T(), s.database.GetContext(ctx, &historyRows,
		"SELECT COUNT(*) FROM rating_history WHERE match_id = $1", match.ID))
	assert.Equal(s.T(), 2, historyRows)
}

// матчи AB и BA параллельно: без дедлоков, а история рейтинга каждой
// программы - непрерывная цепочка, сходящаяся с итоговым рейтингом
func (s *RatingRepositorySuite) TestApplyMatchResult_ConcurrentABBA() {
	tournament, programA := s.setupRatingPrerequisites("apab")
	userB := s.createUser("rating_apab2")
	programB := s.createProgram(userB.ID, "RatingBot_apab2")
	s.addParticipant(tournament.ID, programA.ID, 1500)
	s.addParticipant(tournament.ID, programB.ID, 1500)

	ctx := context.Background()
	svc := s.ratingService()

	const pairs = 15
	var matches []*models.Match
	for range pairs {
		matches = append(matches,
			s.completedMatch(tournament.ID, programA.ID, programB.ID, 1),
			s.completedMatch(tournament.ID, programB.ID, programA.ID, 1),
		)
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(matches))
	for _, m := range matches {
		wg.Go(func() {
			errs <- svc.ProcessMatchResult(ctx, m)
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(s.T(), err)
	}

	for _, programID := range []uuid.UUID{programA.ID, programB.ID} {
		current, wins, losses := s.stats(tournament.ID, programID)
		assert.Equal(s.T(), 2*pairs, wins+losses)

		var history []*models.RatingHistory
		require.NoError(s.T(), s.database.SelectContext(ctx, &history, `
			SELECT id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at
			FROM rating_history WHERE tournament_id = $1 AND program_id = $2
			ORDER BY created_at`, tournament.ID, programID))
		require.Len(s.T(), history, 2*pairs)

		prev := 1500
		for _, h := range history {
			assert.Equal(s.T(), prev, h.OldRating, "old_rating должен совпадать с предыдущим new_rating")
			prev = h.NewRating
		}
		assert.Equal(s.T(), current, prev)
	}
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

	// рейтинг и стата уводятся от дефолтов
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
	// если БД не сериализует корректно, выйдет меньше baseline+N (lost update).
	assert.Equal(s.T(), baseline+n, final,
		"concurrent delta-based UPDATE must not lose updates (MVCC row-lock invariant)")
}
