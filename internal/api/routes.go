package api

import (
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api/handlers"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/observability"
	"github.com/bmstu-itstech/tjudge/internal/web"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/requestid"
	"github.com/go-chi/chi/v5"

	_ "github.com/bmstu-itstech/tjudge/docs/swagger"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Server представляет HTTP сервер
type Server struct {
	router            *chi.Mux
	authHandler       *handlers.AuthHandler
	tournamentHandler *handlers.TournamentHandler
	programHandler    *handlers.ProgramHandler
	matchHandler      *handlers.MatchHandler
	gameHandler       *handlers.GameHandler
	teamHandler       *handlers.TeamHandler
	wsHandler         *handlers.WebSocketHandler
	systemHandler     *handlers.SystemHandler
	auditHandler      *handlers.AuditHandler      // опциональный
	auditLogger       *middleware.AuditLogger     // опциональный
	idempStore        middleware.IdempotencyStore // опциональный
	authService       middleware.AuthService
	rateLimiter       middleware.RateLimiter
	adminChecker      *middleware.VerifiedAdminChecker
	corsConfig        config.CORSConfig
	rateLimitConfig   config.RateLimitConfig
	log               *logger.Logger

	// rateLimitStopCh закрывается при Close(), завершая cleanup-горутины
	// fallback-лимитеров. Без него каждый rebuild через WithXxx() раньше
	// создавал "вечную" горутину на nil-канале (см. ratelimit.go:96).
	rateLimitStopCh chan struct{}
}

// Close останавливает фоновые горутины, созданные Server (например, cleanup-
// горутину fallback-rate-limiter'а). Вызывать после graceful shutdown HTTP.
func (s *Server) Close() {
	if s.rateLimitStopCh != nil {
		select {
		case <-s.rateLimitStopCh:
			// already closed
		default:
			close(s.rateLimitStopCh)
		}
	}
}

// rebuildRouter пересобирает chi-роутер с актуальным набором middleware/routes.
// Используется всеми WithXxx-опциями для переустановки маршрутов с учётом
// установленного состояния (handlers/stores). Перед rebuild останавливает
// горутины предыдущего набора middleware, чтобы избежать goroutine-leak.
func (s *Server) rebuildRouter() {
	// Останавливаем cleanup-горутину предыдущей инстанции rate-limiter.
	s.Close()
	s.rateLimitStopCh = make(chan struct{})
	s.router = chi.NewRouter()
	s.setupMiddleware()
	s.setupRoutes()
}

// NewServer создаёт новый HTTP сервер
func NewServer(
	authHandler *handlers.AuthHandler,
	tournamentHandler *handlers.TournamentHandler,
	programHandler *handlers.ProgramHandler,
	matchHandler *handlers.MatchHandler,
	gameHandler *handlers.GameHandler,
	teamHandler *handlers.TeamHandler,
	wsHandler *handlers.WebSocketHandler,
	systemHandler *handlers.SystemHandler,
	authService middleware.AuthService,
	rateLimiter middleware.RateLimiter,
	corsConfig config.CORSConfig,
	rateLimitConfig config.RateLimitConfig,
	log *logger.Logger,
) *Server {
	s := &Server{
		router:            chi.NewRouter(),
		authHandler:       authHandler,
		tournamentHandler: tournamentHandler,
		programHandler:    programHandler,
		matchHandler:      matchHandler,
		gameHandler:       gameHandler,
		teamHandler:       teamHandler,
		wsHandler:         wsHandler,
		systemHandler:     systemHandler,
		authService:       authService,
		rateLimiter:       rateLimiter,
		corsConfig:        corsConfig,
		rateLimitConfig:   rateLimitConfig,
		log:               log,
		rateLimitStopCh:   make(chan struct{}),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// WithAdminChecker устанавливает проверку admin-роли из БД.
// Если установлен, admin-only роуты будут верифицировать роль из БД (с кешем).
//
// Пересобирает router симметрично остальным опциям; без rebuildRouter опция
// оставалась бы no-op, если после неё не вызывался другой With*.
func (s *Server) WithAdminChecker(checker *middleware.VerifiedAdminChecker) *Server {
	s.adminChecker = checker
	s.rebuildRouter()
	return s
}

// WithIdempotency подключает Idempotency-Key middleware к mutation-эндпоинтам.
// store - обычно *cache.Cache. При nil middleware не применяется.
func (s *Server) WithIdempotency(store middleware.IdempotencyStore) *Server {
	s.idempStore = store
	s.rebuildRouter()
	return s
}

// idempotency возвращает Idempotency middleware или passthrough.
func (s *Server) idempotency() func(http.Handler) http.Handler {
	if s.idempStore == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return middleware.Idempotency(s.idempStore, s.log)
}

// WithAuditLog подключает admin audit log.
// Должен быть вызван до setupRoutes (в main.go). Передаёт опциональные
// компоненты, т.к. без handler'а не будет эндпоинта /admin/audit,
// а без logger'а - не будет middleware записи.
func (s *Server) WithAuditLog(logger *middleware.AuditLogger, handler *handlers.AuditHandler) *Server {
	s.auditLogger = logger
	s.auditHandler = handler
	// Перестраиваем маршруты, чтобы подключить audit middleware к admin-группам.
	s.rebuildRouter()
	return s
}

// setupMiddleware настраивает middleware
func (s *Server) setupMiddleware() {
	// Базовые middleware
	// OTel tracing middleware - no-op, если OTEL_* env не заданы.
	s.router.Use(observability.HTTPMiddleware("tjudge-api"))
	s.router.Use(chiMiddleware.RequestID)
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := chiMiddleware.GetReqID(r.Context())
			if reqID != "" {
				w.Header().Set("X-Request-ID", reqID)
			}
			ctx := requestid.WithContext(r.Context(), reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	s.router.Use(chiMiddleware.RealIP)
	s.router.Use(chiMiddleware.Logger)
	s.router.Use(chiMiddleware.Recoverer)

	// Security-заголовки
	s.router.Use(middleware.SecureHeaders())

	// Сжатие ответов (gzip)
	s.router.Use(middleware.Compress())

	// Smart timeout с контекст cancellation для разных типов операций
	s.router.Use(middleware.SmartTimeout(middleware.DefaultTimeoutConfig()))

	// Rate limiting (если включено в конфиге).
	// Передаём stopCh, чтобы cleanup-горутина fallback-лимитера останавливалась
	// при rebuild/shutdown; без него она зависала на nil-канале.
	if s.rateLimitConfig.Enabled {
		if s.rateLimitStopCh == nil {
			s.rateLimitStopCh = make(chan struct{})
		}
		s.router.Use(middleware.RateLimit(
			s.rateLimiter,
			s.rateLimitConfig.RequestsPerMinute,
			time.Minute,
			s.log,
			s.rateLimitStopCh,
		))
	}

	// CORS с настройками из конфига
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.corsConfig.AllowedOrigins,
		AllowedMethods:   s.corsConfig.AllowedMethods,
		AllowedHeaders:   s.corsConfig.AllowedHeaders,
		ExposedHeaders:   []string{"Link", "X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           s.corsConfig.MaxAge,
	}))

	// CSRF-защита не нужна: JWT хранится в localStorage и шлётся через заголовок
	// Authorization, что делает приложение нечувствительным к CSRF-атакам.
}

// requireAdmin возвращает admin middleware: с проверкой из БД, если сконфигурирована, иначе только JWT.
func (s *Server) requireAdmin() func(http.Handler) http.Handler {
	if s.adminChecker != nil {
		return s.adminChecker.RequireVerifiedAdmin()
	}
	return middleware.RequireAdmin()
}

// auditMiddleware возвращает middleware для записи admin-действий в audit log.
// Если auditLogger не сконфигурирован - возвращает passthrough (no-op).
func (s *Server) auditMiddleware() func(http.Handler) http.Handler {
	if s.auditLogger == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return middleware.Audit(s.auditLogger)
}

// setupRoutes настраивает маршруты
func (s *Server) setupRoutes() {
	// Health check
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Swagger UI (под admin auth)
	s.router.Group(func(r chi.Router) {
		r.Use(middleware.Auth(s.authService, s.log))
		r.Use(s.requireAdmin())
		r.Get("/swagger/*", httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		))
	})

	// pprof за admin-auth. Нужен для диагностики CPU/heap/goroutine в проде.
	// Endpoint'ы: /debug/pprof/{profile,heap,goroutine,allocs,block,mutex,...}.
	// Безопасно за admin, потому что pprof открывает внутреннее состояние процесса.
	s.router.Group(func(r chi.Router) {
		r.Use(middleware.Auth(s.authService, s.log))
		r.Use(s.requireAdmin())
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
		r.Handle("/debug/pprof/{name}", http.HandlerFunc(pprof.Index))
	})

	// Лимит размера тела для JSON-эндпоинтов (1MB). Применяется на уровне группы
	// маршрутов, чтобы file-upload маршруты (/programs) могли задать свой больший лимит.
	bodyLimit := middleware.MaxBodySize(1 << 20)

	// API v1
	s.router.Route("/api/v1", func(r chi.Router) {
		// Auth-маршруты
		r.Route("/auth", func(r chi.Router) {
			r.Use(bodyLimit)
			// Публичные auth-эндпоинты (без аутентификации)
			r.Post("/register", s.authHandler.Register)
			r.Post("/login", s.authHandler.Login)
			r.Post("/refresh", s.authHandler.Refresh)

			// Защищённые auth-эндпоинты (требуют валидный JWT)
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.authService, s.log))
				r.Post("/logout", s.authHandler.Logout)
				r.Get("/me", s.authHandler.Me)
				r.Put("/profile", s.authHandler.UpdateProfile)
			})
		})

		// Маршруты турниров
		r.Route("/tournaments", func(r chi.Router) {
			r.Use(bodyLimit)
			// Публичные маршруты
			r.Get("/", s.tournamentHandler.List)
			r.Get("/{id}", s.tournamentHandler.Get)
			r.Get("/{id}/leaderboard", s.tournamentHandler.GetLeaderboard)
			r.Get("/{id}/cross-game-leaderboard", s.tournamentHandler.GetCrossGameLeaderboard)
			r.Get("/{id}/matches", s.tournamentHandler.GetMatches)
			r.Get("/{id}/matches/rounds", s.tournamentHandler.GetMatchesByRounds)
			r.Get("/{id}/games", s.gameHandler.GetTournamentGames)
			r.Get("/{id}/teams", s.teamHandler.GetTournamentTeams)

			// Эндпоинты для конкретной игры в турнире
			r.Get("/{id}/games/{gameId}/leaderboard", s.gameHandler.GetGameLeaderboard)
			r.Get("/{id}/games/{gameId}/matches", s.gameHandler.GetGameMatches)
			r.Get("/{id}/games/status", s.gameHandler.GetTournamentGamesWithStatus)
			r.Get("/{id}/active-game", s.gameHandler.GetActiveGame)

			// Защищённые маршруты
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.authService, s.log))

				r.Post("/{id}/join", s.tournamentHandler.Join)
				r.Get("/{id}/my-team", s.teamHandler.GetMyTeam)

				// Добавление игры доступно админам или создателю турнира (проверка в handler)
				r.Post("/{id}/games", s.gameHandler.AddGameToTournament)

				// Админские маршруты для турниров
				r.Group(func(r chi.Router) {
					r.Use(s.requireAdmin())
					r.Use(s.auditMiddleware())
					// Idempotency-Key на create-эндпоинтах защищает от
					// двойного создания при retry.
					r.With(s.idempotency()).Post("/", s.tournamentHandler.Create)
					r.Post("/{id}/start", s.tournamentHandler.Start)
					r.Post("/{id}/complete", s.tournamentHandler.Complete)
					r.Post("/{id}/matches", s.tournamentHandler.CreateMatch)
					r.Delete("/{id}", s.tournamentHandler.Delete)
					r.Delete("/{id}/games/{gameId}", s.gameHandler.RemoveGameFromTournament)
					r.Get("/{id}/games/{gameId}/programs", s.gameHandler.GetGamePrograms)
					r.Get("/{id}/programs/download-zip", s.gameHandler.DownloadAllPrograms)
					r.Post("/{id}/games/{gameId}/complete-round", s.gameHandler.MarkGameRoundCompleted)
					r.Post("/{id}/games/{gameId}/reset-round", s.gameHandler.ResetGameRound)
					r.Post("/{id}/games/{gameId}/auto-round", s.gameHandler.SetAutoRound)
					r.Get("/{id}/games/{gameId}/auto-round", s.gameHandler.GetAutoRound)
					r.Post("/{id}/active-game", s.gameHandler.SetActiveGame)
					r.Post("/{id}/games/deactivate-all", s.gameHandler.DeactivateAllGames)
					r.Post("/{id}/run-matches", s.tournamentHandler.RunAllMatches)
					r.Post("/{id}/run-game-matches", s.tournamentHandler.RunGameMatches)
					r.Post("/{id}/retry-matches", s.tournamentHandler.RetryFailedMatches)
					r.Post("/{id}/programs/clear-errors", s.programHandler.ClearProgramErrors)
				})
			})
		})

		// Маршруты игр
		r.Route("/games", func(r chi.Router) {
			r.Use(bodyLimit)
			// Публичные маршруты. Read-only кэшируются 60с + ETag.
			r.With(middleware.CacheControl(60)).Get("/", s.gameHandler.List)
			r.With(middleware.CacheControl(60)).Get("/{id}", s.gameHandler.Get)
			r.With(middleware.CacheControl(60)).Get("/name/{name}", s.gameHandler.GetByName)

			// Админские маршруты
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.authService, s.log))
				r.Use(s.requireAdmin())
				r.Use(s.auditMiddleware())

				r.Post("/", s.gameHandler.Create)
				r.Put("/{id}", s.gameHandler.Update)
				r.Delete("/{id}", s.gameHandler.Delete)
			})
		})

		// Маршруты команд
		r.Route("/teams", func(r chi.Router) {
			r.Use(bodyLimit)
			r.Use(middleware.Auth(s.authService, s.log))

			r.Post("/", s.teamHandler.Create)
			r.Post("/join", s.teamHandler.JoinByCode)
			r.Get("/{id}", s.teamHandler.Get)
			r.Put("/{id}", s.teamHandler.UpdateName)
			r.Get("/{id}/members", s.teamHandler.GetMembers)
			r.Post("/{id}/leave", s.teamHandler.Leave)
			r.Delete("/{id}/members/{userId}", s.teamHandler.RemoveMember)
			r.Get("/{id}/invite", s.teamHandler.GetInviteLink)

			// Админские маршруты
			r.Group(func(r chi.Router) {
				r.Use(s.requireAdmin())
				r.Use(s.auditMiddleware())
				r.Delete("/{id}", s.teamHandler.Delete)
				r.Post("/{id}/disqualify", s.teamHandler.Disqualify)
				r.Post("/{id}/restore", s.teamHandler.Restore)
			})
		})

		// Program routes (все требуют аутентификации)
		r.Route("/programs", func(r chi.Router) {
			r.Use(middleware.Auth(s.authService, s.log))
			r.Use(middleware.MaxBodySize(10 << 20)) // 10MB for file uploads

			// Idempotency-Key на upload: клиент с флейки-сетью не создаст дубль.
			r.With(s.idempotency()).Post("/", s.programHandler.Create)
			r.Get("/", s.programHandler.List)
			r.Get("/versions", s.programHandler.GetVersions) // Список версий программ команды
			r.Get("/{id}", s.programHandler.Get)
			r.Get("/{id}/download", s.programHandler.Download)
			r.Put("/{id}", s.programHandler.Update)
			r.Delete("/{id}", s.programHandler.Delete)
		})

		// Маршруты матчей
		r.Route("/matches", func(r chi.Router) {
			r.Use(bodyLimit)
			// Публичные маршруты с опциональной аутентификацией
			// (если пользователь авторизован, покажет полные ошибки для админов)
			r.Group(func(r chi.Router) {
				r.Use(middleware.OptionalAuth(s.authService, s.log))
				r.Get("/", s.matchHandler.List)
				r.Get("/statistics", s.matchHandler.GetStatistics)
				r.Get("/{id}", s.matchHandler.Get)
			})

			// Админские маршруты для управления очередью матчей
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.authService, s.log))
				r.Use(s.requireAdmin())
				r.Use(s.auditMiddleware())

				r.Get("/queue/stats", s.matchHandler.GetQueueStats)
				r.Post("/queue/clear", s.matchHandler.ClearQueue)
				r.Post("/queue/purge", s.matchHandler.PurgeInvalidMatches)
			})
		})

		// WebSocket маршруты (требуется аутентификация)
		r.Route("/ws", func(r chi.Router) {
			r.Use(middleware.Auth(s.authService, s.log))

			r.Get("/tournaments/{id}", s.wsHandler.HandleTournament)
			r.Get("/stats", s.wsHandler.GetStats)
		})

		// Системные маршруты (только для админов)
		r.Route("/system", func(r chi.Router) {
			r.Use(bodyLimit)
			r.Use(middleware.Auth(s.authService, s.log))
			r.Use(s.requireAdmin())

			r.Get("/metrics", s.systemHandler.GetMetrics)
			r.Get("/health", s.systemHandler.GetHealth)
		})

		// Admin-only audit log.
		// Эндпоинт подключается, только если в сервере зарегистрирован auditHandler.
		if s.auditHandler != nil {
			r.Route("/admin", func(r chi.Router) {
				r.Use(bodyLimit)
				r.Use(middleware.Auth(s.authService, s.log))
				r.Use(s.requireAdmin())

				r.Get("/audit", s.auditHandler.List)
			})
		}
	})

	// Раздаём статику фронтенда (SPA с fallback на index.html)
	s.router.Handle("/*", web.Handler())
}

// Handler возвращает HTTP handler
func (s *Server) Handler() http.Handler {
	return s.router
}

// ServeHTTP реализует интерфейс http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
