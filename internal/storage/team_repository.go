package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	stderrors "errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// id - тай-брейк: при равном created_at порядок в postgres не определён, и
// OFFSET-страницы дублируют или теряют строки
const orderByCreatedAtDesc = " ORDER BY created_at DESC, id DESC"

type TeamRepository struct {
	db *DB
}

func NewTeamRepository(db *DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Create(ctx context.Context, team *models.Team) error {
	query := `
		INSERT INTO teams (id, tournament_id, name, code, leader_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		team.ID,
		team.TournamentID,
		team.Name,
		team.Code,
		team.LeaderID,
	).Scan(&team.CreatedAt, &team.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, "failed to create team")
	}

	return nil
}

func (r *TeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	var team models.Team

	query := `
		SELECT id, tournament_id, name, code, leader_id, is_disqualified, disqualified_at, created_at, updated_at
		FROM teams
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&team.ID,
		&team.TournamentID,
		&team.Name,
		&team.Code,
		&team.LeaderID,
		&team.IsDisqualified,
		&team.DisqualifiedAt,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("team not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get team by id")
	}

	return &team, nil
}

func (r *TeamRepository) GetByCode(ctx context.Context, code string) (*models.Team, error) {
	var team models.Team

	query := `
		SELECT id, tournament_id, name, code, leader_id, is_disqualified, disqualified_at, created_at, updated_at
		FROM teams
		WHERE code = $1
	`

	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&team.ID,
		&team.TournamentID,
		&team.Name,
		&team.Code,
		&team.LeaderID,
		&team.IsDisqualified,
		&team.DisqualifiedAt,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("team not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get team by code")
	}

	return &team, nil
}

func (r *TeamRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Team, error) {
	query := `
		SELECT id, tournament_id, name, code, leader_id, is_disqualified, disqualified_at, created_at, updated_at
		FROM teams
		WHERE tournament_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get teams by tournament id")
	}
	defer rows.Close()

	var teams []*models.Team
	for rows.Next() {
		var team models.Team

		err := rows.Scan(
			&team.ID,
			&team.TournamentID,
			&team.Name,
			&team.Code,
			&team.LeaderID,
			&team.IsDisqualified,
			&team.DisqualifiedAt,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan team")
		}

		teams = append(teams, &team)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return teams, nil
}

func (r *TeamRepository) List(ctx context.Context, filter models.TeamFilter) ([]*models.Team, error) {
	query := `
		SELECT id, tournament_id, name, code, leader_id, is_disqualified, disqualified_at, created_at, updated_at
		FROM teams
		WHERE 1=1
	`
	args := []any{}
	argCount := 1

	if filter.TournamentID != nil {
		query += fmt.Sprintf(" AND tournament_id = $%d", argCount)
		args = append(args, *filter.TournamentID)
		argCount++
	}

	if filter.LeaderID != nil {
		query += fmt.Sprintf(" AND leader_id = $%d", argCount)
		args = append(args, *filter.LeaderID)
		argCount++
	}

	query += orderByCreatedAtDesc

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
		argCount++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list teams")
	}
	defer rows.Close()

	var teams []*models.Team
	for rows.Next() {
		var team models.Team

		err := rows.Scan(
			&team.ID,
			&team.TournamentID,
			&team.Name,
			&team.Code,
			&team.LeaderID,
			&team.IsDisqualified,
			&team.DisqualifiedAt,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan team")
		}

		teams = append(teams, &team)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return teams, nil
}

func (r *TeamRepository) Update(ctx context.Context, team *models.Team) error {
	query := `
		UPDATE teams
		SET name = $2, leader_id = $3
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		team.ID,
		team.Name,
		team.LeaderID,
	).Scan(&team.UpdatedAt)

	if stderrors.Is(err, sql.ErrNoRows) {
		return errors.ErrNotFound.WithMessage("team not found")
	}
	if err != nil {
		return errors.Wrap(err, "failed to update team")
	}

	return nil
}

