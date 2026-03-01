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

// Game DTOs

type GameResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Rules       string `json:"rules"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type CreateGameRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Rules       string `json:"rules"`
}

type UpdateGameRequest struct {
	DisplayName string `json:"display_name"`
	Rules       string `json:"rules"`
}

// =============================================================================
// Helper: register a fresh user and return access token
// =============================================================================

func registerTestUser(t *testing.T, client *TestClient, prefix string) string {
	t.Helper()
	timestamp := time.Now().UnixNano()
	username := fmt.Sprintf("e2e_%s_%d", prefix, timestamp)

	req := RegisterRequest{
		Username: username,
		Email:    username + "@test.com",
		Password: "SecurePass123!",
	}

	resp, err := client.doRequest("POST", "/api/v1/auth/register", req)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Register failed: %d - %s", resp.StatusCode, string(body))
	}

	var authResp AuthResponse
	err = client.parseResponse(resp, &authResp)
	require.NoError(t, err)
	require.NotEmpty(t, authResp.AccessToken)

	return authResp.AccessToken
}

// =============================================================================
// E2E Test: Game List And Get
// =============================================================================

func TestE2E_GameListAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	var games []GameResponse

	// List all games (public, no auth needed)
	t.Run("ListGames", func(t *testing.T) {
		resp, err := client.doRequest("GET", "/api/v1/games", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		err = client.parseResponse(resp, &games)
		require.NoError(t, err)

		// The response should be a valid JSON array (possibly empty)
		assert.NotNil(t, games, "games list should not be nil")
		t.Logf("Found %d games", len(games))
	})

	// If games exist, get one by ID
	t.Run("GetGameByID", func(t *testing.T) {
		if len(games) == 0 {
			t.Skip("No games available to fetch by ID")
		}

		gameID := games[0].ID
		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/games/%s", gameID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var game GameResponse
		err = client.parseResponse(resp, &game)
		require.NoError(t, err)

		assert.Equal(t, gameID, game.ID)
		assert.NotEmpty(t, game.Name, "game name should not be empty")
	})
}

// =============================================================================
// E2E Test: Game CRUD (non-admin gets 403)
// =============================================================================

func TestE2E_GameCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Register a regular (non-admin) user
	accessToken := registerTestUser(t, client, "gamecrud")
	client.SetToken(accessToken)

	timestamp := time.Now().UnixNano()

	// Non-admin user tries to create a game -> expect 403
	t.Run("CreateGameForbidden", func(t *testing.T) {
		req := CreateGameRequest{
			Name:        fmt.Sprintf("test_game_%d", timestamp),
			DisplayName: fmt.Sprintf("Test Game %d", timestamp),
			Rules:       "Some rules for the test game.",
		}

		resp, err := client.doRequest("POST", "/api/v1/games", req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"non-admin user should not be able to create a game")
	})

	// Non-admin user tries to update a non-existent game -> expect 403
	t.Run("UpdateGameForbidden", func(t *testing.T) {
		fakeID := uuid.New().String()
		req := UpdateGameRequest{
			DisplayName: "Updated Name",
			Rules:       "Updated rules.",
		}

		resp, err := client.doRequest("PUT", fmt.Sprintf("/api/v1/games/%s", fakeID), req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// The admin middleware should reject before checking existence
		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"non-admin user should not be able to update a game")
	})

	// Non-admin user tries to delete a game -> expect 403
	t.Run("DeleteGameForbidden", func(t *testing.T) {
		fakeID := uuid.New().String()

		resp, err := client.doRequest("DELETE", fmt.Sprintf("/api/v1/games/%s", fakeID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"non-admin user should not be able to delete a game")
	})

	// Unauthenticated user tries to create a game -> expect 401
	t.Run("CreateGameUnauthorized", func(t *testing.T) {
		client.SetToken("")

		req := CreateGameRequest{
			Name:        fmt.Sprintf("unauth_game_%d", timestamp),
			DisplayName: fmt.Sprintf("Unauth Game %d", timestamp),
			Rules:       "Rules.",
		}

		resp, err := client.doRequest("POST", "/api/v1/games", req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"unauthenticated user should get 401 on game creation")
	})
}

// =============================================================================
// E2E Test: Game Not Found
// =============================================================================

func TestE2E_GameNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	t.Run("GetNonExistentGame", func(t *testing.T) {
		randomID := uuid.New().String()

		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/games/%s", randomID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"fetching a non-existent game should return 404")
	})

	t.Run("GetGameWithInvalidUUID", func(t *testing.T) {
		resp, err := client.doRequest("GET", "/api/v1/games/not-a-valid-uuid", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// The handler parses UUID and returns 400 for invalid format
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, resp.StatusCode,
			"fetching a game with invalid UUID should return 400 or 404")
	})
}

// =============================================================================
// E2E Test: Game By Name
// =============================================================================

func TestE2E_GameByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Try getting a game by a nonexistent name -> expect 404
	t.Run("GetByNameNotFound", func(t *testing.T) {
		resp, err := client.doRequest("GET", "/api/v1/games/name/nonexistent_game_name_12345", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"fetching a game by nonexistent name should return 404")
	})

	// If there are existing games, try getting one by name
	t.Run("GetByNameExisting", func(t *testing.T) {
		// First, list games to find an existing name
		resp, err := client.doRequest("GET", "/api/v1/games", nil)
		require.NoError(t, err)

		var games []GameResponse
		err = client.parseResponse(resp, &games)
		require.NoError(t, err)

		if len(games) == 0 {
			t.Skip("No games available to fetch by name")
		}

		gameName := games[0].Name
		require.NotEmpty(t, gameName)

		resp, err = client.doRequest("GET", fmt.Sprintf("/api/v1/games/name/%s", gameName), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var game GameResponse
		err = client.parseResponse(resp, &game)
		require.NoError(t, err)

		assert.Equal(t, gameName, game.Name)
		assert.NotEmpty(t, game.ID, "game should have an ID")
	})
}

// =============================================================================
// E2E Test: Tournament Games
// =============================================================================

func TestE2E_TournamentGames(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Register user and promote to admin (tournament creation requires admin role)
	accessToken := registerTestUser(t, client, "tourngames")
	client.SetToken(accessToken)

	// Get user info to promote to admin
	meResp, err := client.doRequest("GET", "/api/v1/auth/me", nil)
	require.NoError(t, err)
	var meData struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	err = decodeJSON(meResp.Body, &meData)
	meResp.Body.Close()
	require.NoError(t, err)

	adminToken := promoteToAdmin(t, client, meData.ID, meData.Username, "SecurePass123!")
	client.SetToken(adminToken)

	var tournamentID string

	t.Run("CreateTournament", func(t *testing.T) {
		req := CreateTournamentRequest{
			Name:            fmt.Sprintf("E2E Game Test Tournament %d", time.Now().UnixNano()),
			Description:     "Tournament for game management E2E tests",
			GameType:        "prisoners_dilemma",
			MaxParticipants: 10,
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

		tournamentID = tournamentResp.ID
	})

	// GET tournament games -> expect 200 with a JSON array (possibly empty or null)
	t.Run("GetTournamentGames", func(t *testing.T) {
		require.NotEmpty(t, tournamentID, "tournament must be created first")

		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s/games", tournamentID), nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var games []GameResponse
		err = client.parseResponse(resp, &games)
		require.NoError(t, err)

		// A fresh tournament may return null (nil slice) or empty array — both are valid
		t.Logf("Tournament %s has %d games", tournamentID, len(games))
	})

	// GET tournament games for a non-existent tournament
	t.Run("GetTournamentGamesNotFound", func(t *testing.T) {
		fakeID := uuid.New().String()

		resp, err := client.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s/games", fakeID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Depending on implementation, may return 200 with empty array or 404
		assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, resp.StatusCode,
			"non-existent tournament games should return 200 (empty) or 404")
	})

	// Non-admin user tries to add a game to tournament they did not create
	t.Run("AddGameToTournamentAsNonCreator", func(t *testing.T) {
		// Register a second user who is not the tournament creator
		otherToken := registerTestUser(t, client, "tourngames_other")
		client.SetToken(otherToken)

		req := map[string]string{
			"game_id": uuid.New().String(),
		}

		resp, err := client.doRequest("POST", fmt.Sprintf("/api/v1/tournaments/%s/games", tournamentID), req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be forbidden for non-admin, non-creator
		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"non-admin non-creator should not be able to add games to tournament")
	})
}

// =============================================================================
// E2E Test: Game Validation
// =============================================================================

func TestE2E_GameValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := NewTestClient()

	// Without auth, admin endpoints return 401 before validation
	t.Run("CreateGameEmptyBodyNoAuth", func(t *testing.T) {
		client.SetToken("")

		resp, err := client.doRequest("POST", "/api/v1/games", map[string]string{})
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should get 401 because game creation requires admin auth
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"creating game without auth should return 401")
	})

	// With auth (non-admin), admin endpoints return 403 before validation
	t.Run("CreateGameEmptyBodyNonAdmin", func(t *testing.T) {
		accessToken := registerTestUser(t, client, "gamevalid")
		client.SetToken(accessToken)

		resp, err := client.doRequest("POST", "/api/v1/games", map[string]string{})
		require.NoError(t, err)
		defer resp.Body.Close()

		// Middleware rejects non-admin before handler validates the body
		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"non-admin user should get 403 even with empty body")
	})

	// With auth (non-admin), try to create with invalid body
	t.Run("CreateGameInvalidBodyNonAdmin", func(t *testing.T) {
		accessToken := registerTestUser(t, client, "gamevalid2")
		client.SetToken(accessToken)

		req := CreateGameRequest{
			Name:        "",
			DisplayName: "",
			Rules:       "",
		}

		resp, err := client.doRequest("POST", "/api/v1/games", req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Non-admin is rejected at middleware level
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, resp.StatusCode,
			"should return 400 (validation) or 403 (non-admin)")
	})

	// Try update with nil/empty body on admin-only route without auth
	t.Run("UpdateGameNoAuth", func(t *testing.T) {
		client.SetToken("")
		fakeID := uuid.New().String()

		resp, err := client.doRequest("PUT", fmt.Sprintf("/api/v1/games/%s", fakeID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"updating game without auth should return 401")
	})
}
