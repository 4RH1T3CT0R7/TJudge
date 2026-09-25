package storage

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type MatchRepository struct {
	db *DB
}

func NewMatchRepository(db *DB) *MatchRepository {
	return &MatchRepository{db: db}
}

func (r *MatchRepository) Create(ctx context.Context, match *models.Match) error {
	query := `
		INSERT INTO matches (id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		match.ID,
		match.TournamentID,
		match.Program1ID,
		match.Program2ID,
		match.GameType,
		match.Status,
		match.Priority,
		match.RoundNumber,
		match.CreatedAt.UTC(),
	)

	if err != nil {
		return errors.Wrap(err, "failed to create match")
	}

	return nil
}

// insertMatches вставляет матчи внутри транзакции одной командой COPY: построчный
// INSERT на раунд в десятки тысяч матчей упирался в таймаут запроса.
// created_at пишется в UTC - колонка без зоны, смещение postgres молча отбросил бы
func insertMatches(ctx context.Context, tx *sqlx.Tx, matches []*models.Match) error {
	if len(matches) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("matches",
		"id", "tournament_id", "program1_id", "program2_id", "game_type", "status", "priority", "round_number", "created_at"))
	if err != nil {
		return errors.Wrap(err, "failed to prepare matches copy")
	}
	defer stmt.Close()

	for _, m := range matches {
		if _, err := stmt.ExecContext(ctx,
			m.ID, m.TournamentID, m.Program1ID, m.Program2ID, m.GameType, m.Status, m.Priority, m.RoundNumber, m.CreatedAt.UTC(),
		); err != nil {
			return errors.Wrap(err, "failed to copy match")
		}
	}

	// пустой Exec отправляет накопленные строки на сервер
	if _, err := stmt.ExecContext(ctx); err != nil {
		return errors.Wrap(err, "failed to insert matches")
	}
	return nil
}

// DeleteBatch сносит матчи по списку id одним запросом.
// нужно для отката, если EnqueueBatch упал уже после вставки матчей.
// идемпотентно - если каких-то id уже нет, ошибки не будет
func (r *MatchRepository) DeleteBatch(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	query := `DELETE FROM matches WHERE id = ANY($1)`
	if _, err := r.db.ExecContext(ctx, query, pq.Array(strs)); err != nil {
		return errors.Wrap(err, "failed to delete matches batch")
	}
	return nil
}

func (r *MatchRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Match, error) {
	var match models.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&match.ID,
		&match.TournamentID,
		&match.Program1ID,
		&match.Program2ID,
		&match.GameType,
		&match.Status,
		&match.Priority,
		&match.RoundNumber,
		&match.Score1,
		&match.Score2,
		&match.Winner,
		&match.ErrorCode,
		&match.ErrorMessage,
		&match.StartedAt,
		&match.CompletedAt,
		&match.CreatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("match not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get match by id")
	}

	return &match, nil
}

func (r *MatchRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*models.Match, error) {
	var matches []*models.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE tournament_id = $1
		ORDER BY round_number DESC, created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get matches by tournament id")
	}
	defer rows.Close()

	for rows.Next() {
		var match models.Match
		err := rows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.Program1ID,
			&match.Program2ID,
			&match.GameType,
			&match.Status,
			&match.Priority,
			&match.RoundNumber,
			&match.Score1,
			&match.Score2,
			&match.Winner,
			&match.ErrorCode,
			&match.ErrorMessage,
			&match.StartedAt,
			&match.CompletedAt,
			&match.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan match")
		}
		matches = append(matches, &match)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return matches, nil
}

func (r *MatchRepository) GetPendingByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Match, error) {
	var matches []*models.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE tournament_id = $1 AND status = $2
		ORDER BY
			CASE priority
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, models.MatchPending)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pending matches by tournament id")
	}
	defer rows.Close()

	for rows.Next() {
		var match models.Match
		err := rows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.Program1ID,
			&match.Program2ID,
			&match.GameType,
			&match.Status,
			&match.Priority,
			&match.RoundNumber,
			&match.Score1,
			&match.Score2,
			&match.Winner,
			&match.ErrorCode,
			&match.ErrorMessage,
			&match.StartedAt,
			&match.CompletedAt,
			&match.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan match")
		}
		matches = append(matches, &match)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return matches, nil
}

func (r *MatchRepository) GetPendingByTournamentAndGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*models.Match, error) {
	var matches []*models.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE tournament_id = $1 AND game_type = $2 AND status = $3
		ORDER BY
			CASE priority
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameType, models.MatchPending)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pending matches by tournament and game")
	}
	defer rows.Close()

	for rows.Next() {
		var match models.Match
		err := rows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.Program1ID,
			&match.Program2ID,
			&match.GameType,
			&match.Status,
			&match.Priority,
			&match.RoundNumber,
			&match.Score1,
			&match.Score2,
			&match.Winner,
			&match.ErrorCode,
			&match.ErrorMessage,
			&match.StartedAt,
			&match.CompletedAt,
			&match.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan match")
		}
		matches = append(matches, &match)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return matches, nil
}

func (r *MatchRepository) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*models.MatchRound, error) {
	query := `
		SELECT
			round_number,
			game_type,
			COUNT(*) as total_matches,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE status = 'running') as running_count,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_count,
			MIN(created_at) as created_at
		FROM matches
		WHERE tournament_id = $1
		GROUP BY round_number, game_type
		ORDER BY MIN(created_at) DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rounds")
	}
	defer rows.Close()

	var rounds []*models.MatchRound
	for rows.Next() {
		var round models.MatchRound
		err := rows.Scan(
			&round.RoundNumber,
			&round.GameType,
			&round.TotalMatches,
			&round.CompletedCount,
			&round.PendingCount,
			&round.RunningCount,
			&round.FailedCount,
			&round.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan round")
		}
		rounds = append(rounds, &round)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	// тянутся все матчи турнира разом чтобы не делать N+1
	matchQuery := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE tournament_id = $1
		ORDER BY round_number, game_type, created_at ASC
	`

	matchRows, err := r.db.QueryContext(ctx, matchQuery, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get matches")
	}
	defer matchRows.Close()

	// раунды индексируются по (round_number, game_type) чтобы быстро раскидать матчи
	type roundKey struct {
		roundNumber int
		gameType    string
	}
	roundIndex := make(map[roundKey]*models.MatchRound, len(rounds))
	for _, round := range rounds {
		roundIndex[roundKey{round.RoundNumber, round.GameType}] = round
	}

	for matchRows.Next() {
		var match models.Match
		err := matchRows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.Program1ID,
			&match.Program2ID,
			&match.GameType,
			&match.Status,
			&match.Priority,
			&match.RoundNumber,
			&match.Score1,
			&match.Score2,
			&match.Winner,
			&match.ErrorCode,
			&match.ErrorMessage,
			&match.StartedAt,
			&match.CompletedAt,
			&match.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan match")
		}
		key := roundKey{match.RoundNumber, match.GameType}
		if round, ok := roundIndex[key]; ok {
			round.Matches = append(round.Matches, &match)
		}
	}
	if err := matchRows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return rounds, nil
}

func (r *MatchRepository) List(ctx context.Context, filter models.MatchFilter) ([]*models.Match, error) {
	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE 1=1
	`
	args := []any{}
	argCount := 1

	if filter.TournamentID != nil {
		query += fmt.Sprintf(" AND tournament_id = $%d", argCount)
		args = append(args, *filter.TournamentID)
		argCount++
	}

	// программа могла быть и первой и второй, поэтому OR по обоим полям
	if filter.ProgramID != nil {
		query += fmt.Sprintf(" AND (program1_id = $%d OR program2_id = $%d)", argCount, argCount)
		args = append(args, *filter.ProgramID)
		argCount++
	}

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

	// все матчи раунда создаются с одним created_at, без id страницы OFFSET плывут
	query += " ORDER BY round_number DESC, created_at DESC, id DESC"

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
		return nil, errors.Wrap(err, "failed to list matches")
	}
	defer rows.Close()

	var matches []*models.Match
	for rows.Next() {
		var match models.Match
		err := rows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.Program1ID,
			&match.Program2ID,
			&match.GameType,
			&match.Status,
			&match.Priority,
			&match.RoundNumber,
			&match.Score1,
			&match.Score2,
			&match.Winner,
			&match.ErrorCode,
			&match.ErrorMessage,
			&match.StartedAt,
			&match.CompletedAt,
			&match.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan match")
		}
		matches = append(matches, &match)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return matches, nil
}

