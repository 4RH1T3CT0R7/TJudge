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
		repo:           db.NewMatchRepository(database),
		userRepo:       db.NewUserRepository(database),
		tournamentRepo: db.NewTournamentRepository(database),
		programRepo:    db.NewProgramRepository(database),
	}
	suite.Run(t, s)
}

func (s *MatchRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// порядок важен из-за FK: matches -> programs -> tournaments -> users
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

func (s *MatchRepositorySuite) createUser(suffix string) *domain.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

func (s *MatchRepositorySuite) createTournament(code string, creatorID uuid.UUID) *domain.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

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

// готовит юзера, турнир и две проги - типовой сетап почти для всех тестов матчей
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

	// перечитываем и сверяем
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

	// оба должны создаться
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

	// сортировка round_number DESC, created_at DESC
	assert.GreaterOrEqual(s.T(), matches[0].RoundNumber, matches[1].RoundNumber)
}

func (s *MatchRepositorySuite) TestGetByTournamentID_Pagination() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("getpag")

	for i := 0; i < 5; i++ {
		s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, i+1)
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
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityLow, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityHigh, 1)
	// completed попасть в выборку не должен
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)

	ctx := context.Background()
	matches, err := s.repo.GetPendingByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// high приоритет первым
	assert.Equal(s.T(), domain.PriorityHigh, matches[0].Priority)
	assert.Equal(s.T(), domain.PriorityLow, matches[1].Priority)

	for _, m := range matches {
		assert.Equal(s.T(), domain.MatchPending, m.Status)
	}
}

func (s *MatchRepositorySuite) TestGetPendingByTournamentAndGame() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("pndgm")

	// pending под две разные игры
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

	// вторая игра
	matches, err = s.repo.GetPendingByTournamentAndGame(ctx, tournament.ID, "tug_of_war")
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 1)
}

func (s *MatchRepositorySuite) TestUpdateStatus() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("updst")
	match := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()

	// перевод в running должен проставить started_at
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
	// сразу в completed - started_at не трогаем
	assert.Nil(s.T(), result.StartedAt)
}

// перевод в running атомарно уводит матч из pending. если матча нет или он
// уже не pending - это НЕ not found, а защита от двойной обработки
// (ErrMatchAlreadyProcessed). а вот обычный статус по несуществующему id - not found.
func (s *MatchRepositorySuite) TestUpdateStatus_NotFound() {
	ctx := context.Background()

	// running по несуществующему id: 0 строк -> already processed, не not found
	err := s.repo.UpdateStatus(ctx, uuid.New(), domain.MatchRunning)
	assert.Error(s.T(), err)
	assert.ErrorIs(s.T(), err, domain.ErrMatchAlreadyProcessed)

	// а не-running статус по несуществующему id - это уже not found
	err = s.repo.UpdateStatus(ctx, uuid.New(), domain.MatchCompleted)
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

	// два зафейленных
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchFailed, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchFailed, domain.PriorityMedium, 1)
	// pending трогать нельзя
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()
	affected, err := s.repo.ResetFailedMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), affected)

	// после сброса все pending
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

	// матчей ещё нет - ждём 1
	nextRound, err := s.repo.GetNextRoundNumber(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, nextRound)

	// раунд 1
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	nextRound, err = s.repo.GetNextRoundNumber(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, nextRound)

	// раунд 3 (второй пропустили) - следующий должен быть 4
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 3)

	nextRound, err = s.repo.GetNextRoundNumber(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 4, nextRound)
}

