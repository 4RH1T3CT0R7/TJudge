package db

import (
	"context"
	"database/sql"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// UserRepository - репозиторий для работы с пользователями
type UserRepository struct {
	db *DB
}

// NewUserRepository создаёт новый репозиторий пользователей
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create создаёт нового пользователя
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.PasswordHash,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, "failed to create user")
	}

	return nil
}

// GetByID получает пользователя по ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User

	query := `
		SELECT id, username, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	err := r.db.QueryRowWithMetrics(ctx, "user_get_by_id", &user, query, id)
	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound.WithMessage("user not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user by id")
	}

	return &user, nil
}

// GetByUsername получает пользователя по username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User

	query := `
		SELECT id, username, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE username = $1
	`

	err := r.db.QueryRowWithMetrics(ctx, "user_get_by_username", &user, query, username)
	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound.WithMessage("user not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user by username")
	}

	return &user, nil
}

// GetByEmail получает пользователя по email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	query := `
		SELECT id, username, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	err := r.db.QueryRowWithMetrics(ctx, "user_get_by_email", &user, query, email)
	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound.WithMessage("user not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user by email")
	}

	return &user, nil
}

// Update обновляет пользователя
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET username = $2, email = $3, password_hash = $4
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.PasswordHash,
	).Scan(&user.UpdatedAt)

	if err == sql.ErrNoRows {
		return errors.ErrNotFound.WithMessage("user not found")
	}
	if err != nil {
		return errors.Wrap(err, "failed to update user")
	}

	return nil
}

// Delete удаляет пользователя
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecWithMetrics(ctx, "user_delete", query, id)
	if err != nil {
		return errors.Wrap(err, "failed to delete user")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("user not found")
	}

	return nil
}

// Exists проверяет существование пользователя по username или email
func (r *UserRepository) Exists(ctx context.Context, username, email string) (bool, error) {
	var exists bool

	query := `
		SELECT EXISTS(
			SELECT 1 FROM users
			WHERE username = $1 OR email = $2
		)
	`

	err := r.db.QueryRowContext(ctx, query, username, email).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check user existence")
	}

	return exists, nil
}
