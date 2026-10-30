//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// loadBots - стратегии нагрузочного теста на python. у команды i своя доля P
// от 0 до 1 (щедрость, готовность сотрудничать, предел ставки) и своё зерно
// случайности, поэтому матчи не сводятся к ничьим одинаковых шаблонов
var loadBots = map[string]string{
	"dilemma": `
n = int(input())
for _ in range(n):
    print("COOPERATE" if random.random() < P else "DEFECT", flush=True)
    input()
`,
	"tug_of_war": `
energy = int(input())
n = int(input())
for i in range(n):
    spend = energy if i == n - 1 else min(energy, int(energy / (n - i) * random.uniform(1 - P, 1 + P)))
    energy -= spend
    print(spend, flush=True)
    input()
`,
	"travelers_dilemma": `
low, high, bonus, n = (int(input()) for _ in range(4))
for _ in range(n):
    print(max(low, min(high, round(low + (high - low) * P) + random.randint(-2, 2))), flush=True)
    input()
`,
	"public_goods": `
capital = int(input())
input()
n = int(input())
for _ in range(n):
    print(max(0, min(capital, round(capital * P) + random.randint(-1, 1))), flush=True)
    input()
`,
	"dollar_auction": `
prize = int(input())
input()
limit = max(1, round(prize * P))
for line in sys.stdin:
    bid = int(line) + 1
    print(bid if bid <= limit else 0, flush=True)
`,
}

// TestE2E_Load: нагрузочный прогон турнира на E2E_LOAD_TEAMS команд. каждая
// команда загружает во все игры ботов из loadBots, после сборки раунды всех
// игр запускаются разом: N·(N-1) матчей на игру. турнир остаётся в БД, ход
// раундов видно в интерфейсе по ссылке из лога
func TestE2E_Load(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("E2E_LOAD_TEAMS"))
	if n < 2 {
		t.Skip("нагрузка: задать E2E_LOAD_TEAMS (например 25), нужны api, worker и образы песочниц")
	}

	// на сервере БД закрыта, поэтому вход готовым админом (make test-load-server).
	// токен нужен до старта раундов: статистику матчей можно читать и без него
	admin := NewTestClient()
	if user := os.Getenv("E2E_ADMIN_USER"); user != "" {
		field := "username"
		if strings.Contains(user, "@") {
			field = "email"
		}
		resp, err := admin.doRequest("POST", "/api/v1/auth/login", map[string]string{field: user, "password": os.Getenv("E2E_ADMIN_PASSWORD")})
		require.NoError(t, err)
		if resp.StatusCode != http.StatusOK {
			requireStatus(t, resp, http.StatusOK)
		}
		var auth AuthResponse
		require.NoError(t, admin.parseResponse(resp, &auth))
		admin.SetToken(auth.AccessToken)
	} else {
		registerAndAuthAsAdmin(t, admin, "load_admin")
	}

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
	for game := range loadBots {
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
		head := fmt.Sprintf("import random, sys\nP = %.3f\nrandom.seed(%d)\n", float64(i)/float64(n-1), i)
		for game, gameID := range gameIDs {
			programs[uploadProgram(t, clients[i], id, gameID, teams[i], "main.py", head+loadBots[game])] = clients[i]
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

	want := len(gameIDs) * n * (n - 1)
	var st struct{ Total, Pending, Running, Completed, Failed int }
	getJSON(t, admin, "/api/v1/matches/statistics?tournament_id="+id, &st)
	require.Equal(t, want, st.Total, "матчей в раундах")

	// раунд окончен, когда ждать нечего: матчи дисквалифицированной команды
	// или очищенной очереди отменяются и до want не доигрываются. дедлайна на
	// весь раунд нет, его длина зависит от N и числа воркеров: тест падает,
	// только если матчи перестали завершаться
	done := st.Completed + st.Failed
	for last, moved := -1, time.Now(); st.Pending+st.Running > 0; {
		time.Sleep(5 * time.Second)
		getJSON(t, admin, "/api/v1/matches/statistics?tournament_id="+id, &st)
		done = st.Completed + st.Failed
		if done != last {
			t.Logf("сыграно %d/%d, в очереди %d, идёт %d, с ошибкой %d", done, want, st.Pending, st.Running, st.Failed)
			last, moved = done, time.Now()
		}
		if time.Since(moved) > 5*time.Minute {
			t.Fatalf("5 минут без завершённых матчей: сыграно %d/%d, worker жив?", done, want)
		}
	}
	elapsed := time.Since(phase)
	t.Logf("раунды: %d матчей за %s, %.1f матча/с", done, elapsed.Round(time.Second), float64(done)/elapsed.Seconds())
	if done < want {
		t.Logf("не сыграно %d: матчи отменены или удалены (дисквалификация, очистка очереди)", want-done)
	}
	if st.Failed > 0 {
		t.Errorf("матчей с ошибкой: %d, тексты ошибок во вкладке Матчи турнира", st.Failed)
	}
}
