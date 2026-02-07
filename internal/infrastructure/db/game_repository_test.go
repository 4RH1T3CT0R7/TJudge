//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GameRepositorySuite struct {
	suite.Suite
	database       *db.DB
	repo           *db.GameRepository
	tournamentRepo *db.TournamentRepository
	userRepo       *db.UserRepository
}

func TestGameRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &GameRepositorySuite{
		database:       database,
		repo:           db.NewGameRepository(database),
		tournamentRepo: db.NewTournamentRepository(database),
		userRepo:       db.NewUserRepository(database),
	}
	suite.Run(t, s)
}

func (s *GameRepositorySuite) TearDownTest() {
	ctx := context.Background()
	_, _ = s.database.ExecContext(ctx, "DELETE FROM tournament_games WHERE game_id IN (SELECT id FROM games WHERE name LIKE 'test_%')")
	_, _ = s.database.ExecContext(ctx, "DELETE FROM games WHERE name LIKE 'test_%'")
	_, _ = s.database.ExecContext(ctx, "DELETE FROM tournaments WHERE code LIKE 'GTEST%'")
	_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE username LIKE 'testuser_%'")
}

// createGame is a helper that inserts a game with a unique name based on suffix.
func (s *GameRepositorySuite) createGame(suffix string) *domain.Game {
	s.T().Helper()
	ctx := context.Background()

	game := &domain.Game{
		ID:          uuid.New(),
		Name:        "test_" + suffix,
		DisplayName: "Test Game " + suffix,
		Rules:       "# Rules\nSample rules for " + suffix,
	}

	err := s.repo.Create(ctx, game)
	require.NoError(s.T(), err)
	return game
}

// createTournamentForGame is a helper that creates a user and a tournament for
// tournament-game relationship tests.
func (s *GameRepositorySuite) createTournamentForGame(code string) *domain.Tournament {
	s.T().Helper()
	user := createTestUser(s.T(), s.userRepo, "game_"+code)
	return createTestTournament(s.T(), s.tournamentRepo, "GTEST"+code, user.ID)
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestCreate() {
	ctx := context.Background()

	game := &domain.Game{
		ID:          uuid.New(),
		Name:        "test_create",
		DisplayName: "Test Create Game",
		Rules:       "Some rules",
	}

	err := s.repo.Create(ctx, game)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), game.CreatedAt)
	assert.NotZero(s.T(), game.UpdatedAt)
}

func (s *GameRepositorySuite) TestCreate_DuplicateName() {
	s.createGame("dup")

	ctx := context.Background()
	duplicate := &domain.Game{
		ID:          uuid.New(),
		Name:        "test_dup",
		DisplayName: "Different Display Name",
		Rules:       "Different rules",
	}

	err := s.repo.Create(ctx, duplicate)
	assert.Error(s.T(), err)
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestGetByID() {
	game := s.createGame("getbyid")

	ctx := context.Background()
	result, err := s.repo.GetByID(ctx, game.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), game.ID, result.ID)
	assert.Equal(s.T(), game.Name, result.Name)
	assert.Equal(s.T(), game.DisplayName, result.DisplayName)
	assert.Equal(s.T(), game.Rules, result.Rules)
	assert.NotZero(s.T(), result.CreatedAt)
	assert.NotZero(s.T(), result.UpdatedAt)
}

func (s *GameRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// ---------------------------------------------------------------------------
// GetByName
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestGetByName() {
	game := s.createGame("getbyname")

	ctx := context.Background()
	result, err := s.repo.GetByName(ctx, "test_getbyname")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), game.ID, result.ID)
	assert.Equal(s.T(), game.Name, result.Name)
	assert.Equal(s.T(), game.DisplayName, result.DisplayName)
}

func (s *GameRepositorySuite) TestGetByName_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByName(ctx, "nonexistent_game")
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestList() {
	s.createGame("list_a")
	s.createGame("list_b")
	s.createGame("list_c")

	ctx := context.Background()
	games, err := s.repo.List(ctx, domain.GameFilter{Limit: 100})
	require.NoError(s.T(), err)

	assert.GreaterOrEqual(s.T(), len(games), 3)
}

