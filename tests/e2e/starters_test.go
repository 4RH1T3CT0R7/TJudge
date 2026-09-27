//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// шаблоны ботов со страницы игры: web/src/starters/<игра>/<язык>/main.<ext>
const startersDir = "../../web/src/starters"

// TestE2E_Starters: каждый шаблон собирается в tjudge-builder и проходит
// самопроверку - матч против эталонного бота игры в tjudge-cli. без прогона
// шаблоны на десяти языках незаметно протухают вместе с образами
func TestE2E_Starters(t *testing.T) {
	if os.Getenv("E2E_FULL_CYCLE") != "true" {
		t.Skip("шаблоны: задать E2E_FULL_CYCLE=true, нужны api, worker и образы песочниц")
	}

	files, err := filepath.Glob(filepath.Join(startersDir, "*", "*", "*"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "шаблоны не найдены в %s", startersDir)

	admin := NewTestClient()
	registerAndAuthAsAdmin(t, admin, "st_admin")
	tournamentID := createTournamentHelper(t, admin)

	gameIDs := map[string]string{}
	for _, f := range files {
		name := filepath.Base(filepath.Dir(filepath.Dir(f)))
		if gameIDs[name] != "" {
			continue
		}
		var game GameResponse
		getJSON(t, admin, "/api/v1/games/name/"+name, &game)
		resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/games", map[string]string{"game_id": game.ID})
		require.NoError(t, err)
		requireStatus(t, resp, http.StatusNoContent)
		gameIDs[name] = game.ID
	}

	participant := NewTestClient()
	registerAndAuth(t, participant, "st_user")
	team := createTeamHelper(t, participant, tournamentID)
	// турнир стартует минимум с двумя командами
	createTeamHelper(t, admin, tournamentID)

	resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/start", nil)
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusOK)

	// загрузка всех сразу: сборки и самопроверки идут параллельно ожиданию
	programs := map[string]string{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		game := filepath.Base(filepath.Dir(filepath.Dir(f)))
		programs[f] = uploadProgram(t, participant, tournamentID, gameIDs[game], team.ID, filepath.Base(f), string(src))
	}

	for _, f := range files {
		rel := strings.TrimPrefix(f, startersDir+string(filepath.Separator))
		t.Run(rel, func(t *testing.T) {
			waitProgramReady(t, participant, programs[f])
			waitProgramChecked(t, participant, programs[f])
		})
	}
}

// waitProgramChecked ждёт итога самопроверки собранной программы: нужен ok
func waitProgramChecked(t *testing.T, c *TestClient, programID string) {
	t.Helper()
	deadline := time.Now().Add(fullCycleTimeout)
	for time.Now().Before(deadline) {
		var p struct {
			CheckStatus  *string `json:"check_status"`
			CheckMessage *string `json:"check_message"`
		}
		getJSON(t, c, "/api/v1/programs/"+programID, &p)
		switch {
		case p.CheckStatus == nil:
			t.Fatalf("самопроверка программы %s не состоялась", programID)
		case *p.CheckStatus == "ok":
			return
		case *p.CheckStatus == "failed":
			msg := ""
			if p.CheckMessage != nil {
				msg = *p.CheckMessage
			}
			t.Fatalf("программа %s не прошла самопроверку: %s", programID, msg)
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("самопроверка программы %s не закончилась за %s", programID, fullCycleTimeout)
}
