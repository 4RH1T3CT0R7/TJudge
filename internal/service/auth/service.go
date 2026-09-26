package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository — читает и пишет юзеров в базу, обычный fat-репозиторий
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Exists(ctx context.Context, username, email string) (bool, error)
	Update(ctx context.Context, user *models.User) error
}

// TokenBlacklist — чёрный список токенов (лежит в редисе)
type TokenBlacklist interface {
	Add(ctx context.Context, token string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
	// AddIfNotExists кладёт токен атомарно (setnx) и говорит, был ли он новым.
	// это нужно чтобы не словить TOCTOU при ротации рефреш-токенов
	AddIfNotExists(ctx context.Context, token string, ttl time.Duration) (bool, error)
}

type Service struct {
	userRepo       UserRepository
	jwtManager     *JWTManager
	tokenBlacklist TokenBlacklist
	log            *logger.Logger
}

func NewService(userRepo UserRepository, jwtManager *JWTManager, tokenBlacklist TokenBlacklist, log *logger.Logger) *Service {
	return &Service{
		userRepo:       userRepo,
		jwtManager:     jwtManager,
		tokenBlacklist: tokenBlacklist,
		log:            log,
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest — на вход можно дать либо username, либо email
type LoginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateProfileRequest struct {
	Email           string `json:"email,omitempty"`
	Password        string `json:"password,omitempty"`
	CurrentPassword string `json:"current_password,omitempty"`
}

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         *models.User `json:"user"`
}

// Register регистрирует нового юзера и сразу выдаёт токены
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	if err := models.ValidatePassword(req.Password); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	exists, err := s.userRepo.Exists(ctx, req.Username, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		return nil, errors.ErrAlreadyExists.WithMessage("username or email already exists")
	}

	passwordHash, err := s.hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         models.RoleUser, // по умолчанию обычный юзер
	}

	if err := user.Validate(); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.log.Info("User registered",
		zap.String("user_id", user.ID.String()),
		zap.String("username", user.Username),
	)

	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// пароль наружу не отдаётся
	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// Login проверяет логин/пароль и выдаёт токены
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	var user *models.User
	var err error

	// юзер достаётся по email или по username, что дали
	if req.Email != "" {
		user, err = s.userRepo.GetByEmail(ctx, req.Email)
	} else if req.Username != "" {
		user, err = s.userRepo.GetByUsername(ctx, req.Username)
	} else {
		return nil, errors.ErrInvalidCredentials
	}

	if err != nil {
		if errors.IsAppError(err) && errors.GetAppError(err).Code == 404 {
			// фейковый компейр чтобы по времени ответа не палить есть юзер или нет.
			// без этого ответ на несуществующий логин приходил бы заметно быстрее
			_ = bcrypt.CompareHashAndPassword(
				[]byte("$2a$12$000000000000000000000uGVYlKMFeX7iKOQKZ3d2fXxqFaE6D.e"),
				[]byte(req.Password),
			)
			return nil, errors.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := s.comparePassword(user.PasswordHash, req.Password); err != nil {
		// логируется только user_id, без ника и почты — иначе по логам можно
		// перебором вычислять кто вообще есть в базе
		s.log.Info("Invalid password attempt",
			zap.String("user_id", user.ID.String()),
		)
		return nil, errors.ErrInvalidCredentials
	}

	s.log.Info("User logged in",
		zap.String("user_id", user.ID.String()),
		zap.String("username", user.Username),
	)

	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// RefreshTokens меняет пару токенов на новую.
// ротация: старый рефреш после этого невалиден
func (s *Service) RefreshTokens(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	// сначала проверяется сам токен, это дёшево и без побочек
	userID, issuedAt, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.ErrInvalidToken.WithError(err)
	}

	// юзер достаётся до гашения токена. если сходить в базу после
	// AddIfNotExists и там упасть, человек останется без рефреша и залогиниться
	// заново не сможет (lockout)
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// смена пароля отзывает все сессии, выписанные до неё. access живёт недолго,
	// так что проверки на рефреше хватает. iat усечён вниз до миллисекунды,
	// поэтому отзывается и токен той же миллисекунды
	if user.PasswordChangedAt != nil && !issuedAt.After(*user.PasswordChangedAt) {
		return nil, errors.ErrInvalidToken.WithMessage("refresh token has been revoked")
	}

	// token rotation: старый рефреш атомарно кладётся в блеклист через setnx.
	// это защита от TOCTOU — если прилетело два запроса с одним токеном,
	// пройдёт только первый. на ошибке редиса fail-closed, запрос отклоняется
	wasNew, err := s.tokenBlacklist.AddIfNotExists(ctx, refreshToken, s.jwtManager.RefreshTokenTTL())
	if err != nil {
		s.log.LogError("Failed to atomically blacklist refresh token", err)
		return nil, fmt.Errorf("failed to blacklist refresh token: %w", err)
	}
	if !wasNew {
		// токен уже использован, второй раз хода нет
		s.log.Warn("Attempt to reuse already-consumed refresh token")
		return nil, errors.ErrInvalidToken.WithMessage("refresh token has been revoked")
	}

	s.log.Info("Tokens refreshed with rotation",
		zap.String("user_id", user.ID.String()),
	)

	newAccessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		User:         user,
	}, nil
}

// Logout гасит токены через блеклист
func (s *Service) Logout(ctx context.Context, accessToken, refreshToken string) error {
	// fail-closed: если не смогли занести токен в блеклист — возвращается ошибка,
	// чтобы клиент понял что logout прошёл не до конца
	claims, err := s.jwtManager.ValidateToken(accessToken)
	if err != nil {
		// access мог уже протухнуть, это норм
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

	if refreshToken != "" {
		// ручка публичная: в блеклист идёт только подписанный refresh, иначе
		// аноним забивал бы редис произвольными строками. рефреш кладётся на
		// полный ttl, тк его срок может быть позже чем у access
		if _, _, err := s.jwtManager.ValidateRefreshToken(refreshToken); err != nil {
			s.log.Info("Refresh token validation failed during logout", zap.Error(err))
		} else if err := s.tokenBlacklist.Add(ctx, refreshToken, s.jwtManager.RefreshTokenTTL()); err != nil {
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

// UpdateProfile меняет email и/или пароль
func (s *Service) UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) (*models.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.ErrInvalidInput.WithMessage("invalid user ID")
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// email трогается только если он реально другой
	if req.Email != "" && req.Email != user.Email {
		if err := models.ValidateEmail(req.Email); err != nil {
			return nil, errors.ErrValidation.WithError(err)
		}
		existingUser, existErr := s.userRepo.GetByEmail(ctx, req.Email)
		if existErr != nil && !errors.IsNotFound(existErr) {
			return nil, fmt.Errorf("failed to check email uniqueness: %w", existErr)
		}
		if existErr == nil && existingUser.ID != user.ID {
			return nil, errors.ErrAlreadyExists.WithMessage("email already in use")
		}
		user.Email = req.Email
	}

	if req.Password != "" {
		// пароль меняется только вместе с текущим паролем, иначе угнанный access
		// токен позволил бы сменить пароль без знания старого
		if req.CurrentPassword == "" {
			return nil, errors.ErrValidation.WithMessage("current password is required to change password")
		}
		if err := s.comparePassword(user.PasswordHash, req.CurrentPassword); err != nil {
			return nil, errors.ErrInvalidCredentials.WithMessage("current password is incorrect")
		}

		if err := models.ValidatePassword(req.Password); err != nil {
			return nil, errors.ErrValidation.WithError(err)
		}

		passwordHash, err := s.hashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		user.PasswordHash = passwordHash
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	s.log.Info("Profile updated",
		zap.String("user_id", user.ID.String()),
	)

	user.PasswordHash = ""

	return user, nil
}

func (s *Service) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	return s.tokenBlacklist.IsBlacklisted(ctx, token)
}

func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwtManager.ValidateToken(tokenString)
}

// GetUserByToken достаёт юзера по access токену
func (s *Service) GetUserByToken(ctx context.Context, tokenString string) (*models.User, error) {
	claims, err := s.jwtManager.ValidateToken(tokenString)
	if err != nil {
		return nil, errors.ErrInvalidToken.WithError(err)
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.PasswordHash = ""

	return user, nil
}

// GetUserFromToken — алиас для GetUserByToken
func (s *Service) GetUserFromToken(ctx context.Context, tokenString string) (*models.User, error) {
	return s.GetUserByToken(ctx, tokenString)
}

// BcryptCost — стоимость bcrypt. 12 для прода, менять нельзя: иначе все старые хеши станут невалидными
const BcryptCost = 12

// hashPassword хеширует пароль. bcrypt молча режет всё что длиннее 72 байт,
// поэтому такие пароли отсекаются заранее, а не отдаются ему на тихую обрезку
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

// comparePassword сверяет пароль с хешом.
// те же >72 байта режутся заранее, чтобы не поймать коллизию обрезки bcrypt
func (s *Service) comparePassword(hash, password string) error {
	if len([]byte(password)) > 72 {
		return errors.ErrInvalidCredentials.WithMessage("invalid credentials")
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
