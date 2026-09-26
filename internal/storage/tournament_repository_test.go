//go:build integration

package storage_test

import (
	"context"
	"fmt"
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

type TournamentRepositorySuite struct {
	suite.Suite
	database    *storage.DB
	repo        *storage.TournamentRepository
	userRepo    *storage.UserRepository
	programRepo *storage.ProgramRepository
	teamRepo    *storage.TeamRepository
	gameRepo    *storage.GameRepository
	matchRepo   *storage.MatchRepository
	// айдишники для очистки
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
		repo:        storage.NewTournamentRepository(database),
		userRepo:    storage.NewUserRepository(database),
		programRepo: storage.NewProgramRepository(database),
		teamRepo:    storage.NewTeamRepository(database),
		gameRepo:    storage.NewGameRepository(database),
		matchRepo:   storage.NewMatchRepository(database),
	}
	suite.Run(t, s)
}

func (s *TournamentRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// порядок FK: matches -> rating_history -> tournament_participants -> programs -> teams -> tournaments -> games -> users
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
	// заодно чистка по паттерну кода - для старых тестов
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

func (s *TournamentRepositorySuite) createTrackedUser(suffix string) *models.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

func (s *TournamentRepositorySuite) createTrackedTournament(code string, creatorID uuid.UUID) *models.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo(), code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

// алиас чтобы не путаться - это тот же s.repo
func (s *TournamentRepositorySuite) tournamentRepo() *storage.TournamentRepository {
	return s.repo
}

func (s *TournamentRepositorySuite) createTrackedGame(name string) *models.Game {
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

func (s *TournamentRepositorySuite) createTrackedTeam(tournamentID, leaderID uuid.UUID, code string) *models.Team {
	ctx := context.Background()
	team := &models.Team{
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

func (s *TournamentRepositorySuite) createTrackedProgram(userID uuid.UUID, teamID, tournamentID, gameID *uuid.UUID, name string, version int) *models.Program {
	ctx := context.Background()
	program := &models.Program{
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

func (s *TournamentRepositorySuite) createTestParticipant(tournamentID, programID uuid.UUID, rating int) *models.TournamentParticipant {
	ctx := context.Background()
	participant := &models.TournamentParticipant{
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

func (s *TournamentRepositorySuite) createTrackedMatch(tournamentID, program1ID, program2ID uuid.UUID, gameType string, status models.MatchStatus) *models.Match {
	ctx := context.Background()
	match := &models.Match{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     gameType,
		Status:       status,
		Priority:     models.PriorityMedium,
		RoundNumber:  1,
		CreatedAt:    time.Now(),
	}
	err := s.matchRepo.Create(ctx, match)
	require.NoError(s.T(), err)
	s.matchIDs = append(s.matchIDs, match.ID)
	return match
}

// createTeamEntrant - команда с программой-участником: лидерборд берёт
// только программы команд (INNER JOIN teams)
func (s *TournamentRepositorySuite) createTeamEntrant(tournamentID, gameID uuid.UUID, code string) *models.Program {
	user := s.createTrackedUser("tp_" + code)
	team := s.createTrackedTeam(tournamentID, user.ID, code)
	prog := s.createTrackedProgram(user.ID, &team.ID, &tournamentID, &gameID, "Bot"+code, 1)
	s.createTestParticipant(tournamentID, prog.ID, 1500)
	return prog
}

// completeMatch - завершённый матч с заданным счётом
func (s *TournamentRepositorySuite) completeMatch(tournamentID, p1, p2 uuid.UUID, gameType string, score1, score2, winner int) {
	match := s.createTrackedMatch(tournamentID, p1, p2, gameType, models.MatchRunning)
	_, err := s.database.ExecContext(context.Background(),
		"UPDATE matches SET status = 'completed', score1 = $2, score2 = $3, winner = $4, completed_at = NOW() WHERE id = $1",
		match.ID, score1, score2, winner)
	require.NoError(s.T(), err)
}

func (s *TournamentRepositorySuite) TestCreate() {
	ctx := context.Background()
	creator := createTestUser(s.T(), s.userRepo, "creator_"+uuid.New().String()[:8])

	tournament := &models.Tournament{
		ID:              uuid.New(),
		Code:            "TEST001",
		Name:            "Test Tournament",
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          models.TournamentPending,
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
	// version стартует с 0, не с 1
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

	filter := models.TournamentFilter{Limit: 10}
	tournaments, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	assert.GreaterOrEqual(s.T(), len(tournaments), 3)
}

func (s *TournamentRepositorySuite) TestList_FilterByStatus() {
	ctx := context.Background()

	t1, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST006")
	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST007")

	err := s.repo.UpdateStatus(ctx, t1.ID, models.TournamentActive)
	require.NoError(s.T(), err)

	filter := models.TournamentFilter{
		Status: models.TournamentActive,
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

	err := s.repo.UpdateStatus(ctx, tournament.ID, models.TournamentActive)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TournamentActive, result.Status)
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

// завершение: статус и отмена pending/running одной транзакцией, сыгранные не трогаются.
// устаревшая версия турнира не меняет ни турнир, ни матчи
func (s *TournamentRepositorySuite) TestComplete() {
	ctx := context.Background()
	user := s.createTrackedUser("tp_cmpl")
	created := s.createTrackedTournament("TPCMPL", user.ID)
	p1 := s.createTrackedProgram(user.ID, nil, nil, nil, "BotCmpl1", 1)
	p2 := s.createTrackedProgram(user.ID, nil, nil, nil, "BotCmpl2", 1)
	pending := s.createTrackedMatch(created.ID, p1.ID, p2.ID, "prisoners_dilemma", models.MatchPending)
	running := s.createTrackedMatch(created.ID, p2.ID, p1.ID, "prisoners_dilemma", models.MatchRunning)
	done := s.createTrackedMatch(created.ID, p1.ID, p2.ID, "prisoners_dilemma", models.MatchCompleted)

	tournament, err := s.repo.GetByID(ctx, created.ID)
	require.NoError(s.T(), err)
	now := time.Now()
	tournament.EndTime = &now

	stale := *tournament
	stale.Version++
	_, err = s.repo.Complete(ctx, &stale)
	assert.ErrorIs(s.T(), err, errors.ErrConcurrentUpdate)
	got, err := s.matchRepo.GetByID(ctx, pending.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MatchPending, got.Status)

	cancelled, err := s.repo.Complete(ctx, tournament)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), cancelled)

	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TournamentCompleted, result.Status)
	assert.NotNil(s.T(), result.EndTime)

	for id, want := range map[uuid.UUID]models.MatchStatus{
		pending.ID: models.MatchCancelled,
		running.ID: models.MatchCancelled,
		done.ID:    models.MatchCompleted,
	} {
		got, err := s.matchRepo.GetByID(ctx, id)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), want, got.Status)
	}
}

func (s *TournamentRepositorySuite) TestDelete() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST010")

	err := s.repo.Delete(ctx, tournament.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetByID(ctx, tournament.ID)
	assert.Error(s.T(), err)
}

// --- участники ---

func (s *TournamentRepositorySuite) TestAddParticipant_Success() {
	user := s.createTrackedUser("tp_add")
	tournament := s.createTrackedTournament("TPADD1", user.ID)
	program := s.createTrackedProgram(user.ID, nil, nil, nil, "BotAdd", 1)

	ctx := context.Background()
	participant := &models.TournamentParticipant{
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
	ctx := context.Background()
	user := s.createTrackedUser("tp_lbo")
	tournament := s.createTrackedTournament("TPLBO1", user.ID)
	game := s.createTrackedGame("lbo_game")
	require.NoError(s.T(), s.gameRepo.AddToTournament(ctx, tournament.ID, game.ID))

	a := s.createTeamEntrant(tournament.ID, game.ID, "TLBOA1")
	b := s.createTeamEntrant(tournament.ID, game.ID, "TLBOB1")
	c := s.createTeamEntrant(tournament.ID, game.ID, "TLBOC1")

	// рейтинг - сумма очков по обеим сторонам матчей: a=10+9, b=5+8, c=2+1
	s.completeMatch(tournament.ID, a.ID, b.ID, game.Name, 10, 5, 1)
	s.completeMatch(tournament.ID, b.ID, c.ID, game.Name, 8, 2, 1)
	s.completeMatch(tournament.ID, c.ID, a.ID, game.Name, 1, 9, 2)

	leaderboard, err := s.repo.GetLeaderboard(ctx, tournament.ID, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), leaderboard, 3)

	want := []struct {
		id     uuid.UUID
		rating int
		wins   int
	}{{a.ID, 19, 2}, {b.ID, 13, 1}, {c.ID, 3, 0}}
	for i, w := range want {
		assert.Equal(s.T(), i+1, leaderboard[i].Rank)
		assert.Equal(s.T(), w.id, leaderboard[i].ProgramID)
		assert.Equal(s.T(), w.rating, leaderboard[i].Rating)
		assert.Equal(s.T(), w.wins, leaderboard[i].Wins)
		assert.Equal(s.T(), 2, leaderboard[i].TotalGames)
	}
}

// лидерборд: строка на команду и игру (только последняя версия), статистика
// считается одинаково с лидербордом игры, форфейт - это сыгранный матч
func (s *TournamentRepositorySuite) TestGetLeaderboard_LatestVersionsAndForfeits() {
	ctx := context.Background()
	user1 := s.createTrackedUser("tp_lbf1")
	user2 := s.createTrackedUser("tp_lbf2")
	tournament := s.createTrackedTournament("TPLBF1", user1.ID)
	game := s.createTrackedGame("lbf_game")
	require.NoError(s.T(), s.gameRepo.AddToTournament(ctx, tournament.ID, game.ID))
	team1 := s.createTrackedTeam(tournament.ID, user1.ID, "TLBF01")
	team2 := s.createTrackedTeam(tournament.ID, user2.ID, "TLBF02")

	old1 := s.createTrackedProgram(user1.ID, &team1.ID, &tournament.ID, &game.ID, "BotLBF1v1", 1)
	new1 := s.createTrackedProgram(user1.ID, &team1.ID, &tournament.ID, &game.ID, "BotLBF1v2", 2)
	p2 := s.createTrackedProgram(user2.ID, &team2.ID, &tournament.ID, &game.ID, "BotLBF2", 1)
	// не собравшаяся загрузка не играет, в таблице остаётся new1
	broken1 := s.createTrackedProgram(user1.ID, &team1.ID, &tournament.ID, &game.ID, "BotLBF1v3", 3)
	_, err := s.database.ExecContext(ctx, "UPDATE programs SET status = 'failed' WHERE id = $1", broken1.ID)
	require.NoError(s.T(), err)
	for _, p := range []*models.Program{old1, new1, p2} {
		s.createTestParticipant(tournament.ID, p.ID, 1500)
	}

	finish := func(p1, p2 uuid.UUID, status string, winner int) {
		m := s.createTrackedMatch(tournament.ID, p1, p2, game.Name, models.MatchRunning)
		_, err := s.database.ExecContext(ctx,
			"UPDATE matches SET status = $2, winner = $3, score1 = 3, score2 = 1 WHERE id = $1", m.ID, status, winner)
		require.NoError(s.T(), err)
	}
	finish(new1.ID, p2.ID, "completed", 1) // победа team1
	finish(p2.ID, new1.ID, "failed", 1)    // форфейт: упала программа team1
	finish(p2.ID, new1.ID, "failed", 0)    // сбой без победителя - не считается

	leaderboard, err := s.repo.GetLeaderboard(ctx, tournament.ID, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), leaderboard, 2, "старая версия team1 не должна попадать в таблицу")

	byGame, err := s.repo.GetLeaderboardByGameType(ctx, tournament.ID, game.Name, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), byGame, 2)

	for _, entries := range [][]*models.LeaderboardEntry{leaderboard, byGame} {
		for _, e := range entries {
			assert.Contains(s.T(), []uuid.UUID{new1.ID, p2.ID}, e.ProgramID)
			assert.Equal(s.T(), 2, e.TotalGames)
			assert.Equal(s.T(), e.TotalGames, e.Wins+e.Losses+e.Draws)
			assert.Equal(s.T(), 1, e.Wins)
			assert.Equal(s.T(), 1, e.Losses)
		}
	}

	cross, err := s.repo.GetCrossGameLeaderboard(ctx, tournament.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), cross, 2)
	for _, e := range cross {
		assert.Contains(s.T(), []uuid.UUID{new1.ID, p2.ID}, e.ProgramID)
	}
}

func (s *TournamentRepositorySuite) TestGetLeaderboard_LimitEnforced() {
	ctx := context.Background()
	user := s.createTrackedUser("tp_lbl")
	tournament := s.createTrackedTournament("TPLBL1", user.ID)
	game := s.createTrackedGame("lbl_game")
	require.NoError(s.T(), s.gameRepo.AddToTournament(ctx, tournament.ID, game.ID))

	for i := 0; i < 5; i++ {
		s.createTeamEntrant(tournament.ID, game.ID, fmt.Sprintf("TLBL0%d", i))
	}

	leaderboard, err := s.repo.GetLeaderboard(ctx, tournament.ID, 2)
	require.NoError(s.T(), err)
	assert.Len(s.T(), leaderboard, 2)
}

func (s *TournamentRepositorySuite) TestGetCrossGameLeaderboard() {
	user := s.createTrackedUser("tp_cgl")
	tournament := s.createTrackedTournament("TPCGL1", user.ID)

	// кросс-игровому лидерборду нужны проги с team_id/game_id и завершённые
	// матчи. без матчей выборка пустая - проверка что не падает.
	ctx := context.Background()
	entries, err := s.repo.GetCrossGameLeaderboard(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), entries)

	// теперь полный сценарий: команда + игра + прога + матч
	game := s.createTrackedGame("cgl_game1")
	require.NoError(s.T(), s.gameRepo.AddToTournament(ctx, tournament.ID, game.ID))
	team := s.createTrackedTeam(tournament.ID, user.ID, "TCGL01")
	prog1 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "BotCGL1", 1)

	user2 := s.createTrackedUser("tp_cgl2")
	team2 := s.createTrackedTeam(tournament.ID, user2.ID, "TCGL02")
	prog2 := s.createTrackedProgram(user2.ID, &team2.ID, &tournament.ID, &game.ID, "BotCGL2", 1)

	s.createTestParticipant(tournament.ID, prog1.ID, 1500)
	s.createTestParticipant(tournament.ID, prog2.ID, 1500)

	// завершённый матч чтобы было что агрегировать
	match := s.createTrackedMatch(tournament.ID, prog1.ID, prog2.ID, game.Name, models.MatchRunning)
	score1 := 10
	score2 := 5
	winner := 1
	_, _ = s.database.ExecContext(ctx,
		"UPDATE matches SET status = 'completed', score1 = $2, score2 = $3, winner = $4, completed_at = NOW() WHERE id = $1",
		match.ID, score1, score2, winner)

	entries, err = s.repo.GetCrossGameLeaderboard(ctx, tournament.ID)
	require.NoError(s.T(), err)
	// две записи - по одной на команду
	assert.Len(s.T(), entries, 2)

	// команда с большим счётом должна быть выше
	if len(entries) >= 2 {
		assert.GreaterOrEqual(s.T(), entries[0].TotalRating, entries[1].TotalRating)
	}
}

