package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// utc - время для колонок TIMESTAMP без зоны: смещение postgres молча отбрасывает,
// поэтому в бд пишется UTC, как и у NOW() в сессии с timezone=UTC
func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// TournamentRepository - репозиторий для работы с турнирами
type TournamentRepository struct {
	db *DB
}

func NewTournamentRepository(db *DB) *TournamentRepository {
	return &TournamentRepository{db: db}
}

func (r *TournamentRepository) Create(ctx context.Context, tournament *models.Tournament) error {
	metadata, err := json.Marshal(tournament.Metadata)
	if err != nil {
		return errors.Wrap(err, "failed to marshal metadata")
	}

	query := `
		INSERT INTO tournaments (id, code, name, description, game_type, status, max_participants, max_team_size, is_permanent, creator_id, start_time, end_time, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at, version
	`

	err = r.db.QueryRowContext(ctx, query,
		tournament.ID,
		tournament.Code,
		tournament.Name,
		tournament.Description,
		tournament.GameType,
		tournament.Status,
		tournament.MaxParticipants,
		tournament.MaxTeamSize,
		tournament.IsPermanent,
		tournament.CreatorID,
		utc(tournament.StartTime),
		utc(tournament.EndTime),
		metadata,
	).Scan(&tournament.CreatedAt, &tournament.UpdatedAt, &tournament.Version)

	if err != nil {
		return errors.Wrap(err, "failed to create tournament")
	}

	return nil
}

func (r *TournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error) {
	var tournament models.Tournament
	var metadataJSON []byte

	query := `
		SELECT id, code, name, description, game_type, status, max_participants, max_team_size, is_permanent, creator_id, start_time, end_time,
		       metadata, version, created_at, updated_at
		FROM tournaments
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tournament.ID,
		&tournament.Code,
		&tournament.Name,
		&tournament.Description,
		&tournament.GameType,
		&tournament.Status,
		&tournament.MaxParticipants,
		&tournament.MaxTeamSize,
		&tournament.IsPermanent,
		&tournament.CreatorID,
		&tournament.StartTime,
		&tournament.EndTime,
		&metadataJSON,
		&tournament.Version,
		&tournament.CreatedAt,
		&tournament.UpdatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("tournament not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament by id")
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &tournament.Metadata); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal metadata")
		}
	}

	return &tournament, nil
}

func (r *TournamentRepository) List(ctx context.Context, filter models.TournamentFilter) ([]*models.Tournament, error) {
	query := `
		SELECT id, code, name, description, game_type, status, max_participants, max_team_size, is_permanent, creator_id, start_time, end_time,
		       metadata, version, created_at, updated_at
		FROM tournaments
		WHERE 1=1
	`
	args := []any{}
	argCount := 1

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, filter.Status)
		argCount++
	}

	if filter.GameType != "" {
		query += fmt.Sprintf(" AND game_type = $%d", argCount)
		args = append(args, filter.GameType)
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
		return nil, errors.Wrap(err, "failed to list tournaments")
	}
	defer rows.Close()

	var tournaments []*models.Tournament
	for rows.Next() {
		var tournament models.Tournament
		var metadataJSON []byte

		err := rows.Scan(
			&tournament.ID,
			&tournament.Code,
			&tournament.Name,
			&tournament.Description,
			&tournament.GameType,
			&tournament.Status,
			&tournament.MaxParticipants,
			&tournament.MaxTeamSize,
			&tournament.IsPermanent,
			&tournament.CreatorID,
			&tournament.StartTime,
			&tournament.EndTime,
			&metadataJSON,
			&tournament.Version,
			&tournament.CreatedAt,
			&tournament.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan tournament")
		}

		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &tournament.Metadata); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal metadata")
			}
		}

		tournaments = append(tournaments, &tournament)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return tournaments, nil
}

// Update обновляет турнир с optimistic lock: апдейт проходит только если version
// в базе совпала с прочитанной, иначе кто-то успел обновить раньше и наружу
// отдаётся ErrConcurrentUpdate. version инкрементится тем же запросом
func (r *TournamentRepository) Update(ctx context.Context, tournament *models.Tournament) error {
	metadata, err := json.Marshal(tournament.Metadata)
	if err != nil {
		return errors.Wrap(err, "failed to marshal metadata")
	}

	query := `
		UPDATE tournaments
		SET name = $2, description = $3, status = $4, max_participants = $5, max_team_size = $6,
		    is_permanent = $7, start_time = $8, end_time = $9, metadata = $10, version = version + 1
		WHERE id = $1 AND version = $11
		RETURNING updated_at, version
	`

	err = r.db.QueryRowContext(ctx, query,
		tournament.ID,
		tournament.Name,
		tournament.Description,
		tournament.Status,
		tournament.MaxParticipants,
		tournament.MaxTeamSize,
		tournament.IsPermanent,
		utc(tournament.StartTime),
		utc(tournament.EndTime),
		metadata,
		tournament.Version,
	).Scan(&tournament.UpdatedAt, &tournament.Version)

	if stderrors.Is(err, sql.ErrNoRows) {
		return errors.ErrConcurrentUpdate
	}
	if err != nil {
		return errors.Wrap(err, "failed to update tournament")
	}

	return nil
}

// Complete завершает турнир одной транзакцией: статус completed с end_time (optimistic
// lock по version, как в Update) и отмена pending и running матчей. при ошибке не
// меняется ничего: ни активного турнира с отменённым раундом, ни завершённого с
// матчами в очереди. pending воркер пропустит (в running переводит только из pending).
// возвращает число отменённых матчей
func (r *TournamentRepository) Complete(ctx context.Context, tournament *models.Tournament) (int64, error) {
	var cancelled int64
	err := r.db.RunInTx(ctx, func(tx *sqlx.Tx) error {
		err := tx.QueryRowContext(ctx, `
			UPDATE tournaments
			SET status = 'completed', end_time = $2, version = version + 1
			WHERE id = $1 AND version = $3
			RETURNING updated_at, version
		`, tournament.ID, utc(tournament.EndTime), tournament.Version).Scan(&tournament.UpdatedAt, &tournament.Version)
		if stderrors.Is(err, sql.ErrNoRows) {
			return errors.ErrConcurrentUpdate
		}
		if err != nil {
			return errors.Wrap(err, "failed to complete tournament")
		}

		res, err := tx.ExecContext(ctx, `
			UPDATE matches
			SET status = 'cancelled', error_message = 'Tournament completed'
			WHERE tournament_id = $1 AND status IN ('pending', 'running')
		`, tournament.ID)
		if err != nil {
			return errors.Wrap(err, "failed to cancel active matches")
		}
		cancelled, err = res.RowsAffected()
		return err
	})
	return cancelled, err
}

func (r *TournamentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TournamentStatus) error {
	query := `
		UPDATE tournaments
		SET status = $2, version = version + 1
		WHERE id = $1
	`

	result, err := r.db.ExecWithMetrics(ctx, "tournament_update_status", query, id, status)
	if err != nil {
		return errors.Wrap(err, "failed to update tournament status")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("tournament not found")
	}

	return nil
}

func (r *TournamentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tournaments WHERE id = $1`

	result, err := r.db.ExecWithMetrics(ctx, "tournament_delete", query, id)
	if err != nil {
		return errors.Wrap(err, "failed to delete tournament")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("tournament not found")
	}

	return nil
}

