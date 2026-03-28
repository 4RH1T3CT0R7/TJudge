//go:build contract

package contract

import (
	"net/http"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Public game CRUD endpoints (no auth required)
// ---------------------------------------------------------------------------

func TestContract_Game_List_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	now := time.Now().Truncate(time.Second)
	games := []*domain.Game{
		{
			ID:          uuid.New(),
			Name:        "prisoners_dilemma",
			DisplayName: "Prisoners Dilemma",
			Rules:       "Two players choose to cooperate or defect.",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	h.GameService.EXPECT().
		List(mock.Anything, mock.Anything).
		Return(games, nil).
		Once()

	resp := h.GET("/api/v1/games").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertSecurityHeaders(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_Get_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	gameID := uuid.New()
	now := time.Now().Truncate(time.Second)
	game := &domain.Game{
		ID:          gameID,
		Name:        "prisoners_dilemma",
		DisplayName: "Prisoners Dilemma",
		Rules:       "Two players choose to cooperate or defect.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	h.GameService.EXPECT().
		GetByID(mock.Anything, gameID).
		Return(game, nil).
		Once()

	resp := h.GET("/api/v1/games/" + gameID.String()).Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_GetByName_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	now := time.Now().Truncate(time.Second)
	game := &domain.Game{
		ID:          uuid.New(),
		Name:        "prisoners_dilemma",
		DisplayName: "Prisoners Dilemma",
		Rules:       "Two players choose to cooperate or defect.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	h.GameService.EXPECT().
		GetByName(mock.Anything, "prisoners_dilemma").
		Return(game, nil).
		Once()

	resp := h.GET("/api/v1/games/name/prisoners_dilemma").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

// ---------------------------------------------------------------------------
// Admin game CRUD endpoints
// ---------------------------------------------------------------------------

func TestContract_Game_Create_201(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	now := time.Now().Truncate(time.Second)
	createdGame := &domain.Game{
		ID:          uuid.New(),
		Name:        "tug_of_war",
		DisplayName: "Tug of War",
		Rules:       "Players bid resources in a series of rounds.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	h.GameService.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(createdGame, nil).
		Once()

	resp := h.POST("/api/v1/games").
		WithAuth(h.AdminToken()).
		WithJSON(map[string]interface{}{
			"name":         "tug_of_war",
			"display_name": "Tug of War",
			"rules":        "Players bid resources in a series of rounds.",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_Create_403_User(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.POST("/api/v1/games").
		WithAuth(h.UserToken()).
		WithJSON(map[string]interface{}{
			"name":         "forbidden_game",
			"display_name": "Forbidden Game",
			"rules":        "Nope.",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Game_Update_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	gameID := uuid.New()
	now := time.Now().Truncate(time.Second)
	updatedGame := &domain.Game{
		ID:          gameID,
		Name:        "prisoners_dilemma",
		DisplayName: "Updated Dilemma",
		Rules:       "Updated rules.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	h.GameService.EXPECT().
		Update(mock.Anything, gameID, mock.Anything).
		Return(updatedGame, nil).
		Once()

	resp := h.PUT("/api/v1/games/" + gameID.String()).
		WithAuth(h.AdminToken()).
		WithJSON(map[string]interface{}{
			"display_name": "Updated Dilemma",
			"rules":        "Updated rules.",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_Delete_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	gameID := uuid.New()

	h.GameService.EXPECT().
		Delete(mock.Anything, gameID).
		Return(nil).
		Once()

	resp := h.DELETE("/api/v1/games/" + gameID.String()).
		WithAuth(h.AdminToken()).
		Do()
	_ = ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Tournament-game public endpoints
// ---------------------------------------------------------------------------

func TestContract_Game_GetTournamentGames_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	now := time.Now().Truncate(time.Second)
	games := []*domain.Game{
		{
			ID:          uuid.New(),
			Name:        "prisoners_dilemma",
			DisplayName: "Prisoners Dilemma",
			Rules:       "Cooperate or defect.",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	h.GameService.EXPECT().
		GetByTournamentID(mock.Anything, tournamentID).
		Return(games, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/games").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_GetGameLeaderboard_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()
	programID := uuid.New()
	teamID := uuid.New()
	teamName := "Team Alpha"
	now := time.Now().Truncate(time.Second)

	// Handler first looks up the game to get its Name.
	game := &domain.Game{
		ID:          gameID,
		Name:        "prisoners_dilemma",
		DisplayName: "Prisoners Dilemma",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.GameService.EXPECT().
		GetByID(mock.Anything, gameID).
		Return(game, nil).
		Once()

	entries := []*domain.LeaderboardEntry{
		{
			Rank:        1,
			ProgramID:   programID,
			ProgramName: "AlphaBot",
			TeamID:      &teamID,
			TeamName:    &teamName,
			Rating:      1500,
			Wins:        10,
			Losses:      2,
			Draws:       1,
			TotalGames:  13,
		},
	}
	h.LeaderboardRepo.EXPECT().
		GetLeaderboardByGameType(mock.Anything, tournamentID, "prisoners_dilemma", mock.AnythingOfType("int")).
		Return(entries, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/games/" + gameID.String() + "/leaderboard").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_GetGameMatches_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()
	now := time.Now().Truncate(time.Second)

	// Handler looks up game to get its Name.
	game := &domain.Game{
		ID:          gameID,
		Name:        "prisoners_dilemma",
		DisplayName: "Prisoners Dilemma",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.GameService.EXPECT().
		GetByID(mock.Anything, gameID).
		Return(game, nil).
		Once()

	matches := []*domain.Match{
		{
			ID:           uuid.New(),
			TournamentID: tournamentID,
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchCompleted,
			Priority:     domain.PriorityMedium,
			CreatedAt:    now,
		},
	}
	h.GameMatchRepo.EXPECT().
		List(mock.Anything, mock.Anything).
		Return(matches, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/games/" + gameID.String() + "/matches").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_GetTournamentGamesWithStatus_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()
	details := []*domain.TournamentGameWithDetails{
		{
			TournamentID:          tournamentID,
			GameID:                gameID,
			GameName:              "prisoners_dilemma",
			GameDisplayName:       "Prisoners Dilemma",
			IsActive:              true,
			RoundCompleted:        false,
			CurrentRound:          1,
			AutoRoundEnabled:      false,
			AutoRoundIntervalSecs: 60,
		},
	}

	h.TournamentGameStatusRepo.EXPECT().
		GetTournamentGamesWithDetails(mock.Anything, tournamentID).
		Return(details, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/games/status").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_GetActiveGame_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()
	now := time.Now().Truncate(time.Second)

	activeGame := &domain.TournamentGame{
		TournamentID:          tournamentID,
		GameID:                gameID,
		IsActive:              true,
		RoundCompleted:        false,
		CurrentRound:          1,
		AutoRoundEnabled:      false,
		AutoRoundIntervalSecs: 60,
		CreatedAt:             now,
	}

	h.TournamentGameStatusRepo.EXPECT().
		GetActiveGame(mock.Anything, tournamentID).
		Return(activeGame, nil).
		Once()

	// Handler also calls GameService.GetByID to resolve game details.
	game := &domain.Game{
		ID:          gameID,
		Name:        "prisoners_dilemma",
		DisplayName: "Prisoners Dilemma",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.GameService.EXPECT().
		GetByID(mock.Anything, gameID).
		Return(game, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/active-game").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

// ---------------------------------------------------------------------------
// Tournament-game protected/admin endpoints
// ---------------------------------------------------------------------------

func TestContract_Game_AddToTournament_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()

	// The handler checks if the user is admin or tournament creator.
	// Admin token has RoleAdmin so it skips the tournament ownership check.
	h.GameService.EXPECT().
		AddToTournament(mock.Anything, tournamentID, gameID).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/games").
		WithAuth(h.AdminToken()).
		WithJSON(map[string]string{"game_id": gameID.String()}).
		Do()
	_ = ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Game_AddToTournament_CreatorAuth_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()

	// User token is not admin, so handler looks up tournament to check creator.
	tournament := &domain.Tournament{
		ID:        tournamentID,
		Name:      "Creator's Tournament",
		CreatorID: &h.TestUserID,
		Status:    domain.TournamentPending,
	}
	h.GameTournamentRepo.EXPECT().
		GetByID(mock.Anything, tournamentID).
		Return(tournament, nil).
		Once()

	h.GameService.EXPECT().
		AddToTournament(mock.Anything, tournamentID, gameID).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/games").
		WithAuth(h.UserToken()).
		WithJSON(map[string]string{"game_id": gameID.String()}).
		Do()
	_ = ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Game_RemoveFromTournament_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()

	h.GameService.EXPECT().
		RemoveFromTournament(mock.Anything, tournamentID, gameID).
		Return(nil).
		Once()

	resp := h.DELETE("/api/v1/tournaments/" + tournamentID.String() + "/games/" + gameID.String()).
		WithAuth(h.AdminToken()).
		Do()
	_ = ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Admin game round management endpoints
// ---------------------------------------------------------------------------

func TestContract_Game_MarkRoundCompleted_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()

	h.TournamentGameStatusRepo.EXPECT().
		MarkRoundCompleted(mock.Anything, tournamentID, gameID).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/games/" + gameID.String() + "/complete-round").
		WithAuth(h.AdminToken()).
		Do()
	_ = ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Game_ResetRound_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()
	now := time.Now().Truncate(time.Second)

	// Handler looks up game to get its Name for the full reset.
	game := &domain.Game{
		ID:          gameID,
		Name:        "prisoners_dilemma",
		DisplayName: "Prisoners Dilemma",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.GameService.EXPECT().
		GetByID(mock.Anything, gameID).
		Return(game, nil).
		Once()

	h.TournamentGameStatusRepo.EXPECT().
		ResetGameRoundFull(mock.Anything, tournamentID, gameID, "prisoners_dilemma").
		Return(int64(5), int64(3), int64(10), nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/games/" + gameID.String() + "/reset-round").
		WithAuth(h.AdminToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_SetAutoRound_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()

	h.TournamentGameStatusRepo.EXPECT().
		SetAutoRound(mock.Anything, tournamentID, gameID, true, 300).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/games/" + gameID.String() + "/auto-round").
		WithAuth(h.AdminToken()).
		WithJSON(map[string]interface{}{
			"enabled":          true,
			"interval_seconds": 300,
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Game_GetAutoRound_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	gameID := uuid.New()
	now := time.Now().Truncate(time.Second)

	tg := &domain.TournamentGame{
		TournamentID:          tournamentID,
		GameID:                gameID,
		IsActive:              true,
		AutoRoundEnabled:      true,
		AutoRoundIntervalSecs: 300,
		CreatedAt:             now,
	}

	h.TournamentGameStatusRepo.EXPECT().
		GetTournamentGame(mock.Anything, tournamentID, gameID).
		Return(tg, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/games/" + gameID.String() + "/auto-round").
		WithAuth(h.AdminToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}
