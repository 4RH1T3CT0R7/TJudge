//go:build integration

package storage_test

import (
	"context"
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

type MatchRepositorySuite struct {
	suite.Suite
	database       *storage.DB
	repo           *storage.MatchRepository
	userRepo       *storage.UserRepository
	tournamentRepo *storage.TournamentRepository
	programRepo    *storage.ProgramRepository
	// айдишники того что создали - чтобы прибрать за собой
	matchIDs      []uuid.UUID
	programIDs    []uuid.UUID
	tournamentIDs []uuid.UUID
	userIDs       []uuid.UUID
}

func TestMatchRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &MatchRepositorySuite{
		database:       database,
		repo:           storage.NewMatchRepository(database),
		userRepo:       storage.NewUserRepository(database),
		tournamentRepo: storage.NewTournamentRepository(database),
		programRepo:    storage.NewProgramRepository(database),
	}
	suite.Run(t, s)
}

func (s *MatchRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// порядок важен из-за FK: matches -> programs -> tournaments -> users
	for _, id := range s.matchIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM match_outbox WHERE match_id = $1", id)
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

func (s *MatchRepositorySuite) createUser(suffix string) *models.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

func (s *MatchRepositorySuite) createTournament(code string, creatorID uuid.UUID) *models.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

func (s *MatchRepositorySuite) createProgram(userID uuid.UUID, name string) *models.Program {
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

func (s *MatchRepositorySuite) createMatch(tournamentID, program1ID, program2ID uuid.UUID, gameType string, status models.MatchStatus, priority models.MatchPriority, roundNumber int) *models.Match {
	ctx := context.Background()
	match := &models.Match{
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

// готовит юзера, турнир и две проги - типовой сетап почти для всех тестов матчей
func (s *MatchRepositorySuite) setupMatchPrerequisites(suffix string) (tournament *models.Tournament, prog1, prog2 *models.Program) {
	user := s.createUser("match_" + suffix)
	tournament = s.createTournament("TM"+suffix, user.ID)
	prog1 = s.createProgram(user.ID, "Bot1_"+suffix)
	prog2 = s.createProgram(user.ID, "Bot2_"+suffix)
	return
}

func (s *MatchRepositorySuite) TestCreate() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("crt")

	ctx := context.Background()
	match := &models.Match{
		ID:           uuid.New(),
		TournamentID: tournament.ID,
		Program1ID:   prog1.ID,
		Program2ID:   prog2.ID,
		GameType:     "prisoners_dilemma",
		Status:       models.MatchPending,
		Priority:     models.PriorityMedium,
		RoundNumber:  1,
		CreatedAt:    time.Now(),
	}

	err := s.repo.Create(ctx, match)
	require.NoError(s.T(), err)
	s.matchIDs = append(s.matchIDs, match.ID)

	// перечитывание и сверка
	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), match.ID, result.ID)
	assert.Equal(s.T(), match.TournamentID, result.TournamentID)
	assert.Equal(s.T(), match.Program1ID, result.Program1ID)
	assert.Equal(s.T(), match.Program2ID, result.Program2ID)
	assert.Equal(s.T(), models.MatchPending, result.Status)
	assert.Equal(s.T(), models.PriorityMedium, result.Priority)
	assert.Equal(s.T(), 1, result.RoundNumber)
}

func (s *MatchRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// новый раунд целиком заменяет матчи игры, а при running-матчах не меняет ничего
func (s *MatchRepositorySuite) TestStartNewRound() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("snr")
	ctx := context.Background()

	gameRepo := storage.NewGameRepository(s.database)
	game := &models.Game{ID: uuid.New(), Name: "test_snr_" + uuid.New().String()[:8], DisplayName: "SNR"}
	require.NoError(s.T(), gameRepo.Create(ctx, game))
	s.T().Cleanup(func() { _, _ = s.database.ExecContext(ctx, "DELETE FROM games WHERE id = $1", game.ID) })
	require.NoError(s.T(), gameRepo.AddToTournament(ctx, tournament.ID, game.ID))

	old := s.createMatch(tournament.ID, prog1.ID, prog2.ID, game.Name, models.MatchCompleted, models.PriorityMedium, 1)

	newRound := func() []*models.Match {
		var round []*models.Match
		for _, pair := range [][2]uuid.UUID{{prog1.ID, prog2.ID}, {prog2.ID, prog1.ID}} {
			m := &models.Match{
				ID:           uuid.New(),
				TournamentID: tournament.ID,
				Program1ID:   pair[0],
				Program2ID:   pair[1],
				GameType:     game.Name,
				Status:       models.MatchPending,
				Priority:     models.PriorityHigh,
				RoundNumber:  1,
				CreatedAt:    time.Now(),
			}
			round = append(round, m)
			s.matchIDs = append(s.matchIDs, m.ID)
		}
		return round
	}

	round := newRound()
	require.NoError(s.T(), gameRepo.StartNewRound(ctx, tournament.ID, []string{game.Name}, round))

	_, err := s.repo.GetByID(ctx, old.ID)
	assert.True(s.T(), errors.IsNotFound(err), "матч прошлого раунда должен быть удалён")
	for _, m := range round {
		got, err := s.repo.GetByID(ctx, m.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.MatchPending, got.Status)
	}

	// running-матч: транзакция откатывается, текущий раунд не тронут
	_, err = s.database.ExecContext(ctx, "UPDATE matches SET status = 'running' WHERE id = $1", round[0].ID)
	require.NoError(s.T(), err)
	err = gameRepo.StartNewRound(ctx, tournament.ID, []string{game.Name}, newRound())
	appErr := errors.GetAppError(err)
	require.NotNil(s.T(), appErr)
	assert.Equal(s.T(), 409, appErr.Code)
	_, err = s.repo.GetByID(ctx, round[1].ID)
	require.NoError(s.T(), err)
}

func (s *MatchRepositorySuite) TestGetByTournamentID() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("gettid")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 2)

	ctx := context.Background()
	matches, err := s.repo.GetByTournamentID(ctx, tournament.ID, 10, 0)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// сортировка round_number DESC, created_at DESC
	assert.GreaterOrEqual(s.T(), matches[0].RoundNumber, matches[1].RoundNumber)
}

