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
)

// --- stubs ---

type mockPWResetRepo struct{ mock.Mock }

func (m *mockPWResetRepo) Insert(ctx context.Context, t *domain.PasswordResetToken) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}
func (m *mockPWResetRepo) GetByHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PasswordResetToken), args.Error(1)
}
func (m *mockPWResetRepo) MarkUsed(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockMailer struct {
	mock.Mock
}

func (m *mockMailer) SendPasswordResetEmail(ctx context.Context, email, name, url string) error {
	args := m.Called(ctx, email, name, url)
	return args.Error(0)
}

type mockUserRepoPW struct{ mock.Mock }

func (m *mockUserRepoPW) Create(ctx context.Context, u *domain.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockUserRepoPW) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockUserRepoPW) GetByUsername(ctx context.Context, u string) (*domain.User, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockUserRepoPW) GetByEmail(ctx context.Context, e string) (*domain.User, error) {
	args := m.Called(ctx, e)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockUserRepoPW) Exists(ctx context.Context, u, e string) (bool, error) {
	args := m.Called(ctx, u, e)
	return args.Bool(0), args.Error(1)
}
func (m *mockUserRepoPW) Update(ctx context.Context, u *domain.User) error {
	return m.Called(ctx, u).Error(0)
}

func newPWResetFixture(t *testing.T) (*PasswordResetService, *mockUserRepoPW, *mockPWResetRepo, *mockMailer) {
	t.Helper()
	log, _ := logger.New("error", "json")
	users := new(mockUserRepoPW)
	resets := new(mockPWResetRepo)
	mail := new(mockMailer)
	jwt := NewJWTManager("test-secret-minimum-length-abcdefg", time.Hour, time.Hour)
	authSvc := NewService(users, jwt, nil, log)
	svc := NewPasswordResetService(users, resets, mail, "https://tjudge.example", authSvc, log)
	return svc, users, resets, mail
}

// --- tests ---

func TestPasswordReset_Request_UnknownEmail_NoEnumeration(t *testing.T) {
	svc, users, resets, mail := newPWResetFixture(t)
	users.On("GetByEmail", mock.Anything, "ghost@example.com").
		Return(nil, errors.ErrNotFound)

	err := svc.Request(context.Background(),
		PasswordResetRequest{Email: "ghost@example.com"}, "1.2.3.4")

	assert.NoError(t, err, "должен вернуть успех даже для неизвестного email (anti-enumeration)")
	resets.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	mail.AssertNotCalled(t, "SendPasswordResetEmail")
}

func TestPasswordReset_Request_KnownEmail_SendsEmail(t *testing.T) {
	svc, users, resets, mail := newPWResetFixture(t)
	user := &domain.User{ID: uuid.New(), Email: "u@example.com", Username: "u"}
	users.On("GetByEmail", mock.Anything, user.Email).Return(user, nil)
	resets.On("Insert", mock.Anything, mock.MatchedBy(func(t *domain.PasswordResetToken) bool {
		return t.UserID == user.ID && t.TokenHash != "" && t.RequesterIP == "1.2.3.4"
	})).Return(nil)
	mail.On("SendPasswordResetEmail", mock.Anything, user.Email, user.Username,
		mock.MatchedBy(func(url string) bool {
			return len(url) > len("https://tjudge.example/reset-password?token=")
		})).Return(nil)

	err := svc.Request(context.Background(),
		PasswordResetRequest{Email: user.Email}, "1.2.3.4")
	require.NoError(t, err)
	resets.AssertExpectations(t)
	mail.AssertExpectations(t)
}

func TestPasswordReset_Request_EmptyEmailFails(t *testing.T) {
	svc, _, _, _ := newPWResetFixture(t)
	err := svc.Request(context.Background(), PasswordResetRequest{Email: ""}, "")
	assert.Error(t, err)
}

func TestPasswordReset_Confirm_ShortPassword(t *testing.T) {
	svc, _, _, _ := newPWResetFixture(t)
	err := svc.Confirm(context.Background(),
		PasswordResetConfirm{Token: "abcd", NewPassword: "short"})
	assert.Error(t, err)
}

func TestPasswordReset_Confirm_TokenNotFound(t *testing.T) {
	svc, _, resets, _ := newPWResetFixture(t)
	resets.On("GetByHash", mock.Anything, mock.Anything).Return(nil, errors.ErrNotFound)

	err := svc.Confirm(context.Background(),
		PasswordResetConfirm{Token: "deadbeef", NewPassword: "longenoughpw"})
	assert.Error(t, err)
}

func TestPasswordReset_Confirm_ExpiredToken(t *testing.T) {
	svc, _, resets, _ := newPWResetFixture(t)
	expired := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	resets.On("GetByHash", mock.Anything, mock.Anything).Return(expired, nil)

	err := svc.Confirm(context.Background(),
		PasswordResetConfirm{Token: "validhex", NewPassword: "newgoodpwd"})
	assert.Error(t, err)
}

func TestPasswordReset_Confirm_Success(t *testing.T) {
	svc, users, resets, _ := newPWResetFixture(t)
	userID := uuid.New()
	token := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	resets.On("GetByHash", mock.Anything, mock.Anything).Return(token, nil)
	resets.On("MarkUsed", mock.Anything, token.ID).Return(nil)
	user := &domain.User{ID: userID, Username: "u", Email: "u@e.c"}
	users.On("GetByID", mock.Anything, userID).Return(user, nil)
	users.On("Update", mock.Anything, mock.MatchedBy(func(u *domain.User) bool {
		// Пароль должен быть bcrypt-хэшем (длина > 50)
		return u.ID == userID && len(u.PasswordHash) > 50
	})).Return(nil)

	err := svc.Confirm(context.Background(),
		PasswordResetConfirm{Token: "validhex", NewPassword: "strongpassword123"})
	require.NoError(t, err)
	users.AssertExpectations(t)
	resets.AssertExpectations(t)
}

func TestPasswordReset_Confirm_MarkUsedFailsOnRace(t *testing.T) {
	svc, _, resets, _ := newPWResetFixture(t)
	token := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	resets.On("GetByHash", mock.Anything, mock.Anything).Return(token, nil)
	// MarkUsed возвращает conflict — значит кто-то уже применил токен.
	resets.On("MarkUsed", mock.Anything, token.ID).
		Return(errors.ErrConflict.WithMessage("already used"))

	err := svc.Confirm(context.Background(),
		PasswordResetConfirm{Token: "validhex", NewPassword: "newgoodpwd"})
	assert.Error(t, err, "race при MarkUsed должна возвращать ошибку")
}

func TestHashToken_Deterministic(t *testing.T) {
	h1 := hashToken("abc")
	h2 := hashToken("abc")
	h3 := hashToken("abd")
	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Len(t, h1, 64, "sha256 hex = 64 chars")
}

func TestGenerateResetToken_HighEntropy(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		tok, err := generateResetToken()
		require.NoError(t, err)
		assert.Len(t, tok, 64, "hex(32 bytes) = 64 chars")
		_, dup := seen[tok]
		assert.False(t, dup, "токен не должен повторяться")
		seen[tok] = struct{}{}
	}
}
