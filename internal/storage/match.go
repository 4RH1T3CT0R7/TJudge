package storage

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
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

// CreateBatch вставляет пачку матчей в одной транзакции.
// prepared statement переиспользуем, чтобы не парсить один и тот же запрос на каждый матч
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

func (r *MatchRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error) {
	var match domain.Match

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

func (r *MatchRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error) {
	var matches []*domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
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

func (r *MatchRepository) GetPendingByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Match, error) {
	var matches []*domain.Match

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

func (r *MatchRepository) GetPendingByTournamentAndGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.Match, error) {
	var matches []*domain.Match

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

// GetPlayedProgramPairs - пары программ, которые уже играли в этом турнире и игре (любой статус).
// ключ "uuid1|uuid2" направленный, AB и BA считаем разными - round-robin гоняет обе ориентации
func (r *MatchRepository) GetPlayedProgramPairs(ctx context.Context, tournamentID uuid.UUID, gameType string) (map[string]struct{}, error) {
	query := `
		SELECT program1_id, program2_id
		FROM matches
		WHERE tournament_id = $1 AND game_type = $2
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get played program pairs")
	}
	defer rows.Close()

	pairs := make(map[string]struct{})
	for rows.Next() {
		var p1, p2 uuid.UUID
		if err := rows.Scan(&p1, &p2); err != nil {
			return nil, errors.Wrap(err, "failed to scan program pair")
		}
		key := p1.String() + "|" + p2.String()
		pairs[key] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows iteration error in played program pairs")
	}

	return pairs, nil
}

func (r *MatchRepository) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error) {
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

	var rounds []*domain.MatchRound
	for rows.Next() {
		var round domain.MatchRound
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

	// тянем все матчи турнира разом чтобы не делать N+1
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

	// индексируем раунды по (round_number, game_type) чтобы быстро раскидать матчи
	type roundKey struct {
		roundNumber int
		gameType    string
	}
	roundIndex := make(map[roundKey]*domain.MatchRound, len(rounds))
	for _, round := range rounds {
		roundIndex[roundKey{round.RoundNumber, round.GameType}] = round
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

func (r *MatchRepository) List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error) {
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

	query += " ORDER BY round_number DESC, created_at DESC"

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

func (r *MatchRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Match, error) {
	if len(ids) == 0 {
		return []*domain.Match{}, nil
	}

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
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

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows iteration error")
	}

	return matches, nil
}

func (r *MatchRepository) GetPending(ctx context.Context, limit int) ([]*domain.Match, error) {
	var matches []*domain.Match

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

// ListWithCursor - список матчей через cursor-пагинацию (курсор по created_at)
func (r *MatchRepository) ListWithCursor(ctx context.Context, filter domain.MatchFilter, pageReq *pagination.PageRequest) ([]*domain.Match, bool, error) {
	if err := pageReq.Validate(); err != nil {
		return nil, false, errors.Wrap(err, "invalid pagination request")
	}

	cursor, err := pageReq.GetCursor()
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to decode cursor")
	}

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

	// forward - идём в прошлое (created_at меньше курсора), backward - в обратную сторону
	if cursor != nil && cursor.Type == pagination.CursorTypeTimestamp && cursor.Timestamp != nil {
		if pageReq.IsForward() {
			query += fmt.Sprintf(" AND created_at < $%d", argCount)
		} else {
			query += fmt.Sprintf(" AND created_at > $%d", argCount)
		}
		args = append(args, *cursor.Timestamp)
		argCount++
	}

	if pageReq.IsBackward() {
		query += " ORDER BY round_number ASC, created_at ASC"
	} else {
		query += " ORDER BY round_number DESC, created_at DESC"
	}

	// берём на одну строку больше лимита - если она пришла, значит есть следующая страница
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
			&match.ErrorCode,
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

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("rows iteration error: %w", err)
	}

	hasMore := len(matches) > pageReq.GetLimit()
	if hasMore {
		matches = matches[:len(matches)-1]
	}

	// при backward выбирали в обратном порядке, разворачиваем обратно
	if pageReq.IsBackward() {
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
	}

	return matches, hasMore, nil
}

// GetStuckRunning - матчи, зависшие в running дольше stuckDuration (воркер умер посреди матча)
func (r *MatchRepository) GetStuckRunning(ctx context.Context, stuckDuration time.Duration, limit int) ([]*domain.Match, error) {
	var matches []*domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
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

// MatchStatistics - счётчики матчей по статусам (отдаём в /matches/queue/stats)
type MatchStatistics struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// HasStartedMatches - есть ли по этой игре уже running или completed матчи
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
	err := r.db.QueryRowContext(ctx, query, tournamentID, domain.MatchRunning, domain.MatchPending).Scan(&exists)
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
	err := r.db.QueryRowContext(ctx, query, tournamentID, domain.MatchRunning, domain.MatchPending).Scan(&gameType)
	if err != nil {
		return "", errors.Wrap(err, "failed to get active game type")
	}

	return gameType, nil
}

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

func (r *MatchRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus) error {
	var query string

	if status == domain.MatchRunning {
		// в running переходим только из pending - защита от двойной обработки,
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
		if status == domain.MatchRunning {
			return domain.ErrMatchAlreadyProcessed
		}
		return errors.ErrNotFound.WithMessage("match not found")
	}

	return nil
}

func (r *MatchRepository) UpdateResult(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error {
	query := `
		UPDATE matches
		SET status = $2, score1 = $3, score2 = $4, winner = $5,
		    error_code = $6, error_message = $7, completed_at = NOW()
		WHERE id = $1
	`

	status := domain.MatchCompleted
	if result.ErrorCode != 0 {
		status = domain.MatchFailed
	}

	var errorCode *int
	if result.ErrorCode != 0 {
		errorCode = &result.ErrorCode
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
		errorCode,
		errorMsg,
	)

	if err != nil {
		return errors.Wrap(err, "failed to update match result")
	}

	return nil
}

// UpdateResultWithOutbox пишет результат матча и в той же транзакции кладёт
// outbox-задачу на пересчёт рейтинга. смысл: если результат сохранён, рейтинг
// точно посчитается - сразу воркером или потом аутбокс-диспетчером после сбоя
func (r *MatchRepository) UpdateResultWithOutbox(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error {
	status := domain.MatchCompleted
	if result.ErrorCode != 0 {
		status = domain.MatchFailed
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
			WHERE id = $1
		`
		if _, err := tx.ExecContext(ctx, updateQuery,
			id, status, result.Score1, result.Score2, result.Winner, errorCode, errorMsg,
		); err != nil {
			return errors.Wrap(err, "failed to update match result")
		}

		// outbox только для успешных матчей с победителем/ничьёй (winner>=0)
		if status == domain.MatchCompleted && result.Winner >= 0 {
			outboxQuery := `INSERT INTO match_outbox (match_id, kind) VALUES ($1, $2)`
			if _, err := tx.ExecContext(ctx, outboxQuery, id, OutboxKindRatingUpdate); err != nil {
				return errors.Wrap(err, "failed to insert outbox entry")
			}
		}

		return nil
	})
}

