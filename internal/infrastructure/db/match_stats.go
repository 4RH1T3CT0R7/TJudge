package db

import (
	"context"
	"database/sql"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// MatchStatistics - статистика матчей
type MatchStatistics struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// HasStartedMatches проверяет, есть ли запущенные или завершённые матчи для турнира и игры
// Возвращает true, если есть матчи со статусом running или completed
func (r *MatchRepository) HasStartedMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM matches
			WHERE tournament_id = $1
			AND game_type = $2
			AND status IN ($3, $4)
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, tournamentID, gameType, domain.MatchRunning, domain.MatchCompleted).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check started matches")
	}

	return exists, nil
}

// HasAnyRunningMatches проверяет, есть ли запущенные (running) или ожидающие (pending) матчи
// для любой игры в турнире. Используется для блокировки загрузки программ когда раунд активен.
func (r *MatchRepository) HasAnyRunningMatches(ctx context.Context, tournamentID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM matches
			WHERE tournament_id = $1
			AND status IN ($2, $3)
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, tournamentID, domain.MatchRunning, domain.MatchPending).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check running matches")
	}

	return exists, nil
}

// GetActiveGameType возвращает тип игры, для которой сейчас выполняются матчи.
// Возвращает пустую строку, если нет активных матчей.
func (r *MatchRepository) GetActiveGameType(ctx context.Context, tournamentID uuid.UUID) (string, error) {
	query := `
		SELECT COALESCE(
			(SELECT game_type FROM matches
			 WHERE tournament_id = $1
			 AND status IN ($2, $3)
			 ORDER BY
				CASE WHEN status = $2 THEN 1 ELSE 2 END,
				created_at ASC
			 LIMIT 1),
			''
		)
	`

	var gameType string
	err := r.db.QueryRowContext(ctx, query, tournamentID, domain.MatchRunning, domain.MatchPending).Scan(&gameType)
	if err != nil {
		return "", errors.Wrap(err, "failed to get active game type")
	}

	return gameType, nil
}

// GetNextRoundNumber получает следующий номер раунда для турнира
func (r *MatchRepository) GetNextRoundNumber(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	var maxRound sql.NullInt64

	query := `SELECT MAX(round_number) FROM matches WHERE tournament_id = $1`

	err := r.db.QueryRowContext(ctx, query, tournamentID).Scan(&maxRound)
	if err != nil {
		return 1, errors.Wrap(err, "failed to get max round number")
	}

	if !maxRound.Valid {
		return 1, nil
	}

	return int(maxRound.Int64) + 1, nil
}

// GetNextRoundNumberByGame получает следующий номер раунда для конкретной игры в турнире
func (r *MatchRepository) GetNextRoundNumberByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	query := `
		SELECT COALESCE(MAX(round_number), 0) + 1
		FROM matches
		WHERE tournament_id = $1 AND game_type = $2
	`

	var nextRound int
	err := r.db.QueryRowContext(ctx, query, tournamentID, gameType).Scan(&nextRound)
	if err != nil {
		return 1, errors.Wrap(err, "failed to get next round number by game")
	}

	return nextRound, nil
}

// GetStatistics получает статистику матчей
func (r *MatchRepository) GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*MatchStatistics, error) {
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'running') as running,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM matches
	`

	args := []any{}
	if tournamentID != nil {
		query += " WHERE tournament_id = $1"
		args = append(args, *tournamentID)
	}

	var stats MatchStatistics
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Running,
		&stats.Completed,
		&stats.Failed,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get match statistics")
	}

	return &stats, nil
}