func (r *MatchRepository) GetPending(ctx context.Context, limit int) ([]*models.Match, error) {
	var matches []*models.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE status = $1
		ORDER BY
			CASE priority
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			created_at ASC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, models.MatchPending, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pending matches")
	}
	defer rows.Close()

	for rows.Next() {
		var match models.Match
		err := rows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.Program1ID,
			&match.Program2ID,
			&match.GameType,
			&match.Status,
			&match.Priority,
			&match.RoundNumber,
			&match.Score1,
			&match.Score2,
			&match.Winner,
			&match.ErrorCode,
			&match.ErrorMessage,
			&match.StartedAt,
			&match.CompletedAt,
			&match.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan match")
		}
		matches = append(matches, &match)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return matches, nil
}

// MatchStatistics - счётчики матчей по статусам (отдаются в /matches/queue/stats)
type MatchStatistics struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// HasAnyRunningMatches - есть ли в турнире running/pending матчи по любой игре.
// нужно чтобы не давать грузить программы пока раунд крутится
func (r *MatchRepository) HasAnyRunningMatches(ctx context.Context, tournamentID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM matches
			WHERE tournament_id = $1
			AND status IN ($2, $3)
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, tournamentID, models.MatchRunning, models.MatchPending).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check running matches")
	}

	return exists, nil
}