func (s *MatchRepositorySuite) TestGetByTournamentID_Pagination() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("getpag")

	for i := 0; i < 5; i++ {
		s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, i+1)
	}

	ctx := context.Background()

	// первая страница
	matches, err := s.repo.GetByTournamentID(ctx, tournament.ID, 3, 0)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 3)

	// вторая страница
	matches, err = s.repo.GetByTournamentID(ctx, tournament.ID, 3, 3)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)
}

func (s *MatchRepositorySuite) TestGetPendingByTournamentID() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("pndtid")

	// pending с разными приоритетами
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityLow, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityHigh, 1)
	// completed попасть в выборку не должен
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)

	ctx := context.Background()
	matches, err := s.repo.GetPendingByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// high приоритет первым
	assert.Equal(s.T(), models.PriorityHigh, matches[0].Priority)
	assert.Equal(s.T(), models.PriorityLow, matches[1].Priority)

	for _, m := range matches {
		assert.Equal(s.T(), models.MatchPending, m.Status)
	}
}

func (s *MatchRepositorySuite) TestGetPendingByTournamentAndGame() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("pndgm")

	// pending под две разные игры
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityHigh, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", models.MatchPending, models.PriorityMedium, 1)

	ctx := context.Background()
	matches, err := s.repo.GetPendingByTournamentAndGame(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	for _, m := range matches {
		assert.Equal(s.T(), "prisoners_dilemma", m.GameType)
		assert.Equal(s.T(), models.MatchPending, m.Status)
	}

	// вторая игра
	matches, err = s.repo.GetPendingByTournamentAndGame(ctx, tournament.ID, "tug_of_war")
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 1)
}

