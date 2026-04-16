package domain

import (
	"time"

	"github.com/google/uuid"
)

// PasswordResetToken — одноразовый токен восстановления пароля (P1.11).
// token_hash хранится в БД; сам токен отправляется только в email.
type PasswordResetToken struct {
	ID          uuid.UUID  `db:"id"`
	UserID      uuid.UUID  `db:"user_id"`
	TokenHash   string     `db:"token_hash"`
	CreatedAt   time.Time  `db:"created_at"`
	ExpiresAt   time.Time  `db:"expires_at"`
	UsedAt      *time.Time `db:"used_at"`
	RequesterIP string     `db:"requester_ip"`
}

// IsValid возвращает true если токен не истёк и не использовался.
func (t *PasswordResetToken) IsValid(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}
