package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// TeamRepository - репозиторий для работы с командами
type TeamRepository struct {
	db *DB
}

// NewTeamRepository создаёт новый репозиторий команд
func NewTeamRepository(db *DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create создаёт новую команду
func (r *TeamRepository) Create(ctx context.Context, team *domain.Team) error {
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

// GetByID получает команду по ID
func (r *TeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Team, error) {
	var team domain.Team

	query := `
		SELECT id, tournament_id, name, code, leader_id, created_at, updated_at
		FROM teams
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&team.ID,
		&team.TournamentID,
		&team.Name,
		&team.Code,
		&team.LeaderID,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound.WithMessage("team not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get team by id")
	}

	return &team, nil
}

// GetByCode получает команду по уникальному коду
func (r *TeamRepository) GetByCode(ctx context.Context, code string) (*domain.Team, error) {
	var team domain.Team

	query := `
		SELECT id, tournament_id, name, code, leader_id, created_at, updated_at
		FROM teams
		WHERE code = $1
	`

	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&team.ID,
		&team.TournamentID,
		&team.Name,
		&team.Code,
		&team.LeaderID,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound.WithMessage("team not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get team by code")
	}

	return &team, nil
}

// GetByTournamentID получает все команды турнира
func (r *TeamRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Team, error) {
	query := `
		SELECT id, tournament_id, name, code, leader_id, created_at, updated_at
		FROM teams
		WHERE tournament_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get teams by tournament id")
	}
	defer rows.Close()

	var teams []*domain.Team
	for rows.Next() {
		var team domain.Team

		err := rows.Scan(
			&team.ID,
			&team.TournamentID,
			&team.Name,
			&team.Code,
			&team.LeaderID,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan team")
		}

		teams = append(teams, &team)
	}

	return teams, nil
}

// List получает список команд с фильтрацией
func (r *TeamRepository) List(ctx context.Context, filter domain.TeamFilter) ([]*domain.Team, error) {
	query := `
		SELECT id, tournament_id, name, code, leader_id, created_at, updated_at
		FROM teams
		WHERE 1=1
	`
	args := []interface{}{}
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

	query += " ORDER BY created_at DESC"

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

	var teams []*domain.Team
	for rows.Next() {
		var team domain.Team

		err := rows.Scan(
			&team.ID,
			&team.TournamentID,
			&team.Name,
			&team.Code,
			&team.LeaderID,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan team")
		}

		teams = append(teams, &team)
	}

	return teams, nil
}

// Update обновляет команду
func (r *TeamRepository) Update(ctx context.Context, team *domain.Team) error {
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

	if err == sql.ErrNoRows {
		return errors.ErrNotFound.WithMessage("team not found")
	}
	if err != nil {
		return errors.Wrap(err, "failed to update team")
	}

	return nil
}

// Delete удаляет команду
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

// AddMember добавляет участника в команду
func (r *TeamRepository) AddMember(ctx context.Context, member *domain.TeamMember) error {
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

// RemoveMember удаляет участника из команды
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

// GetMembers получает всех участников команды
func (r *TeamRepository) GetMembers(ctx context.Context, teamID uuid.UUID) ([]*domain.TeamMember, error) {
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

	var members []*domain.TeamMember
	for rows.Next() {
		var member domain.TeamMember

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

	return members, nil
}

// GetMemberCount получает количество участников команды
func (r *TeamRepository) GetMemberCount(ctx context.Context, teamID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM team_members WHERE team_id = $1`

	err := r.db.QueryRowContext(ctx, query, teamID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get member count")
	}

	return count, nil
}

// IsUserInTeam проверяет, является ли пользователь членом команды
func (r *TeamRepository) IsUserInTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2)`

	err := r.db.QueryRowContext(ctx, query, teamID, userID).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check user in team")
	}

	return exists, nil
}

// IsUserInAnyTeamInTournament проверяет, состоит ли пользователь в какой-либо команде турнира
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

// GetUserTeamInTournament получает команду пользователя в турнире
func (r *TeamRepository) GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*domain.Team, error) {
	var team domain.Team

	query := `
		SELECT t.id, t.tournament_id, t.name, t.code, t.leader_id, t.created_at, t.updated_at
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
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound.WithMessage("user not in any team in this tournament")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user team in tournament")
	}

	return &team, nil
}

// GenerateUniqueCode генерирует уникальный код для команды
func (r *TeamRepository) GenerateUniqueCode(ctx context.Context) (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const codeLength = 6
	const maxAttempts = 10

	for attempt := 0; attempt < maxAttempts; attempt++ {
		code := make([]byte, codeLength)
		for i := range code {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", errors.Wrap(err, "failed to generate random number")
			}
			code[i] = charset[n.Int64()]
		}

		codeStr := string(code)

		// Проверяем уникальность
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

// GetTeamWithMembers получает команду вместе с участниками
func (r *TeamRepository) GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*domain.TeamWithMembers, error) {
	team, err := r.GetByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Получаем участников с информацией о пользователях
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

	var members []domain.User
	for rows.Next() {
		var user domain.User
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

	return &domain.TeamWithMembers{
		Team:    *team,
		Members: members,
	}, nil
}
