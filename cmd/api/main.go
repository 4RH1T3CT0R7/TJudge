package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// matchSchedulerAdapter адаптер для tournament.Service.ScheduleNewProgramMatches
type matchSchedulerAdapter struct {
	tournamentService *tournament.Service
	programRepo       *db.ProgramRepository
}

func (a *matchSchedulerAdapter) ScheduleNewProgramMatches(ctx context.Context, tournamentID, gameID, newProgramID, teamID uuid.UUID) error {
	req := &tournament.ScheduleNewProgramMatchesRequest{
		TournamentID: tournamentID,
		GameID:       gameID,
		NewProgramID: newProgramID,
		TeamID:       teamID,
	}
	return a.tournamentService.ScheduleNewProgramMatches(ctx, req, a.programRepo)
}

// programRepoAdapter адаптер для ProgramRepository
type programRepoAdapter struct {
	repo *db.ProgramRepository
}

func (a *programRepoAdapter) GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error) {
	return a.repo.GetByTournamentAndGame(ctx, tournamentID, gameID)
}

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Инициализируем логгер
	log, err := logger.NewWithOptions(logger.Options{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Async:  cfg.Logging.Async,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting TJudge API Server",
		zap.Int("port", cfg.Server.Port),
		zap.String("env", "production"),
	)

	// Инициализируем метрики
	m := metrics.New()

	// Подключаемся к базе данных
	database, err := db.New(&cfg.Database, log, m)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	log.Info("Connected to database",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port),
	)

	// Проверяем здоровье БД
	if err := database.Health(context.Background()); err != nil {
		log.Fatal("Database health check failed", zap.Error(err))
	}

	// Подключаемся к Redis
	redisCache, err := cache.New(&cfg.Redis, log, m)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisCache.Close()

	log.Info("Connected to Redis",
		zap.String("host", cfg.Redis.Host),
		zap.Int("port", cfg.Redis.Port),
	)

	// Инициализируем репозитории
	userRepo := db.NewUserRepository(database)
	programRepo := db.NewProgramRepository(database)
	tournamentRepo := db.NewTournamentRepository(database)
	matchRepo := db.NewMatchRepository(database)
	gameRepo := db.NewGameRepository(database)
	teamRepo := db.NewTeamRepository(database)

	// Инициализируем кэши с метриками
	matchCache := cache.NewMatchCache(redisCache).WithMetrics(m)
	leaderboardCache := cache.NewLeaderboardCache(redisCache).WithMetrics(m)
	tournamentCache := cache.NewTournamentCache(redisCache)
	tokenBlacklist := cache.NewTokenBlacklistCache(redisCache)
	rateLimiter := cache.NewRateLimiter(redisCache)
	distributedLock := cache.NewDistributedLock(redisCache)

	// Инициализируем queue manager
	queueManager := queue.NewQueueManager(redisCache, log, m)

	// Инициализируем WebSocket hub
	wsHub := websocket.NewHub(log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем hub в отдельной горутине
	go wsHub.Run(ctx)

	// Инициализируем сервисы
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authService := auth.NewService(userRepo, jwtManager, tokenBlacklist, log)

	tournamentService := tournament.NewService(
		tournamentRepo,
		matchRepo,
		queueManager,
		tournamentCache,
		leaderboardCache,
		wsHub,           // broadcaster
		distributedLock, // distributed lock
		log,
	)

	gameService := game.NewService(gameRepo, log)
	teamService := team.NewService(teamRepo, tournamentRepo, log)

	// Создаём адаптеры для репозиториев (для game handler)
	// tournamentRepo уже реализует GetLeaderboardByGameType
	// matchRepo уже реализует List

	// Создаём адаптер для планирования матчей
	matchScheduler := &matchSchedulerAdapter{
		tournamentService: tournamentService,
		programRepo:       programRepo,
	}

	// Инициализируем handlers
	authHandler := handlers.NewAuthHandler(authService, log)
	tournamentHandler := handlers.NewTournamentHandler(tournamentService, log)
	programHandler := handlers.NewProgramHandler(programRepo, tournamentRepo, matchScheduler, log)
	matchHandler := handlers.NewMatchHandler(matchRepo, matchCache, log)
	gameHandler := handlers.NewGameHandlerWithRepos(gameService, tournamentRepo, matchRepo, log)
	teamHandler := handlers.NewTeamHandler(teamService, cfg.Server.BaseURL, log)
	wsHandler := handlers.NewWebSocketHandler(wsHub, log)

	// Создаём API сервер
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

	// Создаём HTTP сервер
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      apiServer.Handler(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Metrics server (если включен)
	var metricsSrv *http.Server
	if cfg.Metrics.Enabled {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())

		metricsSrv = &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Metrics.Port),
			Handler:           metricsMux,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			log.Info("Metrics server listening",
				zap.String("addr", metricsSrv.Addr),
			)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("Metrics server error", zap.Error(err))
			}
		}()
	}

	// Канал для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Info("API server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Ждём сигнала остановки
	<-quit
	log.Info("Shutting down servers...")

	// Graceful shutdown с таймаутом
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Останавливаем API сервер
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("API server forced to shutdown", zap.Error(err))
	}

	// Останавливаем metrics сервер
	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			log.Error("Metrics server forced to shutdown", zap.Error(err))
		}
	}

	// Останавливаем WebSocket hub
	cancel()

	log.Info("Servers stopped gracefully")
}