func (r *TournamentRepository) GetParticipantsCount(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var count int

	query := `SELECT COUNT(*) FROM tournament_participants WHERE tournament_id = $1`

	err := r.db.QueryRowContext(ctx, query, tournamentID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get participants count")
	}

	return count, nil
}

func (r *TournamentRepository) GetTeamsCount(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var count int

	query := `SELECT COUNT(*) FROM teams WHERE tournament_id = $1`

	err := r.db.QueryRowContext(ctx, query, tournamentID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get teams count")
	}

	return count, nil
}

func (r *TournamentRepository) AddParticipant(ctx context.Context, participant *models.TournamentParticipant) error {
	query := `
		INSERT INTO tournament_participants (id, tournament_id, program_id, rating)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`

	err := r.db.QueryRowContext(ctx, query,
		participant.ID,
		participant.TournamentID,
		participant.ProgramID,
		participant.Rating,
	).Scan(&participant.CreatedAt)

	if err != nil {
		return errors.Wrap(err, "failed to add tournament participant")
	}

	return nil
}

// GetLatestParticipantsGroupedByGame - участники турнира, сгруппированные по играм (map game_type -> участники)
func (r *TournamentRepository) GetLatestParticipantsGroupedByGame(ctx context.Context, tournamentID uuid.UUID) (map[string][]*models.TournamentParticipant, error) {
	return r.latestReadyParticipants(ctx, tournamentID, "")
}

// GetLatestParticipantsByGame - участники одной игры турнира
func (r *TournamentRepository) GetLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*models.TournamentParticipant, error) {
	byGame, err := r.latestReadyParticipants(ctx, tournamentID, gameType)
	if err != nil {
		return nil, err
	}
	return byGame[gameType], nil
}

// latestReadyParticipants - общий запрос участников для планировщика (RunAll, RunGame,
// авто-раунд). берётся последняя готовая версия программы команды по каждой игре:
// compiling ещё не собралась, failed не собралась вообще, поэтому при сломанной новой
// версии команда играет предыдущей рабочей (MAX(version) среди ready).
// программы игр, не привязанных к турниру, отсекаются JOIN'ом с tournament_games.
// gameType == "" - все игры
func (r *TournamentRepository) latestReadyParticipants(ctx context.Context, tournamentID uuid.UUID, gameType string) (map[string][]*models.TournamentParticipant, error) {
	query := `
		SELECT tp.id, tp.tournament_id, tp.program_id, tp.rating, tp.wins, tp.losses, tp.draws, tp.created_at, g.name as game_type
		FROM tournament_participants tp
		INNER JOIN programs p ON p.id = tp.program_id
		INNER JOIN games g ON g.id = p.game_id
		INNER JOIN tournament_games tg ON tg.tournament_id = tp.tournament_id AND tg.game_id = p.game_id
		INNER JOIN teams t ON t.id = p.team_id AND t.is_disqualified = false
		WHERE tp.tournament_id = $1
		  AND ($2 = '' OR g.name = $2)
		  AND p.status = 'ready'
		  AND p.version = (
		      SELECT MAX(p2.version)
		      FROM programs p2
		      WHERE p2.team_id = p.team_id
		        AND p2.game_id = p.game_id
		        AND p2.tournament_id = p.tournament_id
		        AND p2.status = 'ready'
		  )
		ORDER BY g.name, tp.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest participants")
	}
	defer rows.Close()

	result := make(map[string][]*models.TournamentParticipant)
	for rows.Next() {
		var p models.TournamentParticipant
		var game string
		err := rows.Scan(
			&p.ID,
			&p.TournamentID,
			&p.ProgramID,
			&p.Rating,
			&p.Wins,
			&p.Losses,
			&p.Draws,
			&p.CreatedAt,
			&game,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan participant with game type")
		}
		result[game] = append(result[game], &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}
