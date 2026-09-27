//go:build e2e

package e2e

import (
	"fmt"
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

// TestE2E_Starters: каждый шаблон собирается в tjudge-builder, проходит
// самопроверку (первым игроком против эталонного бота игры) и играет раунд
// с шаблонами на других языках первым и вторым игроком. без прогона шаблоны
// на десяти языках незаметно протухают вместе с образами
func TestE2E_Starters(t *testing.T) {
	if os.Getenv("E2E_FULL_CYCLE") != "true" {
		t.Skip("шаблоны: задать E2E_FULL_CYCLE=true, нужны api, worker и образы песочниц")
	}

	// [^.]: скрытые файлы (.DS_Store) не шаблоны
	files, err := filepath.Glob(filepath.Join(startersDir, "*", "*", "[^.]*"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "шаблоны не найдены в %s", startersDir)

	admin := NewTestClient()
	registerAndAuthAsAdmin(t, admin, "st_admin")
	tournamentID := createTournamentHelper(t, admin)

	// в раунде играет последняя готовая версия команды, поэтому у каждого
	// языка своя команда: иначе шаблоны одной команды вытеснили бы друг друга
	gameIDs := map[string]string{}
	langs := map[string]*TestClient{}
	teams := map[string]string{}
	for _, f := range files {
		game, lang := starterGame(f), filepath.Base(filepath.Dir(f))
		if gameIDs[game] == "" {
			var g GameResponse
			getJSON(t, admin, "/api/v1/games/name/"+game, &g)
			resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/games", map[string]string{"game_id": g.ID})
			require.NoError(t, err)
			requireStatus(t, resp, http.StatusNoContent)
			gameIDs[game] = g.ID
		}
		if langs[lang] == nil {
			c := NewTestClient()
			registerAndAuth(t, c, "st_"+lang)
			langs[lang] = c
			teams[lang] = createTeamHelper(t, c, tournamentID).ID
		}
	}

	resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/start", nil)
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusOK)

	// загрузка всех сразу: сборки и самопроверки идут параллельно ожиданию
	programs := map[string]string{}
	names := map[string]string{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		lang := filepath.Base(filepath.Dir(f))
		id := uploadProgram(t, langs[lang], tournamentID, gameIDs[starterGame(f)], teams[lang], filepath.Base(f), string(src))
		programs[f] = id
		names[id] = strings.TrimPrefix(f, startersDir+string(filepath.Separator))
	}

	for _, f := range files {
		t.Run(names[programs[f]], func(t *testing.T) {
			c := langs[filepath.Base(filepath.Dir(f))]
			waitProgramReady(t, c, programs[f])
			waitProgramChecked(t, c, programs[f])
		})
	}
	if t.Failed() {
		return
	}

	// раунд: каждый шаблон играет с каждым в обеих ролях, N·(N-1) матчей на игру
	for game := range gameIDs {
		resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+tournamentID+"/run-game-matches", map[string]string{"game_type": game})
		require.NoError(t, err)
		requireStatus(t, resp, http.StatusOK)
	}
	for game, gameID := range gameIDs {
		waitRoundCompleted(t, admin, tournamentID, gameID, len(langs)*(len(langs)-1), names)
		t.Logf("%s: раунд сыгран", game)
	}
}

// starterGame - игра шаблона по пути <игра>/<язык>/<файл>
func starterGame(path string) string {
	return filepath.Base(filepath.Dir(filepath.Dir(path)))
}

// waitRoundCompleted ждёт, пока все want матчей раунда игры завершатся; упавший
// матч сразу валит тест с именами обоих шаблонов
func waitRoundCompleted(t *testing.T, c *TestClient, tournamentID, gameID string, want int, names map[string]string) {
	t.Helper()
	require.LessOrEqual(t, want, 100, "раунд не помещается в одну страницу матчей")
	deadline := time.Now().Add(fullCycleTimeout)
	for time.Now().Before(deadline) {
		var matches []struct {
			Status       string  `json:"status"`
			Program1ID   string  `json:"program1_id"`
			Program2ID   string  `json:"program2_id"`
			ErrorMessage *string `json:"error_message"`
		}
		getJSON(t, c, fmt.Sprintf("/api/v1/tournaments/%s/games/%s/matches?limit=100", tournamentID, gameID), &matches)
		completed := 0
		for _, m := range matches {
			switch m.Status {
			case "completed":
				completed++
			case "failed", "cancelled":
				msg := ""
				if m.ErrorMessage != nil {
					msg = *m.ErrorMessage
				}
				t.Fatalf("матч %s против %s: %s %s", names[m.Program1ID], names[m.Program2ID], m.Status, msg)
			}
		}
		if completed == want {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("за %s не завершились %d матчей раунда", fullCycleTimeout, want)
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
