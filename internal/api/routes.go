package api

import (
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/handlers"
	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/observability"
	"github.com/bmstu-itstech/tjudge/internal/web"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"

	_ "github.com/bmstu-itstech/tjudge/docs/swagger"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Server - http сервер со всеми хендлерами и роутером
type Server struct {
	router               *chi.Mux
	authHandler          *handlers.AuthHandler
	tournamentHandler    *handlers.TournamentHandler
	programHandler       *handlers.ProgramHandler
	matchHandler         *handlers.MatchHandler
	gameHandler          *handlers.GameHandler
	teamHandler          *handlers.TeamHandler
	wsHandler            *handlers.WebSocketHandler
	systemHandler        *handlers.SystemHandler
	statusHandler        *handlers.SystemStatusHandler   // может быть nil - тогда нет GET /system/status
	ratingHistoryHandler *handlers.RatingHistoryHandler  // может быть nil
	recoveryHandler      *handlers.SystemRecoveryHandler // может быть nil
	auditHandler         *handlers.AuditHandler          // может быть nil
	auditLogger          *middleware.AuditLogger         // может быть nil - тогда admin-действия не пишутся
	idempStore           middleware.IdempotencyStore     // может быть nil
	authService          middleware.AuthService
	rateLimiter          middleware.RateLimiter
	adminChecker         *middleware.VerifiedAdminChecker
	corsConfig           config.CORSConfig
	rateLimitConfig      config.RateLimitConfig
	log                  *logger.Logger

	// закрывается в Close() и гасит cleanup-горутину fallback-лимитера,
	// без этого она висела бы вечно (см. ratelimit.go)
	rateLimitStopCh chan struct{}
}

// ServerDeps - все зависимости сервера одной пачкой. опциональные поля можно
// оставить nil, соответствующие роуты/мидлвари просто не подключатся
type ServerDeps struct {
	AuthHandler          *handlers.AuthHandler
	TournamentHandler    *handlers.TournamentHandler
	ProgramHandler       *handlers.ProgramHandler
	MatchHandler         *handlers.MatchHandler
	GameHandler          *handlers.GameHandler
	TeamHandler          *handlers.TeamHandler
	WSHandler            *handlers.WebSocketHandler
	SystemHandler        *handlers.SystemHandler
	StatusHandler        *handlers.SystemStatusHandler   // опционально
	RatingHistoryHandler *handlers.RatingHistoryHandler  // опционально
	RecoveryHandler      *handlers.SystemRecoveryHandler // опционально
	AuditHandler         *handlers.AuditHandler          // опционально
	AuditLogger          *middleware.AuditLogger         // опционально
	IdempStore           middleware.IdempotencyStore     // опционально
	AuthService          middleware.AuthService
	RateLimiter          middleware.RateLimiter
	AdminChecker         *middleware.VerifiedAdminChecker // опционально, без него админ проверяется только по jwt
	CORS                 config.CORSConfig
	RateLimit            config.RateLimitConfig
	Log                  *logger.Logger
}

// NewServer собирает сервер целиком за один вызов. раньше тут был билдер
// с WithXxx-методами и пересборкой роутера на каждую опцию - выкинул,
// проще передать всё сразу и один раз настроить роуты
func NewServer(deps ServerDeps) *Server {
	s := &Server{
		router:               chi.NewRouter(),
		authHandler:          deps.AuthHandler,
		tournamentHandler:    deps.TournamentHandler,
		programHandler:       deps.ProgramHandler,
		matchHandler:         deps.MatchHandler,
		gameHandler:          deps.GameHandler,
		teamHandler:          deps.TeamHandler,
		wsHandler:            deps.WSHandler,
		systemHandler:        deps.SystemHandler,
		statusHandler:        deps.StatusHandler,
		ratingHistoryHandler: deps.RatingHistoryHandler,
		recoveryHandler:      deps.RecoveryHandler,
		auditHandler:         deps.AuditHandler,
		auditLogger:          deps.AuditLogger,
		idempStore:           deps.IdempStore,
		authService:          deps.AuthService,
		rateLimiter:          deps.RateLimiter,
		adminChecker:         deps.AdminChecker,
		corsConfig:           deps.CORS,
		rateLimitConfig:      deps.RateLimit,
		log:                  deps.Log,
		rateLimitStopCh:      make(chan struct{}),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// Close гасит фоновые горутины сервера (cleanup-горутину рейтлимитера).
// звать после graceful shutdown http. повторный вызов безопасен
func (s *Server) Close() {
	if s.rateLimitStopCh != nil {
		select {
		case <-s.rateLimitStopCh:
			// уже закрыт
		default:
			close(s.rateLimitStopCh)
		}
	}
}

// idempotency отдаёт Idempotency-мидлварь или пустышку если стор не задан
func (s *Server) idempotency() func(http.Handler) http.Handler {
	if s.idempStore == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return middleware.Idempotency(s.idempStore, s.log)
}

// auth - Auth плюс перепроверка админской роли по базе (если чекер задан),
// чтобы проверки админа в хендлерах не верили одному jwt
func (s *Server) auth() func(http.Handler) http.Handler {
	return s.withVerifiedRole(middleware.Auth(s.authService, s.log))
}

// optionalAuth - то же для публичных ручек с необязательным токеном
func (s *Server) optionalAuth() func(http.Handler) http.Handler {
	return s.withVerifiedRole(middleware.OptionalAuth(s.authService, s.log))
}

func (s *Server) withVerifiedRole(authMW func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	if s.adminChecker == nil {
		return authMW
	}
	verify := s.adminChecker.VerifyRole()
	return func(next http.Handler) http.Handler { return authMW(verify(next)) }
}

// requireAdmin - проверка админа из бд если чекер задан, иначе только по jwt
func (s *Server) requireAdmin() func(http.Handler) http.Handler {
	if s.adminChecker != nil {
		return s.adminChecker.RequireVerifiedAdmin()
	}
	return middleware.RequireAdmin()
}

// строгий лимит на логин и регистрацию с одного ip в минуту: bcrypt дорогой,
// а без него пароли перебирались бы со скоростью общего лимита
const authRateLimit = 30

// rateLimit - лимитер со своим счётчиком name, пустышка если лимит выключен.
// stopCh гасит cleanup-горутину фолбэк-лимитера при Close
func (s *Server) rateLimit(name string, limit int) func(http.Handler) http.Handler {
	if !s.rateLimitConfig.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}
	return middleware.RateLimit(s.rateLimiter, name, limit, time.Minute, s.authService, s.log, s.rateLimitStopCh)
}

// auditMiddleware пишет admin-действия в аудит-лог, без логгера - пустышка
func (s *Server) auditMiddleware() func(http.Handler) http.Handler {
	if s.auditLogger == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return middleware.Audit(s.auditLogger)
}

// setupMiddleware подключает мидлвари. порядок важен: RealIP должен идти
// до рейтлимита и аудита (они читают ip из RemoteAddr), Recoverer оборачивает
// всё что ниже, CORS последним. рейтлимит висит на /api/v1 в setupRoutes
func (s *Server) setupMiddleware() {
	// otel-трейсинг, no-op если OTEL_* не заданы
	s.router.Use(observability.HTTPMiddleware("tjudge-api"))
	s.router.Use(chiMiddleware.RequestID)
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := chiMiddleware.GetReqID(r.Context())
			if reqID != "" {
				w.Header().Set("X-Request-ID", reqID)
			}
			ctx := middleware.WithRequestID(r.Context(), reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	// prometheus-метрики http. раньше был только otel и панели по http пустовали
	s.router.Use(middleware.Metrics())
	s.router.Use(middleware.RealIP(s.rateLimitConfig.TrustedProxies))
	s.router.Use(chiMiddleware.Logger)
	s.router.Use(chiMiddleware.Recoverer)

	s.router.Use(middleware.SecureHeaders())

	s.router.Use(middleware.Compress())

	s.router.Use(middleware.SmartTimeout(middleware.DefaultTimeoutConfig()))

	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.corsConfig.AllowedOrigins,
		AllowedMethods:   s.corsConfig.AllowedMethods,
		AllowedHeaders:   s.corsConfig.AllowedHeaders,
		ExposedHeaders:   []string{"Link", "X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           s.corsConfig.MaxAge,
	}))

	// csrf-защита не нужна: jwt лежит в localStorage и ходит в заголовке
	// Authorization, куки не используются
}

// setupRoutes вешает все маршруты
func (s *Server) setupRoutes() {
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// swagger только под админом
	s.router.Group(func(r chi.Router) {
		r.Use(s.auth())
		r.Use(s.requireAdmin())
		r.Get("/swagger/*", httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		))
	})

	// pprof тоже за админом - он раскрывает внутренности процесса,
	// но для диагностики cpu/heap в проде вещь незаменимая
	s.router.Group(func(r chi.Router) {
		r.Use(s.auth())
		r.Use(s.requireAdmin())
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
		r.Handle("/debug/pprof/{name}", http.HandlerFunc(pprof.Index))
	})

	// лимит тела 1мб для json-ручек, у /programs свой лимит побольше (файлы)
	bodyLimit := middleware.MaxBodySize(1 << 20)

	s.router.Route("/api/v1", func(r chi.Router) {
		// статика spa и /health под лимит не попадают: они дешёвые, а в общем
		// счётчике выбивали бы лимит всей аудитории
		r.Use(s.rateLimit("api", s.rateLimitConfig.RequestsPerMinute))

		r.Route("/auth", func(r chi.Router) {
			r.Use(bodyLimit)
			// публичные
			authLimit := s.rateLimit("auth", authRateLimit)
			r.With(authLimit).Post("/register", s.authHandler.Register)
			r.With(authLimit).Post("/login", s.authHandler.Login)
			r.Post("/refresh", s.authHandler.Refresh)

			// под токеном
			r.Group(func(r chi.Router) {
				r.Use(s.auth())
				r.Post("/logout", s.authHandler.Logout)
				r.Get("/me", s.authHandler.Me)
				r.Put("/profile", s.authHandler.UpdateProfile)
			})
		})

		r.Route("/tournaments", func(r chi.Router) {
			r.Use(bodyLimit)
			// публичные
			r.Get("/", s.tournamentHandler.List)
			r.Get("/{id}", s.tournamentHandler.Get)
			r.Get("/{id}/leaderboard", s.tournamentHandler.GetLeaderboard)
			r.Get("/{id}/cross-game-leaderboard", s.tournamentHandler.GetCrossGameLeaderboard)
			r.Get("/{id}/matches", s.tournamentHandler.GetMatches)
			r.Get("/{id}/matches/rounds", s.tournamentHandler.GetMatchesByRounds)
			r.Get("/{id}/games", s.gameHandler.GetTournamentGames)
			r.Get("/{id}/teams", s.teamHandler.GetTournamentTeams)

			// по конкретной игре турнира
			r.Get("/{id}/games/{gameId}/leaderboard", s.gameHandler.GetGameLeaderboard)
			r.Get("/{id}/games/{gameId}/head-to-head", s.gameHandler.GetHeadToHead)
			r.Get("/{id}/games/{gameId}/matches", s.gameHandler.GetGameMatches)
			r.Get("/{id}/games/status", s.gameHandler.GetTournamentGamesWithStatus)
			r.Get("/{id}/active-game", s.gameHandler.GetActiveGame)
			if s.ratingHistoryHandler != nil {
				r.Get("/{id}/programs/{programId}/rating-history", s.ratingHistoryHandler.GetProgramRatingHistory)
			}

			r.Group(func(r chi.Router) {
				r.Use(s.auth())

				r.Get("/{id}/my-team", s.teamHandler.GetMyTeam)

				// добавить игру может админ или создатель турнира, проверка в хендлере
				r.With(s.auditMiddleware()).Post("/{id}/games", s.gameHandler.AddGameToTournament)

				// админские
				r.Group(func(r chi.Router) {
					r.Use(s.requireAdmin())
					r.Use(s.auditMiddleware())
					// Idempotency-Key на создании - ретрай не даст дубль
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

		r.Route("/games", func(r chi.Router) {
			r.Use(bodyLimit)
			// публичные read-only кэшируются на минуту с etag
			r.With(middleware.CacheControl(60)).Get("/", s.gameHandler.List)
			r.With(middleware.CacheControl(60)).Get("/{id}", s.gameHandler.Get)
			r.With(middleware.CacheControl(60)).Get("/name/{name}", s.gameHandler.GetByName)

			r.Group(func(r chi.Router) {
				r.Use(s.auth())
				r.Use(s.requireAdmin())
				r.Use(s.auditMiddleware())

				r.Post("/", s.gameHandler.Create)
				r.Put("/{id}", s.gameHandler.Update)
				r.Delete("/{id}", s.gameHandler.Delete)
			})
		})

		r.Route("/teams", func(r chi.Router) {
			r.Use(bodyLimit)
			r.Use(s.auth())

			r.Post("/", s.teamHandler.Create)
			r.Post("/join", s.teamHandler.JoinByCode)
			r.Get("/{id}", s.teamHandler.Get)
			r.Put("/{id}", s.teamHandler.UpdateName)
			r.Get("/{id}/members", s.teamHandler.GetMembers)
			r.Post("/{id}/leave", s.teamHandler.Leave)
			r.Delete("/{id}/members/{userId}", s.teamHandler.RemoveMember)
			r.Get("/{id}/invite", s.teamHandler.GetInviteLink)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAdmin())
				r.Use(s.auditMiddleware())
				r.Delete("/{id}", s.teamHandler.Delete)
				r.Post("/{id}/disqualify", s.teamHandler.Disqualify)
				r.Post("/{id}/restore", s.teamHandler.Restore)
			})
		})

		// программы - всё под токеном, лимит тела больше из-за загрузки файлов
		r.Route("/programs", func(r chi.Router) {
			r.Use(s.auth())
			r.Use(middleware.MaxBodySize(10 << 20)) // 10MB for file uploads

			// Idempotency-Key на аплоаде: клиент с флаки-сетью не создаст дубль
			r.With(s.idempotency()).Post("/", s.programHandler.Create)
			r.Get("/", s.programHandler.List)
			r.Get("/versions", s.programHandler.GetVersions) // Список версий программ команды
			r.Get("/{id}", s.programHandler.Get)
			r.Get("/{id}/download", s.programHandler.Download)
			r.Delete("/{id}", s.programHandler.Delete)
		})

		r.Route("/matches", func(r chi.Router) {
			r.Use(bodyLimit)
			// публичные с опциональной авторизацией - админ увидит полные ошибки
			r.Group(func(r chi.Router) {
				r.Use(s.optionalAuth())
				r.Get("/", s.matchHandler.List)
				r.Get("/statistics", s.matchHandler.GetStatistics)
				r.Get("/{id}", s.matchHandler.Get)
			})

			// управление очередью - только админ
			r.Group(func(r chi.Router) {
				r.Use(s.auth())
				r.Use(s.requireAdmin())
				r.Use(s.auditMiddleware())

				r.Get("/queue/stats", s.matchHandler.GetQueueStats)
				r.Post("/queue/clear", s.matchHandler.ClearQueue)
				r.Post("/queue/purge", s.matchHandler.PurgeInvalidMatches)
			})
		})

		r.Route("/ws", func(r chi.Router) {
			r.Use(s.auth())

			r.Get("/tournaments/{id}", s.wsHandler.HandleTournament)
			r.Get("/stats", s.wsHandler.GetStats)
		})

		r.Route("/system", func(r chi.Router) {
			r.Use(bodyLimit)
			r.Use(s.auth())
			r.Use(s.requireAdmin())
			// кнопки восстановления ниже - самые инвазивные действия оператора
			r.Use(s.auditMiddleware())

			r.Get("/metrics", s.systemHandler.GetMetrics)
			r.Get("/health", s.systemHandler.GetHealth)

			if s.statusHandler != nil {
				r.Get("/status", s.statusHandler.GetFullStatus)
			}

			// кнопки восстановления из админки
			if s.recoveryHandler != nil {
				r.Post("/recovery/outbox-retry", s.recoveryHandler.RetryOutboxErrors)
				r.Post("/recovery/requeue-compiling", s.recoveryHandler.RequeueCompiling)
				r.Post("/recovery/reset-stuck-matches", s.recoveryHandler.ResetStuckMatches)
				r.Post("/recovery/clear-dead-letter", s.recoveryHandler.ClearDeadLetter)
			}
		})

		if s.auditHandler != nil {
			r.Route("/admin", func(r chi.Router) {
				r.Use(bodyLimit)
				r.Use(s.auth())
				r.Use(s.requireAdmin())

				r.Get("/audit", s.auditHandler.List)
			})
		}
	})

	// статика фронта, spa с фолбэком на index.html
	s.router.Handle("/*", web.Handler())
}

// Handler возвращает http handler
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
