//go:build contract

package contract

import (
	"net/http"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Public endpoints (no auth required)
// ---------------------------------------------------------------------------

func TestContract_Tournament_List_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	now := time.Now().Truncate(time.Second)
	tournaments := []*domain.Tournament{
		{
			ID:        tournamentID,
			Name:      "Test Tournament",
			Code:      "TEST01",
			GameType:  "prisoners_dilemma",
			Status:    domain.TournamentPending,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	h.TournamentService.EXPECT().
		List(mock.Anything, mock.Anything).
		Return(tournaments, nil).
		Once()

	resp := h.GET("/api/v1/tournaments").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertSecurityHeaders(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_Get_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	now := time.Now().Truncate(time.Second)
	tournament := &domain.Tournament{
		ID:        tournamentID,
		Name:      "Test Tournament",
		Code:      "TEST01",
		GameType:  "prisoners_dilemma",
		Status:    domain.TournamentPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	h.TournamentService.EXPECT().
		GetByID(mock.Anything, tournamentID).
		Return(tournament, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String()).Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_Get_404(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.TournamentService.EXPECT().
		GetByID(mock.Anything, tournamentID).
		Return(nil, errors.ErrNotFound).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String()).Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Tournament_Leaderboard_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	programID := uuid.New()
	teamID := uuid.New()
	teamName := "Team Alpha"
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

	h.TournamentService.EXPECT().
		GetLeaderboard(mock.Anything, tournamentID, mock.AnythingOfType("int")).
		Return(entries, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/leaderboard").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_CrossGameLeaderboard_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	programID := uuid.New()
	entries := []*domain.CrossGameLeaderboardEntry{
		{
			Rank:        1,
			TeamName:    "Team Alpha",
			ProgramID:   programID,
			ProgramName: "AlphaBot",
			GameRatings: map[string]domain.GameRatingInfo{},
			TotalRating: 3000,
			TotalWins:   20,
			TotalLosses: 5,
			TotalGames:  25,
		},
	}

	h.TournamentService.EXPECT().
		GetCrossGameLeaderboard(mock.Anything, tournamentID).
		Return(entries, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/cross-game-leaderboard").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_GetMatches_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	now := time.Now().Truncate(time.Second)
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

	h.TournamentService.EXPECT().
		GetMatches(mock.Anything, tournamentID, mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return(matches, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/matches").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_GetMatchesByRounds_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	now := time.Now().Truncate(time.Second)
	rounds := []*domain.MatchRound{
		{
			RoundNumber:    1,
			GameType:       "prisoners_dilemma",
			TotalMatches:   3,
			CompletedCount: 3,
			PendingCount:   0,
			RunningCount:   0,
			FailedCount:    0,
			Matches:        []*domain.Match{},
			CreatedAt:      now,
		},
	}

	h.TournamentService.EXPECT().
		GetMatchesByRounds(mock.Anything, tournamentID).
		Return(rounds, nil).
		Once()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/matches/rounds").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

// ---------------------------------------------------------------------------
// Protected endpoints (user auth)
// ---------------------------------------------------------------------------

func TestContract_Tournament_Join_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	programID := uuid.New()

	h.TournamentService.EXPECT().
		Join(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/join").
		WithAuth(h.UserToken()).
		WithJSON(map[string]string{"program_id": programID.String()}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_Join_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	programID := uuid.New()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/join").
		WithJSON(map[string]string{"program_id": programID.String()}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Tournament_MyTeam_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	resp := h.GET("/api/v1/tournaments/" + tournamentID.String() + "/my-team").Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

// ---------------------------------------------------------------------------
// Admin endpoints
// ---------------------------------------------------------------------------

func TestContract_Tournament_Create_201(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	now := time.Now().Truncate(time.Second)
	createdTournament := &domain.Tournament{
		ID:        uuid.New(),
		Name:      "New Tournament",
		Code:      "NEW001",
		GameType:  "prisoners_dilemma",
		Status:    domain.TournamentPending,
		CreatorID: &h.TestAdminID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	h.TournamentService.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(createdTournament, nil).
		Once()

	resp := h.POST("/api/v1/tournaments").
		WithAuth(h.AdminToken()).
		WithJSON(map[string]string{
			"name":      "New Tournament",
			"game_type": "prisoners_dilemma",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_Create_403_User(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.POST("/api/v1/tournaments").
		WithAuth(h.UserToken()).
		WithJSON(map[string]string{
			"name":      "Forbidden Tournament",
			"game_type": "test",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Tournament_Create_401_NoAuth(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.POST("/api/v1/tournaments").
		WithJSON(map[string]string{
			"name":      "Unauthorized Tournament",
			"game_type": "test",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Tournament_Start_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.TournamentService.EXPECT().
		Start(mock.Anything, tournamentID).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/start").
		WithAuth(h.AdminToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_Start_403(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/start").
		WithAuth(h.UserToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Tournament_Complete_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.TournamentService.EXPECT().
		Complete(mock.Anything, tournamentID).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/complete").
		WithAuth(h.AdminToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_Delete_204(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.TournamentService.EXPECT().
		Delete(mock.Anything, tournamentID).
		Return(nil).
		Once()

	resp := h.DELETE("/api/v1/tournaments/" + tournamentID.String()).
		WithAuth(h.AdminToken()).
		Do()
	_ = ReadBody(t, resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestContract_Tournament_Delete_403(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	resp := h.DELETE("/api/v1/tournaments/" + tournamentID.String()).
		WithAuth(h.UserToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	AssertJSON(t, resp)
	AssertErrorResponse(t, body)
}

func TestContract_Tournament_CreateMatch_201(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	program1ID := uuid.New()
	program2ID := uuid.New()
	now := time.Now().Truncate(time.Second)

	createdMatch := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     "prisoners_dilemma",
		Status:       domain.MatchPending,
		Priority:     domain.PriorityMedium,
		CreatedAt:    now,
	}

	h.TournamentService.EXPECT().
		CreateMatch(mock.Anything, tournamentID, program1ID, program2ID, domain.PriorityMedium).
		Return(createdMatch, nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/matches").
		WithAuth(h.AdminToken()).
		WithJSON(map[string]string{
			"program1_id": program1ID.String(),
			"program2_id": program2ID.String(),
			"priority":    "medium",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_RunAllMatches_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.SchedulingService.EXPECT().
		RunAllMatches(mock.Anything, tournamentID).
		Return(5, nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/run-matches").
		WithAuth(h.AdminToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_RunGameMatches_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.SchedulingService.EXPECT().
		RunGameMatches(mock.Anything, tournamentID, "prisoners_dilemma").
		Return(3, nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/run-game-matches").
		WithAuth(h.AdminToken()).
		WithJSON(map[string]string{
			"game_type": "prisoners_dilemma",
		}).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}

func TestContract_Tournament_RetryFailedMatches_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()

	h.SchedulingService.EXPECT().
		RetryFailedMatches(mock.Anything, tournamentID).
		Return(2, nil).
		Once()

	resp := h.POST("/api/v1/tournaments/" + tournamentID.String() + "/retry-matches").
		WithAuth(h.AdminToken()).
		Do()
	body := ReadBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	AssertEnvelope(t, body)
}
