package db

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/rating"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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

// UpdateParticipantRating обновляет рейтинг участника турнира (delta-based).
//
// Invariant (P1.13): UPDATE вычисляет `rating + $3` В САМОЙ БД, под row-level
// lock'ом PostgreSQL. Это значит concurrent UPDATE-ы НЕ теряют deltas: БД
// сериализует их через MVCC и суммирует корректно.
//
// Известное ограничение: вычисление delta (в `rating.Service.ProcessMatchResult`)
// использует rating, прочитанный ДО transaction. Для параллельных матчей
// одного участника это даёт snapshot-based delta, а не "тотально синхронное"
// ELO. В массовых RR-турнирах эффект размывается и приемлем. Строгая
// сериализация потребует advisory lock на (tournament_id, program_id) —
// отложено до P2 (см. план).
//
// Все вызовы этого метода должны идти через ProcessMatchResultAtomic, который
// обрабатывает обоих участников матча в одной транзакции (гарантирует
// нулевую сумму delta per match).
func (r *RatingRepository) UpdateParticipantRating(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int) error {
	query := `
		UPDATE tournament_participants
		SET rating = GREATEST(0, rating + $3)
		WHERE tournament_id = $1 AND program_id = $2
	`

	result, err := r.db.ExecWithMetrics(ctx, "rating_update_participant", query, tournamentID, programID, ratingDelta)
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
	if stderrors.Is(err, sql.ErrNoRows) {
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
	if stderrors.Is(err, sql.ErrNoRows) {
		return 0, 0, errors.ErrNotFound.WithMessage("program1 not found in tournament")
	}
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get program1 rating")
	}

	// Получаем рейтинг второго участника
	err = r.db.QueryRowContext(ctx, query, tournamentID, program2ID).Scan(&rating2)
	if stderrors.Is(err, sql.ErrNoRows) {
		return 0, 0, errors.ErrNotFound.WithMessage("program2 not found in tournament")
	}
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get program2 rating")
	}

	return rating1, rating2, nil
}

// ResetParticipantsForGame сбрасывает рейтинги и статистику всех участников для определённой игры
func (r *RatingRepository) ResetParticipantsForGame(ctx context.Context, tournamentID, gameID uuid.UUID) (int64, error) {
	// Сначала получаем программы для этой игры
	query := `
		UPDATE tournament_participants tp
		SET rating = 1500, wins = 0, losses = 0, draws = 0
		FROM programs p
		WHERE tp.program_id = p.id
		AND tp.tournament_id = $1
		AND p.game_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, tournamentID, gameID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to reset participants for game")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return rows, nil
}

// UpdateParticipantRatingAndStats atomically updates rating (delta-based) and stats for a participant
func (r *RatingRepository) UpdateParticipantRatingAndStats(ctx context.Context, tournamentID, programID uuid.UUID, ratingDelta int, won bool, draw bool) error {
	var statsField string
	if won {
		statsField = "wins = wins + 1"
	} else if draw {
		statsField = "draws = draws + 1"
	} else {
		statsField = "losses = losses + 1"
	}

	query := fmt.Sprintf(`
		UPDATE tournament_participants
		SET rating = GREATEST(0, rating + $3), %s
		WHERE tournament_id = $1 AND program_id = $2
	`, statsField)

	result, err := r.db.ExecWithMetrics(ctx, "rating_update_participant_and_stats", query, tournamentID, programID, ratingDelta)
	if err != nil {
		return errors.Wrap(err, "failed to update participant rating and stats")
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

// ProcessMatchResultAtomic выполняет все обновления рейтингов и статистики для обоих участников
// матча в одной транзакции. Если любая операция падает — все откатываются.
func (r *RatingRepository) ProcessMatchResultAtomic(ctx context.Context, update1, update2 *rating.ParticipantUpdate) error {
	return r.db.RunInTx(ctx, func(tx *sqlx.Tx) error {
		for _, u := range []*rating.ParticipantUpdate{update1, update2} {
			// Вставляем запись в историю рейтингов
			insertQuery := `
				INSERT INTO rating_history (id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`
			_, err := tx.ExecContext(ctx, insertQuery,
				u.History.ID,
				u.History.ProgramID,
				u.History.TournamentID,
				u.History.OldRating,
				u.History.NewRating,
				u.History.Change,
				u.History.MatchID,
				u.History.CreatedAt,
			)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to create rating history for program %s", u.ProgramID))
			}

			// Атомарно обновляем рейтинг и статистику участника
			var statsField string
			if u.Won {
				statsField = "wins = wins + 1"
			} else if u.Draw {
				statsField = "draws = draws + 1"
			} else {
				statsField = "losses = losses + 1"
			}

			updateQuery := fmt.Sprintf(`
				UPDATE tournament_participants
				SET rating = GREATEST(0, rating + $3), %s
				WHERE tournament_id = $1 AND program_id = $2
			`, statsField)

			result, err := tx.ExecContext(ctx, updateQuery, u.TournamentID, u.ProgramID, u.RatingDelta)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to update participant rating and stats for program %s", u.ProgramID))
			}

			rows, err := result.RowsAffected()
			if err != nil {
				return errors.Wrap(err, "failed to get rows affected")
			}
			if rows == 0 {
				return errors.ErrNotFound.WithMessage(fmt.Sprintf("tournament participant not found: program %s", u.ProgramID))
			}
		}

		return nil
	})
}

// DeleteRatingHistoryForGame удаляет историю рейтингов для определённой игры
func (r *RatingRepository) DeleteRatingHistoryForGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (int64, error) {
	query := `
		DELETE FROM rating_history rh
		WHERE rh.tournament_id = $1
		AND rh.match_id IN (
			SELECT id FROM matches WHERE tournament_id = $1 AND game_type = $2
		)
	`

	result, err := r.db.ExecContext(ctx, query, tournamentID, gameType)
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete rating history for game")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return rows, nil
}
