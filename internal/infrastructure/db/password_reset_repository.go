package db

import (
	"context"
	"database/sql"
	stderrors "errors"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

// PasswordResetRepository — репозиторий токенов восстановления пароля (P1.11).
type PasswordResetRepository struct {
	db *DB
}

// NewPasswordResetRepository создаёт репозиторий.
func NewPasswordResetRepository(db *DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

// Insert сохраняет новый токен восстановления.
func (r *PasswordResetRepository) Insert(ctx context.Context, t *domain.PasswordResetToken) error {
	const query = `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, created_at, expires_at, requester_ip)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		t.ID, t.UserID, t.TokenHash, t.CreatedAt, t.ExpiresAt, t.RequesterIP,
	)
	if err != nil {
		return errors.Wrap(err, "failed to insert password reset token")
	}
	return nil
}

// GetByHash ищет токен по token_hash. Не фильтрует по used/expired —
// валидацию выполняет сервис (чтобы различать "не найден" vs "уже использован").
func (r *PasswordResetRepository) GetByHash(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error) {
	const query = `
		SELECT id, user_id, token_hash, created_at, expires_at, used_at, requester_ip
		FROM password_reset_tokens
		WHERE token_hash = $1
	`
	var t domain.PasswordResetToken
	err := r.db.QueryRowWithMetrics(ctx, "password_reset_get_by_hash", &t, query, tokenHash)
	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("password reset token not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to query password reset token")
	}
	return &t, nil
}

// MarkUsed атомарно помечает токен использованным, возвращая ошибку если токен
// уже был использован или истёк. Защищает от race при confirm.
func (r *PasswordResetRepository) MarkUsed(ctx context.Context, id interface{}) error {
	const query = `
		UPDATE password_reset_tokens
		SET used_at = now()
		WHERE id = $1 AND used_at IS NULL AND expires_at > now()
	`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, "failed to mark password reset token as used")
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}
	if rows == 0 {
		return errors.ErrConflict.WithMessage("token already used or expired")
	}
	return nil
}

// DeleteExpired удаляет истёкшие токены (background cleanup).
func (r *PasswordResetRepository) DeleteExpired(ctx context.Context) (int64, error) {
	const query = `DELETE FROM password_reset_tokens WHERE expires_at < now()`
	res, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete expired password reset tokens")
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}
	return rows, nil
}
