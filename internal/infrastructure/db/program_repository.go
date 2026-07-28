package db

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type ProgramRepository struct {
	db *DB
}

func NewProgramRepository(db *DB) *ProgramRepository {
	return &ProgramRepository{db: db}
}

func (r *ProgramRepository) Create(ctx context.Context, program *domain.Program) error {
	if program.Status == "" {
		program.Status = domain.ProgramReady
	}

	query := `
		INSERT INTO programs (id, user_id, team_id, tournament_id, game_id, name, game_type, code_path, file_path, language, status, error_message, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
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
		program.Status,
		program.ErrorMessage,
		program.Version,
	).Scan(&program.CreatedAt, &program.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, "failed to create program")
	}

	return nil
}

// CreateWithAtomicVersion создаёт программу и сам считает версию:
// COALESCE(MAX(version),0)+1 прямо внутри INSERT, без отдельного запроса.
// если две загрузки прилетели одновременно — уникальный индекс ругнётся,
// тогда повторяем с новым id, до 3 раз
func (r *ProgramRepository) CreateWithAtomicVersion(ctx context.Context, program *domain.Program) error {
	if program.Status == "" {
		program.Status = domain.ProgramReady
	}

	query := `
		INSERT INTO programs (id, user_id, team_id, tournament_id, game_id, name, game_type, code_path, file_path, language, status, error_message, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			COALESCE((SELECT MAX(version) FROM programs WHERE team_id = $3 AND game_id = $5), 0) + 1
		)
		RETURNING version, created_at, updated_at
	`

	const maxRetries = 3
	for attempt := range maxRetries {
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
			program.Status,
			program.ErrorMessage,
		).Scan(&program.Version, &program.CreatedAt, &program.UpdatedAt)

		if err == nil {
			return nil
		}

		// повтор на конфликте уникального индекса (две версии столкнулись)
		var pqErr *pq.Error
		if stderrors.As(err, &pqErr) && pqErr.Code == "23505" && attempt < maxRetries-1 {
			program.ID = uuid.New() // новый id на повтор, старый мог сгореть на упавшем INSERT
			continue
		}

		return errors.Wrap(err, "failed to create program with atomic version")
	}

	return errors.Wrap(fmt.Errorf("max retries exceeded"), "failed to create program with atomic version")
}

func (r *ProgramRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error) {
	var program domain.Program

	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, status, error_message, version, created_at, updated_at
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
		&program.Status,
		&program.ErrorMessage,
		&program.Version,
		&program.CreatedAt,
		&program.UpdatedAt,
	)
	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrProgramNotFound
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get program by id")
	}

	return &program, nil
}

func (r *ProgramRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Program, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, status, error_message, version, created_at, updated_at
		FROM programs
		WHERE id = ANY($1)
	`

	var programs []*domain.Program
	err := r.db.QueryWithMetrics(ctx, "program_get_by_ids", &programs, query, pq.Array(ids))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get programs by ids")
	}

	return programs, nil
}

func (r *ProgramRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Program, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, status, error_message, version, created_at, updated_at
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
			&p.Status,
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return programs, nil
}

func (r *ProgramRepository) GetByUserIDAndGameType(ctx context.Context, userID uuid.UUID, gameType string) ([]*domain.Program, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, status, error_message, version, created_at, updated_at
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
			&p.Status,
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return programs, nil
}

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

	if stderrors.Is(err, sql.ErrNoRows) {
		return errors.ErrProgramNotFound
	}
	if err != nil {
		return errors.Wrap(err, "failed to update program")
	}

	return nil
}

// UpdateCompileResult пишет итог компиляции: статус, путь к бинарю
// (или к исходнику для интерпретируемых языков) и текст ошибки.
// зовётся из compile-worker'а после сборки в докер-песочнице
func (r *ProgramRepository) UpdateCompileResult(ctx context.Context, id uuid.UUID, status domain.ProgramStatus, codePath string, errorMessage *string) error {
	query := `
		UPDATE programs
		SET status = $2, code_path = $3, error_message = $4, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecWithMetrics(ctx, "program_update_compile_result", query, id, status, codePath, errorMessage)
	if err != nil {
		return errors.Wrap(err, "failed to update compile result")
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

// GetStuckCompiling достаёт программы, застрявшие в compiling дольше olderThan —
// значит задача где-то потерялась (упали между созданием и enqueue, или редис лёг).
// compile-worker периодически закидывает их обратно в очередь
func (r *ProgramRepository) GetStuckCompiling(ctx context.Context, olderThan time.Duration, limit int) ([]*domain.Program, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, status, error_message, version, created_at, updated_at
		FROM programs
		WHERE status = 'compiling' AND updated_at < NOW() - $1::interval
		ORDER BY updated_at ASC
		LIMIT $2
	`

	var programs []*domain.Program
	err := r.db.QueryWithMetrics(ctx, "program_get_stuck_compiling", &programs, query, olderThan.String(), limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stuck compiling programs")
	}

	return programs, nil
}

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

func (r *ProgramRepository) ClearErrorMessages(ctx context.Context, tournamentID uuid.UUID) (int64, error) {
	query := `
		UPDATE programs
		SET error_message = NULL
		WHERE tournament_id = $1 AND error_message IS NOT NULL
	`
	result, err := r.db.ExecContext(ctx, query, tournamentID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to clear error messages")
	}
	return result.RowsAffected()
}

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

// GetByTournamentAndGame отдаёт только последние версии программ по каждой команде турнира
func (r *ProgramRepository) GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error) {
	// DISTINCT ON берёт только последнюю версию по каждой команде
	query := `
		SELECT DISTINCT ON (team_id)
		       id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, status, error_message, version, created_at, updated_at
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
			&p.Status,
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return programs, nil
}

func (r *ProgramRepository) GetAllVersionsByTeamAndGame(ctx context.Context, teamID, gameID uuid.UUID) ([]*domain.Program, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, game_id, name, game_type,
		       code_path, file_path, language, status, error_message, version, created_at, updated_at
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
			&p.Status,
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return programs, nil
}