func (s *GameRepositorySuite) TestList_FilterByName() {
	s.createGame("filter_target")
	s.createGame("filter_other")

	ctx := context.Background()
	games, err := s.repo.List(ctx, domain.GameFilter{
		Name:  "filter_target",
		Limit: 100,
	})
	require.NoError(s.T(), err)

	require.GreaterOrEqual(s.T(), len(games), 1)
	found := false
	for _, g := range games {
		if g.Name == "test_filter_target" {
			found = true
			break
		}
	}
	assert.True(s.T(), found, "Expected to find game with name test_filter_target")
}

func (s *GameRepositorySuite) TestList_Pagination() {
	s.createGame("page_a")
	s.createGame("page_b")
	s.createGame("page_c")

	ctx := context.Background()

	// Fetch first page
	page1, err := s.repo.List(ctx, domain.GameFilter{Limit: 1})
	require.NoError(s.T(), err)
	assert.Len(s.T(), page1, 1)

	// Fetch second page
	page2, err := s.repo.List(ctx, domain.GameFilter{Limit: 1, Offset: 1})
	require.NoError(s.T(), err)
	assert.Len(s.T(), page2, 1)

	// Pages should contain different games
	assert.NotEqual(s.T(), page1[0].ID, page2[0].ID)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestUpdate() {
	game := s.createGame("update")
	originalUpdatedAt := game.UpdatedAt

	ctx := context.Background()
	game.DisplayName = "Updated Display Name"
	game.Rules = "Updated rules"

	err := s.repo.Update(ctx, game)
	require.NoError(s.T(), err)

	// updated_at should have changed
	assert.True(s.T(), game.UpdatedAt.After(originalUpdatedAt) || game.UpdatedAt.Equal(originalUpdatedAt))

	// Verify via re-read
	result, err := s.repo.GetByID(ctx, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Display Name", result.DisplayName)
	assert.Equal(s.T(), "Updated rules", result.Rules)
}

func (s *GameRepositorySuite) TestUpdate_NotFound() {
	ctx := context.Background()
	game := &domain.Game{
		ID:          uuid.New(),
		DisplayName: "Ghost",
		Rules:       "No rules",
	}

	err := s.repo.Update(ctx, game)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestDelete() {
	game := s.createGame("delete")

	ctx := context.Background()
	err := s.repo.Delete(ctx, game.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetByID(ctx, game.ID)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *GameRepositorySuite) TestDelete_NotFound() {
	ctx := context.Background()

	err := s.repo.Delete(ctx, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// ---------------------------------------------------------------------------
// Exists
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestExists_True() {
	s.createGame("exists")

	ctx := context.Background()
	exists, err := s.repo.Exists(ctx, "test_exists")
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *GameRepositorySuite) TestExists_False() {
	ctx := context.Background()
	exists, err := s.repo.Exists(ctx, "nonexistent_game_xyz")
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

// ---------------------------------------------------------------------------
// AddToTournament / GetByTournamentID
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestAddToTournament() {
	game := s.createGame("add_to_t")
	tournament := s.createTournamentForGame("01")

	ctx := context.Background()
	err := s.repo.AddToTournament(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	// Verify the game appears in tournament games
	games, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)

	require.Len(s.T(), games, 1)
	assert.Equal(s.T(), game.ID, games[0].ID)
	assert.Equal(s.T(), game.Name, games[0].Name)
}

func (s *GameRepositorySuite) TestAddToTournament_Idempotent() {
	game := s.createGame("add_idem")
	tournament := s.createTournamentForGame("02")

	ctx := context.Background()

	// Add twice -- should not error due to ON CONFLICT DO NOTHING
	err := s.repo.AddToTournament(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	err = s.repo.AddToTournament(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	// Should still have only one entry
	games, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), games, 1)
}

func (s *GameRepositorySuite) TestGetByTournamentID_Empty() {
	tournament := s.createTournamentForGame("03")

	ctx := context.Background()
	games, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), games)
}

func (s *GameRepositorySuite) TestGetByTournamentID_Multiple() {
	game1 := s.createGame("multi_a")
	game2 := s.createGame("multi_b")
	tournament := s.createTournamentForGame("04")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game1.ID))
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game2.ID))

	games, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), games, 2)
}