func (s *TournamentRepositorySuite) TestGetLatestParticipantsByGame() {
	user := s.createTrackedUser("tp_lpg")
	tournament := s.createTrackedTournament("TPLPG1", user.ID)

	game1 := s.createTrackedGame("lpg_game1")
	game2 := s.createTrackedGame("lpg_game2")

	team := s.createTrackedTeam(tournament.ID, user.ID, "TLPG01")

	ctx := context.Background()
	require.NoError(s.T(), s.gameRepo.AddToTournament(ctx, tournament.ID, game1.ID))
	require.NoError(s.T(), s.gameRepo.AddToTournament(ctx, tournament.ID, game2.ID))

	prog1 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game1.ID, "BotLPG1", 1)
	prog2 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game2.ID, "BotLPG2", 1)

	s.createTestParticipant(tournament.ID, prog1.ID, 1500)
	s.createTestParticipant(tournament.ID, prog2.ID, 1600)

	// фильтр по game1 - только участник с prog1
	participants1, err := s.repo.GetLatestParticipantsByGame(ctx, tournament.ID, game1.Name)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants1, 1)
	assert.Equal(s.T(), prog1.ID, participants1[0].ProgramID)

	// фильтр по game2 - только участник с prog2
	participants2, err := s.repo.GetLatestParticipantsByGame(ctx, tournament.ID, game2.Name)
	require.NoError(s.T(), err)
	assert.Len(s.T(), participants2, 1)
	assert.Equal(s.T(), prog2.ID, participants2[0].ProgramID)
}

