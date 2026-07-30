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

func NewTournamentRepository(db *DB) *TournamentRepository {
	return &TournamentRepository{db: db}
}

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

// Update обновляет турнир с optimistic lock: апдейт проходит только если version
// в базе совпала с прочитанной, иначе кто-то успел обновить раньше нас и мы
// отдаём ErrConcurrentUpdate. version инкрементится тем же запросом.
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

// ListWithCursor - список турниров с курсорной пагинацией
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
			// вперёд: записи после курсора
			query += fmt.Sprintf(" AND created_at < $%d", argCount)
		} else {
			// назад: записи до курсора
			query += fmt.Sprintf(" AND created_at > $%d", argCount)
		}
		args = append(args, *cursor.Timestamp)
		argCount++
	}

	// Сортировка (по умолчанию - от новых к старым)
	if pageReq.IsBackward() {
		query += " ORDER BY created_at ASC" // обратный порядок для пагинации назад
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

	// для пагинации назад разворачиваем результаты
	if pageReq.IsBackward() {
		for i, j := 0, len(tournaments)-1; i < j; i, j = i+1, j-1 {
			tournaments[i], tournaments[j] = tournaments[j], tournaments[i]
		}
	}

	return tournaments, hasMore, nil
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return participants, nil
}

func (r *TournamentRepository) GetLatestParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error) {
	var participants []*domain.TournamentParticipant

	// Выбираем только участников с последней версией программы для каждой команды и игры
	query := `
		SELECT tp.id, tp.tournament_id, tp.program_id, tp.rating, tp.wins, tp.losses, tp.draws, tp.created_at
		FROM tournament_participants tp
		INNER JOIN programs p ON p.id = tp.program_id
		INNER JOIN teams t ON t.id = p.team_id AND t.is_disqualified = false
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return participants, nil
}

// ParticipantWithGameType - участник с типом игры для группировки
type ParticipantWithGameType struct {
	domain.TournamentParticipant
	GameType string `json:"game_type" db:"game_type"`
}

// GetLatestParticipantsGroupedByGame - участники турнира, сгруппированные по играм (map game_type -> участники)
func (r *TournamentRepository) GetLatestParticipantsGroupedByGame(ctx context.Context, tournamentID uuid.UUID) (map[string][]*domain.TournamentParticipant, error) {
	// Выбираем участников с последней ГОТОВОЙ версией программы и их game_type.
	// Только status='ready': compiling ещё не собралась, failed не собралась
	// вообще. Если новая версия сломана, команда продолжает играть предыдущей
	// рабочей версией (MAX(version) берётся среди ready).
	query := `
		SELECT tp.id, tp.tournament_id, tp.program_id, tp.rating, tp.wins, tp.losses, tp.draws, tp.created_at, g.name as game_type
		FROM tournament_participants tp
		INNER JOIN programs p ON p.id = tp.program_id
		INNER JOIN games g ON g.id = p.game_id
		INNER JOIN teams t ON t.id = p.team_id AND t.is_disqualified = false
		WHERE tp.tournament_id = $1
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

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get participants grouped by game")
	}
	defer rows.Close()

	result := make(map[string][]*domain.TournamentParticipant)
	for rows.Next() {
		var p domain.TournamentParticipant
		var gameType string
		err := rows.Scan(
			&p.ID,
			&p.TournamentID,
			&p.ProgramID,
			&p.Rating,
			&p.Wins,
			&p.Losses,
			&p.Draws,
			&p.CreatedAt,
			&gameType,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan participant with game type")
		}
		result[gameType] = append(result[gameType], &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}

func (r *TournamentRepository) GetLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.TournamentParticipant, error) {
	var participants []*domain.TournamentParticipant

	// Выбираем только участников с программами для конкретной игры (последняя версия)
	query := `
		SELECT tp.id, tp.tournament_id, tp.program_id, tp.rating, tp.wins, tp.losses, tp.draws, tp.created_at
		FROM tournament_participants tp
		INNER JOIN programs p ON p.id = tp.program_id
		INNER JOIN games g ON g.id = p.game_id
		INNER JOIN teams t ON t.id = p.team_id AND t.is_disqualified = false
		WHERE tp.tournament_id = $1
		  AND g.name = $2
		  AND p.version = (
		      SELECT MAX(p2.version)
		      FROM programs p2
		      WHERE p2.team_id = p.team_id
		        AND p2.game_id = p.game_id
		        AND p2.tournament_id = p.tournament_id
		  )
		ORDER BY tp.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get participants by game")
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return participants, nil
}

// GetParticipantsByTournamentIDs тянет участников сразу для нескольких турниров одним запросом,
// чтобы не ловить N+1 при загрузке списка турниров с участниками
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
