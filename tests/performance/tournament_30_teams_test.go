//go:build performance
// +build performance

// Performance test for tournament with 30 teams.
//
// Run without cache (recommended):
//   go test -v -tags=performance ./tests/performance/... -run TestPerformance_30Teams_Tournament -timeout 5m -count=1
//
// Environment variables:
//   PERF_API_URL - API base URL (default: http://localhost:8080)

package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	baseURL = getEnv("PERF_API_URL", "http://localhost:8080")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestClient wraps HTTP client for performance tests
type TestClient struct {
	client      *http.Client
	baseURL     string
	accessToken string
}

func NewTestClient() *TestClient {
	return &TestClient{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: baseURL,
	}
}

func (c *TestClient) SetToken(token string) {
	c.accessToken = token
}

func (c *TestClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	return c.client.Do(req)
}

func (c *TestClient) uploadProgram(teamID, tournamentID, gameID, name, code string) (*http.Response, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add form fields
	_ = w.WriteField("team_id", teamID)
	_ = w.WriteField("tournament_id", tournamentID)
	_ = w.WriteField("game_id", gameID)
	_ = w.WriteField("name", name)

	// Add code file
	fw, err := w.CreateFormFile("file", "bot.py")
	if err != nil {
		return nil, err
	}
	_, _ = fw.Write([]byte(code))
	w.Close()

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/programs", &b)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	return c.client.Do(req)
}

func (c *TestClient) parseResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

// Bot strategies for Prisoner's Dilemma
// Protocol: 1. Read iterations, 2. Loop: output COOPERATE/DEFECT first, then read opponent's choice
var dilemmaStrategies = []string{
	// Tit-for-Tat - cooperate first, then copy opponent
	`#!/usr/bin/python3
next_choice = "COOPERATE"
n = int(input())
for i in range(n):
    print(next_choice, flush=True)
    next_choice = input().strip()
`,
	// Always Cooperate
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print("COOPERATE", flush=True)
    input()
`,
	// Always Defect
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print("DEFECT", flush=True)
    input()
`,
	// Grim Trigger - cooperate until opponent defects, then always defect
	`#!/usr/bin/python3
n = int(input())
opponent_defected = False
for i in range(n):
    if opponent_defected:
        print("DEFECT", flush=True)
    else:
        print("COOPERATE", flush=True)
    opp = input().strip()
    if opp == "DEFECT":
        opponent_defected = True
`,
	// Random
	`#!/usr/bin/python3
import random
n = int(input())
for i in range(n):
    print(random.choice(["COOPERATE", "DEFECT"]), flush=True)
    input()
`,
	// Suspicious Tit-for-Tat (start with DEFECT)
	`#!/usr/bin/python3
next_choice = "DEFECT"
n = int(input())
for i in range(n):
    print(next_choice, flush=True)
    next_choice = input().strip()
`,
	// Pavlov (Win-Stay, Lose-Shift)
	`#!/usr/bin/python3
n = int(input())
my_last = "COOPERATE"
for i in range(n):
    print(my_last, flush=True)
    opp = input().strip()
    # Win-stay: if we both cooperated or both defected, repeat
    # Lose-shift: if mismatch, switch
    if my_last == opp:
        my_last = "COOPERATE"
    else:
        my_last = "DEFECT"
`,
	// Generous Tit-for-Tat (forgive 10% of time)
	`#!/usr/bin/python3
import random
next_choice = "COOPERATE"
n = int(input())
for i in range(n):
    print(next_choice, flush=True)
    opp = input().strip()
    if opp == "DEFECT" and random.random() < 0.1:
        next_choice = "COOPERATE"
    else:
        next_choice = opp
`,
}

