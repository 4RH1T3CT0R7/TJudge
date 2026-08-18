package auth

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// мок репозитория юзеров
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Exists(ctx context.Context, username, email string) (bool, error) {
	args := m.Called(ctx, username, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// мок блеклиста токенов
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

func (m *MockTokenBlacklist) AddIfNotExists(ctx context.Context, token string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, token, ttl)
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

// --- Register ---

func TestService_Register_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	req := &RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePass123!",
	}

	userRepo.On("Exists", ctx, req.Username, req.Email).Return(false, nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*models.User")).Return(nil)

	resp, err := service.Register(ctx, req)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, req.Username, resp.User.Username)
	// пароль наружу не отдаём
	assert.Empty(t, resp.User.PasswordHash)

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

	req := &RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "weak",
	}

	resp, err := service.Register(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
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

	// битый email должен отсеять user.Validate уже после проверки Exists
	resp, err := service.Register(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

// --- Login ---

func TestService_Login_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	password := "SecurePass123!"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := &models.User{
		ID:           uuid.New(),
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Role:         models.RoleUser,
	}

	userRepo.On("GetByUsername", ctx, "testuser").Return(user, nil)

	resp, err := service.Login(ctx, &LoginRequest{Username: "testuser", Password: password})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, user.ID, resp.User.ID)
	assert.Empty(t, resp.User.PasswordHash)
	userRepo.AssertExpectations(t)
}

func TestService_Login_ByEmail(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	password := "SecurePass123!"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := &models.User{
		ID:           uuid.New(),
		Username:     "emailuser",
		Email:        "email@example.com",
		PasswordHash: string(hash),
		Role:         models.RoleUser,
	}

	userRepo.On("GetByEmail", ctx, "email@example.com").Return(user, nil)

	resp, err := service.Login(ctx, &LoginRequest{Email: "email@example.com", Password: password})

	require.NoError(t, err)
	assert.Equal(t, user.ID, resp.User.ID)
	userRepo.AssertExpectations(t)
}

func TestService_Login_UserNotFound(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	// на несуществующий логин всё равно должен отработать фейковый compare,
	// иначе по времени ответа палится есть юзер в базе или нет
	userRepo.On("GetByUsername", ctx, "nonexistent").Return(nil, errors.ErrNotFound)

	resp, err := service.Login(ctx, &LoginRequest{Username: "nonexistent", Password: "password"})

	assert.Error(t, err)
	assert.Nil(t, resp)
	userRepo.AssertExpectations(t)
}

func TestService_Login_WrongPassword(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	user := &models.User{
		ID:           uuid.New(),
		Username:     "testuser",
		PasswordHash: string(hash),
	}

	userRepo.On("GetByUsername", ctx, "testuser").Return(user, nil)

	resp, err := service.Login(ctx, &LoginRequest{Username: "testuser", Password: "wrongpassword"})

	assert.Error(t, err)
	assert.Nil(t, resp)
	userRepo.AssertExpectations(t)
}

func TestService_Login_NoUsernameOrEmail(t *testing.T) {
	service, _, _ := newTestService(t)

	resp, err := service.Login(context.Background(), &LoginRequest{Password: "SomePass123!"})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.IsAppError(err))
}

// --- RefreshTokens (ротация) ---