func (r *TeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM teams WHERE id = $1`

	result, err := r.db.ExecWithMetrics(ctx, "team_delete", query, id)
	if err != nil {
		return errors.Wrap(err, "failed to delete team")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("team not found")
	}

	return nil
}

func (r *TeamRepository) AddMember(ctx context.Context, member *models.TeamMember) error {
	query := `
		INSERT INTO team_members (id, team_id, user_id)
		VALUES ($1, $2, $3)
		RETURNING joined_at
	`

	err := r.db.QueryRowContext(ctx, query,
		member.ID,
		member.TeamID,
		member.UserID,
	).Scan(&member.JoinedAt)

	if err != nil {
		return errors.Wrap(err, "failed to add team member")
	}

	return nil
}

func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, teamID, userID)
	if err != nil {
		return errors.Wrap(err, "failed to remove team member")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("team member not found")
	}

	return nil
}

func (r *TeamRepository) GetMembers(ctx context.Context, teamID uuid.UUID) ([]*models.TeamMember, error) {
	query := `
		SELECT id, team_id, user_id, joined_at
		FROM team_members
		WHERE team_id = $1
		ORDER BY joined_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get team members")
	}
	defer rows.Close()

	var members []*models.TeamMember
	for rows.Next() {
		var member models.TeamMember

		err := rows.Scan(
			&member.ID,
			&member.TeamID,
			&member.UserID,
			&member.JoinedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan team member")
		}

		members = append(members, &member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return members, nil
}

func (r *TeamRepository) GetMemberCount(ctx context.Context, teamID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM team_members WHERE team_id = $1`

	err := r.db.QueryRowContext(ctx, query, teamID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get member count")
	}

	return count, nil
}

func (r *TeamRepository) IsUserInTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2)`

	err := r.db.QueryRowContext(ctx, query, teamID, userID).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check user in team")
	}

	return exists, nil
}

func (r *TeamRepository) IsUserInAnyTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM team_members tm
			INNER JOIN teams t ON tm.team_id = t.id
			WHERE t.tournament_id = $1 AND tm.user_id = $2
		)
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID, userID).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check user in tournament teams")
	}

	return exists, nil
}

func (r *TeamRepository) GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*models.Team, error) {
	var team models.Team

	query := `
		SELECT t.id, t.tournament_id, t.name, t.code, t.leader_id, t.is_disqualified, t.disqualified_at, t.created_at, t.updated_at
		FROM teams t
		INNER JOIN team_members tm ON t.id = tm.team_id
		WHERE t.tournament_id = $1 AND tm.user_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID, userID).Scan(
		&team.ID,
		&team.TournamentID,
		&team.Name,
		&team.Code,
		&team.LeaderID,
		&team.IsDisqualified,
		&team.DisqualifiedAt,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("user not in any team in this tournament")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user team in tournament")
	}

	return &team, nil
}

func (r *TeamRepository) GenerateUniqueCode(ctx context.Context) (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	// 32^8 = 2^40 вариантов: код не перебрать даже с сотни ip
	const codeLength = 8
	const maxAttempts = 10

	for range maxAttempts {
		code := make([]byte, codeLength)
		for i := range code {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", errors.Wrap(err, "failed to generate random number")
			}
			code[i] = charset[n.Int64()]
		}

		codeStr := string(code)

		// проверяется что такого кода ещё нет
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM teams WHERE code = $1)`
		err := r.db.QueryRowContext(ctx, query, codeStr).Scan(&exists)
		if err != nil {
			return "", errors.Wrap(err, "failed to check code uniqueness")
		}

		if !exists {
			return codeStr, nil
		}
	}

	return "", errors.ErrInternal.WithMessage("failed to generate unique code after max attempts")
}

func (r *TeamRepository) GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*models.TeamWithMembers, error) {
	team, err := r.GetByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// участники тянутся вместе с инфой о пользователях
	query := `
		SELECT u.id, u.username, u.email, u.role, u.created_at, u.updated_at
		FROM users u
		INNER JOIN team_members tm ON u.id = tm.user_id
		WHERE tm.team_id = $1
		ORDER BY tm.joined_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get team members with users")
	}
	defer rows.Close()

	var members []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan user")
		}
		members = append(members, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return &models.TeamWithMembers{
		Team:    *team,
		Members: members,
	}, nil
}

func (r *TeamRepository) IsTeamDisqualified(ctx context.Context, teamID uuid.UUID) (bool, error) {
	var disqualified bool
	query := `SELECT is_disqualified FROM teams WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, teamID).Scan(&disqualified)
	if stderrors.Is(err, sql.ErrNoRows) {
		return false, errors.ErrNotFound.WithMessage("team not found")
	}
	if err != nil {
		return false, errors.Wrap(err, "failed to check team disqualification")
	}

	return disqualified, nil
}

