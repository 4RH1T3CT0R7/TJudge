package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// MatchRepository - репозиторий для работы с матчами
type MatchRepository struct {
	db *DB
}

// NewMatchRepository создаёт новый репозиторий матчей
func NewMatchRepository(db *DB) *MatchRepository {
	return &MatchRepository{db: db}
}

// Create создаёт новый матч
func (r *MatchRepository) Create(ctx context.Context, match *domain.Match) error {
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
		match.CreatedAt,
	)

	if err != nil {
		return errors.Wrap(err, "failed to create match")
	}

	return nil
}

// GetByID получает матч по ID
func (r *MatchRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error) {
	var match domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
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
		&match.ErrorMessage,
		&match.StartedAt,
		&match.CompletedAt,
		&match.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound.WithMessage("match not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get match by id")
	}

	return &match, nil
}

// GetByTournamentID получает все матчи турнира
func (r *MatchRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error) {
	var matches []*domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE tournament_id = $1
		ORDER BY round_number DESC, created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get matches by tournament id")
	}
	defer rows.Close()

	for rows.Next() {
		var match domain.Match
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

	return matches, nil
}

// GetPendingByTournamentID получает ожидающие матчи турнира по приоритету
func (r *MatchRepository) GetPendingByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Match, error) {
	var matches []*domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
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

	rows, err := r.db.QueryContext(ctx, query, tournamentID, domain.MatchPending)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pending matches by tournament id")
	}
	defer rows.Close()

	for rows.Next() {
		var match domain.Match
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

	return matches, nil
}

// GetPendingByTournamentAndGame получает ожидающие матчи турнира для конкретной игры
func (r *MatchRepository) GetPendingByTournamentAndGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.Match, error) {
	var matches []*domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
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

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameType, domain.MatchPending)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pending matches by tournament and game")
	}
	defer rows.Close()

	for rows.Next() {
		var match domain.Match
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

	return matches, nil
}

// GetNextRoundNumberByGame получает следующий номер раунда для конкретной игры в турнире
func (r *MatchRepository) GetNextRoundNumberByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	query := `
		SELECT COALESCE(MAX(round_number), 0) + 1
		FROM matches
		WHERE tournament_id = $1 AND game_type = $2
	`

	var nextRound int
	err := r.db.QueryRowContext(ctx, query, tournamentID, gameType).Scan(&nextRound)
	if err != nil {
		return 1, errors.Wrap(err, "failed to get next round number by game")
	}

	return nextRound, nil
}