func TestService_RefreshTokens_Success(t *testing.T) {
	service, userRepo, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &models.User{ID: userID, Username: "testuser", Email: "test@example.com", Role: models.RoleUser}

	refreshToken, err := service.jwtManager.GenerateRefreshToken(userID)
	require.NoError(t, err)

	blacklist.On("AddIfNotExists", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(true, nil)
	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	// новый рефреш обязан отличаться от старого
	assert.NotEqual(t, refreshToken, resp.RefreshToken)
	blacklist.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestService_RefreshTokens_ReusedToken(t *testing.T) {
	service, userRepo, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &models.User{ID: userID, Username: "testuser", Role: models.RoleUser}
	refreshToken, _ := service.jwtManager.GenerateRefreshToken(userID)

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	// AddIfNotExists вернул false — токен уже использовали, второй раз не пускаем
	blacklist.On("AddIfNotExists", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(false, nil)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "revoked")
	blacklist.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestService_RefreshTokens_InvalidToken(t *testing.T) {
	service, _, _ := newTestService(t)

	// битый токен не проходит валидацию ещё до похода в блеклист
	resp, err := service.RefreshTokens(context.Background(), "invalid-token")

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_RefreshTokens_GetUserError(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	refreshToken, _ := service.jwtManager.GenerateRefreshToken(userID)

	// GetByID падает ДО AddIfNotExists — токен не гасим, чтобы не залочить юзера
	userRepo.On("GetByID", ctx, userID).Return(nil, errors.ErrNotFound)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	assert.Error(t, err)
	assert.Nil(t, resp)
	userRepo.AssertExpectations(t)
}

func TestService_RefreshTokens_BlacklistError(t *testing.T) {
	service, userRepo, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &models.User{ID: userID, Username: "testuser", Role: models.RoleUser}
	refreshToken, _ := service.jwtManager.GenerateRefreshToken(userID)

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	// редис лёг — fail-closed, запрос отклоняем
	blacklist.On("AddIfNotExists", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(false, errors.ErrInternal)

	resp, err := service.RefreshTokens(ctx, refreshToken)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "blacklist")
	blacklist.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

// --- Logout ---

func TestService_Logout_Success(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	accessToken, _ := service.jwtManager.GenerateAccessToken(userID, "testuser", models.RoleUser)
	refreshToken, _ := service.jwtManager.GenerateRefreshToken(userID)

	blacklist.On("Add", ctx, accessToken, mock.AnythingOfType("time.Duration")).Return(nil)
	blacklist.On("Add", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(nil)

	err := service.Logout(ctx, accessToken, refreshToken)

	require.NoError(t, err)
	blacklist.AssertExpectations(t)
}

func TestService_Logout_ExpiredAccessSkipped(t *testing.T) {
	userRepo := new(MockUserRepository)
	blacklist := new(MockTokenBlacklist)
	// очень короткий ttl чтобы access протух сразу
	jwtManager := NewJWTManager("test-secret-key-123", 1*time.Millisecond, 7*24*time.Hour)
	log, _ := logger.New("debug", "json")
	service := NewService(userRepo, jwtManager, blacklist, log)

	accessToken, _ := jwtManager.GenerateAccessToken(uuid.New(), "testuser", models.RoleUser)
	time.Sleep(10 * time.Millisecond)

	// протухший access не валидируется, поэтому в блеклист не кладём и ошибку не возвращаем
	err := service.Logout(context.Background(), accessToken, "")

	assert.NoError(t, err)
	blacklist.AssertNotCalled(t, "Add")
}

func TestService_Logout_AccessBlacklistError(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	accessToken, _ := service.jwtManager.GenerateAccessToken(uuid.New(), "testuser", models.RoleUser)

	blacklist.On("Add", ctx, accessToken, mock.AnythingOfType("time.Duration")).Return(errors.ErrInternal)

	err := service.Logout(ctx, accessToken, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blacklist access token")
	blacklist.AssertExpectations(t)
}

func TestService_Logout_RefreshBlacklistError(t *testing.T) {
	service, _, blacklist := newTestService(t)
	ctx := context.Background()

	refreshToken, _ := service.jwtManager.GenerateRefreshToken(uuid.New())

	// access битый — для logout это ок, а вот рефреш в блеклист не лёг
	blacklist.On("Add", ctx, refreshToken, mock.AnythingOfType("time.Duration")).Return(errors.ErrInternal)

	err := service.Logout(ctx, "invalid-access-token", refreshToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blacklist refresh token")
	blacklist.AssertExpectations(t)
}

// --- токены и юзер по токену ---

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
	token, _ := service.jwtManager.GenerateAccessToken(userID, "testuser", models.RoleUser)

	claims, err := service.ValidateToken(token)

	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
}

func TestService_GetUserByToken_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &models.User{ID: userID, Username: "testuser", PasswordHash: "hash", Role: models.RoleUser}
	token, _ := service.jwtManager.GenerateAccessToken(userID, "testuser", models.RoleUser)

	// получем юзера по токену, хеш пароля в ответе должен быть затёрт
	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	result, err := service.GetUserByToken(ctx, token)

	require.NoError(t, err)
	assert.Equal(t, userID, result.ID)
	assert.Empty(t, result.PasswordHash)
	userRepo.AssertExpectations(t)
}

func TestService_GetUserByToken_InvalidToken(t *testing.T) {
	service, _, _ := newTestService(t)

	result, err := service.GetUserByToken(context.Background(), "invalid-token")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- bcrypt ---

func TestBcryptCost(t *testing.T) {
	// стоимость bcrypt зафиксирована на 12, менять нельзя
	assert.Equal(t, 12, BcryptCost)
}

func TestService_hashPassword(t *testing.T) {
	service, _, _ := newTestService(t)

	password := "TestPassword123!"
	hash, err := service.hashPassword(password)

	require.NoError(t, err)
	assert.NotEqual(t, password, hash)

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	require.NoError(t, err)
}

func TestService_hashPassword_TooLong(t *testing.T) {
	service, _, _ := newTestService(t)

	// bcrypt молча режет всё что длиннее 72 байт, поэтому такие пароли отбиваем сами
	longPassword := string(make([]byte, 73))
	hash, err := service.hashPassword(longPassword)

	assert.Error(t, err)
	assert.Empty(t, hash)
	assert.Contains(t, err.Error(), "too long")
}

func TestService_comparePassword(t *testing.T) {
	service, _, _ := newTestService(t)

	password := "TestPassword123!"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)

	assert.NoError(t, service.comparePassword(string(hash), password))
	assert.Error(t, service.comparePassword(string(hash), "wrongpassword"))
}

func TestService_comparePassword_TooLong(t *testing.T) {
	service, _, _ := newTestService(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("short"), bcrypt.MinCost)
	longPassword := string(make([]byte, 73))

	err := service.comparePassword(string(hash), longPassword)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

// --- UpdateProfile ---

func TestService_UpdateProfile_Success(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &models.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "old@example.com",
		PasswordHash: "oldhash",
		Role:         models.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("GetByEmail", ctx, "new@example.com").Return(nil, errors.ErrNotFound)
	userRepo.On("Update", ctx, mock.AnythingOfType("*models.User")).Return(nil)

	result, err := service.UpdateProfile(ctx, userID.String(), &UpdateProfileRequest{Email: "new@example.com"})

	require.NoError(t, err)
	assert.Equal(t, "new@example.com", result.Email)
	assert.Empty(t, result.PasswordHash)
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_EmailAlreadyInUse(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &models.User{ID: userID, Username: "testuser", Email: "old@example.com", Role: models.RoleUser}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("GetByEmail", ctx, "taken@example.com").Return(&models.User{ID: uuid.New()}, nil)

	result, err := service.UpdateProfile(ctx, userID.String(), &UpdateProfileRequest{Email: "taken@example.com"})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "email already in use")
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_PasswordChange(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	oldHash, _ := bcrypt.GenerateFromPassword([]byte("OldPassword123!"), bcrypt.MinCost)
	user := &models.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(oldHash),
		Role:         models.RoleUser,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*models.User")).Return(nil)

	result, err := service.UpdateProfile(ctx, userID.String(), &UpdateProfileRequest{
		Password:        "NewSecurePass123!",
		CurrentPassword: "OldPassword123!",
	})

	require.NoError(t, err)
	assert.Empty(t, result.PasswordHash)
	userRepo.AssertExpectations(t)
}

func TestService_UpdateProfile_PasswordWithoutCurrent(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	user := &models.User{ID: userID, Username: "testuser", PasswordHash: "oldhash", Role: models.RoleUser}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	// сменить пароль без текущего нельзя — иначе угнанный access менял бы пароль
	result, err := service.UpdateProfile(ctx, userID.String(), &UpdateProfileRequest{Password: "NewSecurePass123!"})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "current password is required")
}

func TestService_UpdateProfile_WrongCurrentPassword(t *testing.T) {
	service, userRepo, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	oldHash, _ := bcrypt.GenerateFromPassword([]byte("OldPassword123!"), bcrypt.MinCost)
	user := &models.User{ID: userID, Username: "testuser", PasswordHash: string(oldHash), Role: models.RoleUser}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	result, err := service.UpdateProfile(ctx, userID.String(), &UpdateProfileRequest{
		Password:        "NewSecurePass123!",
		CurrentPassword: "WrongPassword123!",
	})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "current password is incorrect")
}

func TestService_UpdateProfile_InvalidUserID(t *testing.T) {
	service, _, _ := newTestService(t)

	result, err := service.UpdateProfile(context.Background(), "not-a-uuid", &UpdateProfileRequest{Email: "new@example.com"})

	assert.Error(t, err)
	assert.Nil(t, result)
}