func (s *MatchRepositorySuite) TestUpdateStatus() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updst")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)

	ctx := context.Background()

	// перевод в running должен проставить started_at
	err := s.repo.UpdateStatus(ctx, match.ID, models.MatchRunning)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchRunning, result.Status)
	assert.NotNil(s.T(), result.StartedAt, "started_at should be set when status is running")
}

func (s *MatchRepositorySuite) TestUpdateStatus_ToCompleted() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updcm")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)

	ctx := context.Background()

	err := s.repo.UpdateStatus(ctx, match.ID, models.MatchCompleted)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchCompleted, result.Status)
	// сразу в completed - started_at не трогается
	assert.Nil(s.T(), result.StartedAt)
}

// перевод в running атомарно уводит матч из pending. если матча нет или он
// уже не pending - это не not found, а защита от двойной обработки
// (ErrMatchAlreadyProcessed). а вот обычный статус по несуществующему id - not found.
func (s *MatchRepositorySuite) TestUpdateStatus_NotFound() {
	ctx := context.Background()

	// running по несуществующему id: 0 строк -> already processed, не not found
	err := s.repo.UpdateStatus(ctx, uuid.New(), models.MatchRunning)
	assert.Error(s.T(), err)
	assert.ErrorIs(s.T(), err, models.ErrMatchAlreadyProcessed)

	// а не-running статус по несуществующему id - это уже not found
	err = s.repo.UpdateStatus(ctx, uuid.New(), models.MatchCompleted)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *MatchRepositorySuite) TestUpdateResult_Success() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updrs")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchRunning, models.PriorityMedium, 1)

	ctx := context.Background()
	result := &models.MatchResult{
		MatchID: match.ID,
		Score1:  10,
		Score2:  5,
		Winner:  1,
	}

	err := s.repo.UpdateResult(ctx, match.ID, result)
	require.NoError(s.T(), err)

	fetched, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchCompleted, fetched.Status)
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
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchRunning, models.PriorityMedium, 1)

	ctx := context.Background()
	result := &models.MatchResult{
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
	assert.Equal(s.T(), models.MatchFailed, fetched.Status)
	assert.NotNil(s.T(), fetched.ErrorCode)
	assert.Equal(s.T(), 1, *fetched.ErrorCode)
	assert.NotNil(s.T(), fetched.ErrorMessage)
	assert.Equal(s.T(), "timeout exceeded", *fetched.ErrorMessage)
	assert.NotNil(s.T(), fetched.CompletedAt)
}

// отменённый (дисквалификация) матч результатом не перезаписывается и
// outbox-задачу не получает
func (s *MatchRepositorySuite) TestUpdateResultWithOutbox_NotRunning() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updnr")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCancelled, models.PriorityMedium, 1)

	ctx := context.Background()
	result := &models.MatchResult{MatchID: match.ID, Score1: 10, Score2: 5, Winner: 1}

	err := s.repo.UpdateResultWithOutbox(ctx, match.ID, result)
	assert.ErrorIs(s.T(), err, models.ErrMatchAlreadyProcessed)
	assert.ErrorIs(s.T(), s.repo.UpdateResult(ctx, match.ID, result), models.ErrMatchAlreadyProcessed)

	fetched, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchCancelled, fetched.Status)
	assert.Nil(s.T(), fetched.Winner)

	var outboxRows int
	require.NoError(s.T(), s.database.GetContext(ctx, &outboxRows,
		"SELECT COUNT(*) FROM match_outbox WHERE match_id = $1", match.ID))
	assert.Zero(s.T(), outboxRows)
}

