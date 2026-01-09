package api

import (
	"net/http"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api/handlers"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/web"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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
	authService       middleware.AuthService
	rateLimiter       middleware.RateLimiter
	corsConfig        config.CORSConfig
	rateLimitConfig   config.RateLimitConfig
	log               *logger.Logger
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
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// setupMiddleware настраивает middleware
func (s *Server) setupMiddleware() {
	// Базовые middleware
	s.router.Use(chiMiddleware.RequestID)
	s.router.Use(chiMiddleware.RealIP)
	s.router.Use(chiMiddleware.Logger)
	s.router.Use(chiMiddleware.Recoverer)

	// Security headers
	s.router.Use(middleware.SecureHeaders())

	// Response compression (gzip)
	s.router.Use(middleware.Compress())

	// Smart timeout с контекст cancellation для разных типов операций
	s.router.Use(middleware.SmartTimeout(middleware.DefaultTimeoutConfig()))

	// Rate limiting (если включено в конфиге)
	if s.rateLimitConfig.Enabled {
		s.router.Use(middleware.RateLimit(
			s.rateLimiter,
			s.rateLimitConfig.RequestsPerMinute,
			time.Minute,
			s.log,
		))
	}

	// CORS с настройками из конфига
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.corsConfig.AllowedOrigins,
		AllowedMethods:   s.corsConfig.AllowedMethods,
		AllowedHeaders:   s.corsConfig.AllowedHeaders,
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           s.corsConfig.MaxAge,
	}))
}

// setupRoutes настраивает маршруты
func (s *Server) setupRoutes() {
	// Health check
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// API v1
	s.router.Route("/api/v1", func(r chi.Router) {
		// Auth routes (публичные)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.authHandler.Register)
			r.Post("/login", s.authHandler.Login)
			r.Post("/refresh", s.authHandler.Refresh)
			r.Post("/logout", s.authHandler.Logout)
			r.Get("/me", s.authHandler.Me)
			r.Put("/profile", s.authHandler.UpdateProfile)
		})

		// Tournament routes
		r.Route("/tournaments", func(r chi.Router) {
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

			// Защищённые маршруты
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.authService, s.log))

				r.Post("/", s.tournamentHandler.Create)
				r.Post("/{id}/join", s.tournamentHandler.Join)
				r.Post("/{id}/start", s.tournamentHandler.Start)
				r.Post("/{id}/complete", s.tournamentHandler.Complete)
				r.Post("/{id}/matches", s.tournamentHandler.CreateMatch)
				r.Get("/{id}/my-team", s.teamHandler.GetMyTeam)

				// Добавление игры доступно админам или создателю турнира (проверка в handler)
				r.Post("/{id}/games", s.gameHandler.AddGameToTournament)

				// Админские маршруты для турниров
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireAdmin())
					r.Delete("/{id}", s.tournamentHandler.Delete)
					r.Delete("/{id}/games/{gameId}", s.gameHandler.RemoveGameFromTournament)
					r.Get("/{id}/games/{gameId}/programs", s.gameHandler.GetGamePrograms)
					r.Post("/{id}/games/{gameId}/complete-round", s.gameHandler.MarkGameRoundCompleted)
					r.Post("/{id}/run-matches", s.tournamentHandler.RunAllMatches)
					r.Post("/{id}/run-game-matches", s.tournamentHandler.RunGameMatches)
					r.Post("/{id}/retry-matches", s.tournamentHandler.RetryFailedMatches)
				})
			})
		})

		// Game routes
		r.Route("/games", func(r chi.Router) {
			// Публичные маршруты
			r.Get("/", s.gameHandler.List)
			r.Get("/{id}", s.gameHandler.Get)
			r.Get("/name/{name}", s.gameHandler.GetByName)

			// Админские маршруты
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.authService, s.log))
				r.Use(middleware.RequireAdmin())

				r.Post("/", s.gameHandler.Create)
				r.Put("/{id}", s.gameHandler.Update)
				r.Delete("/{id}", s.gameHandler.Delete)
			})
		})

		// Team routes
		r.Route("/teams", func(r chi.Router) {
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
				r.Use(middleware.RequireAdmin())
				r.Delete("/{id}", s.teamHandler.Delete)
			})
		})

		// Program routes (все требуют аутентификации)
		r.Route("/programs", func(r chi.Router) {
			r.Use(middleware.Auth(s.authService, s.log))

			r.Post("/", s.programHandler.Create)
			r.Get("/", s.programHandler.List)
			r.Get("/versions", s.programHandler.GetVersions) // Список версий программ команды
			r.Get("/{id}", s.programHandler.Get)
			r.Get("/{id}/download", s.programHandler.Download)
			r.Put("/{id}", s.programHandler.Update)
			r.Delete("/{id}", s.programHandler.Delete)
		})

		// Match routes
		r.Route("/matches", func(r chi.Router) {
			r.Get("/", s.matchHandler.List)
			r.Get("/statistics", s.matchHandler.GetStatistics)
			r.Get("/{id}", s.matchHandler.Get)

			// Админские маршруты для управления очередью матчей
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(s.authService, s.log))
				r.Use(middleware.RequireAdmin())

				r.Get("/queue/stats", s.matchHandler.GetQueueStats)
				r.Post("/queue/clear", s.matchHandler.ClearQueue)
				r.Post("/queue/purge", s.matchHandler.PurgeInvalidMatches)
			})
		})

		// WebSocket routes (требуется аутентификация)
		r.Route("/ws", func(r chi.Router) {
			r.Use(middleware.Auth(s.authService, s.log))

			r.Get("/tournaments/{id}", s.wsHandler.HandleTournament)
			r.Get("/stats", s.wsHandler.GetStats)
		})

		// System routes (только для админов)
		r.Route("/system", func(r chi.Router) {
			r.Use(middleware.Auth(s.authService, s.log))
			r.Use(middleware.RequireAdmin())

			r.Get("/metrics", s.systemHandler.GetMetrics)
			r.Get("/health", s.systemHandler.GetHealth)
		})
	})

	// Serve frontend static files (SPA with fallback to index.html)
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
