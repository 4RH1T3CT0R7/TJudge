//go:build integration

package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProgramRepositorySuite struct {
	suite.Suite
	database       *storage.DB
	repo           *storage.ProgramRepository
	userRepo       *storage.UserRepository
	tournamentRepo *storage.TournamentRepository
	teamRepo       *storage.TeamRepository
	gameRepo       *storage.GameRepository
	// айдишники для очистки
	programIDs    []uuid.UUID
	teamIDs       []uuid.UUID
	tournamentIDs []uuid.UUID
	gameIDs       []uuid.UUID
	userIDs       []uuid.UUID
}

func TestProgramRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &ProgramRepositorySuite{
		database:       database,
		repo:           storage.NewProgramRepository(database),
		userRepo:       storage.NewUserRepository(database),
		tournamentRepo: storage.NewTournamentRepository(database),
		teamRepo:       storage.NewTeamRepository(database),
		gameRepo:       storage.NewGameRepository(database),
	}
	suite.Run(t, s)
}

func (s *ProgramRepositorySuite) TearDownTest() {
	ctx := context.Background()
	// в обратном порядке FK: programs -> teams -> tournaments -> games -> users
	for _, id := range s.programIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM programs WHERE id = $1", id)
	}
	for _, id := range s.teamIDs {
		_, _ = s.database.ExecContext(ctx, "DELETE FROM teams WHERE id = $1", id)
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
	s.programIDs = nil
	s.teamIDs = nil
	s.tournamentIDs = nil
	s.gameIDs = nil
	s.userIDs = nil
}

func (s *ProgramRepositorySuite) createUser(suffix string) *domain.User {
	user := createTestUser(s.T(), s.userRepo, suffix)
	s.userIDs = append(s.userIDs, user.ID)
	return user
}

func (s *ProgramRepositorySuite) createTournament(code string, creatorID uuid.UUID) *domain.Tournament {
	tournament := createTestTournament(s.T(), s.tournamentRepo, code, creatorID)
	s.tournamentIDs = append(s.tournamentIDs, tournament.ID)
	return tournament
}

func (s *ProgramRepositorySuite) createGame(name string) *domain.Game {
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

func (s *ProgramRepositorySuite) createTeam(tournamentID, leaderID uuid.UUID, code string) *domain.Team {
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

func (s *ProgramRepositorySuite) createProgram(userID uuid.UUID, teamID, tournamentID, gameID *uuid.UUID, name string, version int) *domain.Program {
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
	err := s.repo.Create(ctx, program)
	require.NoError(s.T(), err)
	s.programIDs = append(s.programIDs, program.ID)
	return program
}

func (s *ProgramRepositorySuite) TestCreate() {
	user := s.createUser("prog_create")

	ctx := context.Background()
	program := &domain.Program{
		ID:       uuid.New(),
		UserID:   user.ID,
		Name:     "Test Bot",
		GameType: "prisoners_dilemma",
		CodePath: "/tmp/test/bot.py",
		Language: "python",
		Version:  1,
	}

	err := s.repo.Create(ctx, program)
	require.NoError(s.T(), err)
	s.programIDs = append(s.programIDs, program.ID)

	assert.NotZero(s.T(), program.CreatedAt)
	assert.NotZero(s.T(), program.UpdatedAt)
}

func (s *ProgramRepositorySuite) TestCreate_WithTeamAndTournament() {
	user := s.createUser("prog_create_full")
	tournament := s.createTournament("TESTPC1", user.ID)
	game := s.createGame("prog_game_create")
	team := s.createTeam(tournament.ID, user.ID, "PCTM01")

	program := s.createProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "Full Bot", 1)

	assert.Equal(s.T(), &team.ID, program.TeamID)
	assert.Equal(s.T(), &tournament.ID, program.TournamentID)
	assert.Equal(s.T(), &game.ID, program.GameID)
}

func (s *ProgramRepositorySuite) TestGetByID() {
	user := s.createUser("prog_getbyid")
	program := s.createProgram(user.ID, nil, nil, nil, "GetByID Bot", 1)

	ctx := context.Background()
	result, err := s.repo.GetByID(ctx, program.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), program.ID, result.ID)
	assert.Equal(s.T(), program.UserID, result.UserID)
	assert.Equal(s.T(), program.Name, result.Name)
	assert.Equal(s.T(), program.GameType, result.GameType)
	assert.Equal(s.T(), program.CodePath, result.CodePath)
	assert.Equal(s.T(), program.Language, result.Language)
	assert.Equal(s.T(), program.Version, result.Version)
}

