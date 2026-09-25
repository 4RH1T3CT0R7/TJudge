package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/executor"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/internal/service/rating"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/internal/worker"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// логгер
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

	// метрики
	m := metrics.New()

	// подключение к базе данных
	database, err := storage.New(&cfg.Database, log, m)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	log.Info("Connected to database",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port),
	)

	// проверка здоровья БД
	if err := database.Health(context.Background()); err != nil {
		log.Fatal("Database health check failed", zap.Error(err))
	}

	// обеспечение наличия партиций таблиц matches и rating_history
	if err := database.EnsureMatchPartitions(context.Background()); err != nil {
		log.Error("Failed to ensure match partitions", zap.Error(err))
	}
	if err := database.EnsureRatingHistoryPartitions(context.Background()); err != nil {
		log.Error("Failed to ensure rating_history partitions", zap.Error(err))
	}

	// запуск периодического обслуживания партиций (каждые 24ч)
	database.StartPartitionMaintenance(cfg.Database.PartitionRetentionMonths)

	// подключение к Redis
	redisCache, err := cache.New(&cfg.Redis, log, m)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisCache.Close()

	log.Info("Connected to Redis",
		zap.String("host", cfg.Redis.Host),
		zap.Int("port", cfg.Redis.Port),
	)

	// репозитории
	matchRepo := storage.NewMatchRepository(database)
	ratingRepo := storage.NewRatingRepository(database)
	programRepo := storage.NewProgramRepository(database)

	// кэши с метриками
	matchCache := cache.NewMatchCache(redisCache).WithMetrics(m)
	leaderboardCache := cache.NewLeaderboardCache(redisCache).WithMetrics(m)

	// queue manager
	queueManager := queue.NewQueueManager(redisCache, log, m)

	// нотифаер воркера: результат матча кладётся в кэш лидерборда, и заодно пробрасывается
	// событие в редис чтобы апи разослал его по вебсокету. кэш турниров и вебсокет тут не нужны
	redisEventPub := events.NewRedisEventPublisher(redisCache, log)
	notifier := &events.SyncNotifier{
		Leaderboard: leaderboardCache,
		Redis:       redisEventPub,
		Log:         log,
	}

	// rating service
	ratingService := rating.NewService(ratingRepo, notifier, log)

	// executor с путём к программам
	exec, err := executor.NewExecutor(cfg.Executor, cfg.Storage.ProgramsPath, cfg.Storage.HostProgramsPath, log)
	if err != nil {
		log.Fatal("Failed to create executor", zap.Error(err))
	}
	defer exec.Close()

	// без образов матчи и сборки молча уходили бы в infra-ошибку по кругу,
	// поэтому недостающий образ скачивается, а недоступный - отказ старта
	imageCtx, imageCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	for _, img := range []string{cfg.Executor.DockerImage, cfg.Executor.BuilderImage} {
		if err := exec.EnsureImage(imageCtx, img); err != nil {
			log.Fatal("Docker image is not available", zap.String("image", img), zap.Error(err))
		}
	}
	imageCancel()

	// контейнеры матчей и сборок, брошенные прошлым процессом (SIGKILL на деплое)
	if removed, err := exec.RemoveOrphans(context.Background()); err != nil {
		log.Warn("Failed to remove orphan containers", zap.Error(err))
	} else if removed > 0 {
		log.Info("Removed orphan containers", zap.Int("count", removed))
	}

	log.Info("Executor initialized",
		zap.Int64("cpu_quota", cfg.Executor.CPUQuota),
		zap.Int64("memory_limit", cfg.Executor.MemoryLimit),
		zap.Duration("timeout", cfg.Executor.Timeout),
	)

	// processor
	processor := worker.NewProcessor(
		matchRepo,
		ratingRepo,
		programRepo,
		ratingService,
		exec,
		matchCache,
		log,
	)

	// worker pool
	pool := worker.NewPool(
		cfg.Worker,
		queueManager,
		processor,
		log,
		m,
	)

	// recovery service и восстановление застрявших матчей
	recoveryService := worker.NewRecoveryService(
		matchRepo,
		queueManager,
		log,
		worker.RecoveryConfig{
			// дольше таймаута обработки матч никто не ведёт; с фиксированными 120с
			// при WORKER_TIMEOUT > 120s recovery перезапускал бы живые матчи
			StuckDuration:    cfg.Worker.Timeout + 30*time.Second,
			BatchSize:        1000,
			PeriodicInterval: 60 * time.Second, // Проверка каждые 60 секунд
		},
	)

	// запуск восстановления при старте
	if err := recoveryService.RecoverOnStartup(context.Background()); err != nil {
		log.Error("Failed to recover matches on startup", zap.Error(err))
		// работа продолжается, это не критическая ошибка
	}

	// запуск периодического восстановления
	recoveryService.Start()

	// Outbox-диспетчер: доводит до конца обновления рейтингов, потерянные
	// при сбое между записью результата матча и fast-path обработкой.
	outboxRepo := storage.NewOutboxRepository(database)
	outboxDispatcher := worker.NewOutboxDispatcher(
		outboxRepo,
		matchRepo,
		ratingRepo,
		ratingService,
		notifier,
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
	compileWorker := worker.NewCompileWorker(compileQueue, programRepo, compiler,
		cache.NewDistributedLock(redisCache), notifier, log, cfg.Executor.CompileWorkers)
	compileWorker.Start()

	// запуск worker pool
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

	// ожидание сигнала остановки
	<-quit
	log.Info("Shutting down worker pool...")

	// всё гасится параллельно: пул сразу перестаёт брать матчи, а не ждёт,
	// пока остановятся outbox и сборки (docker шлёт SIGKILL через stop_grace_period)
	var stopWg sync.WaitGroup
	for _, stop := range []func(){recoveryService.Stop, outboxDispatcher.Stop, compileWorker.Stop, pool.Stop} {
		stopWg.Go(stop)
	}
	stopWg.Wait()

	// ожидание завершения worker pool
	pool.Wait()

	// остановка metrics сервера
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
