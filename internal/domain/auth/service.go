package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Exists(ctx context.Context, username, email string) (bool, error)
	Update(ctx context.Context, user *domain.User) error
}

// TokenBlacklist интерфейс для работы с чёрным списком токенов
type TokenBlacklist interface {
	Add(ctx context.Context, token string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
}

// Service - сервис аутентификации
type Service struct {
	userRepo       UserRepository
	jwtManager     *JWTManager
	tokenBlacklist TokenBlacklist
	log            *logger.Logger
}

// NewService создаёт новый сервис аутентификации
func NewService(userRepo UserRepository, jwtManager *JWTManager, tokenBlacklist TokenBlacklist, log *logger.Logger) *Service {
	return &Service{
		userRepo:       userRepo,
		jwtManager:     jwtManager,
		tokenBlacklist: tokenBlacklist,
		log:            log,
	}
}

// RegisterRequest - запрос на регистрацию
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest - запрос на вход
// Можно указать username ИЛИ email для входа
type LoginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateProfileRequest - запрос на обновление профиля
type UpdateProfileRequest struct {
	Email           string `json:"email,omitempty"`
	Password        string `json:"password,omitempty"`
	CurrentPassword string `json:"current_password,omitempty"`
}

// AuthResponse - ответ с токенами
type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         *domain.User `json:"user"`
}

// Register регистрирует нового пользователя
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	// Валидация входных данных
	if err := domain.ValidatePassword(req.Password); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	// Проверяем существование пользователя
	exists, err := s.userRepo.Exists(ctx, req.Username, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		return nil, errors.ErrAlreadyExists.WithMessage("username or email already exists")
	}

	// Хешируем пароль
	passwordHash, err := s.hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Создаём пользователя
	user := &domain.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         domain.RoleUser, // По умолчанию роль user
	}

	// Валидируем пользователя
	if err := user.Validate(); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	// Сохраняем в БД
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.log.Info("User registered",
		zap.String("user_id", user.ID.String()),
		zap.String("username", user.Username),
	)

	// Генерируем токены
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Скрываем пароль
	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// Login выполняет вход пользователя
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	var user *domain.User
	var err error

	// Получаем пользователя по username или email
	if req.Email != "" {
		user, err = s.userRepo.GetByEmail(ctx, req.Email)
	} else if req.Username != "" {
		user, err = s.userRepo.GetByUsername(ctx, req.Username)
	} else {
		return nil, errors.ErrInvalidCredentials
	}

	if err != nil {
		if errors.IsAppError(err) && errors.GetAppError(err).Code == 404 {
			// Dummy bcrypt comparison to prevent timing-based user enumeration
			_ = bcrypt.CompareHashAndPassword(
				[]byte("$2a$12$000000000000000000000uGVYlKMFeX7iKOQKZ3d2fXxqFaE6D.e"),
				[]byte(req.Password),
			)
			return nil, errors.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем пароль
	if err := s.comparePassword(user.PasswordHash, req.Password); err != nil {
		s.log.Info("Invalid password attempt",
			zap.String("username", req.Username),
			zap.String("email", req.Email),
			zap.String("user_id", user.ID.String()),
		)
		return nil, errors.ErrInvalidCredentials
	}

	s.log.Info("User logged in",
		zap.String("user_id", user.ID.String()),
		zap.String("username", user.Username),
		zap.String("email", user.Email),
	)

	// Генерируем токены
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Скрываем пароль
	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// RefreshTokens обновляет access token используя refresh token
// Реализует token rotation: старый refresh token инвалидируется
func (s *Service) RefreshTokens(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	// Проверяем, не в blacklist ли токен (token rotation protection).
	// Fail-closed: if the blacklist check fails (e.g. Redis is down), we reject
	// the request rather than allowing a potentially revoked token through.
	isBlacklisted, err := s.tokenBlacklist.IsBlacklisted(ctx, refreshToken)
	if err != nil {
		s.log.LogError("Failed to check token blacklist", err)
		return nil, fmt.Errorf("failed to verify token blacklist: %w", err)
	}
	if isBlacklisted {
		s.log.Warn("Attempt to reuse blacklisted refresh token")
		return nil, errors.ErrInvalidToken.WithMessage("refresh token has been revoked")
	}

	// Валидируем refresh token
	userID, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.ErrInvalidToken.WithError(err)
	}

	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Token Rotation: добавляем старый refresh token в blacklist.
	// Это предотвращает повторное использование токена.
	// Fail-closed: if we cannot blacklist the old token, abort the rotation to
	// prevent the old token from remaining valid and being reused.
	if err := s.tokenBlacklist.Add(ctx, refreshToken, s.jwtManager.RefreshTokenTTL()); err != nil {
		s.log.LogError("Failed to blacklist old refresh token", err)
		return nil, fmt.Errorf("failed to blacklist old refresh token: %w", err)
	}

	s.log.Info("Tokens refreshed with rotation",
		zap.String("user_id", user.ID.String()),
	)

	// Генерируем новые токены
	newAccessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Скрываем пароль
	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		User:         user,
	}, nil
}