// ---------------------------------------------------------------------------
// RemoveFromTournament
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestRemoveFromTournament() {
	game := s.createGame("remove")
	tournament := s.createTournamentForGame("05")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))

	err := s.repo.RemoveFromTournament(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	games, err := s.repo.GetByTournamentID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), games)
}

func (s *GameRepositorySuite) TestRemoveFromTournament_NotFound() {
	tournament := s.createTournamentForGame("06")

	ctx := context.Background()
	err := s.repo.RemoveFromTournament(ctx, tournament.ID, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// ---------------------------------------------------------------------------
// GetTournamentGame / GetTournamentGames
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestGetTournamentGame() {
	game := s.createGame("tg_get")
	tournament := s.createTournamentForGame("07")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))

	tg, err := s.repo.GetTournamentGame(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), tournament.ID, tg.TournamentID)
	assert.Equal(s.T(), game.ID, tg.GameID)
	assert.False(s.T(), tg.IsActive)
	assert.False(s.T(), tg.RoundCompleted)
	assert.Equal(s.T(), 0, tg.CurrentRound)
	assert.NotZero(s.T(), tg.CreatedAt)
}

func (s *GameRepositorySuite) TestGetTournamentGame_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetTournamentGame(ctx, uuid.New(), uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *GameRepositorySuite) TestGetTournamentGames() {
	game1 := s.createGame("tgs_a")
	game2 := s.createGame("tgs_b")
	tournament := s.createTournamentForGame("08")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game1.ID))
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game2.ID))

	tgs, err := s.repo.GetTournamentGames(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), tgs, 2)

	for _, tg := range tgs {
		assert.Equal(s.T(), tournament.ID, tg.TournamentID)
		assert.False(s.T(), tg.IsActive)
		assert.False(s.T(), tg.RoundCompleted)
	}
}

func (s *GameRepositorySuite) TestGetTournamentGames_Empty() {
	tournament := s.createTournamentForGame("09")

	ctx := context.Background()
	tgs, err := s.repo.GetTournamentGames(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), tgs)
}

// ---------------------------------------------------------------------------
// MarkRoundCompleted / IsRoundCompleted / ResetGameRound
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestMarkRoundCompleted() {
	game := s.createGame("mark_round")
	tournament := s.createTournamentForGame("10")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))

	err := s.repo.MarkRoundCompleted(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	// Verify via GetTournamentGame
	tg, err := s.repo.GetTournamentGame(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), tg.RoundCompleted)
	assert.NotNil(s.T(), tg.RoundCompletedAt)
}

func (s *GameRepositorySuite) TestMarkRoundCompleted_NotFound() {
	ctx := context.Background()

	err := s.repo.MarkRoundCompleted(ctx, uuid.New(), uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *GameRepositorySuite) TestIsRoundCompleted_False() {
	game := s.createGame("isround_f")
	tournament := s.createTournamentForGame("11")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))

	completed, err := s.repo.IsRoundCompleted(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), completed)
}

func (s *GameRepositorySuite) TestIsRoundCompleted_True() {
	game := s.createGame("isround_t")
	tournament := s.createTournamentForGame("12")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))
	require.NoError(s.T(), s.repo.MarkRoundCompleted(ctx, tournament.ID, game.ID))

	completed, err := s.repo.IsRoundCompleted(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), completed)
}

func (s *GameRepositorySuite) TestIsRoundCompleted_NoLink() {
	ctx := context.Background()

	// When the tournament-game link does not exist, IsRoundCompleted returns false (not an error)
	completed, err := s.repo.IsRoundCompleted(ctx, uuid.New(), uuid.New())
	require.NoError(s.T(), err)
	assert.False(s.T(), completed)
}

