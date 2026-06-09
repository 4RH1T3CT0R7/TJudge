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
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/internal/domain/game"
	"github.com/bmstu-itstech/tjudge/internal/domain/team"
	"github.com/bmstu-itstech/tjudge/internal/domain/tournament"
	"github.com/bmstu-itstech/tjudge/internal/events"
	eventhandlers "github.com/bmstu-itstech/tjudge/internal/events/handlers"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/internal/observability"
	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// matchSchedulerAdapter адаптер для tournament.SchedulingService.ScheduleNewProgramMatches
type matchSchedulerAdapter struct {
	schedulingService *tournament.SchedulingService
	programRepo       *db.ProgramRepository
}

func (a *matchSchedulerAdapter) ScheduleNewProgramMatches(ctx context.Context, tournamentID, gameID, newProgramID, teamID uuid.UUID) error {
	req := &tournament.ScheduleNewProgramMatchesRequest{
		TournamentID: tournamentID,
		GameID:       gameID,
		NewProgramID: newProgramID,
		TeamID:       teamID,
	}
	return a.schedulingService.ScheduleNewProgramMatches(ctx, req, a.programRepo)
}

// @title TJudge API
// @version 1.0
// @description Tournament system for game theory competitions
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token (format: "Bearer {token}")
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

	// OpenTelemetry tracing (опционально, если задан OTEL_EXPORTER_OTLP_ENDPOINT).
	otelShutdown, err := observability.InitTracerProvider(context.Background(), "tjudge-api", "dev", log)
	if err != nil {
		log.Warn("Failed to init OTel tracing (continuing without)", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(shutdownCtx)
	}()

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

	// Обеспечиваем наличие партиций таблицы matches и rating_history
	if err := database.EnsureMatchPartitions(context.Background()); err != nil {
		log.Error("Failed to ensure match partitions", zap.Error(err))
	}
	if err := database.EnsureRatingHistoryPartitions(context.Background()); err != nil {
		log.Error("Failed to ensure rating_history partitions", zap.Error(err))
	}
	database.StartPartitionMaintenance(cfg.Database.PartitionRetentionMonths)

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

	// Инициализируем event bus
	eventBus := events.NewSyncBus(log)

	eventBus.Subscribe(
		eventhandlers.NewTournamentCacheHandler(tournamentCache, leaderboardCache, log),
		events.TournamentCreated{}, events.TournamentStarted{},
		events.TournamentCompleted{}, events.TournamentDeleted{},
		events.ParticipantJoined{}, events.GameRoundReset{},
	)
	eventBus.Subscribe(
		eventhandlers.NewLeaderboardCacheHandler(leaderboardCache, log),
		events.ParticipantJoined{}, events.MatchResultProcessed{},
		events.GameRoundReset{},
	)
	eventBus.Subscribe(
		eventhandlers.NewBroadcastHandler(wsHub, log),
		events.TournamentStarted{}, events.TournamentCompleted{},
		events.MatchesCreated{}, events.MatchResultProcessed{},
	)

	// Мост Redis Pub/Sub: принимает события из процесса воркера для рассылки по WebSocket.
	// Используется отдельная шина, чтобы события от воркера триггерили только рассылку
	// и не приводили к повторному обновлению кэша (воркер уже обновил свой кэш).
	wsBus := events.NewSyncBus(log)
	wsBus.Subscribe(
		eventhandlers.NewBroadcastHandler(wsHub, log),
		events.MatchResultProcessed{}, events.ProgramCompiled{},
	)
	redisEventSub := events.NewRedisEventSubscriber(redisCache, wsBus, log)
	go redisEventSub.Start(ctx)

	// Инициализируем сервисы
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authService := auth.NewService(userRepo, jwtManager, tokenBlacklist, log)

	tournamentService := tournament.NewService(
		tournamentRepo,
		matchRepo,
		queueManager,
		gameRepo, // game repository for setting active game
		tournamentCache,
		leaderboardCache,
		eventBus,
		distributedLock,
		log,
	)

	schedulingService := tournament.NewSchedulingService(
		tournamentRepo,
		matchRepo,
		queueManager,
		gameRepo,
		distributedLock,
		eventBus,
		log,
	)

	gameService := game.NewService(gameRepo, log)
	teamService := team.NewService(teamRepo, tournamentRepo, distributedLock, log)

	// Авто-раунд планировщик
	autoRoundScheduler := tournament.NewAutoRoundScheduler(
		schedulingService,
		gameRepo,
		distributedLock,
		log,
		5*time.Second,
	)
	autoRoundScheduler.Start(ctx)

	// Создаём адаптеры для репозиториев (для game handler)
	// tournamentRepo уже реализует GetLeaderboardByGameType
	// matchRepo уже реализует List

	// Создаём адаптер для планирования матчей
	matchScheduler := &matchSchedulerAdapter{
		schedulingService: schedulingService,
		programRepo:       programRepo,
	}

	// Очередь асинхронной компиляции: upload ставит задачу, worker
	// компилирует программу в Docker-песочнице.
	compileQueue := queue.NewCompileQueue(redisCache, log)

	// Инициализируем handlers
	authHandler := handlers.NewAuthHandler(authService, log)
	tournamentHandler := handlers.NewTournamentHandler(tournamentService, schedulingService, log)
	programHandler := handlers.NewProgramHandler(
		programRepo, tournamentRepo, tournamentRepo,
		matchScheduler, gameService, matchRepo, gameRepo,
		teamRepo,
		gameRepo, // autoRoundChecker
		compileQueue,
		cfg.Storage.ProgramsPath, log,
	)
	matchHandler := handlers.NewMatchHandler(matchRepo, matchCache, programRepo, queueManager, log)
	gameHandler := handlers.NewGameHandler(
		gameService, tournamentRepo, matchRepo, tournamentRepo,
		programRepo, gameRepo, eventBus, cfg.Storage.ProgramsPath, log,
	)
	teamHandler := handlers.NewTeamHandler(teamService, cfg.Server.BaseURL, log)
	wsHandler := handlers.NewWebSocketHandler(wsHub, log)
	systemHandler := handlers.NewSystemHandler(log)

	// Создаём API сервер
	adminChecker := middleware.NewVerifiedAdminChecker(userRepo, 5*time.Minute)

	// Audit log (async). Буфер 2048: при нагрузке 10 admin-запросов/сек
	// даёт ~200 сек запаса перед drop'ом.
	auditRepo := db.NewAuditLogRepository(database)
	auditLogger := middleware.NewAuditLogger(auditRepo, 2048, log)
	auditCtx, cancelAudit := context.WithCancel(context.Background())
	defer cancelAudit()
	go auditLogger.Run(auditCtx)
	defer auditLogger.Close()
	auditHandler := handlers.NewAuditHandler(auditRepo, log)

	apiServer := api.NewServer(
		authHandler,
		tournamentHandler,
		programHandler,
		matchHandler,
		gameHandler,
		teamHandler,
		wsHandler,
		systemHandler,
		authService,
		rateLimiter,
		cfg.CORS,
		cfg.RateLimit,
		log,
	).WithAdminChecker(adminChecker).
		WithIdempotency(redisCache).
		WithAuditLog(auditLogger, auditHandler).
		WithSystemStatus(handlers.NewSystemStatusHandler(
			db.NewSystemStatusRepository(database),
			queueManager,
			compileQueue,
			wsHub,
			redisCache,
			log,
		))

	// Создаём HTTP сервер
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           apiServer.Handler(),
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		ReadHeaderTimeout: 5 * time.Second,
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
	// Останавливаем background-горутины (rate-limiter cleanup).
	apiServer.Close()

	// Останавливаем metrics сервер
	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			log.Error("Metrics server forced to shutdown", zap.Error(err))
		}
	}

	// Останавливаем авто-раунд планировщик
	autoRoundScheduler.Stop()

	// Останавливаем Redis event subscriber
	redisEventSub.Stop()

	// Останавливаем WebSocket hub
	cancel()

	log.Info("Servers stopped gracefully")
}
