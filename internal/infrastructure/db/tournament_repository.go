package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
)

// TournamentRepository - репозиторий для работы с турнирами
type TournamentRepository struct {
	db *DB
}

// NewTournamentRepository создаёт новый репозиторий турниров
func NewTournamentRepository(db *DB) *TournamentRepository {
	return &TournamentRepository{db: db}
}

// Create создаёт новый турнир
func (r *TournamentRepository) Create(ctx context.Context, tournament *domain.Tournament) error {
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
		tournament.StartTime,
		tournament.EndTime,
		metadata,
	).Scan(&tournament.CreatedAt, &tournament.UpdatedAt, &tournament.Version)

	if err != nil {
		return errors.Wrap(err, "failed to create tournament")
	}

	return nil
}

// GetByID получает турнир по ID
func (r *TournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	var tournament domain.Tournament
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

	if err == sql.ErrNoRows {
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

// List получает список турниров с фильтрацией и пагинацией
func (r *TournamentRepository) List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error) {
	query := `
		SELECT id, code, name, description, game_type, status, max_participants, max_team_size, is_permanent, creator_id, start_time, end_time,
		       metadata, version, created_at, updated_at
		FROM tournaments
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

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

	// Сортировка
	query += " ORDER BY created_at DESC"

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
		return nil, errors.Wrap(err, "failed to list tournaments")
	}
	defer rows.Close()

	var tournaments []*domain.Tournament
	for rows.Next() {
		var tournament domain.Tournament
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

	return tournaments, nil
}

// Update обновляет турнир с optimistic locking
func (r *TournamentRepository) Update(ctx context.Context, tournament *domain.Tournament) error {
	metadata, err := json.Marshal(tournament.Metadata)
	if err != nil {
		return errors.Wrap(err, "failed to marshal metadata")
	}

	query := `
		UPDATE tournaments
		SET name = $2, status = $3, max_participants = $4, start_time = $5,
		    end_time = $6, metadata = $7, version = version + 1
		WHERE id = $1 AND version = $8
		RETURNING updated_at, version
	`

	err = r.db.QueryRowContext(ctx, query,
		tournament.ID,
		tournament.Name,
		tournament.Status,
		tournament.MaxParticipants,
		tournament.StartTime,
		tournament.EndTime,
		metadata,
		tournament.Version,
	).Scan(&tournament.UpdatedAt, &tournament.Version)

	if err == sql.ErrNoRows {
		return errors.ErrConcurrentUpdate
	}
	if err != nil {
		return errors.Wrap(err, "failed to update tournament")
	}

	return nil
}

// UpdateStatus обновляет только статус турнира
func (r *TournamentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TournamentStatus) error {
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

// Delete удаляет турнир
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

// GetParticipantsCount получает количество участников турнира
func (r *TournamentRepository) GetParticipantsCount(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var count int

	query := `SELECT COUNT(*) FROM tournament_participants WHERE tournament_id = $1`

	err := r.db.QueryRowContext(ctx, query, tournamentID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get participants count")
	}

	return count, nil
}

// AddParticipant добавляет участника в турнир
func (r *TournamentRepository) AddParticipant(ctx context.Context, participant *domain.TournamentParticipant) error {
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

// GetParticipants получает список участников турнира
func (r *TournamentRepository) GetParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error) {
	var participants []*domain.TournamentParticipant

	query := `
		SELECT id, tournament_id, program_id, rating, wins, losses, draws, created_at
		FROM tournament_participants
		WHERE tournament_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament participants")
	}
	defer rows.Close()

	for rows.Next() {
		var p domain.TournamentParticipant
		err := rows.Scan(
			&p.ID,
			&p.TournamentID,
			&p.ProgramID,
			&p.Rating,
			&p.Wins,
			&p.Losses,
			&p.Draws,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan participant")
		}
		participants = append(participants, &p)
	}

	return participants, nil
}

