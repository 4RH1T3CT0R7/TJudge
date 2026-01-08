//go:build performance
// +build performance

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

// Team represents a test team
type Team struct {
	UserToken string
	UserID    string
	TeamID    string
	TeamName  string
	ProgramID string
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
}

func (m *PerformanceMetrics) Print() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("   PERFORMANCE TEST RESULTS - 30 TEAMS TOURNAMENT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\n📊 Setup Phase:\n")
	fmt.Printf("   User Registration:     %v\n", m.UserRegistrationTime)
	fmt.Printf("   Team Creation:         %v\n", m.TeamCreationTime)
	fmt.Printf("   Program Upload:        %v\n", m.ProgramUploadTime)
	fmt.Printf("   Tournament Join:       %v\n", m.TournamentJoinTime)

	fmt.Printf("\n🏁 Tournament Phase:\n")
	fmt.Printf("   Round Start Time:      %v\n", m.RoundStartTime)
	fmt.Printf("   Matches Generated:     %d\n", m.MatchesGenerated)

	fmt.Printf("\n⏱️ Match Execution:\n")
	fmt.Printf("   Total Match Time:      %v\n", m.TotalMatchTime)
	fmt.Printf("   Matches Completed:     %d\n", m.MatchesCompleted)
	fmt.Printf("   Matches Failed:        %d\n", m.MatchesFailed)
	if m.MatchesCompleted > 0 {
		fmt.Printf("   Avg Match Duration:    %v\n", m.AvgMatchDuration)
	}

	totalSetupTime := m.UserRegistrationTime + m.TeamCreationTime + m.ProgramUploadTime + m.TournamentJoinTime
	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   Total Setup Time:      %v\n", totalSetupTime)
	fmt.Printf("   Total Execution Time:  %v\n", totalSetupTime+m.RoundStartTime+m.TotalMatchTime)

	// Expected matches for 30 teams: C(30,2) = 435 matches per game
	if m.MatchesGenerated > 0 && m.TotalMatchTime > 0 {
		matchesPerSecond := float64(m.MatchesCompleted) / m.TotalMatchTime.Seconds()
		fmt.Printf("   Matches/Second:        %.2f\n", matchesPerSecond)
	}

	fmt.Println(strings.Repeat("=", 60))
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
	// Phase 1: Get game info
	// ==========================================================================
	fmt.Println("\n🔧 Phase 1: Getting game info...")

	client := NewTestClient()
	var gameID string

	resp, err := client.doRequest("GET", "/api/v1/games", nil)
	require.NoError(t, err)

	var games []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, client.parseResponse(resp, &games))
	require.NotEmpty(t, games, "No games available")

	// Use dilemma game
	for _, g := range games {
		if g.Name == "dilemma" {
			gameID = g.ID
			break
		}
	}
	if gameID == "" {
		gameID = games[0].ID
	}
	fmt.Printf("   Using game: %s\n", gameID)

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
	// Phase 6: Upload programs
	// ==========================================================================
	fmt.Println("\n📦 Phase 6: Uploading programs...")

	start = time.Now()
	successfulUploads := 0

	for i := 0; i < numTeams; i++ {
		if teams[i] == nil || teams[i].TeamID == "" {
			continue
		}

		uploadClient := NewTestClient()
		uploadClient.SetToken(teams[i].UserToken)

		// Select a strategy (cycle through available strategies)
		strategy := dilemmaStrategies[i%len(dilemmaStrategies)]

		resp, err := uploadClient.uploadProgram(
			teams[i].TeamID,
			tournamentID,
			gameID,
			fmt.Sprintf("Bot_%d", i),
			strategy,
		)

		if err != nil {
			fmt.Printf("   Program %d: upload error: %v\n", i, err)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if i < 3 { // Only log first few errors
				fmt.Printf("   Program %d: failed (%d): %s\n", i, resp.StatusCode, string(body))
			}
			continue
		}

		var programResp struct {
			ID string `json:"id"`
		}
		if err := uploadClient.parseResponse(resp, &programResp); err != nil {
			fmt.Printf("   Program %d: parse error: %v\n", i, err)
			continue
		}

		teams[i].ProgramID = programResp.ID
		successfulUploads++
	}
	metrics.ProgramUploadTime = time.Since(start)
	fmt.Printf("   Uploaded %d programs in %v\n", successfulUploads, metrics.ProgramUploadTime)

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

	// Start tournament
	resp, err = adminClient.doRequest("POST", fmt.Sprintf("/api/v1/tournaments/%s/start", tournamentID), nil)
	if err != nil {
		t.Logf("Warning: Could not start tournament: %v", err)
	} else if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("Warning: Start tournament returned %d: %s", resp.StatusCode, string(body))
	}

	// Run all matches (requires admin - may fail without proper permissions)
	resp, err = adminClient.doRequest("POST", fmt.Sprintf("/api/v1/tournaments/%s/run-matches", tournamentID), nil)
	if err != nil {
		t.Logf("Note: Could not run matches (may need admin): %v", err)
	} else if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("Note: Run matches returned %d: %s (may need admin)", resp.StatusCode, string(body))
	} else {
		var runResp struct {
			MatchesCreated int `json:"matches_created"`
		}
		if err := adminClient.parseResponse(resp, &runResp); err == nil {
			fmt.Printf("   Matches created: %d\n", runResp.MatchesCreated)
		}
	}

	metrics.RoundStartTime = time.Since(start)
	fmt.Printf("   Round started in %v\n", metrics.RoundStartTime)

	// ==========================================================================
	// Phase 8: Monitor match execution
	// ==========================================================================
	fmt.Println("\n⏳ Phase 8: Monitoring match execution...")

	start = time.Now()
	maxWaitTime := 5 * time.Minute
	pollInterval := 2 * time.Second

	var lastPending, lastCompleted, lastFailed int

	for {
		if time.Since(start) > maxWaitTime {
			fmt.Printf("   ⚠️ Timeout after %v\n", maxWaitTime)
			break
		}

		// Get match statistics
		resp, err := adminClient.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s/matches", tournamentID), nil)
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

		// All done?
		if pending == 0 && len(matches) > 0 {
			fmt.Printf("   ✅ All matches completed!\n")
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
	// Print results
	// ==========================================================================
	metrics.Print()

	// ==========================================================================
	// Summary
	// ==========================================================================
	t.Logf("Registered users: %d/%d", successfulRegistrations, numTeams)
	t.Logf("Created teams: %d", successfulTeams)
	t.Logf("Joined tournament: %d", successfulJoins)
	t.Logf("Uploaded programs: %d", successfulUploads)
	t.Logf("Matches generated: %d", metrics.MatchesGenerated)
	t.Logf("Matches completed: %d", metrics.MatchesCompleted)
	t.Logf("Matches failed: %d", metrics.MatchesFailed)

	// Expected matches for N programs: C(N,2) = N*(N-1)/2
	expectedMatches := (successfulUploads * (successfulUploads - 1)) / 2
	t.Logf("Expected matches for %d programs: %d", successfulUploads, expectedMatches)

	if metrics.MatchesGenerated > 0 {
		successRate := float64(metrics.MatchesCompleted) / float64(metrics.MatchesGenerated) * 100
		t.Logf("Success rate: %.2f%%", successRate)
	}
}