func (r *TeamRepository) RestoreTeam(ctx context.Context, teamID uuid.UUID) error {
	query := `
		UPDATE teams
		SET is_disqualified = false, disqualified_at = NULL, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, teamID)
	if err != nil {
		return errors.Wrap(err, "failed to restore team")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}
	if rows == 0 {
		return errors.ErrNotFound.WithMessage("team not found")
	}

	return nil
}

// DisqualifyTeamFull целиком дисквалифицирует команду в одной транзакции:
// метит команду, чистит завершённые матчи и их rating_history,
// отменяет pending/running и обнуляет статистику участников
func (r *TeamRepository) DisqualifyTeamFull(ctx context.Context, teamID, tournamentID uuid.UUID) (matchesDeleted, matchesCancelled, ratingHistoryDeleted int64, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	// 1. команда метится дисквалифицированной
	_, err = tx.ExecContext(ctx, `
		UPDATE teams SET is_disqualified = true, disqualified_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, teamID)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to mark team as disqualified")
	}

	// 2. берутся id программ этой команды в этом турнире
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM programs WHERE team_id = $1 AND tournament_id = $2
	`, teamID, tournamentID)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to get team programs")
	}

	var programIDs []uuid.UUID
	for rows.Next() {
		var pid uuid.UUID
		if err := rows.Scan(&pid); err != nil {
			rows.Close()
			return 0, 0, 0, errors.Wrap(err, "failed to scan program id")
		}
		programIDs = append(programIDs, pid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to iterate program ids")
	}

	if len(programIDs) == 0 {
		// программ нет — просто коммит отметки и выход
		if err := tx.Commit(); err != nil {
			return 0, 0, 0, errors.Wrap(err, "failed to commit transaction")
		}
		return 0, 0, 0, nil
	}

	// строю плейсхолдеры руками, sqlx.In лень подключать
	pidStrings := make([]any, len(programIDs))
	var placeholders strings.Builder
	for i, pid := range programIDs {
		pidStrings[i] = pid
		if i > 0 {
			placeholders.WriteString(", ")
		}
		fmt.Fprintf(&placeholders, "$%d", i+2) // $2, $3, ...
	}

	// 3. ELO соперников (виден на графике истории рейтинга) освобождается от
	// матчей с командой: рейтинг участника и более поздние точки его истории
	// сдвигаются на сумму удалённых дельт. wins/losses/draws участников нигде не
	// читаются (лидерборды считаются живым запросом по matches), пересчёт не нужен
	// ponytail: снимается только прямой вклад матчей, последующие дельты не
	// пересчитываются заново; точный вариант - replay ELO по оставшимся матчам
	args := append([]any{tournamentID}, pidStrings...)
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		WITH removed AS (
			SELECT rh.program_id, rh.change, rh.created_at
			FROM rating_history rh
			JOIN matches m ON m.id = rh.match_id AND m.tournament_id = $1
			WHERE rh.tournament_id = $1
			AND m.status IN ('completed', 'failed')
			AND (m.program1_id IN (%[1]s) OR m.program2_id IN (%[1]s))
			AND rh.program_id NOT IN (%[1]s)
		), shifted AS (
			UPDATE rating_history rh
			SET old_rating = rh.old_rating - s.shift, new_rating = rh.new_rating - s.shift
			FROM (
				SELECT h.id, h.created_at, SUM(r.change) AS shift
				FROM rating_history h
				JOIN removed r ON r.program_id = h.program_id AND r.created_at < h.created_at
				WHERE h.tournament_id = $1
				GROUP BY h.id, h.created_at
			) s
			WHERE rh.id = s.id AND rh.created_at = s.created_at
		)
		UPDATE tournament_participants tp
		SET rating = GREATEST(0, tp.rating - t.total)
		FROM (SELECT program_id, SUM(change) AS total FROM removed GROUP BY program_id) t
		WHERE tp.tournament_id = $1 AND tp.program_id = t.program_id
	`, placeholders.String()), args...)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to revert opponents rating")
	}

	// 4. снос rating_history по сыгранным матчам с программами команды.
	// failed тоже: форфейт (упала программа) даёт сопернику победу в лидербордах
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM rating_history
		WHERE tournament_id = $1
		AND match_id IN (
			SELECT id FROM matches
			WHERE tournament_id = $1
			AND status IN ('completed', 'failed')
			AND (program1_id IN (%s) OR program2_id IN (%s))
		)
	`, placeholders.String(), placeholders.String()), args...)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to delete rating history")
	}
	ratingHistoryDeleted, _ = result.RowsAffected()

	// 5. снос самих сыгранных матчей
	result, err = tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM matches
		WHERE tournament_id = $1
		AND status IN ('completed', 'failed')
		AND (program1_id IN (%s) OR program2_id IN (%s))
	`, placeholders.String(), placeholders.String()), args...)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to delete completed matches")
	}
	matchesDeleted, _ = result.RowsAffected()

	// 6. отмена pending и running (running воркер сам пропустит на финализации)
	result, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE matches
		SET status = 'cancelled', error_message = 'Team disqualified'
		WHERE tournament_id = $1
		AND status IN ('pending', 'running')
		AND (program1_id IN (%s) OR program2_id IN (%s))
	`, placeholders.String(), placeholders.String()), args...)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to cancel matches")
	}
	matchesCancelled, _ = result.RowsAffected()

	// 7. обнуление статистики только у этой команды
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE tournament_participants
		SET rating = 1500, wins = 0, losses = 0, draws = 0
		WHERE tournament_id = $1
		AND program_id IN (%s)
	`, placeholders.String()), args...)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to reset participant stats")
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to commit transaction")
	}

	return matchesDeleted, matchesCancelled, ratingHistoryDeleted, nil
}