func (s *MatchRepositorySuite) TestGetNextRoundNumberByGame() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("nxtrng")

	ctx := context.Background()

	// для этой игры матчей нет
	nextRound, err := s.repo.GetNextRoundNumberByGame(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, nextRound)

	// матчи под разные игры
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

	// матчи по нескольким раундам и играм
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "tug_of_war", domain.MatchCompleted, domain.PriorityMedium, 1)
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 2)

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

	// стартовавших нет
	has, err := s.repo.HasStartedMatches(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// pending не считается стартовавшим
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	has, err = s.repo.HasStartedMatches(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// а completed - уже да
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	has, err = s.repo.HasStartedMatches(ctx, tournament.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.True(s.T(), has)
}

func (s *MatchRepositorySuite) TestHasAnyRunningMatches() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("hasrn")

	ctx := context.Background()

	// вообще матчей нет
	has, err := s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// completed не в счёт
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchCompleted, domain.PriorityMedium, 1)
	has, err = s.repo.HasAnyRunningMatches(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), has)

	// pending уже считается "есть незавершённые"
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
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

	// удалённые убираем из трекинга
	s.matchIDs = []uuid.UUID{m3.ID}
	_ = m1 // уже снесён через DeleteMatchesForGame

	// матч tug_of_war должен остаться
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

	// фильтр по статусу
	matches, err := s.repo.List(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
		Status:       domain.MatchPending,
		Limit:        10,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)

	// фильтр по игре
	matches, err = s.repo.List(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
		GameType:     "tug_of_war",
		Limit:        10,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 1)

	// фильтр по проге
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

	// минимум 2 pending (могут быть ещё от других тестов)
	assert.GreaterOrEqual(s.T(), len(matches), 2)

	for _, m := range matches {
		assert.Equal(s.T(), domain.MatchPending, m.Status)
	}
}

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

func (s *MatchRepositorySuite) TestGetStuckRunning() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("stuck")

	ctx := context.Background()

	// running с давним started_at - это и есть "зависший"
	stuckMatch := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	oldTime := time.Now().Add(-2 * time.Hour)
	_, err := s.database.ExecContext(ctx,
		"UPDATE matches SET status = $2, started_at = $3 WHERE id = $1",
		stuckMatch.ID, domain.MatchRunning, oldTime)
	require.NoError(s.T(), err)

	// running, но стартанул только что - зависшим не считается
	recentMatch := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	_, err = s.database.ExecContext(ctx,
		"UPDATE matches SET status = $2, started_at = NOW() WHERE id = $1",
		recentMatch.ID, domain.MatchRunning)
	require.NoError(s.T(), err)

	// pending возвращать не должны
	s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	// порог зависания - 1 час
	stuckMatches, err := s.repo.GetStuckRunning(ctx, 1*time.Hour, 10)
	require.NoError(s.T(), err)

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

	m1 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	m2 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	m3 := s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)

	ctx := context.Background()
	matchIDs := []uuid.UUID{m1.ID, m2.ID, m3.ID}

	err := s.repo.BatchUpdateStatus(ctx, matchIDs, domain.MatchCompleted)
	require.NoError(s.T(), err)

	// все три должны стать completed
	for _, id := range matchIDs {
		result, err := s.repo.GetByID(ctx, id)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), domain.MatchCompleted, result.Status)
	}
}

func (s *MatchRepositorySuite) TestBatchUpdateResults() {
	tournament, prog1, prog2 := s.setupMatchPrerequisites("batur")

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

	// m1 - completed со счётом
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

	// m2 - ничья
	r2, err := s.repo.GetByID(ctx, m2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.MatchCompleted, r2.Status)
	require.NotNil(s.T(), r2.Winner)
	assert.Equal(s.T(), 0, *r2.Winner)

	// m3 - failed с ошибкой
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

	// 5 матчей, created_at у всех чуть разный
	for i := 0; i < 5; i++ {
		s.createMatch(tournament.ID, prog1.ID, prog2.ID, "prisoners_dilemma", domain.MatchPending, domain.PriorityMedium, 1)
	}

	ctx := context.Background()

	// первая страница - первые 2
	first := 2
	pageReq := &pagination.PageRequest{First: &first}
	matches, hasMore, err := s.repo.ListWithCursor(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
	}, pageReq)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches, 2)
	assert.True(s.T(), hasMore, "should have more pages with 5 total items and limit 2")

	// created_at последнего результата - курсор для следующей страницы
	lastMatch := matches[len(matches)-1]
	cursor := pagination.NewTimestampCursor(lastMatch.CreatedAt)
	cursorStr, err := cursor.Encode()
	require.NoError(s.T(), err)

	// вторая страница
	pageReq2 := &pagination.PageRequest{First: &first, After: &cursorStr}
	matches2, hasMore2, err := s.repo.ListWithCursor(ctx, domain.MatchFilter{
		TournamentID: &tournament.ID,
	}, pageReq2)
	require.NoError(s.T(), err)
	assert.Len(s.T(), matches2, 2)
	assert.True(s.T(), hasMore2, "should have one more page")

	// страницы не должны пересекаться
	for _, m1 := range matches {
		for _, m2 := range matches2 {
			assert.NotEqual(s.T(), m1.ID, m2.ID, "pages should not overlap")
		}
	}
}