// GetLatestParticipants получает список участников турнира, но только с последней версией программы каждой команды
func (r *TournamentRepository) GetLatestParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error) {
	var participants []*domain.TournamentParticipant

	// Выбираем только участников с последней версией программы для каждой команды и игры
	query := `
		SELECT tp.id, tp.tournament_id, tp.program_id, tp.rating, tp.wins, tp.losses, tp.draws, tp.created_at
		FROM tournament_participants tp
		INNER JOIN programs p ON p.id = tp.program_id
		WHERE tp.tournament_id = $1
		  AND p.version = (
		      SELECT MAX(p2.version)
		      FROM programs p2
		      WHERE p2.team_id = p.team_id
		        AND p2.game_id = p.game_id
		        AND p2.tournament_id = p.tournament_id
		  )
		ORDER BY tp.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest tournament participants")
	}
	defer rows.Close()

	for rows.Next() {
		var p domain.TournamentParticipant
		err := rows.Scan(
			&p.ID,
			&p.TournamentID,
			&p.ProgramID,
			&p.Rating,
			&p.Wins,
			&p.Losses,
			&p.Draws,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan participant")
		}
		participants = append(participants, &p)
	}

	return participants, nil
}

// GetLeaderboard получает таблицу лидеров турнира
func (r *TournamentRepository) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	// Используем materialized view для быстрого доступа к leaderboard
	// Materialized view периодически обновляется, что даёт O(1) доступ к leaderboard
	query := `
		SELECT
			ROW_NUMBER() OVER (ORDER BY rating DESC, total_matches DESC) as rank,
			program_id,
			program_name,
			rating,
			wins,
			losses,
			draws,
			total_matches as total_games
		FROM leaderboard_tournament
		WHERE tournament_id = $1
		ORDER BY rating DESC, total_matches DESC
		LIMIT $2
	`

	var leaderboard []*domain.LeaderboardEntry

	err := r.db.QueryWithMetrics(ctx, "tournament_leaderboard", &leaderboard, query, tournamentID, limit)
	if err != nil {
		// Fallback к прямому запросу если materialized view ещё не создан
		return r.getLeaderboardFallback(ctx, tournamentID, limit)
	}

	return leaderboard, nil
}

// getLeaderboardFallback - fallback метод для получения leaderboard без materialized view
// Рейтинг = сумма всех очков из всех матчей
func (r *TournamentRepository) getLeaderboardFallback(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	query := `
		WITH program_stats AS (
			SELECT
				p.id as program_id,
				p.name as program_name,
				COUNT(*) FILTER (WHERE
					(m.program1_id = p.id AND m.winner = 1) OR
					(m.program2_id = p.id AND m.winner = 2)
				) as wins,
				COUNT(*) FILTER (WHERE
					(m.program1_id = p.id AND m.winner = 2) OR
					(m.program2_id = p.id AND m.winner = 1)
				) as losses,
				COUNT(*) FILTER (WHERE m.winner = 0 AND m.status = 'completed') as draws,
				COUNT(*) FILTER (WHERE m.status = 'completed') as total_games,
				COALESCE(SUM(
					CASE
						WHEN m.program1_id = p.id THEN COALESCE(m.score1, 0)
						WHEN m.program2_id = p.id THEN COALESCE(m.score2, 0)
						ELSE 0
					END
				), 0) as total_score
			FROM tournament_participants tp
			JOIN programs p ON tp.program_id = p.id
			LEFT JOIN matches m ON (m.program1_id = p.id OR m.program2_id = p.id)
				AND m.tournament_id = $1
				AND m.status = 'completed'
			WHERE tp.tournament_id = $1
			GROUP BY p.id, p.name
		)
		SELECT
			ROW_NUMBER() OVER (ORDER BY total_score DESC, wins DESC) as rank,
			program_id,
			program_name,
			total_score as rating,
			wins,
			losses,
			draws,
			total_games
		FROM program_stats
		ORDER BY total_score DESC, wins DESC
		LIMIT $2
	`

	var leaderboard []*domain.LeaderboardEntry

	err := r.db.QueryWithMetrics(ctx, "tournament_leaderboard_fallback", &leaderboard, query, tournamentID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament leaderboard")
	}

	return leaderboard, nil
}

// GetParticipantsByTournamentIDs получает участников для нескольких турниров одним запросом
// Это предотвращает N+1 проблему при загрузке списка турниров с участниками
func (r *TournamentRepository) GetParticipantsByTournamentIDs(ctx context.Context, tournamentIDs []uuid.UUID) (map[uuid.UUID][]*domain.TournamentParticipant, error) {
	if len(tournamentIDs) == 0 {
		return make(map[uuid.UUID][]*domain.TournamentParticipant), nil
	}

	query := `
		SELECT id, tournament_id, program_id, rating, wins, losses, draws, created_at
		FROM tournament_participants
		WHERE tournament_id = ANY($1)
		ORDER BY tournament_id, rating DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get participants by tournament IDs")
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]*domain.TournamentParticipant)

	for rows.Next() {
		var p domain.TournamentParticipant
		err := rows.Scan(
			&p.ID,
			&p.TournamentID,
			&p.ProgramID,
			&p.Rating,
			&p.Wins,
			&p.Losses,
			&p.Draws,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan participant")
		}

		result[p.TournamentID] = append(result[p.TournamentID], &p)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows iteration error")
	}

	return result, nil
}

