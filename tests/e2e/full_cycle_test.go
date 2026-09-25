//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// полный цикл нужен рабочий worker, образы tjudge-builder и tjudge-cli,
// поэтому тест включается отдельно (nightly.yml)
const fullCycleTimeout = 5 * time.Minute

// C-программа проходит настоящую компиляцию в песочнице, python - только syntax-check
const defectorC = `#include <stdio.h>
int main(void) {
    int n;
    char buf[32];
    if (scanf("%d", &n) != 1) return 1;
    for (int i = 0; i < n; i++) {
        printf("DEFECT\n");
        fflush(stdout);
        if (scanf("%31s", buf) != 1) return 1;
    }
    return 0;
}
`

const cooperatorPy = `import sys
n = int(sys.stdin.readline())
for _ in range(n):
    print("COOPERATE", flush=True)
    sys.stdin.readline()
`

// TestE2E_FullCycle: загрузка -> компиляция в tjudge-builder -> матч в tjudge-cli ->
// лидерборд и ELO. предатель обыгрывает кооператора в обеих ориентациях
func TestE2E_FullCycle(t *testing.T) {
	if os.Getenv("E2E_FULL_CYCLE") != "true" {
		t.Skip("полный цикл: задать E2E_FULL_CYCLE=true, нужны api, worker и образы песочниц")
	}

	admin := NewTestClient()
	registerAndAuthAsAdmin(t, admin, "fc_admin")

	var game GameResponse
	getJSON(t, admin, "/api/v1/games/name/dilemma", &game)

	tournamentID := createTournamentHelper(t, admin)
	resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/games", map[string]string{"game_id": game.ID})
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusNoContent)

	defector, cooperator := NewTestClient(), NewTestClient()
	registerAndAuth(t, defector, "fc_defect")
	registerAndAuth(t, cooperator, "fc_coop")
	defectorTeam := createTeamHelper(t, defector, tournamentID)
	cooperatorTeam := createTeamHelper(t, cooperator, tournamentID)

	resp, err = admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/start", nil)
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusOK)

	defectorProg := uploadProgram(t, defector, tournamentID, game.ID, defectorTeam.ID, "defector.c", defectorC)
	cooperatorProg := uploadProgram(t, cooperator, tournamentID, game.ID, cooperatorTeam.ID, "cooperator.py", cooperatorPy)
	waitProgramReady(t, defector, defectorProg)
	waitProgramReady(t, cooperator, cooperatorProg)

	resp, err = admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/run-game-matches", map[string]string{"game_type": game.Name})
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusOK)

	// AB и BA
	waitMatchesCompleted(t, admin, tournamentID, 2)

	var leaderboard []struct {
		TeamID string `json:"team_id"`
		Wins   int    `json:"wins"`
		Losses int    `json:"losses"`
	}
	getJSON(t, admin, fmt.Sprintf("/api/v1/tournaments/%s/games/%s/leaderboard", tournamentID, game.ID), &leaderboard)
	require.Len(t, leaderboard, 2)
	require.Equal(t, defectorTeam.ID, leaderboard[0].TeamID, "предатель должен быть первым")
	require.Equal(t, 2, leaderboard[0].Wins)
	require.Equal(t, 2, leaderboard[1].Losses)

	// ELO пишется outbox'ом отдельно от результата матча, поэтому ожидание:
	// у каждой программы по записи на каждый из двух матчей
	var defectorHistory, cooperatorHistory []ratingPoint
	require.Eventually(t, func() bool {
		defectorHistory = ratingHistory(t, admin, tournamentID, defectorProg)
		cooperatorHistory = ratingHistory(t, admin, tournamentID, cooperatorProg)
		return len(defectorHistory) == 2 && len(cooperatorHistory) == 2
	}, time.Minute, 2*time.Second, "история рейтинга не появилась")
	require.Greater(t, defectorHistory[1].NewRating, cooperatorHistory[1].NewRating)
}

func requireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: ожидался %d, получен %d - %s", resp.Request.Method, resp.Request.URL.Path, want, resp.StatusCode, body)
	}
}

func getJSON(t *testing.T, c *TestClient, path string, v any) {
	t.Helper()
	resp, err := c.doRequest("GET", path, nil)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		requireStatus(t, resp, http.StatusOK)
	}
	require.NoError(t, c.parseResponse(resp, v))
}

func uploadProgram(t *testing.T, c *TestClient, tournamentID, gameID, teamID, filename, source string) string {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"team_id": teamID, "tournament_id": tournamentID, "game_id": gameID} {
		require.NoError(t, w.WriteField(k, v))
	}
	fw, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = io.WriteString(fw, source)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/programs", &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	resp, err := c.client.Do(req)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusCreated {
		requireStatus(t, resp, http.StatusCreated)
	}

	var prog ProgramResponse
	require.NoError(t, c.parseResponse(resp, &prog))
	require.NotEmpty(t, prog.ID)
	return prog.ID
}

func waitProgramReady(t *testing.T, c *TestClient, programID string) {
	t.Helper()
	deadline := time.Now().Add(fullCycleTimeout)
	for time.Now().Before(deadline) {
		var p struct {
			Status       string  `json:"status"`
			ErrorMessage *string `json:"error_message"`
		}
		getJSON(t, c, "/api/v1/programs/"+programID, &p)
		switch p.Status {
		case "ready":
			return
		case "failed":
			msg := ""
			if p.ErrorMessage != nil {
				msg = *p.ErrorMessage
			}
			t.Fatalf("программа %s не скомпилировалась: %s", programID, msg)
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("программа %s не стала ready за %s", programID, fullCycleTimeout)
}

func waitMatchesCompleted(t *testing.T, c *TestClient, tournamentID string, want int) {
	t.Helper()
	deadline := time.Now().Add(fullCycleTimeout)
	for time.Now().Before(deadline) {
		var matches []struct {
			ID           string  `json:"id"`
			Status       string  `json:"status"`
			ErrorMessage *string `json:"error_message"`
		}
		getJSON(t, c, "/api/v1/tournaments/"+tournamentID+"/matches", &matches)
		completed := 0
		for _, m := range matches {
			switch m.Status {
			case "completed":
				completed++
			case "failed":
				msg := ""
				if m.ErrorMessage != nil {
					msg = *m.ErrorMessage
				}
				t.Fatalf("матч %s упал: %s", m.ID, msg)
			}
		}
		if completed == want {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("за %s не завершились %d матча", fullCycleTimeout, want)
}

type ratingPoint struct {
	NewRating int `json:"new_rating"`
}

// ratingHistory - история рейтинга программы в хронологическом порядке
func ratingHistory(t *testing.T, c *TestClient, tournamentID, programID string) []ratingPoint {
	t.Helper()
	var history []ratingPoint
	getJSON(t, c, fmt.Sprintf("/api/v1/tournaments/%s/programs/%s/rating-history", tournamentID, programID), &history)
	return history
}
