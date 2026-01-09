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

// setupTestServer initializes the test server with all dependencies
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

	// Connect to database
	database, err := db.New(&cfg.Database, log, m)
	if err != nil {
		setupErr = err
		b.Skipf("Database not available: %v", err)
		return
	}

	// Connect to Redis
	redisCache, err := cache.New(&cfg.Redis, log, m)
	if err != nil {
		setupErr = err
		b.Skipf("Redis not available: %v", err)
		return
	}

	// Initialize repositories
	userRepo := db.NewUserRepository(database)
	programRepo := db.NewProgramRepository(database)
	tournamentRepo := db.NewTournamentRepository(database)
	matchRepo := db.NewMatchRepository(database)
	gameRepo := db.NewGameRepository(database)
	teamRepo := db.NewTeamRepository(database)

	// Initialize caches
	matchCache := cache.NewMatchCache(redisCache).WithMetrics(m)
	leaderboardCache := cache.NewLeaderboardCache(redisCache).WithMetrics(m)
	tournamentCache := cache.NewTournamentCache(redisCache)
	tokenBlacklist := cache.NewTokenBlacklistCache(redisCache)
	rateLimiter := cache.NewRateLimiter(redisCache)
	distributedLock := cache.NewDistributedLock(redisCache)

	// Initialize queue manager
	queueManager := queue.NewQueueManager(redisCache, log, m)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(log)

	// Initialize services
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authService := auth.NewService(userRepo, jwtManager, tokenBlacklist, log)

	tournamentService := tournament.NewService(
		tournamentRepo,
		matchRepo,
		queueManager,
		gameRepo,
		tournamentCache,
		leaderboardCache,
		wsHub,
		distributedLock,
		log,
	)

	gameService := game.NewService(gameRepo, log)
	teamService := team.NewService(teamRepo, tournamentRepo, log)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, log)
	tournamentHandler := handlers.NewTournamentHandler(tournamentService, log)
	programHandler := handlers.NewProgramHandler(programRepo, tournamentRepo, nil, log)
	matchHandler := handlers.NewMatchHandler(matchRepo, matchCache, log)
	gameHandler := handlers.NewGameHandler(gameService, log)
	teamHandler := handlers.NewTeamHandler(teamService, cfg.Server.BaseURL, log)
	wsHandler := handlers.NewWebSocketHandler(wsHub, log)

	// Create API server
	apiServer := api.NewServer(
		authHandler,
		tournamentHandler,
		programHandler,
		matchHandler,
		gameHandler,
		teamHandler,
		wsHandler,
		authService,
		rateLimiter,
		cfg.CORS,
		cfg.RateLimit,
		log,
	)

	testHandler = apiServer.Handler()
	testServer = httptest.NewServer(testHandler)
	setupOnce = true

	// Create test user and get token
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
		json.NewDecoder(rec.Body).Decode(&resp)
		testToken = resp.AccessToken
	}
}

// BenchmarkHealthEndpoint measures the health endpoint performance
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

// BenchmarkAuthLogin measures login endpoint performance
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

// BenchmarkTournamentsList measures tournament listing performance
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

// BenchmarkTournamentGet measures single tournament fetch performance
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

// BenchmarkLeaderboard measures leaderboard endpoint performance
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

// BenchmarkProgramsList measures program listing performance
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

// BenchmarkMatchesList measures match listing performance
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

// BenchmarkJSONParsing measures JSON serialization/deserialization
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
		json.Unmarshal(data, &result)
	}
}

// BenchmarkAuthMiddleware measures auth middleware overhead
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

// BenchmarkConcurrentRequests measures performance under concurrent load
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