func (s *ProgramRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *ProgramRepositorySuite) TestGetByUserID() {
	user := s.createUser("prog_getbyuser")
	s.createProgram(user.ID, nil, nil, nil, "User Bot 1", 1)
	s.createProgram(user.ID, nil, nil, nil, "User Bot 2", 1)

	ctx := context.Background()
	programs, err := s.repo.GetByUserID(ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), programs, 2)

	// сортировка created_at DESC
	assert.True(s.T(), !programs[0].CreatedAt.Before(programs[1].CreatedAt),
		"programs should be ordered by created_at DESC")
}

func (s *ProgramRepositorySuite) TestGetByUserID_Empty() {
	ctx := context.Background()

	programs, err := s.repo.GetByUserID(ctx, uuid.New())
	require.NoError(s.T(), err)
	assert.Empty(s.T(), programs)
}

func (s *ProgramRepositorySuite) TestGetByTournamentAndGame() {
	user := s.createUser("prog_tourngame")
	tournament := s.createTournament("TESTPG1", user.ID)
	game := s.createGame("prog_game_tg")
	team1 := s.createTeam(tournament.ID, user.ID, "PGTM01")

	user2 := s.createUser("prog_tourngame2")
	team2 := s.createTeam(tournament.ID, user2.ID, "PGTM02")

	// две версии для team1 - вернуться должна только последняя
	s.createProgram(user.ID, &team1.ID, &tournament.ID, &game.ID, "Team1 Bot v1", 1)
	p1v2 := s.createProgram(user.ID, &team1.ID, &tournament.ID, &game.ID, "Team1 Bot v2", 2)

	// одна версия для team2
	p2 := s.createProgram(user2.ID, &team2.ID, &tournament.ID, &game.ID, "Team2 Bot v1", 1)

	ctx := context.Background()
	programs, err := s.repo.GetByTournamentAndGame(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), programs, 2)

	// в выборке только последние версии
	foundIDs := map[uuid.UUID]bool{}
	for _, p := range programs {
		foundIDs[p.ID] = true
	}
	assert.True(s.T(), foundIDs[p1v2.ID], "should contain latest version of team1 program")
	assert.True(s.T(), foundIDs[p2.ID], "should contain team2 program")
}

func (s *ProgramRepositorySuite) TestGetAllVersionsByTeamAndGame() {
	user := s.createUser("prog_allversions")
	tournament := s.createTournament("TESTAV1", user.ID)
	game := s.createGame("prog_game_av")
	team := s.createTeam(tournament.ID, user.ID, "PAVTM1")

	s.createProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "Bot v1", 1)
	s.createProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "Bot v2", 2)
	s.createProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "Bot v3", 3)

	ctx := context.Background()
	programs, err := s.repo.GetAllVersionsByTeamAndGame(ctx, team.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), programs, 3)

	// сортировка version DESC
	assert.Equal(s.T(), 3, programs[0].Version)
	assert.Equal(s.T(), 2, programs[1].Version)
	assert.Equal(s.T(), 1, programs[2].Version)
}

func (s *ProgramRepositorySuite) TestUpdate() {
	user := s.createUser("prog_update")
	program := s.createProgram(user.ID, nil, nil, nil, "Original Bot", 1)

	ctx := context.Background()
	program.Name = "Updated Bot"
	program.CodePath = "/tmp/updated/bot.py"
	program.Language = "go"
	errMsg := "compilation error"
	program.ErrorMessage = &errMsg

	err := s.repo.Update(ctx, program)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, program.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Bot", result.Name)
	assert.Equal(s.T(), "/tmp/updated/bot.py", result.CodePath)
	assert.Equal(s.T(), "go", result.Language)
	assert.NotNil(s.T(), result.ErrorMessage)
	assert.Equal(s.T(), "compilation error", *result.ErrorMessage)
}

func (s *ProgramRepositorySuite) TestUpdate_NotFound() {
	ctx := context.Background()
	program := &domain.Program{
		ID:       uuid.New(),
		Name:     "Ghost",
		CodePath: "/ghost",
		Language: "python",
	}

	err := s.repo.Update(ctx, program)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *ProgramRepositorySuite) TestDelete() {
	user := s.createUser("prog_delete")
	program := s.createProgram(user.ID, nil, nil, nil, "Delete Bot", 1)

	ctx := context.Background()
	err := s.repo.Delete(ctx, program.ID)
	require.NoError(s.T(), err)

	// убираем из трекинга, он уже удалён
	for i, id := range s.programIDs {
		if id == program.ID {
			s.programIDs = append(s.programIDs[:i], s.programIDs[i+1:]...)
			break
		}
	}

	_, err = s.repo.GetByID(ctx, program.ID)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *ProgramRepositorySuite) TestDelete_NotFound() {
	ctx := context.Background()

	err := s.repo.Delete(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *ProgramRepositorySuite) TestCheckOwnership() {
	user := s.createUser("prog_ownership")
	program := s.createProgram(user.ID, nil, nil, nil, "Owned Bot", 1)

	ctx := context.Background()

	// свой владелец
	owned, err := s.repo.CheckOwnership(ctx, program.ID, user.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), owned)

	// чужой
	owned, err = s.repo.CheckOwnership(ctx, program.ID, uuid.New())
	require.NoError(s.T(), err)
	assert.False(s.T(), owned)
}