// GetActiveGameType - какая игра сейчас крутится (running в приоритете, потом pending).
// пустая строка, если активных матчей нет
func (r *MatchRepository) GetActiveGameType(ctx context.Context, tournamentID uuid.UUID) (string, error) {
	query := `
		SELECT COALESCE(
			(SELECT game_type FROM matches
			 WHERE tournament_id = $1
			 AND status IN ($2, $3)
			 ORDER BY
				CASE WHEN status = $2 THEN 1 ELSE 2 END,
				created_at ASC
			 LIMIT 1),
			''
		)
	`

	var gameType string
	err := r.db.QueryRowContext(ctx, query, tournamentID, models.MatchRunning, models.MatchPending).Scan(&gameType)
	if err != nil {
		return "", errors.Wrap(err, "failed to get active game type")
	}

	return gameType, nil
}

func (r *MatchRepository) GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*MatchStatistics, error) {
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'running') as running,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM matches
	`

	args := []any{}
	if tournamentID != nil {
		query += " WHERE tournament_id = $1"
		args = append(args, *tournamentID)
	}

	var stats MatchStatistics
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Running,
		&stats.Completed,
		&stats.Failed,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get match statistics")
	}

	return &stats, nil
}

func (r *MatchRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.MatchStatus) error {
	var query string

	if status == models.MatchRunning {
		// в running переход только из pending - защита от двойной обработки,
		// если матч случайно оказался в очереди дважды (retry)
		query = `
			UPDATE matches
			SET status = $2, started_at = NOW()
			WHERE id = $1 AND status = 'pending'
		`
	} else {
		query = `
			UPDATE matches
			SET status = $2
			WHERE id = $1
		`
	}

	result, err := r.db.ExecWithMetrics(ctx, "match_update_status", query, id, status)
	if err != nil {
		return errors.Wrap(err, "failed to update match status")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		// строк 0 при running = матч уже не pending, кто-то его увёл (не not found!)
		if status == models.MatchRunning {
			return models.ErrMatchAlreadyProcessed
		}
		return errors.ErrNotFound.WithMessage("match not found")
	}

	return nil
}

// UpdateResult пишет результат только поверх running: матч, отменённый
// дисквалификацией или удалённый сбросом раунда, пока он играл, не воскресает.
// 0 строк = ErrMatchAlreadyProcessed
func (r *MatchRepository) UpdateResult(ctx context.Context, id uuid.UUID, result *models.MatchResult) error {
	query := `
		UPDATE matches
		SET status = $2, score1 = $3, score2 = $4, winner = $5,
		    error_code = $6, error_message = $7, completed_at = NOW()
		WHERE id = $1 AND status = 'running'
	`

	status := models.MatchCompleted
	if result.ErrorCode != 0 {
		status = models.MatchFailed
	}

	var errorCode *int
	if result.ErrorCode != 0 {
		errorCode = &result.ErrorCode
	}

	var errorMsg *string
	if result.ErrorMessage != "" {
		errorMsg = &result.ErrorMessage
	}

	res, err := r.db.ExecWithMetrics(ctx, "match_update_result", query,
		id,
		status,
		result.Score1,
		result.Score2,
		result.Winner,
		errorCode,
		errorMsg,
	)
	if err != nil {
		return errors.Wrap(err, "failed to update match result")
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}
	if rows == 0 {
		return models.ErrMatchAlreadyProcessed
	}

	return nil
}

// UpdateResultWithOutbox пишет результат матча и в той же транзакции кладёт
// outbox-задачу на пересчёт рейтинга. смысл: если результат сохранён, рейтинг
// точно посчитается - сразу воркером или потом аутбокс-диспетчером после сбоя.
// как и UpdateResult, пишет только поверх running, иначе ErrMatchAlreadyProcessed
// и никакой outbox-задачи
func (r *MatchRepository) UpdateResultWithOutbox(ctx context.Context, id uuid.UUID, result *models.MatchResult) error {
	status := models.MatchCompleted
	if result.ErrorCode != 0 {
		status = models.MatchFailed
	}

	var errorCode *int
	if result.ErrorCode != 0 {
		errorCode = &result.ErrorCode
	}

	var errorMsg *string
	if result.ErrorMessage != "" {
		errorMsg = &result.ErrorMessage
	}

	return r.db.RunInTx(ctx, func(tx *sqlx.Tx) error {
		updateQuery := `
			UPDATE matches
			SET status = $2, score1 = $3, score2 = $4, winner = $5,
			    error_code = $6, error_message = $7, completed_at = NOW()
			WHERE id = $1 AND status = 'running'
		`
		res, err := tx.ExecContext(ctx, updateQuery,
			id, status, result.Score1, result.Score2, result.Winner, errorCode, errorMsg,
		)
		if err != nil {
			return errors.Wrap(err, "failed to update match result")
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}
		if rows == 0 {
			return models.ErrMatchAlreadyProcessed
		}

		// outbox только для успешных матчей с победителем/ничьёй (winner>=0)
		if status == models.MatchCompleted && result.Winner >= 0 {
			outboxQuery := `INSERT INTO match_outbox (match_id, kind) VALUES ($1, $2)`
			if _, err := tx.ExecContext(ctx, outboxQuery, id, OutboxKindRatingUpdate); err != nil {
				return errors.Wrap(err, "failed to insert outbox entry")
			}
		}

		return nil
	})
}

// ResetToPending возвращает матч running->pending при транзиентной ошибке
// executor'а (докер недоступен и т.п.) - программа не виновата, матч повторится
func (r *MatchRepository) ResetToPending(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE matches
		SET status = 'pending', started_at = NULL
		WHERE id = $1 AND status = 'running'
	`
	if _, err := r.db.ExecWithMetrics(ctx, "match_reset_pending", query, id); err != nil {
		return errors.Wrap(err, "failed to reset match to pending")
	}
	return nil
}

