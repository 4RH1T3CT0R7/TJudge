package db

import (
	"context"
	"database/sql"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// ProgramRepository - репозиторий для работы с программами
type ProgramRepository struct {
	db *DB
}

// NewProgramRepository создаёт новый репозиторий программ
func NewProgramRepository(db *DB) *ProgramRepository {
	return &ProgramRepository{db: db}
}

// Create создаёт новую программу
func (r *ProgramRepository) Create(ctx context.Context, program *domain.Program) error {
	query := `
		INSERT INTO programs (id, user_id, name, game_type, code_path, language)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		program.ID,
		program.UserID,
		program.Name,
		program.GameType,
		program.CodePath,
		program.Language,
	).Scan(&program.CreatedAt, &program.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, "failed to create program")
	}

	return nil
}

// GetByID получает программу по ID
func (r *ProgramRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error) {
	var program domain.Program

	query := `
		SELECT id, user_id, name, game_type, code_path, language, created_at, updated_at
		FROM programs
		WHERE id = $1
	`

	err := r.db.QueryRowWithMetrics(ctx, "program_get_by_id", &program, query, id)
	if err == sql.ErrNoRows {
		return nil, errors.ErrProgramNotFound
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get program by id")
	}

	return &program, nil
}

// GetByUserID получает все программы пользователя
func (r *ProgramRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Program, error) {
	var programs []*domain.Program

	query := `
		SELECT id, user_id, name, game_type, code_path, language, created_at, updated_at
		FROM programs
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	err := r.db.QueryWithMetrics(ctx, "program_get_by_user_id", &programs, query, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get programs by user id")
	}

	return programs, nil
}

// GetByUserIDAndGameType получает программы пользователя по типу игры
func (r *ProgramRepository) GetByUserIDAndGameType(ctx context.Context, userID uuid.UUID, gameType string) ([]*domain.Program, error) {
	var programs []*domain.Program

	query := `
		SELECT id, user_id, name, game_type, code_path, language, created_at, updated_at
		FROM programs
		WHERE user_id = $1 AND game_type = $2
		ORDER BY created_at DESC
	`

	err := r.db.QueryWithMetrics(ctx, "program_get_by_user_game", &programs, query, userID, gameType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get programs by user and game type")
	}

	return programs, nil
}

// Update обновляет программу
func (r *ProgramRepository) Update(ctx context.Context, program *domain.Program) error {
	query := `
		UPDATE programs
		SET name = $2, code_path = $3, language = $4
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		program.ID,
		program.Name,
		program.CodePath,
		program.Language,
	).Scan(&program.UpdatedAt)

	if err == sql.ErrNoRows {
		return errors.ErrProgramNotFound
	}
	if err != nil {
		return errors.Wrap(err, "failed to update program")
	}

	return nil
}

// Delete удаляет программу
func (r *ProgramRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM programs WHERE id = $1`

	result, err := r.db.ExecWithMetrics(ctx, "program_delete", query, id)
	if err != nil {
		return errors.Wrap(err, "failed to delete program")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrProgramNotFound
	}

	return nil
}

// CheckOwnership проверяет, принадлежит ли программа пользователю
func (r *ProgramRepository) CheckOwnership(ctx context.Context, programID, userID uuid.UUID) (bool, error) {
	var exists bool

	query := `
		SELECT EXISTS(
			SELECT 1 FROM programs
			WHERE id = $1 AND user_id = $2
		)
	`

	err := r.db.QueryRowContext(ctx, query, programID, userID).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check program ownership")
	}

	return exists, nil
}