func (s *MatchRepositorySuite) TestResetFailedMatches() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("rstfld")

	// два зафейленных
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchFailed, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchFailed, models.PriorityMedium, 1)
	// pending трогать нельзя
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)

	ctx := context.Background()
	affected, err := s.repo.ResetFailedMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), affected)

	// после сброса все pending
	matches, err := s.repo.GetByTournamentID(ctx, tournament.ID, 10, 0)
	require.NoError(s.T(), err)
	for _, m := range matches {
		assert.Equal(s.T(), models.MatchPending, m.Status)
	}
}

func (s *MatchRepositorySuite) TestResetFailedMatches_NoFailed() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("rstnf")
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)

	ctx := context.Background()
	affected, err := s.repo.ResetFailedMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), affected)
}

func (s *MatchRepositorySuite) TestGetMatchesByRounds() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("mbrnd")

	// матчи по нескольким раундам и играм
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", models.MatchCompleted, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 2)

	ctx := context.Background()
	rounds, err := s.repo.GetMatchesByRounds(ctx, tournament.ID)
	require.NoError(s.T(), err)

	// три группы: (раунд 1, prisoners_dilemma), (раунд 1, tug_of_war), (раунд 2, prisoners_dilemma)
	assert.Len(s.T(), rounds, 3)

	for _, round := range rounds {
		assert.NotEmpty(s.T(), round.Matches, "round %d/%s should have matches", round.RoundNumber, round.GameType)
		assert.Equal(s.T(), round.TotalMatches, len(round.Matches))
	}
}

func (s *MatchRepositorySuite) TestGetStatistics() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("stats")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchFailed, models.PriorityMedium, 1)

	ctx := context.Background()
	stats, err := s.repo.GetStatistics(ctx, &tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, stats.Total)
	assert.Equal(s.T(), 1, stats.Pending)
	assert.Equal(s.T(), 1, stats.Completed)
	assert.Equal(s.T(), 1, stats.Failed)
	assert.Equal(s.T(), 0, stats.Running)
}

func (s *MatchRepositorySuite) TestHasAnyRunningMatches() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("hasrn")

	ctx := context.Background()

	// вообще матчей нет
	has, err := s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// completed не в счёт
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)
	has, err = s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// pending уже считается "есть незавершённые"
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	has, err = s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), has)
}

func (s *MatchRepositorySuite) TestGetActiveGameType() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("actgm")

	ctx := context.Background()

	// активных матчей нет
	gameType, err := s.repo.GetActiveGameType(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), gameType)

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	gameType, err = s.repo.GetActiveGameType(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "prisoners_dilemma", gameType)
}

func (s *MatchRepositorySuite) TestList_WithFilters() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("listf")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", models.MatchPending, models.PriorityMedium, 1)

	ctx := context.Background()

	// фильтр по статусу
	matches, err := s.repo.List(ctx, models.MatchFilter{
		TournamentID: &tournament.ID,
		Status:       models.MatchPending,
		Limit:        10,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// фильтр по игре
	matches, err = s.repo.List(ctx, models.MatchFilter{
		TournamentID: &tournament.ID,
		GameType:     "tug_of_war",
		Limit:        10,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 1)

	// фильтр по проге
	matches, err = s.repo.List(ctx, models.MatchFilter{
		ProgramID: &prog1.ID,
		Limit:     10,
	})
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(matches), 3)
}

func (s *MatchRepositorySuite) TestGetPending() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("getpd")

	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityLow, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityHigh, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)

	ctx := context.Background()
	matches, err := s.repo.GetPending(ctx, 10)
	require.NoError(s.T(), err)

	// минимум 2 pending (могут быть ещё от других тестов)
	assert.GreaterOrEqual(s.T(), len(matches), 2)

	for _, m := range matches {
		assert.Equal(s.T(), models.MatchPending, m.Status)
	}
}

