//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TournamentRepositorySuite struct {
	suite.Suite
	database *db.DB
	repo     *db.TournamentRepository
	userRepo *db.UserRepository
}

func TestTournamentRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &TournamentRepositorySuite{
		database: database,
		repo:     db.NewTournamentRepository(database),
		userRepo: db.NewUserRepository(database),
	}
	suite.Run(t, s)
}

func (s *TournamentRepositorySuite) TearDownTest() {
	ctx := context.Background()
	_, _ = s.database.ExecContext(ctx, "DELETE FROM tournaments WHERE code LIKE 'TEST%'")
	_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE username LIKE 'testuser_%'")
}

func (s *TournamentRepositorySuite) TestCreate() {
	ctx := context.Background()
	creator := createTestUser(s.T(), s.userRepo, "creator_"+uuid.New().String()[:8])

	tournament := &domain.Tournament{
		ID:              uuid.New(),
		Code:            "TEST001",
		Name:            "Test Tournament",
		Description:     "Test Description",
		GameType:        "prisoners_dilemma",
		Status:          domain.TournamentPending,
		MaxParticipants: intPtr(100),
		MaxTeamSize:     3,
		IsPermanent:     false,
		CreatorID:       uuidPtr(creator.ID),
		Metadata:        map[string]interface{}{"test": "value"},
	}

	err := s.repo.Create(ctx, tournament)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), tournament.CreatedAt)
	assert.NotZero(s.T(), tournament.UpdatedAt)
	assert.Equal(s.T(), 0, tournament.Version)
}

func (s *TournamentRepositorySuite) TestGetByID() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST002")

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

	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST003")
	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST004")
	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST005")

	filter := domain.TournamentFilter{Limit: 10}
	tournaments, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	assert.GreaterOrEqual(s.T(), len(tournaments), 3)
}

func (s *TournamentRepositorySuite) TestList_FilterByStatus() {
	ctx := context.Background()

	t1, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST006")
	createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST007")

	err := s.repo.UpdateStatus(ctx, t1.ID, domain.TournamentActive)
	require.NoError(s.T(), err)

	filter := domain.TournamentFilter{
		Status: domain.TournamentActive,
		Limit:  10,
	}
	tournaments, err := s.repo.List(ctx, filter)
	require.NoError(s.T(), err)

	var found bool
	for _, t := range tournaments {
		if t.ID == t1.ID {
			found = true
			break
		}
	}
	assert.True(s.T(), found, "Active tournament should be in list")
}

func (s *TournamentRepositorySuite) TestUpdateStatus() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST008")

	err := s.repo.UpdateStatus(ctx, tournament.ID, domain.TournamentActive)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), domain.TournamentActive, result.Status)
}

func (s *TournamentRepositorySuite) TestUpdate() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST009")

	tournament.Name = "Updated Name"
	tournament.MaxParticipants = intPtr(200)

	err := s.repo.Update(ctx, tournament)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, tournament.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Name", result.Name)
	assert.Equal(s.T(), intPtr(200), result.MaxParticipants)
}

func (s *TournamentRepositorySuite) TestDelete() {
	ctx := context.Background()
	tournament, _ := createTestTournamentWithUser(s.T(), s.repo, s.userRepo, "TEST010")

	err := s.repo.Delete(ctx, tournament.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetByID(ctx, tournament.ID)
	assert.Error(s.T(), err)
}
