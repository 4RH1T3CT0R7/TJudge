//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Team DTOs

type TeamResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TournamentID string `json:"tournament_id"`
	Code         string `json:"code"`
	LeaderID     string `json:"leader_id"`
}

type TeamWithMembersResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TournamentID string `json:"tournament_id"`
	Code         string `json:"code"`
	LeaderID     string `json:"leader_id"`
	Members      []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"members"`
}

type CreateTeamAPIRequest struct {
	TournamentID string `json:"tournament_id"`
	Name         string `json:"name"`
}

type JoinTeamRequest struct {
	Code string `json:"code"`
}

type InviteLinkResponse struct {
	Code string `json:"code"`
	Link string `json:"link"`
}

type UpdateTeamNameRequest struct {
	Name string `json:"name"`
}

// registerAndAuth registers a new user and returns the access token.
// The client's token is also set for subsequent requests.
func registerAndAuth(t *testing.T, client *TestClient, suffix string) string {
	t.Helper()
	timestamp := time.Now().UnixNano()
	req := RegisterRequest{
		Username: fmt.Sprintf("e2e_%s_%d", suffix, timestamp),
		Email:    fmt.Sprintf("e2e_%s_%d@test.com", suffix, timestamp),
		Password: "SecurePass123!",
	}

	resp, err := client.doRequest("POST", "/api/v1/auth/register", req)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Register failed for %s: %d - %s", suffix, resp.StatusCode, string(body))
	}

	var authResp AuthResponse
	err = client.parseResponse(resp, &authResp)
	require.NoError(t, err)
	require.NotEmpty(t, authResp.AccessToken)

	client.SetToken(authResp.AccessToken)
	return authResp.AccessToken
}

// createTournamentHelper creates a tournament and returns its ID.
// Assumes the client already has a valid auth token set.
func createTournamentHelper(t *testing.T, client *TestClient) string {
	t.Helper()
	req := CreateTournamentRequest{
		Name:            fmt.Sprintf("E2E Team Test Tournament %d", time.Now().UnixNano()),
		Description:     "Tournament for team E2E testing",
		GameType:        "tictactoe",
		MaxParticipants: 20,
		MaxTeamSize:     10,
	}

	resp, err := client.doRequest("POST", "/api/v1/tournaments", req)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Create tournament failed: %d - %s", resp.StatusCode, string(body))
	}

	var tournamentResp TournamentResponse
	err = client.parseResponse(resp, &tournamentResp)
	require.NoError(t, err)
	require.NotEmpty(t, tournamentResp.ID)

	return tournamentResp.ID
}

// createTeamHelper creates a team in the given tournament and returns the team response.
// Assumes the client already has a valid auth token set.
func createTeamHelper(t *testing.T, client *TestClient, tournamentID string) TeamResponse {
	t.Helper()
	req := CreateTeamAPIRequest{
		TournamentID: tournamentID,
		Name:         fmt.Sprintf("E2E Team %d", time.Now().UnixNano()),
	}

	resp, err := client.doRequest("POST", "/api/v1/teams", req)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Create team failed: %d - %s", resp.StatusCode, string(body))
	}

	var teamResp TeamResponse
	err = client.parseResponse(resp, &teamResp)
	require.NoError(t, err)
	require.NotEmpty(t, teamResp.ID)

	return teamResp
}

// =============================================================================
// E2E Test: Team Creation
// =============================================================================

func TestE2E_TeamCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Step 1: Register and authenticate
	registerAndAuth(t, client, "teamcreate")

	// Step 2: Create a tournament
	tournamentID := createTournamentHelper(t, client)

	// Step 3: Create a team in the tournament
	var teamID string
	var teamName string

	t.Run("CreateTeam", func(t *testing.T) {
		teamName = fmt.Sprintf("Alpha Squad %d", time.Now().UnixNano())
		req := CreateTeamAPIRequest{
			TournamentID: tournamentID,
			Name:         teamName,
		}

		resp, err := client.doRequest("POST", "/api/v1/teams", req)
		require.NoError(t, err)
		assert.Contains(t, []int{http.StatusOK, http.StatusCreated}, resp.StatusCode)

		var teamResp TeamResponse
		err = client.parseResponse(resp, &teamResp)
		require.NoError(t, err)

		assert.NotEmpty(t, teamResp.ID)
		assert.Equal(t, teamName, teamResp.Name)
		assert.Equal(t, tournamentID, teamResp.TournamentID)

		teamID = teamResp.ID
	})

	// Step 4: Get the team by ID and verify
	t.Run("GetTeamByID", func(t *testing.T) {
		require.NotEmpty(t, teamID, "teamID must be set from previous step")

		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/teams/%s", teamID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var teamResp TeamWithMembersResponse
		err = client.parseResponse(resp, &teamResp)
		require.NoError(t, err)

		assert.Equal(t, teamID, teamResp.ID)
		assert.Equal(t, teamName, teamResp.Name)
		assert.Equal(t, tournamentID, teamResp.TournamentID)
		assert.NotEmpty(t, teamResp.LeaderID)
	})
}

// =============================================================================
// E2E Test: Team Join By Code
// =============================================================================

func TestE2E_TeamJoinByCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client1 := NewTestClient()
	client2 := NewTestClient()

	// Step 1: Register user 1 (team leader)
	registerAndAuth(t, client1, "teamjoin_leader")

	// Step 2: Create tournament and team as user 1
	tournamentID := createTournamentHelper(t, client1)
	teamResp := createTeamHelper(t, client1, tournamentID)
	teamID := teamResp.ID

	// Step 3: Get the invite link/code for the team
	var inviteCode string

	t.Run("GetInviteLink", func(t *testing.T) {
		resp, err := client1.doRequest("GET", fmt.Sprintf("/api/v1/teams/%s/invite", teamID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var inviteResp InviteLinkResponse
		err = client1.parseResponse(resp, &inviteResp)
		require.NoError(t, err)

		assert.NotEmpty(t, inviteResp.Code)
		inviteCode = inviteResp.Code
	})

	// Step 4: Register user 2 and join the team
	registerAndAuth(t, client2, "teamjoin_member")

	t.Run("JoinTeamByCode", func(t *testing.T) {
		require.NotEmpty(t, inviteCode, "invite code must be set from previous step")

		req := JoinTeamRequest{
			Code: inviteCode,
		}

		resp, err := client2.doRequest("POST", "/api/v1/teams/join", req)
		require.NoError(t, err)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("Join team failed: status=%d body=%s invite_code=%s", resp.StatusCode, string(body), inviteCode)
		}
		resp.Body.Close()
	})

	// Step 5: Verify both users are members
	t.Run("VerifyMembers", func(t *testing.T) {
		resp, err := client1.doRequest("GET", fmt.Sprintf("/api/v1/teams/%s/members", teamID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var members []struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		}
		err = client1.parseResponse(resp, &members)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(members), 2, "Team should have at least 2 members (leader + joined user)")
	})
}

// =============================================================================
// E2E Test: Team Leave
// =============================================================================

func TestE2E_TeamLeave(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Setup: register, create tournament, create team
	registerAndAuth(t, client, "teamleave")
	tournamentID := createTournamentHelper(t, client)
	teamResp := createTeamHelper(t, client, tournamentID)
	teamID := teamResp.ID

	// Leave the team
	t.Run("LeaveTeam", func(t *testing.T) {
		resp, err := client.doRequest("POST", fmt.Sprintf("/api/v1/teams/%s/leave", teamID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, resp.StatusCode,
			"Leave team should return 200 or 204")
	})
}

// =============================================================================
// E2E Test: Team Update Name
// =============================================================================

func TestE2E_TeamUpdateName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Setup: register, create tournament, create team
	registerAndAuth(t, client, "teamupdate")
	tournamentID := createTournamentHelper(t, client)
	teamResp := createTeamHelper(t, client, tournamentID)
	teamID := teamResp.ID

	newName := fmt.Sprintf("Updated Team Name %d", time.Now().UnixNano())

	// Update the team name
	t.Run("UpdateTeamName", func(t *testing.T) {
		req := UpdateTeamNameRequest{
			Name: newName,
		}

		resp, err := client.doRequest("PUT", fmt.Sprintf("/api/v1/teams/%s", teamID), req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var updated TeamResponse
		err = client.parseResponse(resp, &updated)
		require.NoError(t, err)

		assert.Equal(t, newName, updated.Name)
	})

	// Verify the name change by fetching the team again
	t.Run("VerifyUpdatedName", func(t *testing.T) {
		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/teams/%s", teamID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var teamDetail TeamWithMembersResponse
		err = client.parseResponse(resp, &teamDetail)
		require.NoError(t, err)

		assert.Equal(t, newName, teamDetail.Name)
	})
}

// =============================================================================
// E2E Test: Team Not Found
// =============================================================================

func TestE2E_TeamNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Must be authenticated to access team routes
	registerAndAuth(t, client, "teamnotfound")

	t.Run("GetNonExistentTeam", func(t *testing.T) {
		randomID := uuid.New().String()

		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/teams/%s", randomID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// =============================================================================
// E2E Test: Team Unauthorized
// =============================================================================

func TestE2E_TeamUnauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Ensure no auth token is set
	client.SetToken("")

	t.Run("CreateTeamWithoutAuth", func(t *testing.T) {
		req := CreateTeamAPIRequest{
			TournamentID: uuid.New().String(),
			Name:         "Unauthorized Team",
		}

		resp, err := client.doRequest("POST", "/api/v1/teams", req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// =============================================================================
// E2E Test: Tournament Teams
// =============================================================================

func TestE2E_TournamentTeams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Setup: register, create tournament, create team
	registerAndAuth(t, client, "tournteams")
	tournamentID := createTournamentHelper(t, client)
	teamResp := createTeamHelper(t, client, tournamentID)

	// Get tournament teams (public endpoint, no auth required)
	t.Run("GetTournamentTeams", func(t *testing.T) {
		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s/teams", tournamentID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var teams []TeamResponse
		err = client.parseResponse(resp, &teams)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(teams), 1, "Tournament should have at least one team")

		// Verify our team is in the list
		found := false
		for _, team := range teams {
			if team.ID == teamResp.ID {
				found = true
				assert.Equal(t, teamResp.Name, team.Name)
				break
			}
		}
		assert.True(t, found, "Created team should be in the tournament teams list")
	})
}

// =============================================================================
// E2E Test: My Team In Tournament
// =============================================================================

func TestE2E_MyTeamInTournament(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Setup: register, create tournament, create team
	registerAndAuth(t, client, "myteam")
	tournamentID := createTournamentHelper(t, client)
	teamResp := createTeamHelper(t, client, tournamentID)

	// Get my team in the tournament (authenticated)
	t.Run("GetMyTeam", func(t *testing.T) {
		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s/my-team", tournamentID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var myTeam TeamResponse
		err = client.parseResponse(resp, &myTeam)
		require.NoError(t, err)

		assert.Equal(t, teamResp.ID, myTeam.ID)
		assert.Equal(t, teamResp.Name, myTeam.Name)
		assert.Equal(t, tournamentID, myTeam.TournamentID)
	})
}
