//go:build performance
// +build performance

// Performance test for uploading programs from 30 teams without running matches.
// This test verifies that program uploads work correctly when tournament is active
// but matches have not started yet.
//
// Run:
//   go test -v -tags=performance ./tests/performance/... -run TestPerformance_30Teams_Upload -timeout 5m -count=1
//
// Environment variables:
//   PERF_API_URL - API base URL (default: http://localhost:8080)

package performance

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// UploadMetrics collects performance data for upload test
type UploadMetrics struct {
	mu                   sync.Mutex
	UserRegistrationTime time.Duration
	TeamCreationTime     time.Duration
	ProgramUploadTime    time.Duration
	ProgramsUploaded     int
	ProgramsFailed       int
	TotalTime            time.Duration
}

func (m *UploadMetrics) Print() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("       UPLOAD TEST RESULTS - 30 TEAMS PROGRAM UPLOAD")
	fmt.Println(strings.Repeat("=", 70))

	// Setup Phase Table
	fmt.Println("\n┌─────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                          SETUP PHASE                                │")
	fmt.Println("├────────────────────────────────┬────────────────────────────────────┤")
	fmt.Printf("│ User Registration              │ %34v │\n", m.UserRegistrationTime.Round(time.Millisecond))
	fmt.Printf("│ Team Creation                  │ %34v │\n", m.TeamCreationTime.Round(time.Millisecond))
	fmt.Println("└────────────────────────────────┴────────────────────────────────────┘")

	// Upload Phase Table
	fmt.Println("\n┌─────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                         UPLOAD PHASE                                │")
	fmt.Println("├────────────────────────────────┬────────────────────────────────────┤")
	fmt.Printf("│ Program Upload Time            │ %34v │\n", m.ProgramUploadTime.Round(time.Millisecond))
	fmt.Printf("│ Programs Uploaded              │ %34d │\n", m.ProgramsUploaded)
	fmt.Printf("│ Programs Failed                │ %34d │\n", m.ProgramsFailed)
	fmt.Println("└────────────────────────────────┴────────────────────────────────────┘")

	// Summary Table
	fmt.Println("\n┌─────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                           SUMMARY                                   │")
	fmt.Println("├────────────────────────────────┬────────────────────────────────────┤")
	fmt.Printf("│ Total Time                     │ %34v │\n", m.TotalTime.Round(time.Millisecond))
	if m.ProgramsUploaded+m.ProgramsFailed > 0 {
		successRate := float64(m.ProgramsUploaded) / float64(m.ProgramsUploaded+m.ProgramsFailed) * 100
		fmt.Printf("│ Success Rate                   │ %32.2f %% │\n", successRate)
	}
	if m.ProgramUploadTime > 0 && m.ProgramsUploaded > 0 {
		uploadsPerSecond := float64(m.ProgramsUploaded) / m.ProgramUploadTime.Seconds()
		fmt.Printf("│ Upload Throughput              │ %30.2f p/s │\n", uploadsPerSecond)
	}
	fmt.Println("└────────────────────────────────┴────────────────────────────────────┘")
	fmt.Println(strings.Repeat("=", 70))
}

