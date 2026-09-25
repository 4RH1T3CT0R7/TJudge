package storage

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/rating"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RatingRepository struct {
	db *DB
}

func NewRatingRepository(db *DB) *RatingRepository {
	return &RatingRepository{db: db}
}

func (r *RatingRepository) Create(ctx context.Context, history *models.RatingHistory) error {
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

func (r *RatingRepository) GetByProgramID(ctx context.Context, programID uuid.UUID) ([]*models.RatingHistory, error) {
	var history []*models.RatingHistory

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

// GetByProgramAndTournament - история рейтинга программы в турнире в
// хронологии, для графика. limit ограничивает число последних точек
func (r *RatingRepository) GetByProgramAndTournament(ctx context.Context, programID, tournamentID uuid.UUID, limit int) ([]*models.RatingHistory, error) {
	var history []*models.RatingHistory

	// последние limit записей, развёрнутые в хронологию
	query := `
		SELECT id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at
		FROM (
			SELECT id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at
			FROM rating_history
			WHERE program_id = $1 AND tournament_id = $2
			ORDER BY created_at DESC
			LIMIT $3
		) recent
		ORDER BY created_at ASC
	`

	err := r.db.QueryWithMetrics(ctx, "rating_get_by_program_tournament", &history, query, programID, tournamentID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rating history")
	}

	return history, nil
}

func (r *RatingRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.RatingHistory, error) {
	var history []*models.RatingHistory

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

// UpdateParticipantRating прибавляет дельту к рейтингу участника.
// важно: rating + $3 считается в самой бд под row-lock, поэтому
// параллельные апдейты не теряют дельты (mvcc сериализует и суммирует).
// TODO: строгая сериализация через advisory lock на (tournament_id, program_id), пока и так норм
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

// ResetParticipantsForGame сбрасывает рейтинг и статистику участников по игре
func (r *RatingRepository) ResetParticipantsForGame(ctx context.Context, tournamentID, gameID uuid.UUID) (int64, error) {
	// апдейт только участников, чьи программы этой игры
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

// UpdateParticipantRatingAndStats атомарно обновляет рейтинг (дельтой) и статистику участника
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

// ApplyMatchResult применяет рейтинг за матч ровно один раз, всё в одной транзакции:
//  1. outbox-задача матча гасится (pending -> done). строка блокируется, поэтому
//     fast path воркера и OutboxDispatcher сериализуются на ней, и второй видит
//     done. заодно проверяется, что матч ещё completed (не удалён сбросом раунда)
//  2. рейтинги обоих участников читаются FOR UPDATE в порядке program_id: общий
//     порядок блокировок исключает дедлок параллельных матчей AB и BA
//  3. calc считает обновления от заблокированных значений, они пишутся в историю
//     и в tournament_participants
//
// false - применять нечего: рейтинг уже применён или матча больше нет
func (r *RatingRepository) ApplyMatchResult(ctx context.Context, match *models.Match, calc func(rating1, rating2 int) (*rating.ParticipantUpdate, *rating.ParticipantUpdate)) (bool, error) {
	applied := false
	err := r.db.RunInTx(ctx, func(tx *sqlx.Tx) error {
		claim, err := tx.ExecContext(ctx, `
			UPDATE match_outbox SET status = 'done', processed_at = NOW()
			WHERE match_id = $1 AND kind = $2 AND status = 'pending'
			  AND EXISTS (SELECT 1 FROM matches WHERE id = $1 AND status = 'completed' FOR KEY SHARE)
		`, match.ID, OutboxKindRatingUpdate)
		if err != nil {
			return errors.Wrap(err, "failed to claim rating outbox entry")
		}
		claimed, err := claim.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}
		if claimed == 0 {
			return nil
		}

		var rows []struct {
			ProgramID uuid.UUID `db:"program_id"`
			Rating    int       `db:"rating"`
		}
		if err := tx.SelectContext(ctx, &rows, `
			SELECT program_id, rating FROM tournament_participants
			WHERE tournament_id = $1 AND program_id IN ($2, $3)
			ORDER BY program_id
			FOR UPDATE
		`, match.TournamentID, match.Program1ID, match.Program2ID); err != nil {
			return errors.Wrap(err, "failed to lock participant ratings")
		}
		ratings := make(map[uuid.UUID]int, len(rows))
		for _, row := range rows {
			ratings[row.ProgramID] = row.Rating
		}
		rating1, ok1 := ratings[match.Program1ID]
		rating2, ok2 := ratings[match.Program2ID]
		if !ok1 || !ok2 {
			return errors.ErrNotFound.WithMessage("tournament participant not found")
		}

		update1, update2 := calc(rating1, rating2)
		for _, u := range []*rating.ParticipantUpdate{update1, update2} {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO rating_history (id, program_id, tournament_id, old_rating, new_rating, change, match_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				u.History.ID,
				u.History.ProgramID,
				u.History.TournamentID,
				u.History.OldRating,
				u.History.NewRating,
				u.History.Change,
				u.History.MatchID,
				u.History.CreatedAt,
			); err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to create rating history for program %s", u.ProgramID))
			}

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
			if _, err := tx.ExecContext(ctx, updateQuery, u.TournamentID, u.ProgramID, u.RatingDelta); err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to update participant rating and stats for program %s", u.ProgramID))
			}
		}

		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

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
