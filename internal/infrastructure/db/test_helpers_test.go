//go:build integration

package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a database connection for integration tests.
// It reads connection parameters from environment variables with sensible defaults.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()

	if os.Getenv("RUN_INTEGRATION") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION=true to run.")
	}

	cfg := &config.DatabaseConfig{
		Host:           getEnv("DB_HOST", "localhost"),
		Port:           getEnvInt("DB_PORT", 5433),
		User:           getEnv("DB_USER", "tjudge"),
		Password:       getEnv("DB_PASSWORD", "secret"),
		Name:           getEnv("DB_NAME", "tjudge"),
		MaxConnections: 10,
		MaxIdle:        5,
		MaxLifetime:    5 * time.Minute,
	}

	log, _ := logger.New("error", "json")
	m := metrics.New()

	database, err := db.New(cfg, log, m)
	require.NoError(t, err)

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

// cleanupTable removes all rows from the given table that match the WHERE clause.
func cleanupTable(t *testing.T, database *db.DB, table, where string, args ...interface{}) {
	t.Helper()
	ctx := context.Background()
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, where)
	_, err := database.ExecContext(ctx, query, args...)
	if err != nil {
		t.Logf("Warning: failed to cleanup table %s: %v", table, err)
	}
}

// createTestUser creates a user in the database for testing.
func createTestUser(t *testing.T, repo *db.UserRepository, suffix string) *domain.User {
	t.Helper()
	ctx := context.Background()

	user := &domain.User{
		ID:           uuid.New(),
		Username:     "testuser_" + suffix,
		Email:        "testuser_" + suffix + "@test.com",
		PasswordHash: "$2a$10$testhashedpassword000000000000000000000000000000",
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	return user
}

// createTestTournament creates a tournament in the database for testing.
func createTestTournament(t *testing.T, repo *db.TournamentRepository, code string, creatorID uuid.UUID) *domain.Tournament {
	t.Helper()
	ctx := context.Background()

	tournament := &domain.Tournament{
		ID:              uuid.New(),
		Code:            code,
		Name:            "Test Tournament " + code,
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          domain.TournamentPending,
		MaxParticipants: intPtr(100),
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       uuidPtr(creatorID),
	}

	err := repo.Create(ctx, tournament)
	require.NoError(t, err)

	return tournament
}

// Helper functions for reading environment variables.

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

func intPtr(v int) *int {
	return &v
}

func uuidPtr(v uuid.UUID) *uuid.UUID {
	return &v
}
