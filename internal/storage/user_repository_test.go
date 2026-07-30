//go:build integration

package storage_test

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UserRepositorySuite struct {
	suite.Suite
	database *storage.DB
	repo     *storage.UserRepository
}

func TestUserRepositorySuite(t *testing.T) {
	database := setupTestDB(t)
	s := &UserRepositorySuite{
		database: database,
		repo:     storage.NewUserRepository(database),
	}
	suite.Run(t, s)
}

func (s *UserRepositorySuite) TearDownTest() {
	ctx := context.Background()
	_, _ = s.database.ExecContext(ctx, "DELETE FROM users WHERE username LIKE 'testuser_%'")
}

func (s *UserRepositorySuite) TestCreate() {
	user := createTestUser(s.T(), s.repo, "create1")

	assert.NotZero(s.T(), user.CreatedAt)
	assert.NotZero(s.T(), user.UpdatedAt)
}

func (s *UserRepositorySuite) TestCreate_DuplicateUsername() {
	createTestUser(s.T(), s.repo, "dup_user")

	ctx := context.Background()
	duplicate := &domain.User{
		ID:           uuid.New(),
		Username:     "testuser_dup_user",
		Email:        "different@test.com",
		PasswordHash: "$2a$10$testhashedpassword000000000000000000000000000000",
	}

	err := s.repo.Create(ctx, duplicate)
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestCreate_DuplicateEmail() {
	createTestUser(s.T(), s.repo, "dup_email")

	ctx := context.Background()
	duplicate := &domain.User{
		ID:           uuid.New(),
		Username:     "different_user",
		Email:        "testuser_dup_email@test.com",
		PasswordHash: "$2a$10$testhashedpassword000000000000000000000000000000",
	}

	err := s.repo.Create(ctx, duplicate)
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestGetByID() {
	user := createTestUser(s.T(), s.repo, "getbyid")

	ctx := context.Background()
	result, err := s.repo.GetByID(ctx, user.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), user.ID, result.ID)
	assert.Equal(s.T(), user.Username, result.Username)
	assert.Equal(s.T(), user.Email, result.Email)
}

func (s *UserRepositorySuite) TestGetByID_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByID(ctx, uuid.New())
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestGetByUsername() {
	user := createTestUser(s.T(), s.repo, "getbyname")

	ctx := context.Background()
	result, err := s.repo.GetByUsername(ctx, "testuser_getbyname")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), user.ID, result.ID)
	assert.Equal(s.T(), user.Username, result.Username)
}

func (s *UserRepositorySuite) TestGetByUsername_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByUsername(ctx, "nonexistent_user")
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestGetByEmail() {
	user := createTestUser(s.T(), s.repo, "getbyemail")

	ctx := context.Background()
	result, err := s.repo.GetByEmail(ctx, "testuser_getbyemail@test.com")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), user.ID, result.ID)
	assert.Equal(s.T(), user.Email, result.Email)
}

func (s *UserRepositorySuite) TestGetByEmail_NotFound() {
	ctx := context.Background()

	_, err := s.repo.GetByEmail(ctx, "nonexistent@test.com")
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestUpdate() {
	user := createTestUser(s.T(), s.repo, "update")

	ctx := context.Background()
	user.Username = "testuser_updated"
	user.Email = "testuser_updated@test.com"

	err := s.repo.Update(ctx, user)
	require.NoError(s.T(), err)

	result, err := s.repo.GetByID(ctx, user.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "testuser_updated", result.Username)
	assert.Equal(s.T(), "testuser_updated@test.com", result.Email)
}

func (s *UserRepositorySuite) TestUpdate_NotFound() {
	ctx := context.Background()
	user := &domain.User{
		ID:           uuid.New(),
		Username:     "nonexistent",
		Email:        "nonexistent@test.com",
		PasswordHash: "$2a$10$testhashedpassword000000000000000000000000000000",
	}

	err := s.repo.Update(ctx, user)
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestDelete() {
	user := createTestUser(s.T(), s.repo, "delete")

	ctx := context.Background()
	err := s.repo.Delete(ctx, user.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetByID(ctx, user.ID)
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestDelete_NotFound() {
	ctx := context.Background()

	err := s.repo.Delete(ctx, uuid.New())
	assert.Error(s.T(), err)
}

func (s *UserRepositorySuite) TestExists() {
	createTestUser(s.T(), s.repo, "exists")

	ctx := context.Background()

	// совпадение по username
	exists, err := s.repo.Exists(ctx, "testuser_exists", "nonexistent@test.com")
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)

	// совпадение по email
	exists, err = s.repo.Exists(ctx, "nonexistent_user", "testuser_exists@test.com")
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *UserRepositorySuite) TestExists_False() {
	ctx := context.Background()

	exists, err := s.repo.Exists(ctx, "nonexistent_user", "nonexistent@test.com")
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}
