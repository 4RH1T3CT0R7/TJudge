package db

import (
	"context"
	"database/sql"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// RatingRepository - репозиторий для работы с рейтингами
type RatingRepository struct {
	db *DB
}

// NewRatingRepository создаёт новый репозиторий рейтингов
func NewRatingRepository(db *DB) *RatingRepository {
	return &RatingRepository{db: db}
}

// Create создаёт запись в истории рейтингов
func (r *RatingRepository) Create(ctx context.Context, history *domain.RatingHistory) error {
	query := `
		INSERT INTO rating_history (id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		history.ID,
		history.ProgramID,
		history.TournamentID,
		history.OldRating,
		history.NewRating,
		history.Change,
		history.MatchID,
		history.CreatedAt,
	)

	if err != nil {
		return errors.Wrap(err, "failed to create rating history")
	}

	return nil
}

// GetByProgramID получает историю рейтинга программы
func (r *RatingRepository) GetByProgramID(ctx context.Context, programID uuid.UUID) ([]*domain.RatingHistory, error) {
	var history []*domain.RatingHistory

	query := `
		SELECT id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at
		FROM rating_history
		WHERE program_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`

	err := r.db.QueryWithMetrics(ctx, "rating_get_by_program", &history, query, programID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rating history")
	}

	return history, nil
}

// GetByTournamentID получает историю рейтинга в турнире
func (r *RatingRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.RatingHistory, error) {
	var history []*domain.RatingHistory

	query := `
		SELECT id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at
		FROM rating_history
		WHERE tournament_id = $1
		ORDER BY created_at DESC
		LIMIT 1000
	`

	err := r.db.QueryWithMetrics(ctx, "rating_get_by_tournament", &history, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rating history by tournament")
	}

	return history, nil
}

// UpdateParticipantRating обновляет рейтинг участника турнира
func (r *RatingRepository) UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, newRating int) error {
	query := `
		UPDATE tournament_participants
		SET rating = $3
		WHERE tournament_id = $1 AND program_id = $2
	`

	result, err := r.db.ExecWithMetrics(ctx, "rating_update_participant", query, tournamentID, programID, newRating)
	if err != nil {
		return errors.Wrap(err, "failed to update participant rating")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("tournament participant not found")
	}

	return nil
}

// UpdateParticipantStats обновляет статистику участника (wins/losses/draws)
func (r *RatingRepository) UpdateParticipantStats(ctx context.Context, tournamentID, programID uuid.UUID, won bool, draw bool) error {
	var query string

	if won {
		query = `
			UPDATE tournament_participants
			SET wins = wins + 1
			WHERE tournament_id = $1 AND program_id = $2
		`
	} else if draw {
		query = `
			UPDATE tournament_participants
			SET draws = draws + 1
			WHERE tournament_id = $1 AND program_id = $2
		`
	} else {
		query = `
			UPDATE tournament_participants
			SET losses = losses + 1
			WHERE tournament_id = $1 AND program_id = $2
		`
	}

	result, err := r.db.ExecWithMetrics(ctx, "rating_update_stats", query, tournamentID, programID)
	if err != nil {
		return errors.Wrap(err, "failed to update participant stats")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("tournament participant not found")
	}

	return nil
}

// GetParticipantRating получает текущий рейтинг участника турнира
func (r *RatingRepository) GetParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID) (int, error) {
	var rating int

	query := `
		SELECT rating
		FROM tournament_participants
		WHERE tournament_id = $1 AND program_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID, programID).Scan(&rating)
	if err == sql.ErrNoRows {
		return 0, errors.ErrNotFound.WithMessage("tournament participant not found")
	}
	if err != nil {
		return 0, errors.Wrap(err, "failed to get participant rating")
	}

	return rating, nil
}

// GetParticipantRatings получает рейтинги обоих участников матча
func (r *RatingRepository) GetParticipantRatings(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID) (rating1, rating2 int, err error) {
	query := `
		SELECT rating
		FROM tournament_participants
		WHERE tournament_id = $1 AND program_id = $2
	`

	// Получаем рейтинг первого участника
	err = r.db.QueryRowContext(ctx, query, tournamentID, program1ID).Scan(&rating1)
	if err == sql.ErrNoRows {
		return 0, 0, errors.ErrNotFound.WithMessage("program1 not found in tournament")
	}
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get program1 rating")
	}

	// Получаем рейтинг второго участника
	err = r.db.QueryRowContext(ctx, query, tournamentID, program2ID).Scan(&rating2)
	if err == sql.ErrNoRows {
		return 0, 0, errors.ErrNotFound.WithMessage("program2 not found in tournament")
	}
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get program2 rating")
	}

	return rating1, rating2, nil
}
