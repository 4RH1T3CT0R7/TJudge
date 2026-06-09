//go:build benchmark
// +build benchmark

package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api"
	"github.com/bmstu-itstech/tjudge/internal/api/handlers"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/internal/domain/game"
	"github.com/bmstu-itstech/tjudge/internal/domain/team"
	"github.com/bmstu-itstech/tjudge/internal/domain/tournament"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
)

var (
	testServer  *httptest.Server
	testHandler http.Handler
	setupOnce   bool
	testToken   string
	setupErr    error
)

func setupTestServer(b *testing.B) {
	if setupErr != nil {
		b.Skipf("Setup failed previously: %v", setupErr)
		return
	}

	if setupOnce {
		return
	}

	cfg, err := config.Load()
	if err != nil {
		setupErr = err
		b.Skipf("Failed to load config: %v", err)
		return
	}

	log, err := logger.New("error", "json")
	if err != nil {
		setupErr = err
		b.Skipf("Failed to create logger: %v", err)
		return
	}

	m := metrics.New()

	database, err := db.New(&cfg.Database, log, m)
	if err != nil {
		setupErr = err
		b.Skipf("Database not available: %v", err)
		return
	}

	redisCache, err := cache.New(&cfg.Redis, log, m)
	if err != nil {
		setupErr = err
		b.Skipf("Redis not available: %v", err)
		return
	}

	userRepo := db.NewUserRepository(database)
	programRepo := db.NewProgramRepository(database)
	tournamentRepo := db.NewTournamentRepository(database)
	matchRepo := db.NewMatchRepository(database)
	gameRepo := db.NewGameRepository(database)
	teamRepo := db.NewTeamRepository(database)

	matchCache := cache.NewMatchCache(redisCache).WithMetrics(m)
	leaderboardCache := cache.NewLeaderboardCache(redisCache).WithMetrics(m)
	tournamentCache := cache.NewTournamentCache(redisCache)
	tokenBlacklist := cache.NewTokenBlacklistCache(redisCache)
	rateLimiter := cache.NewRateLimiter(redisCache)
	distributedLock := cache.NewDistributedLock(redisCache)

	queueManager := queue.NewQueueManager(redisCache, log, m)

	wsHub := websocket.NewHub(log)
	eventBus := events.NewSyncBus(log)

	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authService := auth.NewService(userRepo, jwtManager, tokenBlacklist, log)

	tournamentService := tournament.NewService(
		tournamentRepo, matchRepo, queueManager, gameRepo,
		tournamentCache, leaderboardCache, eventBus,
		distributedLock, log,
	)

	schedulingService := tournament.NewSchedulingService(
		tournamentRepo, matchRepo, queueManager, gameRepo,
		distributedLock, eventBus, log,
	)

	gameService := game.NewService(gameRepo, log)
	teamService := team.NewService(teamRepo, tournamentRepo, distributedLock, log)

	authHandler := handlers.NewAuthHandler(authService, log)
	tournamentHandler := handlers.NewTournamentHandler(tournamentService, schedulingService, log)
	programHandler := handlers.NewProgramHandler(
		programRepo, tournamentRepo, tournamentRepo,
		nil, gameService, matchRepo, gameRepo,
		teamRepo, gameRepo, nil, cfg.Storage.ProgramsPath, log,
	)
	matchHandler := handlers.NewMatchHandler(matchRepo, matchCache, programRepo, queueManager, log)
	gameHandler := handlers.NewGameHandler(
		gameService, tournamentRepo, matchRepo, tournamentRepo,
		programRepo, gameRepo, eventBus, cfg.Storage.ProgramsPath, log,
	)
	teamHandler := handlers.NewTeamHandler(teamService, cfg.Server.BaseURL, log)
	wsHandler := handlers.NewWebSocketHandler(wsHub, log)
	systemHandler := handlers.NewSystemHandler(log)

	apiServer := api.NewServer(
		authHandler, tournamentHandler, programHandler, matchHandler,
		gameHandler, teamHandler, wsHandler, systemHandler,
		authService, rateLimiter,
		cfg.CORS, cfg.RateLimit, log,
	)

	testHandler = apiServer.Handler()
	testServer = httptest.NewServer(testHandler)
	setupOnce = true

	timestamp := time.Now().UnixNano()
	registerReq := map[string]string{
		"username": fmt.Sprintf("bench_user_%d", timestamp),
		"email":    fmt.Sprintf("bench_%d@test.com", timestamp),
		"password": "BenchmarkPass123!",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testHandler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
		var resp struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&resp)
		testToken = resp.AccessToken
	}
}

func BenchmarkHealthEndpoint(b *testing.B) {
	setupTestServer(b)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			testHandler.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkAuthLogin(b *testing.B) {
	setupTestServer(b)

	loginReq := map[string]string{
		"username": "test_user",
		"password": "TestPass123!",
	}
	body, _ := json.Marshal(loginReq)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req)
	}
}

func BenchmarkTournamentsList(b *testing.B) {
	setupTestServer(b)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/v1/tournaments?limit=20", nil)
			if testToken != "" {
				req.Header.Set("Authorization", "Bearer "+testToken)
			}
			rec := httptest.NewRecorder()
			testHandler.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkTournamentGet(b *testing.B) {
	setupTestServer(b)

	tournamentID := uuid.New().String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/tournaments/"+tournamentID, nil)
		if testToken != "" {
			req.Header.Set("Authorization", "Bearer "+testToken)
		}
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req)
	}
}

func BenchmarkLeaderboard(b *testing.B) {
	setupTestServer(b)

	tournamentID := uuid.New().String()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/v1/tournaments/"+tournamentID+"/leaderboard", nil)
			if testToken != "" {
				req.Header.Set("Authorization", "Bearer "+testToken)
			}
			rec := httptest.NewRecorder()
			testHandler.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkProgramsList(b *testing.B) {
	setupTestServer(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/programs", nil)
		if testToken != "" {
			req.Header.Set("Authorization", "Bearer "+testToken)
		}
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req)
	}
}

func BenchmarkMatchesList(b *testing.B) {
	setupTestServer(b)

	tournamentID := uuid.New().String()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/v1/tournaments/"+tournamentID+"/matches?limit=50", nil)
			if testToken != "" {
				req.Header.Set("Authorization", "Bearer "+testToken)
			}
			rec := httptest.NewRecorder()
			testHandler.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkJSONParsing(b *testing.B) {
	tournament := domain.Tournament{
		ID:              uuid.New(),
		Name:            "Benchmark Tournament",
		Code:            "BENCH1",
		Description:     "A tournament for benchmarking",
		GameType:        "tictactoe",
		Status:          domain.TournamentActive,
		MaxParticipants: intPtr(100),
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(tournament)
		var result domain.Tournament
		_ = json.Unmarshal(data, &result)
	}
}

func BenchmarkAuthMiddleware(b *testing.B) {
	setupTestServer(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req)
	}
}

func BenchmarkConcurrentRequests(b *testing.B) {
	setupTestServer(b)

	endpoints := []string{
		"/health",
		"/api/v1/tournaments",
		"/api/v1/programs",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			endpoint := endpoints[i%len(endpoints)]
			req := httptest.NewRequest("GET", endpoint, nil)
			if testToken != "" {
				req.Header.Set("Authorization", "Bearer "+testToken)
			}
			rec := httptest.NewRecorder()
			testHandler.ServeHTTP(rec, req)
			i++
		}
	})
}

func intPtr(i int) *int {
	return &i
}
