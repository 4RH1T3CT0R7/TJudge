package db

import (
	"context"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
)

// GetByTournamentID получает все матчи турнира
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

// GetPendingByTournamentID получает ожидающие матчи турнира по приоритету
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

// GetPendingByTournamentAndGame получает ожидающие матчи турнира для конкретной игры
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

// GetPlayedProgramPairs возвращает множество пар (program1_id, program2_id),
// для которых уже существуют матчи (любого статуса) в данном турнире и игре.
// Ключи формата "uuid1|uuid2" направленные (AB не равно BA), что соответствует
// round-robin генерации, которая создаёт матчи в обоих направлениях.
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

// GetMatchesByRounds получает матчи турнира сгруппированные по раундам и играм
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

	// Получаем все матчи турнира одним запросом вместо N+1
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

	// Индексируем раунды по (round_number, game_type) для быстрого поиска
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

// List получает список матчей с фильтрацией и пагинацией
func (r *MatchRepository) List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error) {
	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE 1=1
	`
	args := []any{}
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

// GetByIDs получает несколько матчей по их ID за один запрос
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

// GetPending получает ожидающие матчи по приоритету
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
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE 1=1
	`
	args := []any{}
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

// GetStuckRunning получает матчи, застрявшие в статусе running дольше указанного времени
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
