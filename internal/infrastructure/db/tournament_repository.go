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
		INSERT INTO tournaments (id, name, game_type, status, max_participants, start_time, end_time, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at, version
	`

	err = r.db.QueryRowContext(ctx, query,
		tournament.ID,
		tournament.Name,
		tournament.GameType,
		tournament.Status,
		tournament.MaxParticipants,
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
		SELECT id, name, game_type, status, max_participants, start_time, end_time,
		       metadata, version, created_at, updated_at
		FROM tournaments
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tournament.ID,
		&tournament.Name,
		&tournament.GameType,
		&tournament.Status,
		&tournament.MaxParticipants,
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
		SELECT id, name, game_type, status, max_participants, start_time, end_time,
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
			&tournament.Name,
			&tournament.GameType,
			&tournament.Status,
			&tournament.MaxParticipants,
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
func (r *TournamentRepository) getLeaderboardFallback(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	query := `
		SELECT
			ROW_NUMBER() OVER (ORDER BY tp.rating DESC) as rank,
			tp.program_id,
			p.name as program_name,
			tp.rating,
			tp.wins,
			tp.losses,
			tp.draws,
			(tp.wins + tp.losses + tp.draws) as total_games
		FROM tournament_participants tp
		JOIN programs p ON tp.program_id = p.id
		WHERE tp.tournament_id = $1
		ORDER BY tp.rating DESC
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
		SELECT id, name, game_type, status, max_participants, start_time, end_time,
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
			&tournament.Name,
			&tournament.GameType,
			&tournament.Status,
			&tournament.MaxParticipants,
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