// v1 ready, v2 failed: в раунд идёт v1; программа игры, не привязанной к турниру, не участвует
// авто-раунд: новой считается последняя готовая версия команды, ставшая ready после
// since. компиляция и неудачная сборка раунд не запускают, правка старой версии тоже
func (s *TournamentRepositorySuite) TestHasNewProgramsSince() {
	ctx := context.Background()
	user := s.createTrackedUser("tp_hnp")
	tournament := s.createTrackedTournament("TPHNP1", user.ID)
	game := s.createTrackedGame("hnp_game")
	team := s.createTrackedTeam(tournament.ID, user.ID, "THNP01")

	dbNow := func() time.Time {
		var now time.Time
		require.NoError(s.T(), s.database.GetContext(ctx, &now, "SELECT NOW()"))
		return now
	}
	hasNew := func(since time.Time) bool {
		ok, err := s.gameRepo.HasNewProgramsSince(ctx, tournament.ID, game.Name, since)
		require.NoError(s.T(), err)
		return ok
	}
	compiling := func(version int) *models.Program {
		p := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game.ID, fmt.Sprintf("BotHNP%d", version), version)
		_, err := s.database.ExecContext(ctx, "UPDATE programs SET status = 'compiling' WHERE id = $1", p.ID)
		require.NoError(s.T(), err)
		return p
	}

	v1 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "BotHNP1", 1)
	since := dbNow()

	v2 := compiling(2)
	assert.False(s.T(), hasNew(since), "compiling")
	applied, err := s.programRepo.UpdateCompileResult(ctx, v2.ID, models.ProgramFailed, v2.CodePath, nil)
	require.NoError(s.T(), err)
	require.True(s.T(), applied)
	assert.False(s.T(), hasNew(since), "failed")

	v3 := compiling(3)
	applied, err = s.programRepo.UpdateCompileResult(ctx, v3.ID, models.ProgramReady, v3.CodePath, nil)
	require.NoError(s.T(), err)
	require.True(s.T(), applied)
	assert.True(s.T(), hasNew(since), "ready")

	since = dbNow()
	v1.Name = "BotHNP1_renamed"
	require.NoError(s.T(), s.programRepo.Update(ctx, v1))
	assert.False(s.T(), hasNew(since), "правка старой версии")
}

