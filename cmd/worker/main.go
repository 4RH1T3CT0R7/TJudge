package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain/rating"
	"github.com/bmstu-itstech/tjudge/internal/events"
	eventhandlers "github.com/bmstu-itstech/tjudge/internal/events/handlers"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/executor"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/internal/worker"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

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

	log.Info("Starting TJudge Worker",
		zap.Int("min_workers", cfg.Worker.MinWorkers),
		zap.Int("max_workers", cfg.Worker.MaxWorkers),
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

	// Обеспечиваем наличие партиций таблицы matches и rating_history
	if err := database.EnsureMatchPartitions(context.Background()); err != nil {
		log.Error("Failed to ensure match partitions", zap.Error(err))
	}
	if err := database.EnsureRatingHistoryPartitions(context.Background()); err != nil {
		log.Error("Failed to ensure rating_history partitions", zap.Error(err))
	}

	// Запускаем периодическое обслуживание партиций (каждые 24ч)
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
	matchRepo := db.NewMatchRepository(database)
	ratingRepo := db.NewRatingRepository(database)
	programRepo := db.NewProgramRepository(database)

	// Инициализируем кэши с метриками
	matchCache := cache.NewMatchCache(redisCache).WithMetrics(m)
	leaderboardCache := cache.NewLeaderboardCache(redisCache).WithMetrics(m)

	// Инициализируем queue manager
	queueManager := queue.NewQueueManager(redisCache, log, m)

	// Инициализируем event bus для worker
	eventBus := events.NewSyncBus(log)
	eventBus.Subscribe(
		eventhandlers.NewLeaderboardCacheHandler(leaderboardCache, log),
		events.MatchResultProcessed{},
	)

	// Мост Redis Pub/Sub: пересылает события в процесс API для рассылки по WebSocket.
	redisEventPub := events.NewRedisEventPublisher(redisCache, log)
	eventBus.Subscribe(
		redisEventPub,
		events.MatchResultProcessed{}, events.ProgramCompiled{},
	)

	// Инициализируем rating service
	ratingService := rating.NewService(ratingRepo, eventBus, log)

	// Проверяем наличие образа tjudge-cli
	checkTJudgeCLIImage(log)

	// Проверяем наличие builder-образа для компиляции программ (warn-only:
	// без него compile-задачи будут копиться, stuck-recovery повторит их
	// после появления образа).
	if !imageExists(cfg.Executor.BuilderImage, log) {
		log.Warn("Builder image not found - compile tasks will wait until it is available",
			zap.String("image", cfg.Executor.BuilderImage),
			zap.String("hint", "make docker-build-builder"),
		)
	}

	// Инициализируем executor с путём к программам
	exec, err := executor.NewExecutor(cfg.Executor, cfg.Storage.ProgramsPath, cfg.Storage.HostProgramsPath, log)
	if err != nil {
		log.Fatal("Failed to create executor", zap.Error(err))
	}
	defer exec.Close()

	log.Info("Executor initialized",
		zap.Int64("cpu_quota", cfg.Executor.CPUQuota),
		zap.Int64("memory_limit", cfg.Executor.MemoryLimit),
		zap.Duration("timeout", cfg.Executor.Timeout),
	)

	// Инициализируем processor
	processor := worker.NewProcessor(
		matchRepo,
		ratingRepo,
		programRepo,
		ratingService,
		exec,
		matchCache,
		log,
	)

	// Инициализируем worker pool
	pool := worker.NewPool(
		cfg.Worker,
		queueManager,
		processor,
		log,
		m,
	)

	// Инициализируем recovery service и восстанавливаем застрявшие матчи
	recoveryService := worker.NewRecoveryService(
		matchRepo,
		queueManager,
		log,
		worker.RecoveryConfig{
			StuckDuration:    120 * time.Second, // Матч считается застрявшим после 120 секунд (> worker timeout 90s)
			BatchSize:        1000,
			PeriodicInterval: 60 * time.Second, // Проверка каждые 60 секунд
		},
	)

	// Запускаем восстановление при старте
	if err := recoveryService.RecoverOnStartup(context.Background()); err != nil {
		log.Error("Failed to recover matches on startup", zap.Error(err))
		// Продолжаем работу, это не критическая ошибка
	}

	// Запускаем периодическое восстановление
	recoveryService.Start()

	// Outbox-диспетчер: доводит до конца обновления рейтингов, потерянные
	// при сбое между записью результата матча и fast-path обработкой.
	outboxRepo := db.NewOutboxRepository(database)
	outboxDispatcher := worker.NewOutboxDispatcher(
		outboxRepo,
		matchRepo,
		ratingRepo,
		ratingService,
		eventBus,
		log,
	)
	outboxDispatcher.Start()

	// Compile-worker: асинхронная компиляция загруженных программ
	// в Docker-песочнице (builder-образ с тулчейнами).
	compiler, err := executor.NewCompiler(
		cfg.Executor.BuilderImage,
		cfg.Storage.ProgramsPath,
		cfg.Storage.HostProgramsPath,
		cfg.Executor.CompileTimeout,
		log,
	)
	if err != nil {
		log.Fatal("Failed to create sandbox compiler", zap.Error(err))
	}
	defer compiler.Close()

	compileQueue := queue.NewCompileQueue(redisCache, log)
	compileWorker := worker.NewCompileWorker(compileQueue, programRepo, compiler, eventBus, log)
	compileWorker.Start()

	// Запускаем worker pool
	pool.Start()
	log.Info("Worker pool started",
		zap.Int("initial_workers", cfg.Worker.MinWorkers),
	)

	// Metrics server (если включен)
	var metricsSrv *http.Server
	if cfg.Metrics.Enabled {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())

		// Health check endpoint для worker
		metricsMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

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

	// Ждём сигнала остановки
	<-quit
	log.Info("Shutting down worker pool...")

	// Останавливаем recovery service
	recoveryService.Stop()

	// Останавливаем outbox-диспетчер
	outboxDispatcher.Stop()

	// Останавливаем compile-worker
	compileWorker.Stop()

	// Останавливаем worker pool
	pool.Stop()

	// Ждём завершения worker pool
	pool.Wait()

	// Останавливаем metrics сервер
	if metricsSrv != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			log.Error("Metrics server forced to shutdown", zap.Error(err))
		}
	}

	log.Info("Worker pool stopped gracefully",
		zap.Int64("total_matches_processed", pool.GetMatchesProcessed()),
	)
}

