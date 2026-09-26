//go:build integration

package storage_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// setupTestDB поднимает коннект к БД для интеграционных тестов.
// параметры берутся из env, с дефолтами под локальный docker-compose.
func setupTestDB(t *testing.T) *storage.DB {
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
		SSLMode:        getEnv("DB_SSLMODE", "disable"),
		MaxConnections: 10,
		MaxIdle:        5,
		MaxLifetime:    5 * time.Minute,
	}

	log, _ := logger.New("error", "json")
	m := metrics.New()

	database, err := storage.New(cfg, log, m)
	require.NoError(t, err)

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

// createTestUser создаёт юзера для теста
func createTestUser(t *testing.T, repo *storage.UserRepository, suffix string) *models.User {
	t.Helper()
	ctx := context.Background()

	user := &models.User{
		ID:           uuid.New(),
		Username:     "testuser_" + suffix,
		Email:        "testuser_" + suffix + "@test.com",
		PasswordHash: "$2a$10$testhashedpassword000000000000000000000000000000",
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	return user
}

// createTestTournament создаёт турнир. creatorID должен ссылаться на существующего юзера.
func createTestTournament(t *testing.T, repo *storage.TournamentRepository, code string, creatorID uuid.UUID) *models.Tournament {
	t.Helper()
	ctx := context.Background()

	tournament := &models.Tournament{
		ID:              uuid.New(),
		Code:            code,
		Name:            "Test Tournament " + code,
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          models.TournamentPending,
		MaxParticipants: intPtr(100),
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       uuidPtr(creatorID),
	}

	err := repo.Create(ctx, tournament)
	require.NoError(t, err)

	return tournament
}

// createTestTournamentWithUser создаёт и юзера, и турнир - чтобы не ловить FK по creator_id
func createTestTournamentWithUser(t *testing.T, tournamentRepo *storage.TournamentRepository, userRepo *storage.UserRepository, code string) (*models.Tournament, *models.User) {
	t.Helper()
	ctx := context.Background()

	user := createTestUser(t, userRepo, "tourney_"+code)

	tournament := &models.Tournament{
		ID:              uuid.New(),
		Code:            code,
		Name:            "Test Tournament " + code,
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          models.TournamentPending,
		MaxParticipants: intPtr(100),
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       uuidPtr(user.ID),
	}

	err := tournamentRepo.Create(ctx, tournament)
	require.NoError(t, err)

	return tournament, user
}

// чтение env с дефолтом

// addTestParticipant - строка tournament_participants для фикстур, в проде её
// вставляет CreateWithAtomicVersion вместе с программой
func addTestParticipant(t *testing.T, db *storage.DB, tournamentID, programID uuid.UUID, rating int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO tournament_participants (id, tournament_id, program_id, rating) VALUES ($1, $2, $3, $4)",
		id, tournamentID, programID, rating)
	require.NoError(t, err)
	return id
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

func intPtr(v int) *int {
	return &v
}

func uuidPtr(v uuid.UUID) *uuid.UUID {
	return &v
}

// сессия всегда в UTC, даже если сервер настроен на другую зону: иначе NOW() в
// колонках TIMESTAMP и время из Go расходятся на смещение
func TestDBSessionTimeZoneIsUTC(t *testing.T) {
	database := setupTestDB(t)
	var tz string
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT current_setting('TimeZone')").Scan(&tz))
	require.Equal(t, "UTC", tz)
}