// ListWithCursor получает список турниров с cursor-based пагинацией
func (r *TournamentRepository) ListWithCursor(ctx context.Context, filter domain.TournamentFilter, pageReq *pagination.PageRequest) ([]*domain.Tournament, bool, error) {
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
		SELECT id, code, name, description, game_type, status, max_participants, max_team_size, is_permanent, creator_id, start_time, end_time,
		       metadata, version, created_at, updated_at
		FROM tournaments
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

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
			// Forward pagination: получаем записи после курсора
			query += fmt.Sprintf(" AND created_at < $%d", argCount)
		} else {
			// Backward pagination: получаем записи до курсора
			query += fmt.Sprintf(" AND created_at > $%d", argCount)
		}
		args = append(args, *cursor.Timestamp)
		argCount++
	}

	// Сортировка (по умолчанию - от новых к старым)
	if pageReq.IsBackward() {
		query += " ORDER BY created_at ASC" // Обратный порядок для backward pagination
	} else {
		query += " ORDER BY created_at DESC"
	}

	// Добавляем +1 к лимиту для определения hasNextPage
	limit := pageReq.GetLimit() + 1
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list tournaments with cursor")
	}
	defer rows.Close()

	var tournaments []*domain.Tournament
	for rows.Next() {
		var tournament domain.Tournament
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
			return nil, false, errors.Wrap(err, "failed to scan tournament")
		}

		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &tournament.Metadata); err != nil {
				return nil, false, errors.Wrap(err, "failed to unmarshal metadata")
			}
		}

		tournaments = append(tournaments, &tournament)
	}

	// Определяем, есть ли ещё страницы
	hasMore := len(tournaments) > pageReq.GetLimit()
	if hasMore {
		// Удаляем последний элемент (он был добавлен только для проверки hasMore)
		tournaments = tournaments[:len(tournaments)-1]
	}

	// Для backward pagination нужно развернуть результаты
	if pageReq.IsBackward() {
		for i, j := 0, len(tournaments)-1; i < j; i, j = i+1, j-1 {
			tournaments[i], tournaments[j] = tournaments[j], tournaments[i]
		}
	}

	return tournaments, hasMore, nil
}

// GetTournamentCursor возвращает курсор для турнира (для использования с pagination.NewConnection)
func GetTournamentCursor(tournament *domain.Tournament) (*pagination.Cursor, error) {
	return pagination.NewTimestampCursor(tournament.CreatedAt), nil
}