// ResetFailedMatches - все failed матчи турнира обратно в pending
func (r *MatchRepository) ResetFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int64, error) {
	query := `
		UPDATE matches
		SET status = $1, error_code = NULL, error_message = NULL, started_at = NULL, completed_at = NULL,
		    score1 = NULL, score2 = NULL, winner = NULL
		WHERE tournament_id = $2 AND status = $3
	`

	result, err := r.db.ExecContext(ctx, query, models.MatchPending, tournamentID, models.MatchFailed)
	if err != nil {
		return 0, errors.Wrap(err, "failed to reset failed matches")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return rows, nil
}

// ResetStuckRunning возвращает в pending матчи, зависшие в running дольше
// stuckDuration (воркер умер посреди матча). выборка и сброс одним запросом по
// часам бд: матч, успевший завершиться или заново стартовать, не трогается
func (r *MatchRepository) ResetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) (int64, error) {
	query := `
		UPDATE matches
		SET status = 'pending', started_at = NULL
		WHERE id IN (
			SELECT id FROM matches
			WHERE status = 'running' AND started_at < NOW() - make_interval(secs => $1)
			ORDER BY started_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
	`
	result, err := r.db.ExecWithMetrics(ctx, "match_reset_stuck", query, stuckDuration.Seconds(), limit)
	if err != nil {
		return 0, errors.Wrap(err, "failed to reset stuck running matches")
	}
	return result.RowsAffected()
}

// CancelPending отменяет все pending матчи (админская очистка очереди): без
// этого recovery воркера вернул бы их в очередь. running доигрывают
func (r *MatchRepository) CancelPending(ctx context.Context) (int64, error) {
	query := `
		UPDATE matches
		SET status = 'cancelled', error_message = 'Cancelled by admin'
		WHERE status = 'pending'
	`
	result, err := r.db.ExecWithMetrics(ctx, "match_cancel_pending", query)
	if err != nil {
		return 0, errors.Wrap(err, "failed to cancel pending matches")
	}
	return result.RowsAffected()
}
