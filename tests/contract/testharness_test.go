//go:build contract

package contract

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/api"
	"github.com/bmstu-itstech/tjudge/internal/api/handlers"
	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	contractmocks "github.com/bmstu-itstech/tjudge/tests/contract/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// TestHarness wraps a real api.Server (with full chi router and middleware
// chain) backed by mockery-generated mocks for all service dependencies. It
// provides ready-to-use JWT tokens and helper methods for contract/API tests.
type TestHarness struct {
	Server *httptest.Server
	URL    string
	Client *http.Client

	// UploadDir is the temporary directory used by ProgramHandler for file uploads.
	UploadDir string

	// Handler-level mocks
	AuthService       *contractmocks.MockAuthService
	TournamentService *contractmocks.MockTournamentService
	SchedulingService *contractmocks.MockSchedulingService
	GameService       *contractmocks.MockGameService
	TeamService       *contractmocks.MockTeamService
	ProgramRepo       *contractmocks.MockProgramRepository
	MatchRepo         *contractmocks.MockMatchRepository
	MatchCache        *contractmocks.MockMatchCache
	ProgramLookup     *contractmocks.MockMatchProgramLookup
	QueueManager      *contractmocks.MockMatchQueueManager
	TournamentAdder   *contractmocks.MockTournamentParticipantAdder
	TournamentStatus  *contractmocks.MockTournamentStatusChecker
	MatchScheduler    *contractmocks.MockMatchScheduler
	GameLookup        *contractmocks.MockGameLookup
	MatchChecker      *contractmocks.MockMatchExistenceChecker
	RoundChecker      *contractmocks.MockRoundCompletionChecker
	TeamChecker       *contractmocks.MockTeamMembershipChecker
	AutoRoundChecker  *contractmocks.MockAutoRoundChecker

	LeaderboardRepo          *contractmocks.MockGameLeaderboardRepository
	GameMatchRepo            *contractmocks.MockGameMatchRepository
	GameTournamentRepo       *contractmocks.MockGameTournamentRepository
	GameProgramRepo          *contractmocks.MockGameProgramRepository
	TournamentGameStatusRepo *contractmocks.MockTournamentGameStatusRepository

	// Middleware mock
	MiddlewareAuth *contractmocks.MockMiddlewareAuthService

	// JWT for generating real tokens
	JWTManager *auth.JWTManager

	// Pre-generated identities
	TestUserID  uuid.UUID
	TestAdminID uuid.UUID

	t *testing.T
}

