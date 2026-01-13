//go:build integration

package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TournamentRepositorySuite struct {
	suite.Suite
	db   *db.DB
	repo *db.TournamentRepository
}

func TestTournamentRepositorySuite(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION=true to run.")
	}
	suite.Run(t, new(TournamentRepositorySuite))
}

func (s *TournamentRepositorySuite) SetupSuite() {
	config := db.Config{
		Host:           getEnv("DB_HOST", "localhost"),
		Port:           getEnvInt("DB_PORT", 5433),
		User:           getEnv("DB_USER", "tjudge"),
		Password:       getEnv("DB_PASSWORD", "secret"),
		Database:       getEnv("DB_NAME", "tjudge"),
		MaxConnections: 10,
		MaxIdle:        5,
		MaxLifetime:    time.Minute * 5,
	}

	database, err := db.NewDB(config)
	require.NoError(s.T(), err)
	s.db = database
	s.repo = db.NewTournamentRepository(database)
}

func (s *TournamentRepositorySuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *TournamentRepositorySuite) TearDownTest() {
	// Clean up test data after each test
	ctx := context.Background()
	_, _ = s.db.ExecContext(ctx, "DELETE FROM tournaments WHERE code LIKE 'TEST%'")
}

func (s *TournamentRepositorySuite) TestCreate() {
	ctx := context.Background()
	creatorID := uuid.New()

	tournament := &domain.Tournament{
		ID:              uuid.New(),
		Code:            "TEST001",
		Name:            "Test Tournament",
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          domain.TournamentStatusDraft,
		MaxParticipants: 100,
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       creatorID,
		Metadata:        map[string]interface{}{"test": "value"},
	}

	err := s.repo.Create(ctx, tournament)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), tournament.CreatedAt)
	assert.NotZero(s.T(), tournament.UpdatedAt)
	assert.Equal(s.T(), int64(1), tournament.Version)
}

func (s *TournamentRepositorySuite) TestGetByID() {
	ctx := context.Background()

	// Create tournament first
	tournament := s.createTestTournament("TEST002")

	// Get by ID
	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), tournament.ID, result.ID)
	assert.Equal(s.T(), tournament.Code, result.Code)
	assert.Equal(s.T(), tournament.Name, result.Name)
	assert.Equal(s.T(), tournament.Status, result.Status)
}

func (s *TournamentRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
}

func (s *TournamentRepositorySuite) TestList() {
	ctx := context.Background()

	// Create multiple tournaments
	s.createTestTournament("TEST003")
	s.createTestTournament("TEST004")
	s.createTestTournament("TEST005")

	// List all
	filter := domain.TournamentFilter{Limit: 10}
	tournaments, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	assert.GreaterOrEqual(s.T(), len(tournaments), 3)
}

func (s *TournamentRepositorySuite) TestList_FilterByStatus() {
	ctx := context.Background()

	// Create tournaments with different statuses
	t1 := s.createTestTournament("TEST006")
	t2 := s.createTestTournament("TEST007")

	// Update status of one tournament
	err := s.repo.UpdateStatus(ctx, t1.ID, domain.TournamentStatusActive)
	require.NoError(s.T(), err)

	// List only active tournaments
	filter := domain.TournamentFilter{
		Status: domain.TournamentStatusActive,
		Limit:  10,
	}
	tournaments, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	// Should contain t1 but not necessarily t2
	var found bool
	for _, t := range tournaments {
		if t.ID == t1.ID {
			found = true
			break
		}
	}
	assert.True(s.T(), found, "Active tournament should be in list")

	// t2 should not be in active list (it's still draft)
	for _, t := range tournaments {
		if t.ID == t2.ID {
			assert.Equal(s.T(), domain.TournamentStatusActive, t.Status)
		}
	}
}

func (s *TournamentRepositorySuite) TestUpdateStatus() {
	ctx := context.Background()

	tournament := s.createTestTournament("TEST008")

	// Update status
	err := s.repo.UpdateStatus(ctx, tournament.ID, domain.TournamentStatusActive)
	require.NoError(s.T(), err)

	// Verify
	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.TournamentStatusActive, result.Status)
}

func (s *TournamentRepositorySuite) TestUpdate() {
	ctx := context.Background()

	tournament := s.createTestTournament("TEST009")

	// Update tournament
	tournament.Name = "Updated Name"
	tournament.Description = "Updated Description"
	tournament.MaxParticipants = 200

	err := s.repo.Update(ctx, tournament)
	require.NoError(s.T(), err)

	// Verify
	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Name", result.Name)
	assert.Equal(s.T(), "Updated Description", result.Description)
	assert.Equal(s.T(), 200, result.MaxParticipants)
}

func (s *TournamentRepositorySuite) TestDelete() {
	ctx := context.Background()

	tournament := s.createTestTournament("TEST010")

	// Delete
	err := s.repo.Delete(ctx, tournament.ID)
	require.NoError(s.T(), err)

	// Verify not found
	_, err = s.repo.GetByID(ctx, tournament.ID)
	assert.Error(s.T(), err)
}

func (s *TournamentRepositorySuite) TestGetByCode() {
	ctx := context.Background()

	tournament := s.createTestTournament("TEST011")

	// Get by code
	result, err := s.repo.GetByCode(ctx, "TEST011")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), tournament.ID, result.ID)
	assert.Equal(s.T(), tournament.Code, result.Code)
}

func (s *TournamentRepositorySuite) TestGetByCode_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByCode(ctx, "NONEXISTENT")
	assert.Error(s.T(), err)
}

// Helper functions

func (s *TournamentRepositorySuite) createTestTournament(code string) *domain.Tournament {
	ctx := context.Background()
	creatorID := uuid.New()

	tournament := &domain.Tournament{
		ID:              uuid.New(),
		Code:            code,
		Name:            "Test Tournament " + code,
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          domain.TournamentStatusDraft,
		MaxParticipants: 100,
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       creatorID,
	}

	err := s.repo.Create(ctx, tournament)
	require.NoError(s.T(), err)

	return tournament
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return fallback
}