func (s *ProgramRepositorySuite) TestClearErrorMessages() {
	user := s.createUser("prog_clearerr")
	tournament := s.createTournament("TESTCE1", user.ID)

	// проги с записанными ошибками
	ctx := context.Background()
	errMsg := "some error"
	for i := 0; i < 3; i++ {
		p := &domain.Program{
			ID:           uuid.New(),
			UserID:       user.ID,
			TournamentID: &tournament.ID,
			Name:         fmt.Sprintf("Error Bot %d", i),
			GameType:     "prisoners_dilemma",
			CodePath:     fmt.Sprintf("/tmp/test/err%d.py", i),
			Language:     "python",
			ErrorMessage: &errMsg,
			Version:      1,
		}
		err := s.repo.Create(ctx, p)
		require.NoError(s.T(), err)
		s.programIDs = append(s.programIDs, p.ID)
	}

	affected, err := s.repo.ClearErrorMessages(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), affected)

	// второй вызов - уже 0, ошибки затёрты
	affected, err = s.repo.ClearErrorMessages(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), affected)
}

func (s *ProgramRepositorySuite) TestGetLatestVersion() {
	user := s.createUser("prog_latestver")
	tournament := s.createTournament("TESTLV1", user.ID)
	game := s.createGame("prog_game_lv")
	team := s.createTeam(tournament.ID, user.ID, "PLVTM1")

	ctx := context.Background()

	// версий ещё нет
	ver, err := s.repo.GetLatestVersion(ctx, team.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, ver)

	s.createProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "Bot v1", 1)
	s.createProgram(user.ID, &team.ID, &tournament.ID, &game.ID, "Bot v2", 2)

	ver, err = s.repo.GetLatestVersion(ctx, team.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, ver)
}

func (s *ProgramRepositorySuite) TestGetByUserIDAndGameType() {
	user := s.createUser("prog_usergtype")

	s.createProgram(user.ID, nil, nil, nil, "PD Bot 1", 1)
	s.createProgram(user.ID, nil, nil, nil, "PD Bot 2", 1)

	// прога с другой игрой
	ctx := context.Background()
	otherProg := &domain.Program{
		ID:       uuid.New(),
		UserID:   user.ID,
		Name:     "Other Bot",
		GameType: "tug_of_war",
		CodePath: "/tmp/test/other.py",
		Language: "python",
		Version:  1,
	}
	err := s.repo.Create(ctx, otherProg)
	require.NoError(s.T(), err)
	s.programIDs = append(s.programIDs, otherProg.ID)

	programs, err := s.repo.GetByUserIDAndGameType(ctx, user.ID, "prisoners_dilemma")
	require.NoError(s.T(), err)
	assert.Len(s.T(), programs, 2)

	for _, p := range programs {
		assert.Equal(s.T(), "prisoners_dilemma", p.GameType)
	}

	programs, err = s.repo.GetByUserIDAndGameType(ctx, user.ID, "tug_of_war")
	require.NoError(s.T(), err)
	assert.Len(s.T(), programs, 1)
	assert.Equal(s.T(), otherProg.ID, programs[0].ID)
}

func (s *ProgramRepositorySuite) TestGetByTournamentAndGame_Empty() {
	ctx := context.Background()

	programs, err := s.repo.GetByTournamentAndGame(ctx, uuid.New(), uuid.New())
	require.NoError(s.T(), err)
	assert.Empty(s.T(), programs)
}

// TZ-чувствительный: зелёный только когда Go-процесс и PG в одной таймзоне (в харнессе TZ=UTC)
func (s *ProgramRepositorySuite) TestCreate_Timestamps() {
	user := s.createUser("prog_timestamps")

	before := time.Now().Add(-time.Second)
	program := s.createProgram(user.ID, nil, nil, nil, "Timestamp Bot", 1)
	after := time.Now().Add(time.Second)

	assert.True(s.T(), program.CreatedAt.After(before), "created_at should be after test start")
	assert.True(s.T(), program.CreatedAt.Before(after), "created_at should be before test end")
	assert.True(s.T(), program.UpdatedAt.After(before), "updated_at should be after test start")
	assert.True(s.T(), program.UpdatedAt.Before(after), "updated_at should be before test end")
}
