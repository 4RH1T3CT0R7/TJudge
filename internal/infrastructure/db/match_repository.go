package db

import (
	"context"
	"database/sql"
	stderrors "errors"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// MatchRepository - репозиторий для работы с матчами
type MatchRepository struct {
	db *DB
}

// NewMatchRepository создаёт новый репозиторий матчей
func NewMatchRepository(db *DB) *MatchRepository {
	return &MatchRepository{db: db}
}

// Create создаёт новый матч
func (r *MatchRepository) Create(ctx context.Context, match *domain.Match) error {
	query := `
		INSERT INTO matches (id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		match.ID,
		match.TournamentID,
		match.Program1ID,
		match.Program2ID,
		match.GameType,
		match.Status,
		match.Priority,
		match.RoundNumber,
		match.CreatedAt,
	)

	if err != nil {
		return errors.Wrap(err, "failed to create match")
	}

	return nil
}

// CreateBatch создаёт несколько матчей одновременно
func (r *MatchRepository) CreateBatch(ctx context.Context, matches []*domain.Match) error {
	if len(matches) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		INSERT INTO matches (id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}
	defer stmt.Close()

	for _, match := range matches {
		_, err := stmt.ExecContext(ctx,
			match.ID,
			match.TournamentID,
			match.Program1ID,
			match.Program2ID,
			match.GameType,
			match.Status,
			match.Priority,
			match.RoundNumber,
			match.CreatedAt,
		)
		if err != nil {
			return errors.Wrap(err, "failed to insert match")
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// DeleteBatch удаляет матчи по списку ID одним запросом.
// Используется для компенсации (rollback) при ошибке EnqueueBatch (P0.5).
// Идемпотентен: отсутствующие ID просто пропускаются без ошибки.
func (r *MatchRepository) DeleteBatch(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	query := `DELETE FROM matches WHERE id = ANY($1)`
	if _, err := r.db.ExecContext(ctx, query, pq.Array(strs)); err != nil {
		return errors.Wrap(err, "failed to delete matches batch")
	}
	return nil
}

// GetByID получает матч по ID
func (r *MatchRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error) {
	var match domain.Match

	query := `
		SELECT id, tournament_id, program1_id, program2_id, game_type, status, priority, round_number,
		       score1, score2, winner, error_code, error_message, started_at, completed_at, created_at
		FROM matches
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&match.ID,
		&match.TournamentID,
		&match.Program1ID,
		&match.Program2ID,
		&match.GameType,
		&match.Status,
		&match.Priority,
		&match.RoundNumber,
		&match.Score1,
		&match.Score2,
		&match.Winner,
		&match.ErrorCode,
		&match.ErrorMessage,
		&match.StartedAt,
		&match.CompletedAt,
		&match.CreatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("match not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get match by id")
	}

	return &match, nil
}
