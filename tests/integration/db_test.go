//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// DBTestSuite is the integration test suite for database operations
type DBTestSuite struct {
	suite.Suite
	db          *db.DB
	userRepo    *db.UserRepository
	programRepo *db.ProgramRepository
	matchRepo   *db.MatchRepository
	ctx         context.Context
}

func (s *DBTestSuite) SetupSuite() {
	if os.Getenv("RUN_INTEGRATION") != "true" {
		s.T().Skip("Skipping integration tests (set RUN_INTEGRATION=true)")
	}

	s.ctx = context.Background()

	// Get database config from environment
	host := getEnv("DB_HOST", "localhost")
	port := getEnvInt("DB_PORT", 5432)
	user := getEnv("DB_USER", "tjudge")
	password := getEnv("DB_PASSWORD", "secret")
	dbName := getEnv("DB_NAME", "tjudge_test")

	log, _ := logger.New("debug", "json")
	m := metrics.New()

	var err error
	s.db, err = db.New(&config.DatabaseConfig{
		Host:           host,
		Port:           port,
		User:           user,
		Password:       password,
		Name:           dbName,
		MaxConnections: 10,
		MaxIdle:        5,
		MaxLifetime:    5 * time.Minute,
	}, log, m)
	require.NoError(s.T(), err)

	s.userRepo = db.NewUserRepository(s.db)
	s.programRepo = db.NewProgramRepository(s.db)
	s.matchRepo = db.NewMatchRepository(s.db)
}

func (s *DBTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *DBTestSuite) SetupTest() {
	// Clean up test data before each test
	s.cleanupTestData()
}

func (s *DBTestSuite) cleanupTestData() {
	// Clean up in reverse order of dependencies
	s.db.ExecContext(s.ctx, "DELETE FROM matches WHERE game_type = 'integration_test'")
	s.db.ExecContext(s.ctx, "DELETE FROM programs WHERE code_path LIKE 'integration_test%'")
	s.db.ExecContext(s.ctx, "DELETE FROM users WHERE username LIKE 'integration_test_%'")
}

// =============================================================================
// User Repository Tests
// =============================================================================

func (s *DBTestSuite) TestUserRepository_Create() {
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}

	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), user.CreatedAt)
	assert.NotZero(s.T(), user.UpdatedAt)
}

func (s *DBTestSuite) TestUserRepository_GetByID() {
	// Create user first
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)

	// Get by ID
	found, err := s.userRepo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.ID, found.ID)
	assert.Equal(s.T(), user.Username, found.Username)
	assert.Equal(s.T(), user.Email, found.Email)
}

func (s *DBTestSuite) TestUserRepository_GetByID_NotFound() {
	_, err := s.userRepo.GetByID(s.ctx, uuid.New())
	assert.Error(s.T(), err)
}

func (s *DBTestSuite) TestUserRepository_GetByUsername() {
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)

	found, err := s.userRepo.GetByUsername(s.ctx, user.Username)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.ID, found.ID)
}

func (s *DBTestSuite) TestUserRepository_GetByEmail() {
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)

	found, err := s.userRepo.GetByEmail(s.ctx, user.Email)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.ID, found.ID)
}

func (s *DBTestSuite) TestUserRepository_Update() {
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)

	// Update
	user.Username = "integration_test_updated_" + uuid.New().String()[:8]
	err = s.userRepo.Update(s.ctx, user)
	require.NoError(s.T(), err)

	// Verify
	found, err := s.userRepo.GetByID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), user.Username, found.Username)
}

func (s *DBTestSuite) TestUserRepository_Delete() {
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)

	// Delete
	err = s.userRepo.Delete(s.ctx, user.ID)
	require.NoError(s.T(), err)

	// Verify deleted
	_, err = s.userRepo.GetByID(s.ctx, user.ID)
	assert.Error(s.T(), err)
}

func (s *DBTestSuite) TestUserRepository_Exists() {
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)

	exists, err := s.userRepo.Exists(s.ctx, user.Username, user.Email)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)

	exists, err = s.userRepo.Exists(s.ctx, "nonexistent", "nonexistent@test.com")
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

// =============================================================================
// Program Repository Tests
// =============================================================================

func (s *DBTestSuite) TestProgramRepository_CRUD() {
	// Create user first
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "integration_test_user_" + uuid.New().String()[:8],
		Email:        "integration_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash: "hashed_password",
	}
	err := s.userRepo.Create(s.ctx, user)
	require.NoError(s.T(), err)

	// Create program
	program := &domain.Program{
		ID:       uuid.New(),
		UserID:   user.ID,
		Name:     "Test Program",
		Language: "python",
		CodePath: "integration_test_print_hello",
		GameType: "tictactoe",
	}

	err = s.programRepo.Create(s.ctx, program)
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), program.CreatedAt)

	// Get by ID
	found, err := s.programRepo.GetByID(s.ctx, program.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), program.Name, found.Name)

	// Get by user ID
	programs, err := s.programRepo.GetByUserID(s.ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), programs, 1)

	// Update
	program.Name = "Updated Program"
	err = s.programRepo.Update(s.ctx, program)
	require.NoError(s.T(), err)

	// Verify update
	found, err = s.programRepo.GetByID(s.ctx, program.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Program", found.Name)

	// Delete
	err = s.programRepo.Delete(s.ctx, program.ID)
	require.NoError(s.T(), err)

	_, err = s.programRepo.GetByID(s.ctx, program.ID)
	assert.Error(s.T(), err)
}

// =============================================================================
// Concurrent Operations Tests
// =============================================================================

func (s *DBTestSuite) TestConcurrentCreates() {
	const numUsers = 10
	users := make([]*domain.User, numUsers)
	errs := make(chan error, numUsers)

	for i := 0; i < numUsers; i++ {
		users[i] = &domain.User{
			ID:           uuid.New(),
			Username:     "integration_test_concurrent_" + uuid.New().String()[:8],
			Email:        "concurrent_" + uuid.New().String()[:8] + "@test.com",
			PasswordHash: "hashed_password",
		}
	}

	// Create users concurrently
	for i := 0; i < numUsers; i++ {
		go func(user *domain.User) {
			errs <- s.userRepo.Create(s.ctx, user)
		}(users[i])
	}

	// Collect results
	for i := 0; i < numUsers; i++ {
		err := <-errs
		assert.NoError(s.T(), err)
	}

	// Verify all users were created
	for _, user := range users {
		found, err := s.userRepo.GetByID(s.ctx, user.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), user.Username, found.Username)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func TestDBSuite(t *testing.T) {
	suite.Run(t, new(DBTestSuite))
}
