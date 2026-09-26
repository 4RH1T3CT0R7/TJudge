package storage

import (
	"context"
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

// ApplyMatchResult применяет рейтинг за матч ровно один раз, всё в одной транзакции:
//  1. outbox-задача матча гасится (pending -> done). строка блокируется, поэтому
//     fast path воркера и OutboxDispatcher сериализуются на ней, и второй видит
//     done. заодно проверяется, что матч ещё completed (не удалён сбросом раунда)
//     и что истории по нему нет: старый воркер писал рейтинг до закрытия
//     outbox, и pending-задача с готовой историей после него штатна
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
			  AND NOT EXISTS (SELECT 1 FROM rating_history WHERE match_id = $1)
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
