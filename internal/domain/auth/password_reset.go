package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// passwordResetTokenTTL — срок жизни токена восстановления (1 час).
const passwordResetTokenTTL = time.Hour

// PasswordResetRepository — интерфейс хранилища токенов.
type PasswordResetRepository interface {
	Insert(ctx context.Context, t *domain.PasswordResetToken) error
	GetByHash(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error)
	MarkUsed(ctx context.Context, id interface{}) error
}

// Mailer отправляет email-ы. Интерфейс конкретизируется реализацией (SMTP, noop, stub).
type Mailer interface {
	SendPasswordResetEmail(ctx context.Context, email, name, resetURL string) error
}

// PasswordResetService реализует request/confirm flow восстановления пароля (P1.11).
//
// Security properties:
//   - Токены генерируются из crypto/rand (256 бит), хранятся в БД только как
//     sha256-хэш → при утечке БД нельзя reset'ить чужой пароль без оригинала из email.
//   - TTL 1 час.
//   - Idempotent response: если email не найден, API отвечает так же, как
//     при успешном запросе — это предотвращает user enumeration.
//   - MarkUsed атомарно защищает от race (нельзя использовать один токен дважды).
type PasswordResetService struct {
	users       UserRepository
	resets      PasswordResetRepository
	mailer      Mailer
	publicURL   string
	authService *Service
	log         *logger.Logger
}

// NewPasswordResetService создаёт сервис восстановления пароля.
// authService нужен для хэширования нового пароля (общий bcrypt-cost).
// publicURL — базовый URL фронтенда (используется в reset-ссылке email'а).
func NewPasswordResetService(
	users UserRepository,
	resets PasswordResetRepository,
	mailer Mailer,
	publicURL string,
	authService *Service,
	log *logger.Logger,
) *PasswordResetService {
	return &PasswordResetService{
		users:       users,
		resets:      resets,
		mailer:      mailer,
		publicURL:   publicURL,
		authService: authService,
		log:         log,
	}
}

// PasswordResetRequest — запрос на начало восстановления.
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// PasswordResetConfirm — запрос на завершение восстановления.
type PasswordResetConfirm struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// Request начинает процесс восстановления: генерирует токен, сохраняет в БД,
// отправляет email. Всегда возвращает nil для неизвестных email
// (чтобы не позволить enumeration).
func (s *PasswordResetService) Request(ctx context.Context, req PasswordResetRequest, requesterIP string) error {
	if req.Email == "" {
		return errors.ErrValidation.WithMessage("email is required")
	}

	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.IsAppError(err) && errors.GetAppError(err).Code == 404 {
			// User enumeration protection: возвращаем успех даже если email не найден.
			s.log.Info("Password reset requested for unknown email")
			return nil
		}
		return fmt.Errorf("failed to look up user: %w", err)
	}

	token, err := generateResetToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	entry := &domain.PasswordResetToken{
		ID:          uuid.New(),
		UserID:      user.ID,
		TokenHash:   hashToken(token),
		CreatedAt:   now,
		ExpiresAt:   now.Add(passwordResetTokenTTL),
		RequesterIP: requesterIP,
	}
	if err := s.resets.Insert(ctx, entry); err != nil {
		return fmt.Errorf("failed to save reset token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.publicURL, token)
	if err := s.mailer.SendPasswordResetEmail(ctx, user.Email, user.Username, resetURL); err != nil {
		// Email-delivery ошибка не блокирует ответ: токен уже создан, админ сможет
		// посмотреть в логе и переотправить. Возвращаем OK, логируем для алертов.
		s.log.Error("Failed to send password reset email",
			zap.Error(err),
			zap.String("user_id", user.ID.String()),
		)
	}
	s.log.Info("Password reset token created",
		zap.String("user_id", user.ID.String()),
		zap.Time("expires_at", entry.ExpiresAt),
	)
	return nil
}

// Confirm завершает восстановление: валидирует токен, атомарно помечает used,
// обновляет пароль пользователя.
func (s *PasswordResetService) Confirm(ctx context.Context, req PasswordResetConfirm) error {
	if req.Token == "" || len(req.NewPassword) < 8 {
		return errors.ErrValidation.WithMessage("invalid token or password too short")
	}

	tokenHash := hashToken(req.Token)
	record, err := s.resets.GetByHash(ctx, tokenHash)
	if err != nil {
		if errors.IsAppError(err) && errors.GetAppError(err).Code == 404 {
			return errors.ErrInvalidCredentials.WithMessage("invalid or expired token")
		}
		return fmt.Errorf("failed to look up token: %w", err)
	}
	if !record.IsValid(time.Now()) {
		return errors.ErrInvalidCredentials.WithMessage("invalid or expired token")
	}

	// Атомарно помечаем used. Если в этот момент другой confirm обогнал — получим error.
	if err := s.resets.MarkUsed(ctx, record.ID); err != nil {
		return errors.ErrInvalidCredentials.WithMessage("invalid or expired token")
	}

	user, err := s.users.GetByID(ctx, record.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	hash, err := s.authService.hashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = hash
	if err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	s.log.Info("Password reset completed",
		zap.String("user_id", user.ID.String()),
	)
	return nil
}

// generateResetToken создаёт 32-байтовый (256-битный) случайный токен в hex.
// Длина и энтропия достаточны чтобы не поддаваться brute-force (2^256 вариантов).
func generateResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken вычисляет sha256 в hex для stored-сравнения.
// Использование "обычного" sha256 вместо bcrypt приемлемо, т.к. входной токен
// уже имеет 256 бит случайности — bcrypt'овская соль не добавляет безопасности.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