// GetCrossGameLeaderboard получает кросс-игровой рейтинг турнира
// Рейтинг = сумма всех очков из всех матчей
func (r *TournamentRepository) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error) {
	// Получаем все команды и программы в турнире со статистикой по каждой игре
	// Рейтинг = сумма очков (score1 когда program1, score2 когда program2)
	query := `
		WITH team_programs AS (
			SELECT DISTINCT ON (p.team_id, p.game_id)
				p.id as program_id,
				p.name as program_name,
				p.team_id,
				t.name as team_name,
				p.game_id,
				g.name as game_name,
				g.display_name as game_display_name
			FROM programs p
			LEFT JOIN teams t ON p.team_id = t.id
			LEFT JOIN games g ON p.game_id = g.id
			WHERE p.tournament_id = $1
			ORDER BY p.team_id, p.game_id, p.version DESC
		),
		game_stats AS (
			SELECT
				tp.program_id,
				tp.program_name,
				tp.team_id,
				tp.team_name,
				tp.game_id,
				tp.game_name,
				COUNT(*) FILTER (WHERE
					(m.program1_id = tp.program_id AND m.winner = 1) OR
					(m.program2_id = tp.program_id AND m.winner = 2)
				) as wins,
				COUNT(*) FILTER (WHERE
					(m.program1_id = tp.program_id AND m.winner = 2) OR
					(m.program2_id = tp.program_id AND m.winner = 1)
				) as losses,
				COUNT(*) FILTER (WHERE m.winner = 0 AND m.status = 'completed') as draws,
				COUNT(*) FILTER (WHERE m.status = 'completed') as total_games,
				COALESCE(SUM(
					CASE
						WHEN m.program1_id = tp.program_id THEN COALESCE(m.score1, 0)
						WHEN m.program2_id = tp.program_id THEN COALESCE(m.score2, 0)
						ELSE 0
					END
				), 0) as total_score
			FROM team_programs tp
			LEFT JOIN matches m ON (m.program1_id = tp.program_id OR m.program2_id = tp.program_id)
				AND m.tournament_id = $1
				AND m.status IN ('completed', 'failed')
			GROUP BY tp.program_id, tp.program_name, tp.team_id, tp.team_name, tp.game_id, tp.game_name
		),
		aggregated AS (
			SELECT
				COALESCE(team_id::text, program_id::text) as group_key,
				team_id,
				team_name,
				program_id,
				program_name,
				json_object_agg(
					COALESCE(game_id::text, 'unknown'),
					json_build_object(
						'game_id', game_id,
						'game_name', game_name,
						'rating', total_score,
						'wins', wins,
						'losses', losses,
						'draws', draws,
						'total_games', total_games
					)
				) as game_ratings,
				SUM(wins) as total_wins,
				SUM(losses) as total_losses,
				SUM(total_games) as total_games,
				SUM(total_score) as total_rating
			FROM game_stats
			GROUP BY group_key, team_id, team_name, program_id, program_name
		)
		SELECT
			ROW_NUMBER() OVER (ORDER BY total_rating DESC, total_wins DESC) as rank,
			team_id,
			team_name,
			program_id,
			program_name,
			game_ratings,
			total_rating,
			total_wins,
			total_losses,
			total_games
		FROM aggregated
		ORDER BY total_rating DESC, total_wins DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cross-game leaderboard")
	}
	defer rows.Close()

	var entries []*domain.CrossGameLeaderboardEntry
	for rows.Next() {
		var entry domain.CrossGameLeaderboardEntry
		var gameRatingsJSON []byte

		err := rows.Scan(
			&entry.Rank,
			&entry.TeamID,
			&entry.TeamName,
			&entry.ProgramID,
			&entry.ProgramName,
			&gameRatingsJSON,
			&entry.TotalRating,
			&entry.TotalWins,
			&entry.TotalLosses,
			&entry.TotalGames,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan cross-game leaderboard entry")
		}

		// Parse game ratings JSON
		entry.GameRatings = make(map[string]domain.GameRatingInfo)
		if gameRatingsJSON != nil {
			var rawRatings map[string]domain.GameRatingInfo
			if err := json.Unmarshal(gameRatingsJSON, &rawRatings); err == nil {
				entry.GameRatings = rawRatings
			}
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

// GetLeaderboardByGameType получает таблицу лидеров для конкретной игры в турнире
// gameType - имя игры (game.name), используется для фильтрации матчей
// Рейтинг = сумма всех очков из всех матчей
func (r *TournamentRepository) GetLeaderboardByGameType(ctx context.Context, tournamentID uuid.UUID, gameType string, limit int) ([]*domain.LeaderboardEntry, error) {
	// Получаем рейтинг на основе результатов матчей для конкретной игры
	// Рейтинг = сумма очков (score1 когда program1, score2 когда program2)
	query := `
		WITH game_stats AS (
			SELECT
				p.id as program_id,
				p.name as program_name,
				COUNT(*) FILTER (WHERE
					(m.program1_id = p.id AND m.winner = 1) OR
					(m.program2_id = p.id AND m.winner = 2)
				) as wins,
				COUNT(*) FILTER (WHERE
					(m.program1_id = p.id AND m.winner = 2) OR
					(m.program2_id = p.id AND m.winner = 1)
				) as losses,
				COUNT(*) FILTER (WHERE m.winner = 0 AND m.status = 'completed') as draws,
				COUNT(*) FILTER (WHERE m.status = 'completed') as total_games,
				COALESCE(SUM(
					CASE
						WHEN m.program1_id = p.id THEN COALESCE(m.score1, 0)
						WHEN m.program2_id = p.id THEN COALESCE(m.score2, 0)
						ELSE 0
					END
				), 0) as total_score
			FROM programs p
			JOIN matches m ON (m.program1_id = p.id OR m.program2_id = p.id)
			WHERE m.tournament_id = $1
			  AND m.game_type = $2
			  AND m.status IN ('completed', 'failed')
			GROUP BY p.id, p.name
		)
		SELECT
			ROW_NUMBER() OVER (ORDER BY total_score DESC, wins DESC) as rank,
			program_id,
			program_name,
			total_score as rating,
			wins,
			losses,
			draws,
			total_games
		FROM game_stats
		ORDER BY total_score DESC, wins DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameType, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get leaderboard by game type")
	}
	defer rows.Close()

	var leaderboard []*domain.LeaderboardEntry
	for rows.Next() {
		var entry domain.LeaderboardEntry
		err := rows.Scan(
			&entry.Rank,
			&entry.ProgramID,
			&entry.ProgramName,
			&entry.Rating,
			&entry.Wins,
			&entry.Losses,
			&entry.Draws,
			&entry.TotalGames,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan leaderboard entry")
		}
		leaderboard = append(leaderboard, &entry)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows iteration error")
	}

	return leaderboard, nil
}
