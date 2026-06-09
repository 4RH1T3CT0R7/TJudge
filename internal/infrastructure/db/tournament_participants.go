package db

import (
	"context"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

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

// GetTeamsCount получает количество команд в турнире
func (r *TournamentRepository) GetTeamsCount(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var count int

	query := `SELECT COUNT(*) FROM teams WHERE tournament_id = $1`

	err := r.db.QueryRowContext(ctx, query, tournamentID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get teams count")
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
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

// GetLatestParticipantsGroupedByGame получает участников турнира сгруппированных по играм
// Возвращает map[game_type] -> participants
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

// GetLatestParticipantsByGame получает участников турнира для конкретной игры
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
