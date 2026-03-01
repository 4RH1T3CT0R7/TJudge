package auth

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// Mock UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) Exists(ctx context.Context, username, email string) (bool, error) {
	args := m.Called(ctx, username, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// Mock TokenBlacklist
type MockTokenBlacklist struct {
	mock.Mock
}

func (m *MockTokenBlacklist) Add(ctx context.Context, token string, ttl time.Duration) error {
	args := m.Called(ctx, token, ttl)
	return args.Error(0)
}

func (m *MockTokenBlacklist) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	args := m.Called(ctx, token)
	return args.Bool(0), args.Error(1)
}

func newTestService(t *testing.T) (*Service, *MockUserRepository, *MockTokenBlacklist) {
	userRepo := new(MockUserRepository)
	blacklist := new(MockTokenBlacklist)
	jwtManager := NewJWTManager("test-secret-key-123", 15*time.Minute, 7*24*time.Hour)
	log, _ := logger.New("debug", "json")

	service := NewService(userRepo, jwtManager, blacklist, log)
	return service, userRepo, blacklist
}

func TestService_Register_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePass123!",
	}

	userRepo.On("Exists", ctx, req.Username, req.Email).Return(false, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	resp, err := service.Register(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, req.Username, resp.User.Username)
	assert.Equal(t, req.Email, resp.User.Email)
	assert.Empty(t, resp.User.PasswordHash) // Password should be hidden

	userRepo.AssertExpectations(t)
}

func TestService_Register_UserAlreadyExists(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &RegisterRequest{
		Username: "existinguser",
		Email:    "existing@example.com",
		Password: "SecurePass123!",
	}

	userRepo.On("Exists", ctx, req.Username, req.Email).Return(true, nil)

	resp, err := service.Register(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.IsAppError(err))

	userRepo.AssertExpectations(t)
}

func TestService_Register_WeakPassword(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	req := &RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "weak", // Too short
	}

	resp, err := service.Register(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Login_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	password := "SecurePass123!"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &domain.User{
		ID:           uuid.New(),
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
	}

	req := &LoginRequest{
		Username: "testuser",
		Password: password,
	}

	userRepo.On("GetByUsername", ctx, req.Username).Return(user, nil)

	resp, err := service.Login(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, user.ID, resp.User.ID)
	assert.Empty(t, resp.User.PasswordHash)

	userRepo.AssertExpectations(t)
}