func (s *MatchRepositorySuite) TestGetByID_Success() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("gbids")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityHigh, 3)

	ctx := context.Background()
	result, err := s.repo.GetByID(ctx, match.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), match.ID, result.ID)
	assert.Equal(s.T(), tournament.ID, result.TournamentID)
	assert.Equal(s.T(), prog1.ID, result.Program1ID)
	assert.Equal(s.T(), prog2.ID, result.Program2ID)
	assert.Equal(s.T(), "prisoners_dilemma", result.GameType)
	assert.Equal(s.T(), models.MatchPending, result.Status)
	assert.Equal(s.T(), models.PriorityHigh, result.Priority)
	assert.Equal(s.T(), 3, result.RoundNumber)
	// опциональные поля у свежего матча должны быть nil
	assert.Nil(s.T(), result.Score1)
	assert.Nil(s.T(), result.Score2)
	assert.Nil(s.T(), result.Winner)
	assert.Nil(s.T(), result.ErrorCode)
	assert.Nil(s.T(), result.ErrorMessage)
	assert.Nil(s.T(), result.StartedAt)
	assert.Nil(s.T(), result.CompletedAt)
	assert.NotZero(s.T(), result.CreatedAt)
}

// сбрасываются только running старше порога: свежий running и завершённый
// матч с давним started_at не трогаются
func (s *MatchRepositorySuite) TestResetStuckRunning() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("rstst")
	ctx := context.Background()

	setState := func(m *models.Match, status models.MatchStatus, startedAgo string) {
		_, err := s.database.ExecContext(ctx,
			"UPDATE matches SET status = $2, started_at = NOW() - $3::interval WHERE id = $1",
			m.ID, status, startedAgo)
		require.NoError(s.T(), err)
	}

	stuck := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	setState(stuck, models.MatchRunning, "2 hours")
	recent := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	setState(recent, models.MatchRunning, "1 second")
	done := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	setState(done, models.MatchCompleted, "2 hours")

	n, err := s.repo.ResetStuckRunning(ctx, time.Hour, 100)
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), n, int64(1))

	got, err := s.repo.GetByID(ctx, stuck.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchPending, got.Status)
	assert.Nil(s.T(), got.StartedAt)

	got, err = s.repo.GetByID(ctx, recent.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchRunning, got.Status)

	got, err = s.repo.GetByID(ctx, done.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchCompleted, got.Status)
}

// отменяются только pending, running и завершённые не трогаются
func (s *MatchRepositorySuite) TestCancelPending() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("cnclp")
	ctx := context.Background()

	pending := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	running := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchRunning, models.PriorityMedium, 1)
	completed := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)

	// отмена глобальная, в базе могут быть pending от других тестов
	n, err := s.repo.CancelPending(ctx)
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), n, int64(1))

	for id, want := range map[uuid.UUID]models.MatchStatus{
		pending.ID:   models.MatchCancelled,
		running.ID:   models.MatchRunning,
		completed.ID: models.MatchCompleted,
	} {
		got, err := s.repo.GetByID(ctx, id)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), want, got.Status)
	}

	// отменённый матч воркер уже не возьмёт
	assert.ErrorIs(s.T(), s.repo.UpdateStatus(ctx, pending.ID, models.MatchRunning), models.ErrMatchAlreadyProcessed)
}

// завершение турнира: pending и running отменяются, сыгранные не трогаются
func (s *MatchRepositorySuite) TestCancelActiveByTournament() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("cancel")
	pending := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchPending, models.PriorityMedium, 1)
	running := s.createMatch(tournament.ID, prog2.ID, prog1.ID, "prisoners_dilemma", models.MatchRunning, models.PriorityMedium, 1)
	done := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", models.MatchCompleted, models.PriorityMedium, 1)

	ctx := context.Background()
	cancelled, err := s.repo.CancelActiveByTournament(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), cancelled)

	for id, want := range map[uuid.UUID]models.MatchStatus{
		pending.ID: models.MatchCancelled,
		running.ID: models.MatchCancelled,
		done.ID:    models.MatchCompleted,
	} {
		got, err := s.repo.GetByID(ctx, id)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), want, got.Status)
	}
}