// ResetFailedMatches сбрасывает все failed матчи турнира в pending
func (r *MatchRepository) ResetFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int64, error) {
	query := `
		UPDATE matches
		SET status = $1, error_message = NULL, started_at = NULL, completed_at = NULL,
		    score1 = NULL, score2 = NULL, winner = NULL
		WHERE tournament_id = $2 AND status = $3
	`

	result, err := r.db.ExecContext(ctx, query, domain.MatchPending, tournamentID, domain.MatchFailed)
	if err != nil {
		return 0, errors.Wrap(err, "failed to reset failed matches")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return rows, nil
}

// GetPending получает ожидающие матчи по приоритету
func (r *MatchRepository) GetPending(ctx context.Context, limit int) ([]*domain.Match, error) {
	var matches []*domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
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

	rows, err := r.db.QueryContext(ctx, query, domain.MatchPending, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pending matches")
	}
	defer rows.Close()

	for rows.Next() {
		var match domain.Match
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

	return matches, nil
}

// UpdateStatus обновляет статус матча
func (r *MatchRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus) error {
	var query string

	if status == domain.MatchRunning {
		query = `
			UPDATE matches
			SET status = $2, started_at = NOW()
			WHERE id = $1
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
		return errors.ErrNotFound.WithMessage("match not found")
	}

	return nil
}

// UpdateResult обновляет результат матча
func (r *MatchRepository) UpdateResult(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error {
	query := `
		UPDATE matches
		SET status = $2, score1 = $3, score2 = $4, winner = $5,
		    error_message = $6, completed_at = NOW()
		WHERE id = $1
	`

	status := domain.MatchCompleted
	if result.ErrorCode != 0 {
		status = domain.MatchFailed
	}

	var errorMsg *string
	if result.ErrorMessage != "" {
		errorMsg = &result.ErrorMessage
	}

	_, err := r.db.ExecWithMetrics(ctx, "match_update_result", query,
		id,
		status,
		result.Score1,
		result.Score2,
		result.Winner,
		errorMsg,
	)

	if err != nil {
		return errors.Wrap(err, "failed to update match result")
	}

	return nil
}

// HasStartedMatches проверяет, есть ли запущенные или завершённые матчи для турнира и игры
// Возвращает true, если есть матчи со статусом running или completed
func (r *MatchRepository) HasStartedMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM matches
			WHERE tournament_id = $1
			AND game_type = $2
			AND status IN ($3, $4)
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, tournamentID, gameType, domain.MatchRunning, domain.MatchCompleted).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check started matches")
	}

	return exists, nil
}

// GetStatistics получает статистику матчей
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

	args := []interface{}{}
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

// CreateBatch создаёт несколько матчей одновременно
func (r *MatchRepository) CreateBatch(ctx context.Context, matches []*domain.Match) error {
	if len(matches) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		INSERT INTO matches (id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}
	defer stmt.Close()

	for _, match := range matches {
		_, err := stmt.ExecContext(ctx,
			match.ID,
			match.TournamentID,
			match.Program1ID,
			match.Program2ID,
			match.GameType,
			match.Status,
			match.Priority,
			match.RoundNumber,
			match.CreatedAt,
		)
		if err != nil {
			return errors.Wrap(err, "failed to insert match")
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// List получает список матчей с фильтрацией и пагинацией
func (r *MatchRepository) List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error) {
	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	// Фильтр по турниру
	if filter.TournamentID != nil {
		query += fmt.Sprintf(" AND tournament_id = $%d", argCount)
		args = append(args, *filter.TournamentID)
		argCount++
	}

	// Фильтр по программе (участвует как program1 или program2)
	if filter.ProgramID != nil {
		query += fmt.Sprintf(" AND (program1_id = $%d OR program2_id = $%d)", argCount, argCount)
		args = append(args, *filter.ProgramID)
		argCount++
	}

	// Фильтр по статусу
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, filter.Status)
		argCount++
	}

	// Фильтр по типу игры
	if filter.GameType != "" {
		query += fmt.Sprintf(" AND game_type = $%d", argCount)
		args = append(args, filter.GameType)
		argCount++
	}

	// Сортировка (по умолчанию - сначала новые раунды)
	query += " ORDER BY round_number DESC, created_at DESC"

	// Пагинация
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

	var matches []*domain.Match
	for rows.Next() {
		var match domain.Match
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

	return matches, nil
}

// GetByIDs получает несколько матчей по их ID за один запрос
func (r *MatchRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Match, error) {
	if len(ids) == 0 {
		return []*domain.Match{}, nil
	}

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE id = ANY($1)
		ORDER BY round_number DESC, created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, ids)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get matches by IDs")
	}
	defer rows.Close()

	var matches []*domain.Match
	for rows.Next() {
		var match domain.Match
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

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows iteration error")
	}

	return matches, nil
}

// BatchUpdateStatus обновляет статус для нескольких матчей одновременно
func (r *MatchRepository) BatchUpdateStatus(ctx context.Context, matchIDs []uuid.UUID, status domain.MatchStatus) error {
	if len(matchIDs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	var query string
	if status == domain.MatchRunning {
		query = `
			UPDATE matches
			SET status = $1, started_at = NOW()
			WHERE id = ANY($2)
		`
	} else {
		query = `
			UPDATE matches
			SET status = $1
			WHERE id = ANY($2)
		`
	}

	_, err = tx.ExecContext(ctx, query, status, pq.Array(matchIDs))
	if err != nil {
		return errors.Wrap(err, "failed to batch update match status")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// BatchUpdateResults обновляет результаты для нескольких матчей одновременно
func (r *MatchRepository) BatchUpdateResults(ctx context.Context, results map[uuid.UUID]*domain.MatchResult) error {
	if len(results) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		UPDATE matches
		SET status = $2, score1 = $3, score2 = $4, winner = $5,
		    error_message = $6, completed_at = NOW()
		WHERE id = $1
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}
	defer stmt.Close()

	for matchID, result := range results {
		status := domain.MatchCompleted
		if result.ErrorCode != 0 {
			status = domain.MatchFailed
		}

		var errorMsg *string
		if result.ErrorMessage != "" {
			errorMsg = &result.ErrorMessage
		}

		_, err := stmt.ExecContext(ctx,
			matchID,
			status,
			result.Score1,
			result.Score2,
			result.Winner,
			errorMsg,
		)
		if err != nil {
			return errors.Wrap(err, "failed to update match result in batch")
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// ListWithCursor получает список матчей с cursor-based пагинацией
func (r *MatchRepository) ListWithCursor(ctx context.Context, filter domain.MatchFilter, pageReq *pagination.PageRequest) ([]*domain.Match, bool, error) {
	// Валидация запроса пагинации
	if err := pageReq.Validate(); err != nil {
		return nil, false, errors.Wrap(err, "invalid pagination request")
	}

	// Получаем курсор
	cursor, err := pageReq.GetCursor()
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to decode cursor")
	}

	// Базовый запрос
	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	// Фильтр по турниру
	if filter.TournamentID != nil {
		query += fmt.Sprintf(" AND tournament_id = $%d", argCount)
		args = append(args, *filter.TournamentID)
		argCount++
	}

	// Фильтр по программе
	if filter.ProgramID != nil {
		query += fmt.Sprintf(" AND (program1_id = $%d OR program2_id = $%d)", argCount, argCount)
		args = append(args, *filter.ProgramID)
		argCount++
	}

	// Фильтр по статусу
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, filter.Status)
		argCount++
	}

	// Фильтр по типу игры
	if filter.GameType != "" {
		query += fmt.Sprintf(" AND game_type = $%d", argCount)
		args = append(args, filter.GameType)
		argCount++
	}

	// Применяем курсор для пагинации
	if cursor != nil && cursor.Type == pagination.CursorTypeTimestamp && cursor.Timestamp != nil {
		if pageReq.IsForward() {
			query += fmt.Sprintf(" AND created_at < $%d", argCount)
		} else {
			query += fmt.Sprintf(" AND created_at > $%d", argCount)
		}
		args = append(args, *cursor.Timestamp)
		argCount++
	}

	// Сортировка
	if pageReq.IsBackward() {
		query += " ORDER BY round_number ASC, created_at ASC"
	} else {
		query += " ORDER BY round_number DESC, created_at DESC"
	}

	// Добавляем +1 к лимиту для определения hasNextPage
	limit := pageReq.GetLimit() + 1
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list matches with cursor")
	}
	defer rows.Close()

	var matches []*domain.Match
	for rows.Next() {
		var match domain.Match
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
			&match.ErrorMessage,
			&match.StartedAt,
			&match.CompletedAt,
			&match.CreatedAt,
		)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to scan match")
		}
		matches = append(matches, &match)
	}

	// Определяем, есть ли ещё страницы
	hasMore := len(matches) > pageReq.GetLimit()
	if hasMore {
		matches = matches[:len(matches)-1]
	}

	// Для backward pagination разворачиваем результаты
	if pageReq.IsBackward() {
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
	}

	return matches, hasMore, nil
}

// GetMatchCursor возвращает курсор для матча (для использования с pagination.NewConnection)
func GetMatchCursor(match *domain.Match) (*pagination.Cursor, error) {
	return pagination.NewTimestampCursor(match.CreatedAt), nil
}

// GetStuckRunning получает матчи, застрявшие в статусе running дольше указанного времени
func (r *MatchRepository) GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*domain.Match, error) {
	var matches []*domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE status = $1 AND started_at < $2
		ORDER BY started_at ASC
		LIMIT $3
	`

	threshold := time.Now().Add(-stuckDuration)

	rows, err := r.db.QueryContext(ctx, query, domain.MatchRunning, threshold, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stuck running matches")
	}
	defer rows.Close()

	for rows.Next() {
		var match domain.Match
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

	return matches, nil
}

// GetNextRoundNumber получает следующий номер раунда для турнира
func (r *MatchRepository) GetNextRoundNumber(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var maxRound sql.NullInt64

	query := `SELECT MAX(round_number) FROM matches WHERE tournament_id = $1`

	err := r.db.QueryRowContext(ctx, query, tournamentID).Scan(&maxRound)
	if err != nil {
		return 1, errors.Wrap(err, "failed to get max round number")
	}

	if !maxRound.Valid {
		return 1, nil
	}

	return int(maxRound.Int64) + 1, nil
}

// GetMatchesByRounds получает матчи турнира сгруппированные по раундам
func (r *MatchRepository) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error) {
	query := `
		SELECT
			round_number,
			COUNT(*) as total_matches,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE status = 'running') as running_count,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_count,
			MIN(created_at) as created_at
		FROM matches
		WHERE tournament_id = $1
		GROUP BY round_number
		ORDER BY round_number DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rounds")
	}
	defer rows.Close()

	var rounds []*domain.MatchRound
	for rows.Next() {
		var round domain.MatchRound
		err := rows.Scan(
			&round.RoundNumber,
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

	// Теперь получаем матчи для каждого раунда
	for _, round := range rounds {
		matchQuery := `
			SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
			       score1, score2, winner, error_message, started_at, completed_at, created_at
			FROM matches
			WHERE tournament_id = $1 AND round_number = $2
			ORDER BY created_at ASC
		`

		matchRows, err := r.db.QueryContext(ctx, matchQuery, tournamentID, round.RoundNumber)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get matches for round")
		}

		for matchRows.Next() {
			var match domain.Match
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
				&match.ErrorMessage,
				&match.StartedAt,
				&match.CompletedAt,
				&match.CreatedAt,
			)
			if err != nil {
				matchRows.Close()
				return nil, errors.Wrap(err, "failed to scan match")
			}
			round.Matches = append(round.Matches, &match)
		}
		matchRows.Close()
	}

	return rounds, nil
}

// MatchStatistics - статистика матчей
type MatchStatistics struct {
	Total     int
	Pending   int
	Running   int
	Completed int
	Failed    int
}
