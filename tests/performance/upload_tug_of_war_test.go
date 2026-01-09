//go:build performance
// +build performance

// Performance test for uploading tug_of_war programs for existing teams.
// This test logs in as existing users and uploads tug_of_war programs.
//
// Run:
//   go test -v -tags=performance ./tests/performance/... -run TestPerformance_UploadTugOfWar -timeout 5m -count=1
//
// Environment variables:
//   PERF_API_URL - API base URL (default: http://localhost:8080)
//   TOURNAMENT_ID - Existing tournament ID (optional, will be auto-detected)

package performance

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Tug of War strategies - programs output integer (energy to spend per round)
// Protocol:
// 1. Read m (number of rounds)
// 2. Each round: output integer (0-100), receive opponent's bid
// 3. If received -1 or EOF, round is over
// Player starts with 100 energy total
var tugOfWarProgramStrategies = []string{
	// Even distribution
	`#!/usr/bin/python3
import sys
m = int(input())
remaining = 100
for i in range(m):
    rounds_left = m - i
    spend = min(remaining, remaining // max(1, rounds_left))
    remaining -= spend
    print(max(0, spend), flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
	// Conservative (max 20 per round)
	`#!/usr/bin/python3
import sys
m = int(input())
remaining = 100
for i in range(m):
    spend = min(remaining, 20)
    remaining -= spend
    print(spend, flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
	// Aggressive early
	`#!/usr/bin/python3
import sys
m = int(input())
remaining = 100
for i in range(m):
    if remaining <= 0:
        print(0, flush=True)
    elif i < 3:
        spend = min(remaining, 25)
        remaining -= spend
        print(spend, flush=True)
    else:
        rounds_left = m - i
        spend = min(remaining, remaining // max(1, rounds_left))
        remaining -= spend
        print(spend, flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
	// Random within budget
	`#!/usr/bin/python3
import random
import sys
m = int(input())
remaining = 100
for i in range(m):
    if remaining <= 0:
        print(0, flush=True)
    else:
        rounds_left = m - i
        avg = remaining // max(1, rounds_left)
        spend = min(remaining, random.randint(max(0, avg - 5), min(remaining, avg + 5)))
        remaining -= spend
        print(spend, flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
	// Match opponent + 1
	`#!/usr/bin/python3
import sys
m = int(input())
remaining = 100
last_opp = 10
for i in range(m):
    spend = min(remaining, last_opp + 1)
    remaining -= spend
    print(spend, flush=True)
    try:
        line = input()
        if line.strip() == '':
            break
        opp = int(line)
        if opp < 0:
            break
        last_opp = opp
    except (ValueError, EOFError):
        break
`,
	// Fibonacci distribution
	`#!/usr/bin/python3
import sys
m = int(input())
remaining = 100
fib = [1, 1, 2, 3, 5, 8, 13, 21, 34, 55]
total_fib = sum(fib[:min(m, len(fib))])
for i in range(m):
    if remaining <= 0:
        print(0, flush=True)
    else:
        f = fib[i] if i < len(fib) else fib[-1]
        spend = min(remaining, int(100 * f / max(1, total_fib)))
        remaining -= spend
        print(max(1, spend), flush=True)
    try:
        line = input()
        if line.strip() == '' or int(line) < 0:
            break
    except (ValueError, EOFError):
        break
`,
}

// TestPerformance_UploadTugOfWar uploads tug_of_war programs for existing teams
func TestPerformance_UploadTugOfWar(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Get tournament ID from env or use default
	tournamentID := os.Getenv("TOURNAMENT_ID")
	if tournamentID == "" {
		tournamentID = "9e0baeb4-060a-4ffa-8382-2b8d18fa3b69"
	}

	client := NewTestClient()

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("       UPLOAD TUG OF WAR PROGRAMS FOR EXISTING TEAMS")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Tournament ID: %s\n", tournamentID)

	// ==========================================================================
	// Phase 1: Get game info
	// ==========================================================================
	fmt.Println("\n🔧 Phase 1: Getting tug_of_war game info...")

	resp, err := client.doRequest("GET", "/api/v1/games/name/tug_of_war", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "tug_of_war game should exist")

	var game struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, client.parseResponse(resp, &game))
	fmt.Printf("   Found game: %s (%s)\n", game.Name, game.ID)

	tugOfWarGameID := game.ID

	// ==========================================================================
	// Phase 2: Get existing teams
	// ==========================================================================
	fmt.Println("\n📋 Phase 2: Getting existing teams...")

	resp, err = client.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s/teams", tournamentID), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var teams []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, client.parseResponse(resp, &teams))
	fmt.Printf("   Found %d teams\n", len(teams))

	// ==========================================================================
	// Phase 3: Login as each user and upload programs
	// ==========================================================================
	fmt.Println("\n📦 Phase 3: Uploading tug_of_war programs...")
	fmt.Println("   Using password: TestPass123!")

	// Known timestamp from the previous test run (upload_30_teams_test.go)
	// Team names follow pattern UploadTeam_N, usernames follow upload_<timestamp>_N
	knownTimestamp := "1767999455040448000"

	successfulUploads := 0
	failedUploads := 0

	for i, team := range teams {
		// Extract team number from name (UploadTeam_N)
		var teamNum string
		if n, err := fmt.Sscanf(team.Name, "UploadTeam_%s", &teamNum); err != nil || n != 1 {
			fmt.Printf("   Team %s: cannot parse team name\n", team.Name)
			failedUploads++
			continue
		}

		// Derive username from team name
		username := fmt.Sprintf("upload_%s_%s", knownTimestamp, teamNum)
		userClient := NewTestClient()

		// Login
		resp, err := userClient.doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username": username,
			"password": "TestPass123!",
		})

		if err != nil {
			fmt.Printf("   Team %s: login error: %v\n", team.Name, err)
			failedUploads++
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("   Team %s: login failed (%d): %s\n", team.Name, resp.StatusCode, string(body))
			failedUploads++
			continue
		}

		var authResp struct {
			AccessToken string `json:"access_token"`
		}
		if err := userClient.parseResponse(resp, &authResp); err != nil {
			fmt.Printf("   Team %s: parse error: %v\n", team.Name, err)
			failedUploads++
			continue
		}

		userClient.SetToken(authResp.AccessToken)

		// Upload program
		strategy := tugOfWarProgramStrategies[i%len(tugOfWarProgramStrategies)]

		resp, err = userClient.uploadProgram(
			team.ID,
			tournamentID,
			tugOfWarGameID,
			fmt.Sprintf("TugBot_%s", teamNum),
			strategy,
		)

		if err != nil {
			fmt.Printf("   Team %s: upload error: %v\n", team.Name, err)
			failedUploads++
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("   Team %s: upload failed (%d): %s\n", team.Name, resp.StatusCode, string(body))
			failedUploads++
			continue
		}

		var programResp struct {
			ID string `json:"id"`
		}
		if err := userClient.parseResponse(resp, &programResp); err != nil {
			fmt.Printf("   Team %s: parse error: %v\n", team.Name, err)
			failedUploads++
			continue
		}

		successfulUploads++
		if i < 3 || (i+1)%10 == 0 {
			fmt.Printf("   Team %s: uploaded TugBot_%s\n", team.Name, teamNum)
		}

		time.Sleep(50 * time.Millisecond) // Small delay to avoid rate limiting
	}

	// ==========================================================================
	// Summary
	// ==========================================================================
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("                         SUMMARY")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Total teams:       %d\n", len(teams))
	fmt.Printf("Programs uploaded: %d\n", successfulUploads)
	fmt.Printf("Programs failed:   %d\n", failedUploads)
	fmt.Println(strings.Repeat("=", 70))

	require.Greater(t, successfulUploads, 0, "Should have at least one successful upload")
	fmt.Printf("\n✅ TEST PASSED: %d tug_of_war programs uploaded\n", successfulUploads)
}
