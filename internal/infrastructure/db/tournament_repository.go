package db

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
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

// List получает список турниров с фильтрацией и пагинацией
func (r *TournamentRepository) List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error) {
	query := `
		SELECT id, code, name, description, game_type, status, max_participants, max_team_size, is_permanent, creator_id, start_time, end_time,
		       metadata, version, created_at, updated_at
		FROM tournaments
		WHERE 1=1
	`
	args := []any{}
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
	query += orderByCreatedAtDesc

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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
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
		tournament.StartTime,
		tournament.EndTime,
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
	args := []any{}
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
		query += orderByCreatedAtDesc
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

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("rows iteration error: %w", err)
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