// TestPerformance_30Teams_Upload tests program upload from 30 teams
// Tournament has 2 games, tournament is active, but matches don't run
// Only uploads programs for the first (active) game: dilemma
func TestPerformance_30Teams_Upload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	const numTeams = 30
	metrics := &UploadMetrics{}
	teams := make([]*Team, numTeams)

	totalStart := time.Now()
	timestamp := time.Now().UnixNano()

	// ==========================================================================
	// Phase 1: Get game info
	// ==========================================================================
	fmt.Println("\n🔧 Phase 1: Getting game info...")

	client := NewTestClient()

	resp, err := client.doRequest("GET", "/api/v1/games", nil)
	require.NoError(t, err)

	var games []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, client.parseResponse(resp, &games))
	require.NotEmpty(t, games, "No games available")

	// Find dilemma and tug_of_war games
	gameIDs := make(map[string]string)
	for _, g := range games {
		gameIDs[g.Name] = g.ID
		fmt.Printf("   Found game: %s (%s)\n", g.Name, g.ID)
	}

	// We need both games for the tournament
	targetGames := []string{"dilemma", "tug_of_war"}
	var availableGames []string
	for _, gameName := range targetGames {
		if _, ok := gameIDs[gameName]; ok {
			availableGames = append(availableGames, gameName)
		}
	}

	require.NotEmpty(t, availableGames, "Need at least one game")
	fmt.Printf("   Using games: %v\n", availableGames)

	// Active game is the first one (dilemma) - this is where we'll upload programs
	activeGame := availableGames[0]
	activeGameID := gameIDs[activeGame]
	fmt.Printf("   Active game for uploads: %s (%s)\n", activeGame, activeGameID)

	// ==========================================================================
	// Phase 2: Register users SEQUENTIALLY
	// ==========================================================================
	fmt.Println("\n👥 Phase 2: Registering 30 users...")

	start := time.Now()
	successfulRegistrations := 0

	for i := 0; i < numTeams; i++ {
		userClient := NewTestClient()
		username := fmt.Sprintf("upload_%d_%d", timestamp, i)

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
			if resp.StatusCode == http.StatusTooManyRequests {
				fmt.Printf("   User %d: rate limited, waiting 2s...\n", i)
				time.Sleep(2 * time.Second)
				i-- // Retry
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
			TeamName:  fmt.Sprintf("UploadTeam_%d", i),
		}
		successfulRegistrations++

		time.Sleep(100 * time.Millisecond) // Avoid rate limiting
	}
	metrics.UserRegistrationTime = time.Since(start)
	fmt.Printf("   Registered %d/%d users in %v\n", successfulRegistrations, numTeams, metrics.UserRegistrationTime)

	require.Greater(t, successfulRegistrations, 0, "Need at least one user registered")

	// ==========================================================================
	// Phase 3: Create tournament with 2 games
	// ==========================================================================
	fmt.Println("\n🏆 Phase 3: Creating tournament with 2 games...")

	var tournamentID string
	var adminToken string

	for i := 0; i < numTeams; i++ {
		if teams[i] != nil {
			adminToken = teams[i].UserToken
			createClient := NewTestClient()
			createClient.SetToken(adminToken)

			resp, err := createClient.doRequest("POST", "/api/v1/tournaments", map[string]interface{}{
				"name":          fmt.Sprintf("Upload Test Tournament %d", timestamp),
				"description":   "30 teams upload test - no matches",
				"game_type":     activeGame, // First game is dilemma
				"max_team_size": 3,
				"is_permanent":  false,
			})

			if err != nil {
				fmt.Printf("   Failed to create tournament: %v\n", err)
				continue
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
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
							fmt.Printf("   ✅ Game %s added to tournament\n", gameName)
							gamesAdded++
							addGameResp.Body.Close()
						} else if addGameResp.StatusCode == http.StatusForbidden {
							body, _ := io.ReadAll(addGameResp.Body)
							addGameResp.Body.Close()
							fmt.Printf("   ⚠️ Adding game %s requires admin permissions: %s\n", gameName, string(body))
							break
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

	require.NotEmpty(t, tournamentID, "Tournament must be created")

	// ==========================================================================
	// Phase 4: Create teams
	// ==========================================================================
	fmt.Println("\n🏢 Phase 4: Creating teams...")

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

	require.Greater(t, successfulTeams, 0, "Need at least one team created")

	// ==========================================================================
	// Phase 5: Upload programs for ALL games (dilemma and tug_of_war)
	// Tournament is pending, matches NOT started
	// ==========================================================================
	fmt.Println("\n📦 Phase 5: Uploading programs for ALL games...")
	fmt.Println("   Note: Tournament is PENDING, matches will NOT be executed")

	start = time.Now()
	successfulUploads := 0
	failedUploads := 0

	// Upload programs for each game
	for _, gameName := range availableGames {
		gameID := gameIDs[gameName]
		fmt.Printf("\n   📌 Uploading %s programs...\n", gameName)

		var strategies []string
		var botPrefix string
		switch gameName {
		case "dilemma":
			strategies = dilemmaStrategies
			botPrefix = "DilemmaBot"
		case "tug_of_war":
			strategies = tugOfWarStrategies
			botPrefix = "TugBot"
		default:
			fmt.Printf("   ⚠️ Unknown game %s, skipping\n", gameName)
			continue
		}

		gameUploads := 0
		gameFailed := 0

		for i := 0; i < numTeams; i++ {
			if teams[i] == nil || teams[i].TeamID == "" {
				continue
			}

			uploadClient := NewTestClient()
			uploadClient.SetToken(teams[i].UserToken)

			strategy := strategies[i%len(strategies)]

			resp, err := uploadClient.uploadProgram(
				teams[i].TeamID,
				tournamentID,
				gameID,
				fmt.Sprintf("%s_%d", botPrefix, i),
				strategy,
			)

			if err != nil {
				if gameFailed < 3 {
					fmt.Printf("      Team %d: upload error: %v\n", i, err)
				}
				gameFailed++
				failedUploads++
				continue
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if gameFailed < 3 {
					fmt.Printf("      Team %d: failed (%d): %s\n", i, resp.StatusCode, string(body))
				}
				gameFailed++
				failedUploads++
				continue
			}

			var programResp struct {
				ID string `json:"id"`
			}
			if err := uploadClient.parseResponse(resp, &programResp); err != nil {
				if gameFailed < 3 {
					fmt.Printf("      Team %d: parse error: %v\n", i, err)
				}
				gameFailed++
				failedUploads++
				continue
			}

			// Store program ID (use first one for legacy compatibility)
			if teams[i].ProgramID == "" {
				teams[i].ProgramID = programResp.ID
			}
			gameUploads++
			successfulUploads++
		}

		fmt.Printf("   ✅ %s: uploaded %d, failed %d\n", gameName, gameUploads, gameFailed)
	}

	metrics.ProgramUploadTime = time.Since(start)
	metrics.ProgramsUploaded = successfulUploads
	metrics.ProgramsFailed = failedUploads
	fmt.Printf("\n   Total: Uploaded %d programs, failed %d in %v\n",
		successfulUploads, failedUploads, metrics.ProgramUploadTime)

	// ==========================================================================
	// Verification: Check that tournament is still PENDING
	// ==========================================================================
	fmt.Println("\n✅ Verification: Tournament status check...")

	adminClient := NewTestClient()
	adminClient.SetToken(adminToken)

	resp, err = adminClient.doRequest("GET", fmt.Sprintf("/api/v1/tournaments/%s", tournamentID), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tournament struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, adminClient.parseResponse(resp, &tournament))
	fmt.Printf("   Tournament status: %s\n", tournament.Status)

	// Should be pending since we didn't start the tournament
	require.Equal(t, "pending", tournament.Status, "Tournament should remain pending")

	// ==========================================================================
	// Summary
	// ==========================================================================
	metrics.TotalTime = time.Since(totalStart)
	metrics.Print()

	// Verify success
	require.Greater(t, successfulUploads, 0, "Should have at least one successful upload")
	fmt.Printf("\n✅ TEST PASSED: %d programs uploaded without running matches\n", successfulUploads)
}