// checkTJudgeCLIImage проверяет наличие Docker образа tjudge-cli и пытается его собрать
func checkTJudgeCLIImage(log *logger.Logger) {
	const imageName = "tjudge-cli:latest"

	// Проверяем наличие образа
	if imageExists(imageName, log) {
		log.Info("Docker image tjudge-cli verified",
			zap.String("image", imageName),
		)
		return
	}

	log.Warn("Docker image tjudge-cli:latest not found, attempting to build...",
		zap.String("image", imageName),
	)

	// Пытаемся собрать образ через docker compose
	if tryBuildWithCompose(log) {
		if imageExists(imageName, log) {
			log.Info("Docker image tjudge-cli built successfully",
				zap.String("image", imageName),
			)
			return
		}
	}

	// Пытаемся собрать напрямую через docker build
	if tryBuildDirectly(log) {
		if imageExists(imageName, log) {
			log.Info("Docker image tjudge-cli built successfully",
				zap.String("image", imageName),
			)
			return
		}
	}

	log.Error("Failed to build tjudge-cli image!",
		zap.String("image", imageName),
		zap.String("hint", "Run 'docker compose build tjudge-cli' manually or check docker/tjudge/Dockerfile"),
	)
	log.Warn("Worker will fail to execute matches without tjudge-cli image")
}

// imageExists проверяет существование Docker образа
func imageExists(imageName string, log *logger.Logger) bool {
	// #nosec G204 -- "docker" и "images -q" hardcoded; imageName приходит
	// из config/env (EXECUTOR_DOCKER_IMAGE), не от внешнего input.
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.Output()
	if err != nil {
		log.Debug("Failed to check image existence",
			zap.Error(err),
			zap.String("image", imageName),
		)
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// tryBuildWithCompose пытается собрать образ через docker compose
func tryBuildWithCompose(log *logger.Logger) bool {
	log.Info("Trying to build tjudge-cli with docker compose...")

	// Проверяем возможные пути к docker-compose.yml
	composePaths := []string{
		"docker-compose.yml",
		"../docker-compose.yml",
		"../../docker-compose.yml",
		"/app/docker-compose.yml",
	}

	for _, path := range composePaths {
		if _, err := os.Stat(path); err == nil {
			// #nosec G204 -- "docker compose" hardcoded; path - hardcoded список
			// composePaths, не из внешнего input.
			cmd := exec.Command("docker", "compose", "-f", path, "build", "tjudge-cli")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			log.Info("Running docker compose build",
				zap.String("compose_file", path),
			)

			if err := cmd.Run(); err != nil {
				log.Debug("Docker compose build failed",
					zap.Error(err),
					zap.String("path", path),
				)
				continue
			}
			return true
		}
	}

	return false
}

// tryBuildDirectly пытается собрать образ напрямую через docker build
func tryBuildDirectly(log *logger.Logger) bool {
	log.Info("Trying to build tjudge-cli with docker build...")

	// Проверяем возможные пути к Dockerfile
	dockerfilePaths := []struct {
		dockerfile string
		context    string
	}{
		{"docker/tjudge/Dockerfile", "."},
		{"../docker/tjudge/Dockerfile", ".."},
		{"../../docker/tjudge/Dockerfile", "../.."},
		{"/app/docker/tjudge/Dockerfile", "/app"},
	}

	for _, paths := range dockerfilePaths {
		if _, err := os.Stat(paths.dockerfile); err == nil {
			// #nosec G204 -- все args hardcoded (docker, build, -t, tag);
			// paths.dockerfile/context - hardcoded struct-literal.
			cmd := exec.Command("docker", "build",
				"-t", "tjudge-cli:latest",
				"-f", paths.dockerfile,
				paths.context,
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			log.Info("Running docker build",
				zap.String("dockerfile", paths.dockerfile),
				zap.String("context", paths.context),
			)

			if err := cmd.Run(); err != nil {
				log.Debug("Docker build failed",
					zap.Error(err),
					zap.String("dockerfile", paths.dockerfile),
				)
				continue
			}
			return true
		}
	}

	return false
}
