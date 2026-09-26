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
	// айдишники для очистки
	ratingHistoryIDs []uuid.UUID
	participantIDs   []uuid.UUID
	matchIDs         []uuid.UUID
	programIDs       []uuid.UUID
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
	}
	suite.Run(t, s)
}

func (s *RatingRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// порядок FK: rating_history -> tournament_participants -> programs -> tournaments -> users
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
	for _, id := range s.userIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	}
	s.ratingHistoryIDs = nil
	s.matchIDs = nil
	s.participantIDs = nil
	s.programIDs = nil
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

func (s *RatingRepositorySuite) addParticipant(tournamentID, programID uuid.UUID, rating int) {
	s.participantIDs = append(s.participantIDs, addTestParticipant(s.T(), s.database, tournamentID, programID, rating))
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
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO rating_history (id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		history.ID, history.ProgramID, history.TournamentID, history.OldRating,
		history.NewRating, history.Change, history.MatchID, history.CreatedAt)
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

// последние limit точек программы в этом турнире, в хронологии
func (s *RatingRepositorySuite) TestGetByProgramAndTournament() {
	tournament, program := s.setupRatingPrerequisites("gbpt")
	other := s.createTournament("TRgbpt2", program.UserID)

	s.createRatingHistory(program.ID, tournament.ID, 1500, 1520, 20, nil)
	s.createRatingHistory(program.ID, tournament.ID, 1520, 1510, -10, nil)
	s.createRatingHistory(program.ID, tournament.ID, 1510, 1540, 30, nil)
	s.createRatingHistory(program.ID, other.ID, 1500, 1600, 100, nil)

	ctx := context.Background()
	history, err := s.repo.GetByProgramAndTournament(ctx, program.ID, tournament.ID, 2)
	require.NoError(s.T(), err)
	require.Len(s.T(), history, 2)
	assert.Equal(s.T(), 1510, history[0].NewRating)
	assert.Equal(s.T(), 1540, history[1].NewRating)
}

func (s *RatingRepositorySuite) TestRatingHistoryFields() {
	tournament, program := s.setupRatingPrerequisites("flds")
	matchID := uuid.New()

	history := s.createRatingHistory(program.ID, tournament.ID, 1500, 1530, 30, &matchID)

	ctx := context.Background()
	results, err := s.repo.GetByProgramAndTournament(ctx, program.ID, tournament.ID, 10)
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
