//go:build e2e

package e2e

import (
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestE2E_Load: нагрузочный прогон турнира на E2E_LOAD_TEAMS команд. каждая
// команда загружает шаблоны ботов во все игры (языки по кругу), после сборки
// раунды всех игр запускаются разом: N·(N-1) матчей на игру. турнир остаётся
// в БД, ход раундов видно в интерфейсе по ссылке из лога
func TestE2E_Load(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("E2E_LOAD_TEAMS"))
	if n < 2 {
		t.Skip("нагрузка: задать E2E_LOAD_TEAMS (например 25), нужны api, worker и образы песочниц")
	}

	files, err := filepath.Glob(filepath.Join(startersDir, "*", "*", "[^.]*"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "шаблоны не найдены в %s", startersDir)
	starters := map[string]map[string]string{} // игра -> язык -> файл
	for _, f := range files {
		game := starterGame(f)
		if starters[game] == nil {
			starters[game] = map[string]string{}
		}
		starters[game][filepath.Base(filepath.Dir(f))] = f
	}
	langs := slices.Sorted(maps.Keys(starters[starterGame(files[0])]))

	admin := NewTestClient()
	registerAndAuthAsAdmin(t, admin, "load_admin")

	// createTournamentHelper ограничивает турнир 20 командами
	resp, err := admin.doRequest("POST", "/api/v1/tournaments", CreateTournamentRequest{
		Name:            fmt.Sprintf("Нагрузка: %d команд, %s", n, time.Now().Format("02.01 15:04")),
		Description:     "TestE2E_Load",
		GameType:        "dilemma",
		MaxParticipants: n,
	})
	require.NoError(t, err)
	if resp.StatusCode != http.StatusCreated {
		requireStatus(t, resp, http.StatusCreated)
	}
	var tournament TournamentResponse
	require.NoError(t, admin.parseResponse(resp, &tournament))
	id := tournament.ID

	gameIDs := map[string]string{}
	for game := range starters {
		var g GameResponse
		getJSON(t, admin, "/api/v1/games/name/"+game, &g)
		resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+id+"/games", map[string]string{"game_id": g.ID})
		require.NoError(t, err)
		requireStatus(t, resp, http.StatusNoContent)
		gameIDs[game] = g.ID
	}
	t.Logf("турнир: %s/tournaments/%s", baseURL, id)

	phase := time.Now()
	clients := make([]*TestClient, n)
	teams := make([]string, n)
	for i := range n {
		clients[i] = NewTestClient()
		registerAndAuth(t, clients[i], fmt.Sprintf("load%02d", i+1))
		teams[i] = createTeamHelper(t, clients[i], id).ID
	}
	t.Logf("%d команд за %s", n, time.Since(phase).Round(time.Millisecond))

	resp, err = admin.doRequest("POST", "/api/v1/tournaments/"+id+"/start", nil)
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusOK)

	// загрузка всех сразу: сборки идут параллельно ожиданию
	phase = time.Now()
	programs := map[string]*TestClient{}
	for i := range n {
		lang := langs[i%len(langs)]
		for game, gameID := range gameIDs {
			f := starters[game][lang]
			src, err := os.ReadFile(f)
			require.NoError(t, err)
			programs[uploadProgram(t, clients[i], id, gameID, teams[i], filepath.Base(f), string(src))] = clients[i]
		}
	}
	for prog, c := range programs {
		waitProgramReady(t, c, prog)
	}
	t.Logf("%d программ загружены и собраны за %s", len(programs), time.Since(phase).Round(time.Second))

	phase = time.Now()
	for game := range gameIDs {
		resp, err := admin.doRequest("POST", "/api/v1/tournaments/"+id+"/run-game-matches", map[string]string{"game_type": game})
		require.NoError(t, err)
		requireStatus(t, resp, http.StatusOK)
	}

	// дедлайна на весь раунд нет, его длина зависит от N и числа воркеров:
	// тест падает, только если матчи перестали завершаться
	want := len(gameIDs) * n * (n - 1)
	var st struct{ Total, Pending, Running, Completed, Failed int }
	for last, moved := -1, time.Now(); ; time.Sleep(5 * time.Second) {
		getJSON(t, admin, "/api/v1/matches/statistics?tournament_id="+id, &st)
		done := st.Completed + st.Failed
		if done != last {
			t.Logf("сыграно %d/%d, в очереди %d, идёт %d, с ошибкой %d", done, want, st.Pending, st.Running, st.Failed)
			last, moved = done, time.Now()
		}
		if done >= want {
			break
		}
		if time.Since(moved) > 5*time.Minute {
			t.Fatalf("5 минут без завершённых матчей: сыграно %d/%d, worker жив?", done, want)
		}
	}
	elapsed := time.Since(phase)
	t.Logf("раунды: %d матчей за %s, %.1f матча/с", st.Total, elapsed.Round(time.Second), float64(st.Total)/elapsed.Seconds())
	require.Equal(t, want, st.Total, "матчей в раундах")
	if st.Failed > 0 {
		t.Errorf("матчей с ошибкой: %d, тексты ошибок во вкладке Матчи турнира", st.Failed)
	}
}