// NewTestHarness creates a fully wired TestHarness with all mocks,
// a real chi router, and a running httptest.Server.
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()

	h := &TestHarness{
		TestUserID:  uuid.New(),
		TestAdminID: uuid.New(),
		t:           t,
	}

	// Create all mocks.
	h.AuthService = contractmocks.NewMockAuthService(t)
	h.TournamentService = contractmocks.NewMockTournamentService(t)
	h.SchedulingService = contractmocks.NewMockSchedulingService(t)
	h.GameService = contractmocks.NewMockGameService(t)
	h.TeamService = contractmocks.NewMockTeamService(t)
	h.ProgramRepo = contractmocks.NewMockProgramRepository(t)
	h.MatchRepo = contractmocks.NewMockMatchRepository(t)
	h.MatchCache = contractmocks.NewMockMatchCache(t)
	h.ProgramLookup = contractmocks.NewMockMatchProgramLookup(t)
	h.QueueManager = contractmocks.NewMockMatchQueueManager(t)
	h.TournamentAdder = contractmocks.NewMockTournamentParticipantAdder(t)
	h.TournamentStatus = contractmocks.NewMockTournamentStatusChecker(t)
	h.MatchScheduler = contractmocks.NewMockMatchScheduler(t)
	h.GameLookup = contractmocks.NewMockGameLookup(t)
	h.MatchChecker = contractmocks.NewMockMatchExistenceChecker(t)
	h.RoundChecker = contractmocks.NewMockRoundCompletionChecker(t)
	h.TeamChecker = contractmocks.NewMockTeamMembershipChecker(t)
	h.AutoRoundChecker = contractmocks.NewMockAutoRoundChecker(t)
	h.LeaderboardRepo = contractmocks.NewMockGameLeaderboardRepository(t)
	h.GameMatchRepo = contractmocks.NewMockGameMatchRepository(t)
	h.GameTournamentRepo = contractmocks.NewMockGameTournamentRepository(t)
	h.GameProgramRepo = contractmocks.NewMockGameProgramRepository(t)
	h.TournamentGameStatusRepo = contractmocks.NewMockTournamentGameStatusRepository(t)
	h.MiddlewareAuth = contractmocks.NewMockMiddlewareAuthService(t)

	// Reusable HTTP client for all requests from this harness.
	h.Client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Upload directory for ProgramHandler.
	h.UploadDir = t.TempDir()

	// JWT manager with a deterministic secret (>= 32 chars).
	h.JWTManager = auth.NewJWTManager(
		"contract-test-secret-key-32bytes!!",
		15*time.Minute,
		24*time.Hour,
	)

	// Configure middleware auth mock to delegate token validation to the
	// real JWTManager so that the full Auth middleware chain works.
	h.MiddlewareAuth.EXPECT().
		ValidateToken(mock.Anything).
		RunAndReturn(func(token string) (*auth.Claims, error) {
			return h.JWTManager.ValidateToken(token)
		}).Maybe()

	h.MiddlewareAuth.EXPECT().
		IsTokenBlacklisted(mock.Anything, mock.Anything).
		Return(false, nil).Maybe()

	// Logger (suppressed output).
	log, _ := logger.New("error", "json")

	// Build all handlers via their real constructors.
	authHandler := handlers.NewAuthHandler(h.AuthService, log)
	tournamentHandler := handlers.NewTournamentHandler(h.TournamentService, h.SchedulingService, log)
	programHandler := handlers.NewProgramHandler(
		h.ProgramRepo,
		h.TournamentAdder,
		h.TournamentStatus,
		h.MatchScheduler,
		h.GameLookup,
		h.MatchChecker,
		h.RoundChecker,
		h.TeamChecker,
		h.AutoRoundChecker,
		nil, // compileQueue: в contract-тестах компиляция не выполняется
		h.UploadDir,
		log,
	)
	matchHandler := handlers.NewMatchHandler(
		h.MatchRepo,
		h.MatchCache,
		h.ProgramLookup,
		h.QueueManager,
		log,
	)
	gameHandler := handlers.NewGameHandler(
		h.GameService,
		h.LeaderboardRepo,
		h.GameMatchRepo,
		h.GameTournamentRepo,
		h.GameProgramRepo,
		h.TournamentGameStatusRepo,
		events.NewSyncBus(log),
		t.TempDir(),
		log,
	)
	teamHandler := handlers.NewTeamHandler(h.TeamService, "http://test.local", log)
	wsHandler := handlers.NewWebSocketHandler(websocket.NewHub(log), log)
	systemHandler := handlers.NewSystemHandler(log)

	// CORS config - permissive for tests.
	corsConfig := config.CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		MaxAge:         300,
	}

	// Rate limiting disabled.
	rateLimitConfig := config.RateLimitConfig{Enabled: false}

	// Build the real server with the full middleware chain.
	srv := api.NewServer(
		authHandler,
		tournamentHandler,
		programHandler,
		matchHandler,
		gameHandler,
		teamHandler,
		wsHandler,
		systemHandler,
		h.MiddlewareAuth,
		nil, // rateLimiter - nil is safe when Enabled=false
		corsConfig,
		rateLimitConfig,
		log,
	)

	h.Server = httptest.NewServer(srv.Handler())
	h.URL = h.Server.URL
	t.Cleanup(h.Server.Close)

	return h
}

// UserToken returns a valid JWT access token for the pre-generated test user.
func (h *TestHarness) UserToken() string {
	return h.TokenForUser(h.TestUserID, "testuser", domain.RoleUser)
}

// AdminToken returns a valid JWT access token for the pre-generated test admin.
func (h *TestHarness) AdminToken() string {
	return h.TokenForUser(h.TestAdminID, "admin", domain.RoleAdmin)
}

// TokenForUser generates a valid JWT access token for the given identity.
func (h *TestHarness) TokenForUser(id uuid.UUID, username string, role domain.Role) string {
	h.t.Helper()
	token, err := h.JWTManager.GenerateAccessToken(id, username, role)
	if err != nil {
		h.t.Fatalf("failed to generate access token: %v", err)
	}
	return token
}
