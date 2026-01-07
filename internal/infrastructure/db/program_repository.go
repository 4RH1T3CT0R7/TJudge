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
		INSERT INTO programs (id, user_id, team_id, tournament_id, game_id, name, game_type, code_path, file_path, language, error_message, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		program.ID,
		program.UserID,
		program.TeamID,
		program.TournamentID,
		program.GameID,
		program.Name,
		program.GameType,
		program.CodePath,
		program.FilePath,
		program.Language,
		program.ErrorMessage,
		program.Version,
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
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, error_message, version, created_at, updated_at
		FROM programs
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&program.ID,
		&program.UserID,
		&program.TeamID,
		&program.TournamentID,
		&program.GameID,
		&program.Name,
		&program.GameType,
		&program.CodePath,
		&program.FilePath,
		&program.Language,
		&program.ErrorMessage,
		&program.Version,
		&program.CreatedAt,
		&program.UpdatedAt,
	)
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
	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, error_message, version, created_at, updated_at
		FROM programs
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get programs by user id")
	}
	defer rows.Close()

	var programs []*domain.Program
	for rows.Next() {
		var p domain.Program
		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.TeamID,
			&p.TournamentID,
			&p.GameID,
			&p.Name,
			&p.GameType,
			&p.CodePath,
			&p.FilePath,
			&p.Language,
			&p.ErrorMessage,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan program")
		}
		programs = append(programs, &p)
	}

	return programs, nil
}

// GetByUserIDAndGameType получает программы пользователя по типу игры
func (r *ProgramRepository) GetByUserIDAndGameType(ctx context.Context, userID uuid.UUID, gameType string) ([]*domain.Program, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, error_message, version, created_at, updated_at
		FROM programs
		WHERE user_id = $1 AND game_type = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, gameType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get programs by user and game type")
	}
	defer rows.Close()

	var programs []*domain.Program
	for rows.Next() {
		var p domain.Program
		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.TeamID,
			&p.TournamentID,
			&p.GameID,
			&p.Name,
			&p.GameType,
			&p.CodePath,
			&p.FilePath,
			&p.Language,
			&p.ErrorMessage,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan program")
		}
		programs = append(programs, &p)
	}

	return programs, nil
}

// Update обновляет программу
func (r *ProgramRepository) Update(ctx context.Context, program *domain.Program) error {
	query := `
		UPDATE programs
		SET name = $2, code_path = $3, language = $4, error_message = $5
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		program.ID,
		program.Name,
		program.CodePath,
		program.Language,
		program.ErrorMessage,
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

// GetLatestVersion получает последнюю версию программы для команды и игры
func (r *ProgramRepository) GetLatestVersion(ctx context.Context, teamID, gameID uuid.UUID) (int, error) {
	var version int

	query := `
		SELECT COALESCE(MAX(version), 0)
		FROM programs
		WHERE team_id = $1 AND game_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, teamID, gameID).Scan(&version)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get latest version")
	}

	return version, nil
}

// GetByTournamentAndGame получает только ПОСЛЕДНИЕ версии программ для каждой команды в турнире
func (r *ProgramRepository) GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error) {
	// Используем DISTINCT ON для получения только последней версии программы для каждой команды
	query := `
		SELECT DISTINCT ON (team_id)
		       id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, error_message, version, created_at, updated_at
		FROM programs
		WHERE tournament_id = $1 AND game_id = $2 AND team_id IS NOT NULL
		ORDER BY team_id, version DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get programs by tournament and game")
	}
	defer rows.Close()

	var programs []*domain.Program
	for rows.Next() {
		var p domain.Program
		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.TeamID,
			&p.TournamentID,
			&p.GameID,
			&p.Name,
			&p.GameType,
			&p.CodePath,
			&p.FilePath,
			&p.Language,
			&p.ErrorMessage,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan program")
		}
		programs = append(programs, &p)
	}

	return programs, nil
}

// GetAllVersionsByTeamAndGame получает ВСЕ версии программ для команды и игры
func (r *ProgramRepository) GetAllVersionsByTeamAndGame(ctx context.Context, teamID, gameID uuid.UUID) ([]*domain.Program, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, error_message, version, created_at, updated_at
		FROM programs
		WHERE team_id = $1 AND game_id = $2
		ORDER BY version DESC
	`

	rows, err := r.db.QueryContext(ctx, query, teamID, gameID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get program versions")
	}
	defer rows.Close()

	var programs []*domain.Program
	for rows.Next() {
		var p domain.Program
		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.TeamID,
			&p.TournamentID,
			&p.GameID,
			&p.Name,
			&p.GameType,
			&p.CodePath,
			&p.FilePath,
			&p.Language,
			&p.ErrorMessage,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan program")
		}
		programs = append(programs, &p)
	}

	return programs, nil
}
