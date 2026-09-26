package api

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/handlers"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	guardPublic = "public" // без токена, в том числе с необязательным
	guardUser   = "user"   // нужен токен
	guardAdmin  = "admin"  // нужен токен админа
	// без мидлвари, токен проверяет сам хендлер: logout принимает и протухший
	guardHandler = "handler"
)

// защита каждого маршрута роутера. маршрут без записи тут валит тест:
// у новой ручки защиту приходится указать явно
var routeGuards = map[string]string{
	"GET /health": guardPublic,

	"POST /api/v1/auth/register": guardPublic,
	"POST /api/v1/auth/login":    guardPublic,
	"POST /api/v1/auth/refresh":  guardPublic,
	"POST /api/v1/auth/logout":   guardHandler,
	"GET /api/v1/auth/me":        guardUser,
	"PUT /api/v1/auth/profile":   guardUser,

	"GET /api/v1/tournaments/":                                         guardPublic,
	"GET /api/v1/tournaments/{id}":                                     guardPublic,
	"GET /api/v1/tournaments/{id}/leaderboard":                         guardPublic,
	"GET /api/v1/tournaments/{id}/cross-game-leaderboard":              guardPublic,
	"GET /api/v1/tournaments/{id}/matches":                             guardPublic,
	"GET /api/v1/tournaments/{id}/matches/rounds":                      guardPublic,
	"GET /api/v1/tournaments/{id}/games":                               guardPublic,
	"GET /api/v1/tournaments/{id}/teams":                               guardPublic,
	"GET /api/v1/tournaments/{id}/games/{gameId}/leaderboard":          guardPublic,
	"GET /api/v1/tournaments/{id}/games/{gameId}/head-to-head":         guardPublic,
	"GET /api/v1/tournaments/{id}/games/{gameId}/matches":              guardPublic,
	"GET /api/v1/tournaments/{id}/games/status":                        guardPublic,
	"GET /api/v1/tournaments/{id}/active-game":                         guardPublic,
	"GET /api/v1/tournaments/{id}/programs/{programId}/rating-history": guardPublic,
	"GET /api/v1/tournaments/{id}/my-team":                             guardUser,
	"POST /api/v1/tournaments/{id}/games":                              guardUser, // админ или создатель, проверка в хендлере
	"POST /api/v1/tournaments/":                                        guardAdmin,
	"POST /api/v1/tournaments/{id}/start":                              guardAdmin,
	"POST /api/v1/tournaments/{id}/complete":                           guardAdmin,
	"POST /api/v1/tournaments/{id}/matches":                            guardAdmin,
	"DELETE /api/v1/tournaments/{id}":                                  guardAdmin,
	"DELETE /api/v1/tournaments/{id}/games/{gameId}":                   guardAdmin,
	"GET /api/v1/tournaments/{id}/games/{gameId}/programs":             guardAdmin,
	"GET /api/v1/tournaments/{id}/programs/download-zip":               guardAdmin,
	"POST /api/v1/tournaments/{id}/games/{gameId}/complete-round":      guardAdmin,
	"POST /api/v1/tournaments/{id}/games/{gameId}/reset-round":         guardAdmin,
	"POST /api/v1/tournaments/{id}/games/{gameId}/auto-round":          guardAdmin,
	"GET /api/v1/tournaments/{id}/games/{gameId}/auto-round":           guardAdmin,
	"POST /api/v1/tournaments/{id}/active-game":                        guardAdmin,
	"POST /api/v1/tournaments/{id}/games/deactivate-all":               guardAdmin,
	"POST /api/v1/tournaments/{id}/run-matches":                        guardAdmin,
	"POST /api/v1/tournaments/{id}/run-game-matches":                   guardAdmin,
	"POST /api/v1/tournaments/{id}/retry-matches":                      guardAdmin,
	"POST /api/v1/tournaments/{id}/programs/clear-errors":              guardAdmin,

	"GET /api/v1/games/":            guardPublic,
	"GET /api/v1/games/{id}":        guardPublic,
	"GET /api/v1/games/name/{name}": guardPublic,
	"POST /api/v1/games/":           guardAdmin,
	"PUT /api/v1/games/{id}":        guardAdmin,
	"DELETE /api/v1/games/{id}":     guardAdmin,

	"POST /api/v1/teams/":                        guardUser,
	"POST /api/v1/teams/join":                    guardUser,
	"GET /api/v1/teams/{id}":                     guardUser,
	"PUT /api/v1/teams/{id}":                     guardUser,
	"GET /api/v1/teams/{id}/members":             guardUser,
	"POST /api/v1/teams/{id}/leave":              guardUser,
	"DELETE /api/v1/teams/{id}/members/{userId}": guardUser,
	"GET /api/v1/teams/{id}/invite":              guardUser,
	"DELETE /api/v1/teams/{id}":                  guardAdmin,
	"POST /api/v1/teams/{id}/disqualify":         guardAdmin,
	"POST /api/v1/teams/{id}/restore":            guardAdmin,

	"POST /api/v1/programs/":             guardUser,
	"GET /api/v1/programs/":              guardUser,
	"GET /api/v1/programs/versions":      guardUser,
	"GET /api/v1/programs/{id}":          guardUser,
	"GET /api/v1/programs/{id}/download": guardUser,
	"DELETE /api/v1/programs/{id}":       guardUser,

	"GET /api/v1/matches/":             guardPublic,
	"GET /api/v1/matches/statistics":   guardPublic,
	"GET /api/v1/matches/{id}":         guardPublic,
	"GET /api/v1/matches/queue/stats":  guardAdmin,
	"POST /api/v1/matches/queue/clear": guardAdmin,
	"POST /api/v1/matches/queue/purge": guardAdmin,

	"GET /api/v1/ws/tournaments/{id}": guardUser,
	"GET /api/v1/ws/stats":            guardUser,

	"GET /api/v1/system/metrics":                       guardAdmin,
	"GET /api/v1/system/health":                        guardAdmin,
	"GET /api/v1/system/status":                        guardAdmin,
	"POST /api/v1/system/recovery/outbox-retry":        guardAdmin,
	"POST /api/v1/system/recovery/requeue-compiling":   guardAdmin,
	"POST /api/v1/system/recovery/reset-stuck-matches": guardAdmin,
	"POST /api/v1/system/recovery/clear-dead-letter":   guardAdmin,
	"GET /api/v1/admin/audit":                          guardAdmin,
}