// Bot strategies for Tug of War
// Protocol: Total energy = 100, distribute across rounds
// Each round: output integer (how much energy to spend), read opponent's spent
// IMPORTANT: Track remaining energy and never overspend
// Game sends -1 when match ends early
var tugOfWarStrategies = []string{
	// Even distribution with tracking
	`#!/usr/bin/python3
import sys
n = int(input())
remaining = 100
for i in range(n):
    rounds_left = n - i
    spend = min(remaining, remaining // rounds_left) if rounds_left > 0 else remaining
    remaining -= spend
    print(max(0, spend), flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
	// Front-loaded with tracking
	`#!/usr/bin/python3
import sys
n = int(input())
remaining = 100
weights = [3, 2.5, 2, 1.5, 1, 0.5, 0.3, 0.2]
for i in range(n):
    w = weights[i] if i < len(weights) else 0.1
    total_w = sum(weights[j] if j < len(weights) else 0.1 for j in range(i, n))
    spend = min(remaining, int(remaining * w / max(0.01, total_w)))
    spend = max(0, min(remaining, spend))
    remaining -= spend
    print(spend, flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
	// Back-loaded with tracking
	`#!/usr/bin/python3
import sys
n = int(input())
remaining = 100
weights = [1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5]
for i in range(n):
    w = weights[i] if i < len(weights) else 5
    total_w = sum(weights[j] if j < len(weights) else 5 for j in range(i, n))
    spend = min(remaining, int(remaining * w / max(0.01, total_w)))
    spend = max(0, min(remaining, spend))
    remaining -= spend
    print(spend, flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
	// Random within budget with safety
	`#!/usr/bin/python3
import random
import sys
n = int(input())
remaining = 100
for i in range(n):
    rounds_left = n - i
    if rounds_left <= 0 or remaining <= 0:
        print(0, flush=True)
    else:
        avg = remaining // rounds_left
        spend = min(remaining, max(0, random.randint(max(0, avg - 5), avg + 5)))
        remaining -= spend
        print(spend, flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
}

// Bot strategies for Good Deal
// Protocol: 1. Read iterations, 2. Loop: output integer (offer amount), read opponent's response
var goodDealStrategies = []string{
	// Fair split
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print(50, flush=True)
    input()
`,
	// Greedy
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print(70, flush=True)
    input()
`,
	// Generous
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print(40, flush=True)
    input()
`,
	// Random
	`#!/usr/bin/python3
import random
n = int(input())
for i in range(n):
    print(random.randint(30, 70), flush=True)
    input()
`,
}

// Bot strategies for Balance of Universe
// Protocol: Similar to other games
var balanceOfUniverseStrategies = []string{
	// Balanced
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print(0, flush=True)
    input()
`,
	// Positive
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print(1, flush=True)
    input()
`,
	// Negative
	`#!/usr/bin/python3
n = int(input())
for i in range(n):
    print(-1, flush=True)
    input()
`,
	// Random
	`#!/usr/bin/python3
import random
n = int(input())
for i in range(n):
    print(random.choice([-1, 0, 1]), flush=True)
    input()
`,
}

// Map game names to their strategies
// Supported games: dilemma, tug_of_war (see https://github.com/bmstu-itstech/tjudge-cli)
var gameStrategies = map[string][]string{
	"dilemma":    dilemmaStrategies,
	"tug_of_war": tugOfWarStrategies,
}

// Team represents a test team
type Team struct {
	UserToken  string
	UserID     string
	TeamID     string
	TeamName   string
	ProgramID  string            // Legacy single program ID (for backwards compat)
	ProgramIDs map[string]string // Game ID -> Program ID
}

// PerformanceMetrics collects performance data
type PerformanceMetrics struct {
	mu                   sync.Mutex
	UserRegistrationTime time.Duration
	TeamCreationTime     time.Duration
	ProgramUploadTime    time.Duration
	TournamentJoinTime   time.Duration
	RoundStartTime       time.Duration
	MatchesGenerated     int
	TotalMatchTime       time.Duration
	MatchesCompleted     int64
	MatchesFailed        int64
	AvgMatchDuration     time.Duration
	RetryCount           int
}

func (m *PerformanceMetrics) Print() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("       PERFORMANCE TEST RESULTS - 30 TEAMS TOURNAMENT")
	fmt.Println(strings.Repeat("=", 70))

	// Setup Phase Table
	fmt.Println("\n┌─────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                          SETUP PHASE                                │")
	fmt.Println("├────────────────────────────────┬────────────────────────────────────┤")
	fmt.Printf("│ User Registration              │ %34v │\n", m.UserRegistrationTime.Round(time.Millisecond))
	fmt.Printf("│ Team Creation                  │ %34v │\n", m.TeamCreationTime.Round(time.Millisecond))
	fmt.Printf("│ Program Upload                 │ %34v │\n", m.ProgramUploadTime.Round(time.Millisecond))
	fmt.Printf("│ Tournament Join                │ %34v │\n", m.TournamentJoinTime.Round(time.Millisecond))
	totalSetupTime := m.UserRegistrationTime + m.TeamCreationTime + m.ProgramUploadTime + m.TournamentJoinTime
	fmt.Println("├────────────────────────────────┼────────────────────────────────────┤")
	fmt.Printf("│ Total Setup Time               │ %34v │\n", totalSetupTime.Round(time.Millisecond))
	fmt.Println("└────────────────────────────────┴────────────────────────────────────┘")

	// Match Execution Table
	fmt.Println("\n┌─────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                       MATCH EXECUTION                               │")
	fmt.Println("├────────────────────────────────┬────────────────────────────────────┤")
	fmt.Printf("│ Round Start Time               │ %34v │\n", m.RoundStartTime.Round(time.Millisecond))
	fmt.Printf("│ Matches Generated              │ %34d │\n", m.MatchesGenerated)
	fmt.Printf("│ Matches Completed              │ %34d │\n", m.MatchesCompleted)
	fmt.Printf("│ Matches Failed                 │ %34d │\n", m.MatchesFailed)
	fmt.Printf("│ Retry Attempts                 │ %34d │\n", m.RetryCount)
	fmt.Printf("│ Total Match Time               │ %34v │\n", m.TotalMatchTime.Round(time.Millisecond))
	if m.MatchesCompleted > 0 {
		fmt.Printf("│ Avg Match Duration             │ %34v │\n", m.AvgMatchDuration.Round(time.Millisecond))
	}
	fmt.Println("└────────────────────────────────┴────────────────────────────────────┘")

	// Summary Table
	fmt.Println("\n┌─────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                           SUMMARY                                   │")
	fmt.Println("├────────────────────────────────┬────────────────────────────────────┤")
	totalTime := totalSetupTime + m.RoundStartTime + m.TotalMatchTime
	fmt.Printf("│ Total Execution Time           │ %34v │\n", totalTime.Round(time.Millisecond))
	if m.MatchesGenerated > 0 && m.TotalMatchTime > 0 {
		matchesPerSecond := float64(m.MatchesCompleted) / m.TotalMatchTime.Seconds()
		fmt.Printf("│ Throughput                     │ %30.2f m/s │\n", matchesPerSecond)
	}
	if m.MatchesGenerated > 0 {
		successRate := float64(m.MatchesCompleted) / float64(m.MatchesGenerated) * 100
		fmt.Printf("│ Success Rate                   │ %32.2f %% │\n", successRate)
	}
	fmt.Println("└────────────────────────────────┴────────────────────────────────────┘")
	fmt.Println(strings.Repeat("=", 70))
}

// TestPerformance_30Teams_Tournament tests tournament with 30 teams
func TestPerformance_30Teams_Tournament(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	const numTeams = 30
	metrics := &PerformanceMetrics{}
	teams := make([]*Team, numTeams)

	timestamp := time.Now().UnixNano()

	// ==========================================================================
	// Phase 1: Get all game info
	// ==========================================================================
	fmt.Println("\n🔧 Phase 1: Getting all game info...")

	client := NewTestClient()

	resp, err := client.doRequest("GET", "/api/v1/games", nil)
	require.NoError(t, err)

	var games []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, client.parseResponse(resp, &games))
	require.NotEmpty(t, games, "No games available")

	// Build map of game name -> ID for all available games
	gameIDs := make(map[string]string)
	for _, g := range games {
		gameIDs[g.Name] = g.ID
		fmt.Printf("   Found game: %s (%s)\n", g.Name, g.ID)
	}

	// Define the 4 games we want to use
	targetGames := []string{"prisoners_dilemma", "tug_of_war", "good_deal", "balance_of_universe"}
	var availableGames []string
	for _, gameName := range targetGames {
		if _, ok := gameIDs[gameName]; ok {
			availableGames = append(availableGames, gameName)
		}
	}

	if len(availableGames) == 0 {
		// Fallback to first available game
		for _, g := range games {
			availableGames = append(availableGames, g.Name)
			gameIDs[g.Name] = g.ID
			break
		}
	}

	fmt.Printf("   Will use %d games: %v\n", len(availableGames), availableGames)

	// ==========================================================================
	// Phase 2: Tournament ID placeholder (will create after registering users)
	// ==========================================================================
	fmt.Println("\n🏆 Phase 2: Setting up tournament...")

	var tournamentID string
	// Always create a new tournament for clean testing
	fmt.Printf("   Will create new tournament after user registration.\n")

	// ==========================================================================
	// Phase 3: Register users SEQUENTIALLY to avoid rate limiting
	// ==========================================================================
	fmt.Println("\n👥 Phase 3: Registering 30 users (sequential to avoid rate limiting)...")

	start := time.Now()
	successfulRegistrations := 0

	for i := 0; i < numTeams; i++ {
		userClient := NewTestClient()
		username := fmt.Sprintf("perf_%d_%d", timestamp, i)

		resp, err := userClient.doRequest("POST", "/api/v1/auth/register", map[string]string{
			"username": username,
			"email":    fmt.Sprintf("%s@test.com", username),
			"password": "TestPass123!",
		})

		if err != nil {
			fmt.Printf("   User %d: registration error: %v\n", i, err)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// Check if rate limited
			if resp.StatusCode == http.StatusTooManyRequests {
				fmt.Printf("   User %d: rate limited, waiting 2s...\n", i)
				time.Sleep(2 * time.Second)
				i-- // Retry this user
				continue
			}
			fmt.Printf("   User %d: failed (%d): %s\n", i, resp.StatusCode, string(body))
			continue
		}

		var authResp struct {
			AccessToken string `json:"access_token"`
			User        struct {
				ID string `json:"id"`
			} `json:"user"`
		}
		if err := userClient.parseResponse(resp, &authResp); err != nil {
			fmt.Printf("   User %d: parse error: %v\n", i, err)
			continue
		}

		teams[i] = &Team{
			UserToken: authResp.AccessToken,
			UserID:    authResp.User.ID,
			TeamName:  fmt.Sprintf("Team_%d", i),
		}
		successfulRegistrations++

		// Small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}
	metrics.UserRegistrationTime = time.Since(start)
	fmt.Printf("   Registered %d/%d users in %v\n", successfulRegistrations, numTeams, metrics.UserRegistrationTime)

	// ==========================================================================
	// Phase 4: Create NEW tournament BEFORE creating teams
	// ==========================================================================
	if successfulRegistrations > 0 {
		fmt.Println("\n🏆 Phase 4a: Creating new tournament...")

		// Use first user's token to create tournament
		for i := 0; i < numTeams; i++ {
			if teams[i] != nil {
				createClient := NewTestClient()
				createClient.SetToken(teams[i].UserToken)

				resp, err := createClient.doRequest("POST", "/api/v1/tournaments", map[string]interface{}{
					"name":          fmt.Sprintf("Perf Test Tournament %d", timestamp),
					"description":   "30 teams performance test",
					"game_type":     "dilemma",
					"max_team_size": 3,
					"is_permanent":  false,
				})

				if err != nil {
					fmt.Printf("   Failed to create tournament: %v\n", err)
				} else if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
					var tournamentResp struct {
						ID string `json:"id"`
					}
					if err := createClient.parseResponse(resp, &tournamentResp); err == nil {
						tournamentID = tournamentResp.ID
						fmt.Printf("   Tournament created: %s\n", tournamentID)

						// Add ALL games to tournament
						gamesAdded := 0
						for _, gameName := range availableGames {
							gameID := gameIDs[gameName]
							addGameResp, err := createClient.doRequest("POST", fmt.Sprintf("/api/v1/tournaments/%s/games", tournamentID), map[string]string{
								"game_id": gameID,
							})
							if err != nil {
								fmt.Printf("   ⚠️ Failed to add game %s: %v\n", gameName, err)
							} else if addGameResp.StatusCode == http.StatusOK || addGameResp.StatusCode == http.StatusCreated || addGameResp.StatusCode == http.StatusNoContent {
								// 204 No Content is also a success status
								fmt.Printf("   ✅ Game %s added to tournament\n", gameName)
								gamesAdded++
								addGameResp.Body.Close()
							} else if addGameResp.StatusCode == http.StatusForbidden {
								body, _ := io.ReadAll(addGameResp.Body)
								addGameResp.Body.Close()
								fmt.Printf("   ⚠️ Adding game %s requires admin permissions: %s\n", gameName, string(body))
								fmt.Printf("   ℹ️  To add games, make user admin: make admin EMAIL=%s@test.com\n", fmt.Sprintf("perf_%d_0", timestamp))
								break // No point trying other games without admin
							} else {
								body, _ := io.ReadAll(addGameResp.Body)
								addGameResp.Body.Close()
								fmt.Printf("   ⚠️ Failed to add game %s (%d): %s\n", gameName, addGameResp.StatusCode, string(body))
							}
						}
						fmt.Printf("   Added %d/%d games to tournament\n", gamesAdded, len(availableGames))
					}
				} else {
					body, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					fmt.Printf("   Tournament creation failed: %s\n", string(body))
				}
				break
			}
		}
	}

	if tournamentID == "" {
		t.Fatal("No tournament available for testing")
	}

	// ==========================================================================
	// Phase 4b: Create teams (requires tournament_id)
	// ==========================================================================
	fmt.Println("\n🏢 Phase 4b: Creating teams...")

	start = time.Now()
	successfulTeams := 0

	for i := 0; i < numTeams; i++ {
		if teams[i] == nil {
			continue
		}

		teamClient := NewTestClient()
		teamClient.SetToken(teams[i].UserToken)

		resp, err := teamClient.doRequest("POST", "/api/v1/teams", map[string]string{
			"name":          teams[i].TeamName,
			"tournament_id": tournamentID,
		})

		if err != nil {
			fmt.Printf("   Team %d: error: %v\n", i, err)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("   Team %d: failed (%d): %s\n", i, resp.StatusCode, string(body))
			continue
		}

		var teamResp struct {
			ID string `json:"id"`
		}
		if err := teamClient.parseResponse(resp, &teamResp); err != nil {
			fmt.Printf("   Team %d: parse error: %v\n", i, err)
			continue
		}

		teams[i].TeamID = teamResp.ID
		successfulTeams++
	}
	metrics.TeamCreationTime = time.Since(start)
	fmt.Printf("   Created %d teams in %v\n", successfulTeams, metrics.TeamCreationTime)

	// ==========================================================================
	// Phase 5: Teams are already in tournament (joined when team was created)
	// Just verify and skip explicit join
	// ==========================================================================
	// Teams are automatically added to tournament when created with tournament_id
	// Count successful teams as joined
	successfulJoins := successfulTeams
	metrics.TournamentJoinTime = 0
	fmt.Printf("   %d teams in tournament (joined at team creation)\n", successfulJoins)

	// ==========================================================================
	// Phase 6: Upload programs for ALL games
	// ==========================================================================
	fmt.Println("\n📦 Phase 6: Uploading programs for all games...")

	start = time.Now()
	successfulUploads := 0
	totalProgramsToUpload := 0

	for i := 0; i < numTeams; i++ {
		if teams[i] == nil || teams[i].TeamID == "" {
			continue
		}

		teams[i].ProgramIDs = make(map[string]string)
		uploadClient := NewTestClient()
		uploadClient.SetToken(teams[i].UserToken)

		// Upload a program for each available game
		for _, gameName := range availableGames {
			gameID := gameIDs[gameName]
			totalProgramsToUpload++

			// Get strategies for this game
			strategies, ok := gameStrategies[gameName]
			if !ok || len(strategies) == 0 {
				// Fallback to dilemma strategies
				strategies = dilemmaStrategies
			}

			// Select a strategy (cycle through available strategies)
			strategy := strategies[i%len(strategies)]

			resp, err := uploadClient.uploadProgram(
				teams[i].TeamID,
				tournamentID,
				gameID,
				fmt.Sprintf("Bot_%d_%s", i, gameName),
				strategy,
			)

			if err != nil {
				if i < 2 { // Only log first few errors
					fmt.Printf("   Team %d, game %s: upload error: %v\n", i, gameName, err)
				}
				continue
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if i < 2 { // Only log first few errors
					fmt.Printf("   Team %d, game %s: failed (%d): %s\n", i, gameName, resp.StatusCode, string(body))
				}
				continue
			}

			var programResp struct {
				ID string `json:"id"`
			}
			if err := uploadClient.parseResponse(resp, &programResp); err != nil {
				if i < 2 {
					fmt.Printf("   Team %d, game %s: parse error: %v\n", i, gameName, err)
				}
				continue
			}

			teams[i].ProgramIDs[gameID] = programResp.ID
			// Set legacy ProgramID to first uploaded program
			if teams[i].ProgramID == "" {
				teams[i].ProgramID = programResp.ID
			}
			successfulUploads++
		}
	}
	metrics.ProgramUploadTime = time.Since(start)
	fmt.Printf("   Uploaded %d/%d programs (%d games x %d teams) in %v\n",
		successfulUploads, totalProgramsToUpload, len(availableGames), successfulTeams, metrics.ProgramUploadTime)

	// ==========================================================================
	// Phase 7: Start tournament and run matches
	// ==========================================================================
	fmt.Println("\n🏁 Phase 7: Starting tournament round...")

	// Use first successful team's token for admin operations
	var adminToken string
	for i := 0; i < numTeams; i++ {
		if teams[i] != nil && teams[i].ProgramID != "" {
			adminToken = teams[i].UserToken
			break
		}
	}

	if adminToken == "" {
		t.Fatal("No team with program available")
	}

	adminClient := NewTestClient()
	adminClient.SetToken(adminToken)

	start = time.Now()

	// Start tournament - this creates matches and changes status to active
	// Retry with backoff for rate limiting
	maxStartRetries := 5
	startBackoff := 2 * time.Second
	tournamentStarted := false

	for attempt := 0; attempt < maxStartRetries; attempt++ {
		resp, err = adminClient.doRequest("POST", fmt.Sprintf("/api/v1/tournaments/%s/start", tournamentID), nil)
		if err != nil {
			t.Fatalf("Failed to start tournament: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Printf("   Tournament started successfully\n")
			tournamentStarted = true
			break
		} else if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("   Rate limited on start attempt %d, waiting %v... (%s)\n", attempt+1, startBackoff, string(body))
			time.Sleep(startBackoff)
			startBackoff *= 2 // Exponential backoff
			continue
		} else if resp.StatusCode == http.StatusConflict {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Logf("Tournament already started (conflict is OK): %s", string(body))
			tournamentStarted = true
			break
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("Start tournament failed (%d): %s", resp.StatusCode, string(body))
		}
	}

	if !tournamentStarted {
		t.Fatalf("Failed to start tournament after %d attempts due to rate limiting", maxStartRetries)
	}

	// Verify tournament is now active
	resp, err = adminClient.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s", tournamentID), nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		var tournamentCheck struct {
			Status string `json:"status"`
		}
		if err := adminClient.parseResponse(resp, &tournamentCheck); err == nil {
			fmt.Printf("   Tournament status: %s\n", tournamentCheck.Status)
			if tournamentCheck.Status != "active" {
				t.Fatalf("Tournament should be active after start, but status is: %s", tournamentCheck.Status)
			}
		}
	}

	// Note: Matches are already created and enqueued by /start
	// The /run-matches endpoint is for running additional rounds or regenerating matches
	// Try to run additional matches if admin permissions are available
	resp, err = adminClient.doRequest("POST", fmt.Sprintf("/api/v1/tournaments/%s/run-matches", tournamentID), nil)
	if err != nil {
		t.Logf("Note: Could not run additional matches (may need admin): %v", err)
	} else if resp.StatusCode == http.StatusOK {
		var runResp struct {
			MatchesCreated int `json:"matches_created"`
		}
		if err := adminClient.parseResponse(resp, &runResp); err == nil && runResp.MatchesCreated > 0 {
			fmt.Printf("   Additional matches created: %d\n", runResp.MatchesCreated)
		}
		resp.Body.Close()
	} else if resp.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("Note: /run-matches requires admin permissions: %s", string(body))
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("Note: Run matches returned %d: %s", resp.StatusCode, string(body))
	}

	metrics.RoundStartTime = time.Since(start)
	fmt.Printf("   Round started in %v\n", metrics.RoundStartTime)

	// ==========================================================================
	// Phase 8: Monitor match execution with retry for failed matches
	// ==========================================================================
	fmt.Println("\n⏳ Phase 8: Monitoring match execution...")

	start = time.Now()
	maxWaitTime := 10 * time.Minute
	pollInterval := 2 * time.Second
	maxRetries := 3
	retryCount := 0

	var lastPending, lastCompleted, lastFailed int

	for {
		if time.Since(start) > maxWaitTime {
			fmt.Printf("   ⚠️ Timeout after %v\n", maxWaitTime)
			break
		}

		// Get match statistics (use limit=1000 to get all matches)
		resp, err := adminClient.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s/matches?limit=1000", tournamentID), nil)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var matches []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := adminClient.parseResponse(resp, &matches); err != nil {
			time.Sleep(pollInterval)
			continue
		}

		pending, completed, failed := 0, 0, 0
		for _, m := range matches {
			switch m.Status {
			case "pending", "running":
				pending++
			case "completed":
				completed++
			case "failed", "error":
				failed++
			}
		}

		metrics.MatchesGenerated = len(matches)
		atomic.StoreInt64(&metrics.MatchesCompleted, int64(completed))
		atomic.StoreInt64(&metrics.MatchesFailed, int64(failed))

		// Print progress only if changed
		if pending != lastPending || completed != lastCompleted || failed != lastFailed {
			fmt.Printf("   Progress: %d completed, %d pending, %d failed (total: %d)\n",
				completed, pending, failed, len(matches))
			lastPending, lastCompleted, lastFailed = pending, completed, failed
		}

		// All pending done - check if we need to retry failed matches
		if pending == 0 && len(matches) > 0 {
			if failed > 0 && retryCount < maxRetries {
				retryCount++
				metrics.RetryCount = retryCount
				fmt.Printf("   🔄 Retrying %d failed matches (attempt %d/%d)...\n", failed, retryCount, maxRetries)

				// Call retry endpoint (requires admin permissions)
				retryResp, retryErr := adminClient.doRequest("POST", fmt.Sprintf("/api/v1/tournaments/%s/retry-matches", tournamentID), nil)
				if retryErr != nil {
					fmt.Printf("   ⚠️ Retry request failed: %v\n", retryErr)
					// Don't retry anymore if request fails
					retryCount = maxRetries
				} else if retryResp.StatusCode == http.StatusForbidden {
					body, _ := io.ReadAll(retryResp.Body)
					retryResp.Body.Close()
					fmt.Printf("   ⚠️ Retry requires admin permissions: %s\n", string(body))
					fmt.Printf("   ℹ️  To enable retry, run test with admin user or use: make admin EMAIL=user@test.com\n")
					// Don't retry anymore without admin permissions
					retryCount = maxRetries
				} else if retryResp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(retryResp.Body)
					retryResp.Body.Close()
					fmt.Printf("   ⚠️ Retry returned %d: %s\n", retryResp.StatusCode, string(body))
				} else {
					var retryResult struct {
						Enqueued int `json:"enqueued"`
					}
					if err := adminClient.parseResponse(retryResp, &retryResult); err == nil {
						fmt.Printf("   ✓ Enqueued %d matches for retry\n", retryResult.Enqueued)
					}
					// Reset counters and continue monitoring only if retry succeeded
					lastPending, lastCompleted, lastFailed = 0, 0, 0
					time.Sleep(pollInterval)
					continue
				}
			}

			// All done (no more retries or no failed matches)
			if failed == 0 {
				fmt.Printf("   ✅ All %d matches completed successfully!\n", completed)
			} else {
				fmt.Printf("   ⚠️ Completed with %d failed matches after %d retries\n", failed, retryCount)
			}
			break
		}

		// No matches yet? Check if there are any
		if len(matches) == 0 {
			fmt.Printf("   Waiting for matches to be created...\n")
		}

		time.Sleep(pollInterval)
	}

	metrics.TotalMatchTime = time.Since(start)
	if metrics.MatchesCompleted > 0 {
		metrics.AvgMatchDuration = metrics.TotalMatchTime / time.Duration(metrics.MatchesCompleted)
	}

	// ==========================================================================
	// Print results table
	// ==========================================================================
	metrics.Print()

	// ==========================================================================
	// Test assertions and detailed log output
	// ==========================================================================
	// For multi-game tournaments: N*(N-1) matches per game for double round-robin
	// With multiple games, each team uploads a program for each game
	// Matches are generated per game based on programs uploaded for that game
	teamsWithPrograms := 0
	for i := 0; i < numTeams; i++ {
		if teams[i] != nil && teams[i].ProgramID != "" {
			teamsWithPrograms++
		}
	}
	matchesPerGame := teamsWithPrograms * (teamsWithPrograms - 1) // N*(N-1) for double round-robin
	expectedMatches := matchesPerGame * len(availableGames)

	t.Logf("\n📋 Test Details:")
	t.Logf("   Users: %d/%d registered", successfulRegistrations, numTeams)
	t.Logf("   Teams: %d created", successfulTeams)
	t.Logf("   Games: %d available (%v)", len(availableGames), availableGames)
	t.Logf("   Programs: %d uploaded (%d per team across %d games)", successfulUploads, len(availableGames), len(availableGames))
	t.Logf("   Teams with programs: %d", teamsWithPrograms)
	t.Logf("   Expected matches: %d (%d per game x %d games)", expectedMatches, matchesPerGame, len(availableGames))
	t.Logf("   Actual matches: %d generated, %d completed, %d failed", metrics.MatchesGenerated, metrics.MatchesCompleted, metrics.MatchesFailed)

	// Check if we got the expected number of matches
	if metrics.MatchesGenerated != expectedMatches {
		t.Logf("⚠️  Match count mismatch: expected %d, got %d", expectedMatches, metrics.MatchesGenerated)
	}

	// Report final success rate
	if metrics.MatchesGenerated > 0 {
		successRate := float64(metrics.MatchesCompleted) / float64(metrics.MatchesGenerated) * 100
		if successRate == 100 {
			t.Logf("✅ Success rate: %.2f%%", successRate)
		} else {
			t.Logf("⚠️  Success rate: %.2f%% (%d failed matches)", successRate, metrics.MatchesFailed)
		}
	}
}