// MarkRatingApplied - fast path: воркер сразу посчитал рейтинг, гасим outbox-задачу
func (r *MatchRepository) MarkRatingApplied(ctx context.Context, matchID uuid.UUID) error {
	query := `
		UPDATE match_outbox
		SET status = 'done', processed_at = NOW()
		WHERE match_id = $1 AND kind = $2 AND status = 'pending'
	`
	if _, err := r.db.ExecContext(ctx, query, matchID, OutboxKindRatingUpdate); err != nil {
		return errors.Wrap(err, "failed to mark rating applied")
	}
	return nil
}

// ResetToPending возвращает матч running->pending при транзиентной ошибке
// executor'а (докер недоступен и т.п.) - программа не виновата, матч повторим
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
		// тот же guard что в UpdateStatus, только пачкой
		query = `
			UPDATE matches
			SET status = $1, started_at = NOW()
			WHERE id = ANY($2) AND status = 'pending'
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
		    error_code = $6, error_message = $7, completed_at = NOW()
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

		var errorCode *int
		if result.ErrorCode != 0 {
			errorCode = &result.ErrorCode
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
			errorCode,
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

// DeleteMatchesForGame - снести все матчи турнира по игре
func (r *MatchRepository) DeleteMatchesForGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (int64, error) {
	query := `DELETE FROM matches WHERE tournament_id = $1 AND game_type = $2`

	result, err := r.db.ExecContext(ctx, query, tournamentID, gameType)
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete matches for game")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return rows, nil
}