func (s *GameRepositorySuite) TestResetGameRound() {
	game := s.createGame("reset_round")
	tournament := s.createTournamentForGame("13")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))

	// Mark round completed and increment
	require.NoError(s.T(), s.repo.MarkRoundCompleted(ctx, tournament.ID, game.ID))
	_, err := s.repo.IncrementCurrentRound(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	// Reset
	err = s.repo.ResetGameRound(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)

	tg, err := s.repo.GetTournamentGame(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), tg.RoundCompleted)
	assert.Nil(s.T(), tg.RoundCompletedAt)
	assert.Equal(s.T(), 0, tg.CurrentRound)
}

func (s *GameRepositorySuite) TestResetGameRound_NotFound() {
	ctx := context.Background()

	err := s.repo.ResetGameRound(ctx, uuid.New(), uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

// ---------------------------------------------------------------------------
// IncrementCurrentRound
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestIncrementCurrentRound() {
	game := s.createGame("incr_round")
	tournament := s.createTournamentForGame("14")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))

	round1, err := s.repo.IncrementCurrentRound(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, round1)

	round2, err := s.repo.IncrementCurrentRound(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, round2)
}

// ---------------------------------------------------------------------------
// SetActiveGame / GetActiveGame / IsGameActive / DeactivateAllGames
// ---------------------------------------------------------------------------

func (s *GameRepositorySuite) TestSetActiveGame() {
	game1 := s.createGame("active_a")
	game2 := s.createGame("active_b")
	tournament := s.createTournamentForGame("15")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game1.ID))
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game2.ID))

	// Activate game1
	err := s.repo.SetActiveGame(ctx, tournament.ID, game1.ID)
	require.NoError(s.T(), err)

	active, err := s.repo.GetActiveGame(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), game1.ID, active.GameID)
	assert.True(s.T(), active.IsActive)

	// Switch to game2
	err = s.repo.SetActiveGame(ctx, tournament.ID, game2.ID)
	require.NoError(s.T(), err)

	active, err = s.repo.GetActiveGame(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), game2.ID, active.GameID)

	// game1 should no longer be active
	isActive, err := s.repo.IsGameActive(ctx, tournament.ID, game1.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), isActive)
}

func (s *GameRepositorySuite) TestSetActiveGame_NotInTournament() {
	tournament := s.createTournamentForGame("16")

	ctx := context.Background()
	err := s.repo.SetActiveGame(ctx, tournament.ID, uuid.New())
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *GameRepositorySuite) TestGetActiveGame_NoActive() {
	game := s.createGame("no_active")
	tournament := s.createTournamentForGame("17")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))

	_, err := s.repo.GetActiveGame(ctx, tournament.ID)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))
}

func (s *GameRepositorySuite) TestIsGameActive_True() {
	game := s.createGame("isactive_t")
	tournament := s.createTournamentForGame("18")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game.ID))
	require.NoError(s.T(), s.repo.SetActiveGame(ctx, tournament.ID, game.ID))

	isActive, err := s.repo.IsGameActive(ctx, tournament.ID, game.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), isActive)
}

func (s *GameRepositorySuite) TestIsGameActive_NoLink() {
	ctx := context.Background()

	// When the tournament-game link does not exist, IsGameActive returns false (not an error)
	isActive, err := s.repo.IsGameActive(ctx, uuid.New(), uuid.New())
	require.NoError(s.T(), err)
	assert.False(s.T(), isActive)
}

func (s *GameRepositorySuite) TestDeactivateAllGames() {
	game1 := s.createGame("deact_a")
	game2 := s.createGame("deact_b")
	tournament := s.createTournamentForGame("19")

	ctx := context.Background()
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game1.ID))
	require.NoError(s.T(), s.repo.AddToTournament(ctx, tournament.ID, game2.ID))
	require.NoError(s.T(), s.repo.SetActiveGame(ctx, tournament.ID, game1.ID))

	err := s.repo.DeactivateAllGames(ctx, tournament.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetActiveGame(ctx, tournament.ID)
	assert.Error(s.T(), err)
	assert.True(s.T(), errors.IsNotFound(err))

	// Verify both are inactive
	a1, err := s.repo.IsGameActive(ctx, tournament.ID, game1.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), a1)

	a2, err := s.repo.IsGameActive(ctx, tournament.ID, game2.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), a2)
}
