package db

import (
	"context"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// UpdateStatus обновляет статус матча
func (r *MatchRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus) error {
	var query string

	if status == domain.MatchRunning {
		// Только pending матчи могут перейти в running.
		// Это предотвращает повторную обработку матча, если он оказался
		// в очереди дважды (например, при retry).
		query = `
			UPDATE matches
			SET status = $2, started_at = NOW()
			WHERE id = $1 AND status = 'pending'
		`
	} else {
		query = `
			UPDATE matches
			SET status = $2
			WHERE id = $1
		`
	}

	result, err := r.db.ExecWithMetrics(ctx, "match_update_status", query, id, status)
	if err != nil {
		return errors.Wrap(err, "failed to update match status")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		if status == domain.MatchRunning {
			return domain.ErrMatchAlreadyProcessed
		}
		return errors.ErrNotFound.WithMessage("match not found")
	}

	return nil
}

// UpdateResult обновляет результат матча
func (r *MatchRepository) UpdateResult(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error {
	query := `
		UPDATE matches
		SET status = $2, score1 = $3, score2 = $4, winner = $5,
		    error_code = $6, error_message = $7, completed_at = NOW()
		WHERE id = $1
	`

	status := domain.MatchCompleted
	if result.ErrorCode != 0 {
		status = domain.MatchFailed
	}

	var errorCode *int
	if result.ErrorCode != 0 {
		errorCode = &result.ErrorCode
	}

	var errorMsg *string
	if result.ErrorMessage != "" {
		errorMsg = &result.ErrorMessage
	}

	_, err := r.db.ExecWithMetrics(ctx, "match_update_result", query,
		id,
		status,
		result.Score1,
		result.Score2,
		result.Winner,
		errorCode,
		errorMsg,
	)

	if err != nil {
		return errors.Wrap(err, "failed to update match result")
	}

	return nil
}

// UpdateResultWithOutbox обновляет результат матча и в той же транзакции
// записывает outbox-задачу «обновить рейтинг» (для успешно завершённых
// матчей с определившимся победителем). Гарантия: если результат
// зафиксирован, рейтинг будет обработан - сразу воркером (fast path)
// или OutboxDispatcher'ом после сбоя.
func (r *MatchRepository) UpdateResultWithOutbox(ctx context.Context, id uuid.UUID, result *domain.MatchResult) error {
	status := domain.MatchCompleted
	if result.ErrorCode != 0 {
		status = domain.MatchFailed
	}

	var errorCode *int
	if result.ErrorCode != 0 {
		errorCode = &result.ErrorCode
	}

	var errorMsg *string
	if result.ErrorMessage != "" {
		errorMsg = &result.ErrorMessage
	}

	return r.db.RunInTx(ctx, func(tx *sqlx.Tx) error {
		updateQuery := `
			UPDATE matches
			SET status = $2, score1 = $3, score2 = $4, winner = $5,
			    error_code = $6, error_message = $7, completed_at = NOW()
			WHERE id = $1
		`
		if _, err := tx.ExecContext(ctx, updateQuery,
			id, status, result.Score1, result.Score2, result.Winner, errorCode, errorMsg,
		); err != nil {
			return errors.Wrap(err, "failed to update match result")
		}

		if status == domain.MatchCompleted && result.Winner >= 0 {
			outboxQuery := `INSERT INTO match_outbox (match_id, kind) VALUES ($1, $2)`
			if _, err := tx.ExecContext(ctx, outboxQuery, id, OutboxKindRatingUpdate); err != nil {
				return errors.Wrap(err, "failed to insert outbox entry")
			}
		}

		return nil
	})
}

// MarkRatingApplied помечает outbox-задачу рейтинга выполненной (fast path:
// воркер успел обновить рейтинг сразу после записи результата).
func (r *MatchRepository) MarkRatingApplied(ctx context.Context, matchID uuid.UUID) error {
	query := `
		UPDATE match_outbox
		SET status = 'done', processed_at = NOW()
		WHERE match_id = $1 AND kind = $2 AND status = 'pending'
	`
	if _, err := r.db.ExecContext(ctx, query, matchID, OutboxKindRatingUpdate); err != nil {
		return errors.Wrap(err, "failed to mark rating applied")
	}
	return nil
}

// ResetToPending возвращает матч из running в pending после транзиентной
// инфраструктурной ошибки executor'а (Docker daemon недоступен и т.п.):
// программа участника не виновата, матч будет повторён - ретраем пула
// или периодическим recovery-сервисом.
func (r *MatchRepository) ResetToPending(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE matches
		SET status = 'pending', started_at = NULL
		WHERE id = $1 AND status = 'running'
	`
	if _, err := r.db.ExecWithMetrics(ctx, "match_reset_pending", query, id); err != nil {
		return errors.Wrap(err, "failed to reset match to pending")
	}
	return nil
}

// ResetFailedMatches сбрасывает все failed матчи турнира в pending
func (r *MatchRepository) ResetFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int64, error) {
	query := `
		UPDATE matches
		SET status = $1, error_code = NULL, error_message = NULL, started_at = NULL, completed_at = NULL,
		    score1 = NULL, score2 = NULL, winner = NULL
		WHERE tournament_id = $2 AND status = $3
	`

	result, err := r.db.ExecContext(ctx, query, domain.MatchPending, tournamentID, domain.MatchFailed)
	if err != nil {
		return 0, errors.Wrap(err, "failed to reset failed matches")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return rows, nil
}

// BatchUpdateStatus обновляет статус для нескольких матчей одновременно
func (r *MatchRepository) BatchUpdateStatus(ctx context.Context, matchIDs []uuid.UUID, status domain.MatchStatus) error {
	if len(matchIDs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	var query string
	if status == domain.MatchRunning {
		// Только pending матчи могут перейти в running (защита от дублирования)
		query = `
			UPDATE matches
			SET status = $1, started_at = NOW()
			WHERE id = ANY($2) AND status = 'pending'
		`
	} else {
		query = `
			UPDATE matches
			SET status = $1
			WHERE id = ANY($2)
		`
	}

	_, err = tx.ExecContext(ctx, query, status, pq.Array(matchIDs))
	if err != nil {
		return errors.Wrap(err, "failed to batch update match status")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// BatchUpdateResults обновляет результаты для нескольких матчей одновременно
func (r *MatchRepository) BatchUpdateResults(ctx context.Context, results map[uuid.UUID]*domain.MatchResult) error {
	if len(results) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		UPDATE matches
		SET status = $2, score1 = $3, score2 = $4, winner = $5,
		    error_code = $6, error_message = $7, completed_at = NOW()
		WHERE id = $1
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}
	defer stmt.Close()

	for matchID, result := range results {
		status := domain.MatchCompleted
		if result.ErrorCode != 0 {
			status = domain.MatchFailed
		}

		var errorCode *int
		if result.ErrorCode != 0 {
			errorCode = &result.ErrorCode
		}

		var errorMsg *string
		if result.ErrorMessage != "" {
			errorMsg = &result.ErrorMessage
		}

		_, err := stmt.ExecContext(ctx,
			matchID,
			status,
			result.Score1,
			result.Score2,
			result.Winner,
			errorCode,
			errorMsg,
		)
		if err != nil {
			return errors.Wrap(err, "failed to update match result in batch")
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// DeleteMatchesForGame удаляет все матчи турнира для определённой игры
func (r *MatchRepository) DeleteMatchesForGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (int64, error) {
	query := `DELETE FROM matches WHERE tournament_id = $1 AND game_type = $2`

	result, err := r.db.ExecContext(ctx, query, tournamentID, gameType)
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete matches for game")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return rows, nil
}