func TestService_Login_UserNotFound(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &LoginRequest{
		Username: "nonexistent",
		Password: "password",
	}

	userRepo.On("GetByUsername", ctx, req.Username).Return(nil, errors.ErrNotFound)

	resp, err := service.Login(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	userRepo.AssertExpectations(t)
}

func TestService_Login_WrongPassword(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	user := &domain.User{
		ID:           uuid.New(),
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(hash),
	}

	req := &LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}

	userRepo.On("GetByUsername", ctx, req.Username).Return(user, nil)

	resp, err := service.Login(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	userRepo.AssertExpectations(t)
}

func TestService_RefreshTokens_Success(t *testing.T) {
	service, userRepo, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     domain.RoleUser,
	}

	// Generate a valid refresh token
	refreshToken, err := service.jwtManager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	blacklist.On("IsBlacklisted", ctx, refreshToken).Return(false, nil)
	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	blacklist.On("Add", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(nil)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.NotEqual(t, refreshToken, resp.RefreshToken) // New token should be different

	blacklist.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestService_RefreshTokens_BlacklistedToken(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	refreshToken, _ := service.jwtManager.GenerateRefreshToken(userID)

	blacklist.On("IsBlacklisted", ctx, refreshToken).Return(true, nil)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "revoked")

	blacklist.AssertExpectations(t)
}

func TestService_RefreshTokens_InvalidToken(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	blacklist.On("IsBlacklisted", ctx, "invalid-token").Return(false, nil)

	resp, err := service.RefreshTokens(ctx, "invalid-token")

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Logout_Success(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	token, err := service.jwtManager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	blacklist.On("Add", ctx, token, mock.AnythingOfType("time.Duration")).Return(nil)

	err = service.Logout(ctx, token, "")

	require.NoError(t, err)
	blacklist.AssertExpectations(t)
}

func TestService_Logout_InvalidToken(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	// Invalid token doesn't cause error now - it just logs and continues
	err := service.Logout(ctx, "invalid-token", "")

	assert.NoError(t, err)
}

func TestService_Logout_ExpiredToken(t *testing.T) {
	// Create service with very short TTL
	userRepo := new(MockUserRepository)
	blacklist := new(MockTokenBlacklist)
	jwtManager := NewJWTManager("test-secret", 1*time.Millisecond, 7*24*time.Hour)
	log, _ := logger.New("debug", "json")
	service := NewService(userRepo, jwtManager, blacklist, log)

	ctx := context.Background()

	token, err := jwtManager.GenerateAccessToken(uuid.New(), "testuser")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	// Expired token doesn't cause error - Logout is now more lenient
	err = service.Logout(ctx, token, "")
	assert.NoError(t, err)
}

func TestService_IsTokenBlacklisted(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	blacklist.On("IsBlacklisted", ctx, "some-token").Return(true, nil)

	isBlacklisted, err := service.IsTokenBlacklisted(ctx, "some-token")

	require.NoError(t, err)
	assert.True(t, isBlacklisted)

	blacklist.AssertExpectations(t)
}

func TestService_ValidateToken(t *testing.T) {
	service, _, _ := newTestService(t)

	userID := uuid.New()
	token, err := service.jwtManager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	claims, err := service.ValidateToken(token)

	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
}

func TestService_GetUserByToken_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	}

	token, err := service.jwtManager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	result, err := service.GetUserByToken(ctx, token)

	require.NoError(t, err)
	assert.Equal(t, userID, result.ID)
	assert.Empty(t, result.PasswordHash) // Password should be hidden

	userRepo.AssertExpectations(t)
}

func TestService_GetUserByToken_InvalidToken(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	result, err := service.GetUserByToken(ctx, "invalid-token")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_GetUserByToken_UserNotFound(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	token, err := service.jwtManager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	userRepo.On("GetByID", ctx, userID).Return(nil, errors.ErrNotFound)

	result, err := service.GetUserByToken(ctx, token)

	assert.Error(t, err)
	assert.Nil(t, result)

	userRepo.AssertExpectations(t)
}

func TestService_GetUserFromToken_Alias(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     domain.RoleUser,
	}

	token, err := service.jwtManager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	result, err := service.GetUserFromToken(ctx, token)

	require.NoError(t, err)
	assert.Equal(t, userID, result.ID)

	userRepo.AssertExpectations(t)
}

func TestBcryptCost(t *testing.T) {
	// Ensure bcrypt cost is set correctly for security
	assert.Equal(t, 12, BcryptCost)
}

func TestService_hashPassword(t *testing.T) {
	service, _, _ := newTestService(t)

	password := "TestPassword123!"
	hash, err := service.hashPassword(password)

	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// Verify the hash
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	require.NoError(t, err)
}

func TestService_comparePassword(t *testing.T) {
	service, _, _ := newTestService(t)

	password := "TestPassword123!"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)

	// Correct password
	err := service.comparePassword(string(hash), password)
	require.NoError(t, err)

	// Wrong password
	err = service.comparePassword(string(hash), "wrongpassword")
	assert.Error(t, err)
}

// --- Login edge cases ---

func TestService_Login_ByEmail(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	password := "SecurePass123!"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &domain.User{
		ID:           uuid.New(),
		Username:     "emailuser",
		Email:        "email@example.com",
		PasswordHash: string(hash),
		Role:         domain.RoleUser,
	}

	req := &LoginRequest{
		Email:    "email@example.com",
		Password: password,
	}

	userRepo.On("GetByEmail", ctx, req.Email).Return(user, nil)

	resp, err := service.Login(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, user.ID, resp.User.ID)
	assert.Empty(t, resp.User.PasswordHash)
	userRepo.AssertExpectations(t)
}