// Logout выполняет выход пользователя, добавляя токены в чёрный список
func (s *Service) Logout(ctx context.Context, accessToken, refreshToken string) error {
	// Добавляем access token в blacklist.
	// Fail-closed: if we cannot blacklist the tokens, return an error so the
	// client knows the logout was not fully processed.
	claims, err := s.jwtManager.ValidateToken(accessToken)
	if err != nil {
		// Access token может быть уже истёкшим, это OK
		s.log.Info("Access token validation failed during logout", zap.Error(err))
	} else {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			if err := s.tokenBlacklist.Add(ctx, accessToken, ttl); err != nil {
				s.log.LogError("Failed to blacklist access token", err)
				return fmt.Errorf("failed to blacklist access token: %w", err)
			}
		}
	}

	// Добавляем refresh token в blacklist (если предоставлен)
	if refreshToken != "" {
		// Refresh token добавляем с полным TTL, т.к. его expiry может быть позже
		if err := s.tokenBlacklist.Add(ctx, refreshToken, s.jwtManager.RefreshTokenTTL()); err != nil {
			s.log.LogError("Failed to blacklist refresh token", err)
			return fmt.Errorf("failed to blacklist refresh token: %w", err)
		}
	}

	if claims != nil {
		s.log.Info("User logged out",
			zap.String("user_id", claims.UserID.String()),
		)
	}

	return nil
}

// UpdateProfile обновляет профиль пользователя
func (s *Service) UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) (*domain.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.ErrInvalidInput.WithMessage("invalid user ID")
	}

	// Получаем текущего пользователя
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Обновляем email если указан
	if req.Email != "" && req.Email != user.Email {
		existingUser, existErr := s.userRepo.GetByEmail(ctx, req.Email)
		if existErr != nil && !errors.IsNotFound(existErr) {
			return nil, fmt.Errorf("failed to check email uniqueness: %w", existErr)
		}
		if existErr == nil && existingUser.ID != user.ID {
			return nil, errors.ErrAlreadyExists.WithMessage("email already in use")
		}
		user.Email = req.Email
	}

	// Обновляем пароль если указан
	if req.Password != "" {
		// Требуем текущий пароль для смены пароля
		if req.CurrentPassword == "" {
			return nil, errors.ErrValidation.WithMessage("current password is required to change password")
		}
		if err := s.comparePassword(user.PasswordHash, req.CurrentPassword); err != nil {
			return nil, errors.ErrInvalidCredentials.WithMessage("current password is incorrect")
		}

		if err := domain.ValidatePassword(req.Password); err != nil {
			return nil, errors.ErrValidation.WithError(err)
		}

		passwordHash, err := s.hashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		user.PasswordHash = passwordHash
	}

	// Сохраняем изменения
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	s.log.Info("Profile updated",
		zap.String("user_id", user.ID.String()),
	)

	// Скрываем пароль
	user.PasswordHash = ""

	return user, nil
}

// IsTokenBlacklisted проверяет, находится ли токен в чёрном списке
func (s *Service) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	return s.tokenBlacklist.IsBlacklisted(ctx, token)
}

// ValidateToken валидирует access token
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwtManager.ValidateToken(tokenString)
}

// GetUserByToken получает пользователя по токену
func (s *Service) GetUserByToken(ctx context.Context, tokenString string) (*domain.User, error) {
	claims, err := s.jwtManager.ValidateToken(tokenString)
	if err != nil {
		return nil, errors.ErrInvalidToken.WithError(err)
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Скрываем пароль
	user.PasswordHash = ""

	return user, nil
}

// GetUserFromToken - алиас для GetUserByToken
func (s *Service) GetUserFromToken(ctx context.Context, tokenString string) (*domain.User, error) {
	return s.GetUserByToken(ctx, tokenString)
}

// BcryptCost стоимость хеширования bcrypt (12 для production security)
const BcryptCost = 12

// hashPassword хеширует пароль используя bcrypt с повышенной стоимостью.
// bcrypt silently truncates inputs longer than 72 bytes, so we reject them explicitly.
func (s *Service) hashPassword(password string) (string, error) {
	if len([]byte(password)) > 72 {
		return "", errors.ErrValidation.WithMessage("password is too long")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// comparePassword сравнивает пароль с хешом.
// Rejects passwords > 72 bytes to prevent bcrypt truncation collisions.
func (s *Service) comparePassword(hash, password string) error {
	if len([]byte(password)) > 72 {
		return errors.ErrInvalidCredentials.WithMessage("invalid credentials")
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