func (s *TournamentRepositorySuite) TestGetLatestParticipantsByGame_UsesLatestReadyVersion() {
	ctx := context.Background()
	user := s.createTrackedUser("tp_lrv")
	tournament := s.createTrackedTournament("TPLRV1", user.ID)
	game := s.createTrackedGame("lrv_game")
	detached := s.createTrackedGame("lrv_detached")
	require.NoError(s.T(), s.gameRepo.AddToTournament(ctx, tournament.ID, game.ID))
	team := s.createTrackedTeam(tournament.ID, user.ID, "TLRV01")

	v1 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "BotLRV1", 1)
	v2 := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "BotLRV2", 2)
	_, err := s.database.ExecContext(ctx, "UPDATE programs SET status = 'failed' WHERE id = $1", v2.ID)
	require.NoError(s.T(), err)
	other := s.createTrackedProgram(user.ID, &team.ID, &tournament.ID, &detached.ID, "BotLRV3", 1)

	s.createTestParticipant(tournament.ID, v1.ID, 1500)
	s.createTestParticipant(tournament.ID, v2.ID, 1500)
	s.createTestParticipant(tournament.ID, other.ID, 1500)

	participants, err := s.repo.GetLatestParticipantsByGame(ctx, tournament.ID, game.Name)
	require.NoError(s.T(), err)
	require.Len(s.T(), participants, 1)
	assert.Equal(s.T(), v1.ID, participants[0].ProgramID)

	byGame, err := s.repo.GetLatestParticipantsGroupedByGame(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), byGame, 1)
	assert.NotContains(s.T(), byGame, detached.Name)
}