func TestService_Login_NoUsernameOrEmail(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	req := &LoginRequest{
		Password: "SomePass123!",
	}

	resp, err := service.Login(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.IsAppError(err))
}

func TestService_Login_RepoError(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &LoginRequest{
		Username: "testuser",
		Password: "SomePass123!",
	}

	userRepo.On("GetByUsername", ctx, "testuser").Return(nil, errors.ErrInternal)

	resp, err := service.Login(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	userRepo.AssertExpectations(t)
}

// --- UpdateProfile ---

func TestService_UpdateProfile_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "old@example.com",
		PasswordHash: "oldhash",
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("GetByEmail", ctx, "new@example.com").Return(nil, errors.ErrNotFound)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	req := &UpdateProfileRequest{
		Email: "new@example.com",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	require.NoError(t, err)
	assert.Equal(t, "new@example.com", result.Email)
	assert.Empty(t, result.PasswordHash)
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_EmailAlreadyInUse(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	otherUserID := uuid.New()
	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "old@example.com",
		PasswordHash: "oldhash",
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("GetByEmail", ctx, "taken@example.com").Return(&domain.User{ID: otherUserID}, nil)

	req := &UpdateProfileRequest{
		Email: "taken@example.com",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "email already in use")
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_SameEmail(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "same@example.com",
		PasswordHash: "oldhash",
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	req := &UpdateProfileRequest{
		Email: "same@example.com",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	require.NoError(t, err)
	assert.Equal(t, "same@example.com", result.Email)
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_PasswordOnly(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	// Use a real bcrypt hash for "OldPassword123!" so comparePassword works
	oldHash, err := bcrypt.GenerateFromPassword([]byte("OldPassword123!"), bcrypt.MinCost)
	require.NoError(t, err)

	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(oldHash),
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	req := &UpdateProfileRequest{
		Password:        "NewSecurePass123!",
		CurrentPassword: "OldPassword123!",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.PasswordHash)
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_PasswordWithoutCurrent(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "oldhash",
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	req := &UpdateProfileRequest{
		Password: "NewSecurePass123!",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "current password is required")
}

func TestService_UpdateProfile_WrongCurrentPassword(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	oldHash, err := bcrypt.GenerateFromPassword([]byte("OldPassword123!"), bcrypt.MinCost)
	require.NoError(t, err)

	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(oldHash),
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	req := &UpdateProfileRequest{
		Password:        "NewSecurePass123!",
		CurrentPassword: "WrongPassword123!",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "current password is incorrect")
}

func TestService_UpdateProfile_WeakPassword(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	oldHash, err := bcrypt.GenerateFromPassword([]byte("OldPassword123!"), bcrypt.MinCost)
	require.NoError(t, err)

	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(oldHash),
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	req := &UpdateProfileRequest{
		Password:        "weak",
		CurrentPassword: "OldPassword123!",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_UpdateProfile_UserNotFound(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	userRepo.On("GetByID", ctx, userID).Return(nil, errors.ErrNotFound)

	req := &UpdateProfileRequest{
		Email: "new@example.com",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_InvalidUserID(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	req := &UpdateProfileRequest{
		Email: "new@example.com",
	}

	result, err := service.UpdateProfile(ctx, "not-a-uuid", req)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_UpdateProfile_UpdateError(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "oldhash",
		Role:         domain.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("GetByEmail", ctx, "new@example.com").Return(nil, errors.ErrNotFound)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(errors.ErrInternal)

	req := &UpdateProfileRequest{
		Email: "new@example.com",
	}

	result, err := service.UpdateProfile(ctx, userID.String(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
	userRepo.AssertExpectations(t)
}

// --- Register edge cases ---

func TestService_Register_CreateError(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePass123!",
	}

	userRepo.On("Exists", ctx, req.Username, req.Email).Return(false, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(errors.ErrInternal)

	resp, err := service.Register(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	userRepo.AssertExpectations(t)
}

func TestService_Register_ExistsError(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePass123!",
	}

	userRepo.On("Exists", ctx, req.Username, req.Email).Return(false, errors.ErrInternal)

	resp, err := service.Register(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	userRepo.AssertExpectations(t)
}

// --- RefreshTokens edge cases ---

func TestService_RefreshTokens_GetUserError(t *testing.T) {
	service, userRepo, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	refreshToken, err := service.jwtManager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	blacklist.On("IsBlacklisted", ctx, refreshToken).Return(false, nil)
	userRepo.On("GetByID", ctx, userID).Return(nil, errors.ErrNotFound)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	assert.Error(t, err)
	assert.Nil(t, resp)
	blacklist.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

// --- Additional edge cases ---

func TestService_RefreshTokens_BlacklistCheckError(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	refreshToken, err := service.jwtManager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	// Simulate Redis down — fail-closed should reject the request
	blacklist.On("IsBlacklisted", ctx, refreshToken).Return(false, errors.ErrInternal)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "blacklist")
	blacklist.AssertExpectations(t)
}

func TestService_Logout_WithBothTokens(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	accessToken, err := service.jwtManager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)
	refreshToken, err := service.jwtManager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	blacklist.On("Add", ctx, accessToken, mock.AnythingOfType("time.Duration")).Return(nil)
	blacklist.On("Add", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(nil)

	err = service.Logout(ctx, accessToken, refreshToken)

	require.NoError(t, err)
	blacklist.AssertExpectations(t)
}

func TestService_RefreshTokens_BlacklistAddError(t *testing.T) {
	service, userRepo, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &domain.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     domain.RoleUser,
	}

	refreshToken, err := service.jwtManager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	blacklist.On("IsBlacklisted", ctx, refreshToken).Return(false, nil)
	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	// Old token blacklist fails — should abort rotation (fail-closed)
	blacklist.On("Add", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(errors.ErrInternal)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "blacklist")
	blacklist.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestService_Register_InvalidEmail(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &RegisterRequest{
		Username: "testuser",
		Email:    "not-an-email",
		Password: "SecurePass123!",
	}

	userRepo.On("Exists", ctx, req.Username, req.Email).Return(false, nil)

	resp, err := service.Register(ctx, req)

	// The user.Validate() should catch invalid email
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// --- hashPassword / comparePassword edge cases ---

func TestService_hashPassword_TooLong(t *testing.T) {
	service, _, _ := newTestService(t)

	// bcrypt silently truncates > 72 bytes; hashPassword should reject explicitly
	longPassword := string(make([]byte, 73))
	hash, err := service.hashPassword(longPassword)

	assert.Error(t, err)
	assert.Empty(t, hash)
	assert.Contains(t, err.Error(), "too long")
}

func TestService_hashPassword_Empty(t *testing.T) {
	service, _, _ := newTestService(t)

	hash, err := service.hashPassword("")

	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify empty password matches the hash
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(""))
	assert.NoError(t, err)
}

func TestService_comparePassword_TooLong(t *testing.T) {
	service, _, _ := newTestService(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("short"), bcrypt.MinCost)
	longPassword := string(make([]byte, 73))

	err := service.comparePassword(string(hash), longPassword)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestService_comparePassword_InvalidHash(t *testing.T) {
	service, _, _ := newTestService(t)

	err := service.comparePassword("not-a-bcrypt-hash", "password")

	assert.Error(t, err)
}

// --- Logout edge cases ---

func TestService_Logout_AccessBlacklistError(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	accessToken, err := service.jwtManager.GenerateAccessToken(userID, "testuser")
	require.NoError(t, err)

	// Blacklist add fails for access token
	blacklist.On("Add", ctx, accessToken, mock.AnythingOfType("time.Duration")).Return(errors.ErrInternal)

	err = service.Logout(ctx, accessToken, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blacklist access token")
	blacklist.AssertExpectations(t)
}

func TestService_Logout_RefreshBlacklistError(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	refreshToken, err := service.jwtManager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	// Access token is invalid — that's OK for logout, it just logs
	// Refresh token blacklist fails
	blacklist.On("Add", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(errors.ErrInternal)

	err = service.Logout(ctx, "invalid-access-token", refreshToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blacklist refresh token")
	blacklist.AssertExpectations(t)
}