// маршруты на все методы: pprof за админом, статика spa публичная
func guardFor(method, route string) (string, bool) {
	switch {
	case strings.HasPrefix(route, "/debug/pprof/"):
		return guardAdmin, true
	case route == "/*":
		return guardPublic, true
	}
	g, ok := routeGuards[method+" "+route]
	return g, ok
}

var routeParam = regexp.MustCompile(`\{[^}]+\}`)

// обход настоящего роутера: без токена защищённый маршрут отдаёт 401, публичный
// нет, админский с токеном обычного пользователя - 403
func TestRouter_RBAC(t *testing.T) {
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	// хендлеры пустые: защищённые маршруты до них не доходят
	srv := NewServer(ServerDeps{
		AuthHandler:       &handlers.AuthHandler{},
		TournamentHandler: &handlers.TournamentHandler{},
		ProgramHandler:    &handlers.ProgramHandler{},
		MatchHandler:      &handlers.MatchHandler{},
		GameHandler: &handlers.GameHandler{
			GameCRUDHandler:       &handlers.GameCRUDHandler{},
			TournamentGameHandler: &handlers.TournamentGameHandler{},
			GameRoundHandler:      &handlers.GameRoundHandler{},
		},
		TeamHandler:          &handlers.TeamHandler{},
		SystemHandler:        &handlers.SystemHandler{},
		WSHandler:            &handlers.WebSocketHandler{},
		StatusHandler:        &handlers.SystemStatusHandler{},
		RatingHistoryHandler: &handlers.RatingHistoryHandler{},
		RecoveryHandler:      &handlers.SystemRecoveryHandler{},
		AuditHandler:         &handlers.AuditHandler{},
		AuthService:          stubAuth{userID: uuid.New()},
		Log:                  log,
	})
	t.Cleanup(srv.Close)

	status := func(method, path, token string) int {
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	seen := make(map[string]bool)
	err = chi.Walk(srv.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		guard, ok := guardFor(method, route)
		if !assert.True(t, ok, "у маршрута %s %s нет записи в routeGuards", method, route) {
			return nil
		}
		seen[method+" "+route] = true
		// у маршрутов на все методы хватает GET
		if guard == guardHandler || (strings.HasPrefix(route, "/debug/pprof/") && method != http.MethodGet) {
			return nil
		}

		path := routeParam.ReplaceAllString(route, uuid.NewString())
		// пустой хендлер публичной ручки падает в Recoverer (500), важно лишь не 401
		if guard == guardPublic {
			assert.NotEqual(t, http.StatusUnauthorized, status(method, path, ""), "%s %s публичный", method, route)
			return nil
		}
		assert.Equal(t, http.StatusUnauthorized, status(method, path, ""), "%s %s без токена", method, route)
		if guard == guardAdmin {
			assert.Equal(t, http.StatusForbidden, status(method, path, "user"), "%s %s с токеном пользователя", method, route)
		}
		return nil
	})
	require.NoError(t, err)

	for route := range routeGuards {
		assert.True(t, seen[route], "маршрута %s больше нет, запись в routeGuards лишняя", route)
	}
}
