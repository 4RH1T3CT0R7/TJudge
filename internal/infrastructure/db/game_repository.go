package db

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// GameRepository - репозиторий для работы с играми
type GameRepository struct {
	db *DB
}

// NewGameRepository создаёт новый репозиторий игр
func NewGameRepository(db *DB) *GameRepository {
	return &GameRepository{db: db}
}

// Create создаёт новую игру
func (r *GameRepository) Create(ctx context.Context, game *domain.Game) error {
	query := `
		INSERT INTO games (id, name, display_name, rules)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		game.ID,
		game.Name,
		game.DisplayName,
		game.Rules,
	).Scan(&game.CreatedAt, &game.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, "failed to create game")
	}

	return nil
}

// GetByID получает игру по ID
func (r *GameRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error) {
	var game domain.Game

	query := `
		SELECT id, name, display_name, rules, created_at, updated_at
		FROM games
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&game.ID,
		&game.Name,
		&game.DisplayName,
		&game.Rules,
		&game.CreatedAt,
		&game.UpdatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("game not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get game by id")
	}

	return &game, nil
}

// GetByName получает игру по имени
func (r *GameRepository) GetByName(ctx context.Context, name string) (*domain.Game, error) {
	var game domain.Game

	query := `
		SELECT id, name, display_name, rules, created_at, updated_at
		FROM games
		WHERE name = $1
	`

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&game.ID,
		&game.Name,
		&game.DisplayName,
		&game.Rules,
		&game.CreatedAt,
		&game.UpdatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("game not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get game by name")
	}

	return &game, nil
}

// List получает список всех игр
func (r *GameRepository) List(ctx context.Context, filter domain.GameFilter) ([]*domain.Game, error) {
	query := `
		SELECT id, name, display_name, rules, created_at, updated_at
		FROM games
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	// Фильтр по имени (partial match)
	if filter.Name != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR display_name ILIKE $%d)", argCount, argCount)
		args = append(args, "%"+filter.Name+"%")
		argCount++
	}

	// Сортировка
	query += " ORDER BY display_name ASC"

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
		return nil, errors.Wrap(err, "failed to list games")
	}
	defer rows.Close()

	var games []*domain.Game
	for rows.Next() {
		var game domain.Game

		err := rows.Scan(
			&game.ID,
			&game.Name,
			&game.DisplayName,
			&game.Rules,
			&game.CreatedAt,
			&game.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan game")
		}

		games = append(games, &game)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return games, nil
}

// Update обновляет игру
func (r *GameRepository) Update(ctx context.Context, game *domain.Game) error {
	query := `
		UPDATE games
		SET display_name = $2, rules = $3
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		game.ID,
		game.DisplayName,
		game.Rules,
	).Scan(&game.UpdatedAt)

	if stderrors.Is(err, sql.ErrNoRows) {
		return errors.ErrNotFound.WithMessage("game not found")
	}
	if err != nil {
		return errors.Wrap(err, "failed to update game")
	}

	return nil
}

// Delete удаляет игру
func (r *GameRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM games WHERE id = $1`

	result, err := r.db.ExecWithMetrics(ctx, "game_delete", query, id)
	if err != nil {
		return errors.Wrap(err, "failed to delete game")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("game not found")
	}

	return nil
}

// GetByTournamentID получает игры, связанные с турниром
func (r *GameRepository) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Game, error) {
	query := `
		SELECT g.id, g.name, g.display_name, g.rules, g.created_at, g.updated_at
		FROM games g
		INNER JOIN tournament_games tg ON g.id = tg.game_id
		WHERE tg.tournament_id = $1
		ORDER BY g.display_name ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get games by tournament id")
	}
	defer rows.Close()

	var games []*domain.Game
	for rows.Next() {
		var game domain.Game

		err := rows.Scan(
			&game.ID,
			&game.Name,
			&game.DisplayName,
			&game.Rules,
			&game.CreatedAt,
			&game.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan game")
		}

		games = append(games, &game)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return games, nil
}

// AddToTournament добавляет игру к турниру
func (r *GameRepository) AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	query := `
		INSERT INTO tournament_games (tournament_id, game_id)
		VALUES ($1, $2)
		ON CONFLICT (tournament_id, game_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, tournamentID, gameID)
	if err != nil {
		return errors.Wrap(err, "failed to add game to tournament")
	}

	return nil
}

// RemoveFromTournament удаляет игру из турнира
func (r *GameRepository) RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	query := `DELETE FROM tournament_games WHERE tournament_id = $1 AND game_id = $2`

	result, err := r.db.ExecContext(ctx, query, tournamentID, gameID)
	if err != nil {
		return errors.Wrap(err, "failed to remove game from tournament")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("game not in tournament")
	}

	return nil
}

// Exists проверяет существует ли игра с данным именем
func (r *GameRepository) Exists(ctx context.Context, name string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM games WHERE name = $1)`

	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to check game existence")
	}

	return exists, nil
}

// GetTournamentGame получает связь турнира с игрой
func (r *GameRepository) GetTournamentGame(ctx context.Context, tournamentID, gameID uuid.UUID) (*domain.TournamentGame, error) {
	var tg domain.TournamentGame

	query := `
		SELECT tournament_id, game_id, COALESCE(is_active, false), COALESCE(round_completed, false), round_completed_at, COALESCE(current_round, 0), created_at
		FROM tournament_games
		WHERE tournament_id = $1 AND game_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID, gameID).Scan(
		&tg.TournamentID,
		&tg.GameID,
		&tg.IsActive,
		&tg.RoundCompleted,
		&tg.RoundCompletedAt,
		&tg.CurrentRound,
		&tg.CreatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("tournament game not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament game")
	}

	return &tg, nil
}

// GetTournamentGames получает все связи турнира с играми
func (r *GameRepository) GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGame, error) {
	query := `
		SELECT tg.tournament_id, tg.game_id, COALESCE(tg.is_active, false), COALESCE(tg.round_completed, false), tg.round_completed_at, COALESCE(tg.current_round, 0), tg.created_at
		FROM tournament_games tg
		WHERE tg.tournament_id = $1
		ORDER BY tg.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament games")
	}
	defer rows.Close()

	var tgs []*domain.TournamentGame
	for rows.Next() {
		var tg domain.TournamentGame

		err := rows.Scan(
			&tg.TournamentID,
			&tg.GameID,
			&tg.IsActive,
			&tg.RoundCompleted,
			&tg.RoundCompletedAt,
			&tg.CurrentRound,
			&tg.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan tournament game")
		}

		tgs = append(tgs, &tg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return tgs, nil
}

// GetTournamentGamesWithDetails получает все связи турнира с играми, включая данные об играх
// Выполняет один JOIN запрос вместо N+1 отдельных запросов
func (r *GameRepository) GetTournamentGamesWithDetails(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGameWithDetails, error) {
	query := `
		SELECT tg.tournament_id, tg.game_id,
		       g.name AS game_name, g.display_name AS game_display_name,
		       COALESCE(tg.is_active, false) AS is_active,
		       COALESCE(tg.round_completed, false) AS round_completed,
		       tg.round_completed_at,
		       COALESCE(tg.current_round, 0) AS current_round
		FROM tournament_games tg
		INNER JOIN games g ON g.id = tg.game_id
		WHERE tg.tournament_id = $1
		ORDER BY tg.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament games with details")
	}
	defer rows.Close()

	var results []*domain.TournamentGameWithDetails
	for rows.Next() {
		var d domain.TournamentGameWithDetails
		err := rows.Scan(
			&d.TournamentID,
			&d.GameID,
			&d.GameName,
			&d.GameDisplayName,
			&d.IsActive,
			&d.RoundCompleted,
			&d.RoundCompletedAt,
			&d.CurrentRound,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan tournament game with details")
		}
		results = append(results, &d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return results, nil
}

// MarkRoundCompleted отмечает раунд игры как завершённый
func (r *GameRepository) MarkRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	query := `
		UPDATE tournament_games
		SET round_completed = true, round_completed_at = NOW()
		WHERE tournament_id = $1 AND game_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, tournamentID, gameID)
	if err != nil {
		return errors.Wrap(err, "failed to mark round completed")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("tournament game not found")
	}

	return nil
}

// IsRoundCompleted проверяет, завершён ли раунд для игры в турнире
func (r *GameRepository) IsRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error) {
	var completed bool
	query := `
		SELECT COALESCE(round_completed, false)
		FROM tournament_games
		WHERE tournament_id = $1 AND game_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID, gameID).Scan(&completed)
	if stderrors.Is(err, sql.ErrNoRows) {
		// Если связи нет, считаем раунд не завершённым
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, "failed to check round completion")
	}

	return completed, nil
}

// IncrementCurrentRound увеличивает номер текущего раунда
func (r *GameRepository) IncrementCurrentRound(ctx context.Context, tournamentID, gameID uuid.UUID) (int, error) {
	var newRound int
	query := `
		UPDATE tournament_games
		SET current_round = COALESCE(current_round, 0) + 1
		WHERE tournament_id = $1 AND game_id = $2
		RETURNING current_round
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID, gameID).Scan(&newRound)
	if err != nil {
		return 0, errors.Wrap(err, "failed to increment current round")
	}

	return newRound, nil
}

// SetActiveGame устанавливает активную игру для турнира
// Деактивирует все остальные игры и активирует указанную
func (r *GameRepository) SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	// Используем транзакцию для атомарности
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	// Деактивируем все игры турнира
	deactivateQuery := `
		UPDATE tournament_games
		SET is_active = false
		WHERE tournament_id = $1
	`
	if _, err := tx.ExecContext(ctx, deactivateQuery, tournamentID); err != nil {
		return errors.Wrap(err, "failed to deactivate games")
	}

	// Активируем указанную игру
	activateQuery := `
		UPDATE tournament_games
		SET is_active = true
		WHERE tournament_id = $1 AND game_id = $2
	`
	result, err := tx.ExecContext(ctx, activateQuery, tournamentID, gameID)
	if err != nil {
		return errors.Wrap(err, "failed to activate game")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}
	if rows == 0 {
		return errors.ErrNotFound.WithMessage("game not found in tournament")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// DeactivateAllGames деактивирует все игры в турнире
func (r *GameRepository) DeactivateAllGames(ctx context.Context, tournamentID uuid.UUID) error {
	query := `
		UPDATE tournament_games
		SET is_active = false
		WHERE tournament_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, tournamentID)
	if err != nil {
		return errors.Wrap(err, "failed to deactivate all games")
	}
	return nil
}

// GetActiveGame получает активную игру для турнира
func (r *GameRepository) GetActiveGame(ctx context.Context, tournamentID uuid.UUID) (*domain.TournamentGame, error) {
	var tg domain.TournamentGame

	query := `
		SELECT tournament_id, game_id, COALESCE(is_active, false), COALESCE(round_completed, false), round_completed_at, COALESCE(current_round, 0), created_at
		FROM tournament_games
		WHERE tournament_id = $1 AND is_active = true
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID).Scan(
		&tg.TournamentID,
		&tg.GameID,
		&tg.IsActive,
		&tg.RoundCompleted,
		&tg.RoundCompletedAt,
		&tg.CurrentRound,
		&tg.CreatedAt,
	)

	if stderrors.Is(err, sql.ErrNoRows) {
		return nil, errors.ErrNotFound.WithMessage("no active game found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active game")
	}

	return &tg, nil
}

// IsGameActive проверяет, является ли игра активной
func (r *GameRepository) IsGameActive(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error) {
	var isActive bool
	query := `
		SELECT COALESCE(is_active, false)
		FROM tournament_games
		WHERE tournament_id = $1 AND game_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, tournamentID, gameID).Scan(&isActive)
	if stderrors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, "failed to check game active status")
	}

	return isActive, nil
}

// ResetGameRound сбрасывает номер раунда и статус завершения для игры в турнире
func (r *GameRepository) ResetGameRound(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	query := `
		UPDATE tournament_games
		SET current_round = 0, round_completed = false, round_completed_at = NULL
		WHERE tournament_id = $1 AND game_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, tournamentID, gameID)
	if err != nil {
		return errors.Wrap(err, "failed to reset game round")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rows == 0 {
		return errors.ErrNotFound.WithMessage("tournament game not found")
	}

	return nil
}

// ResetGameRoundFull выполняет полный сброс раунда игры в одной транзакции:
// удаляет историю рейтингов, удаляет матчи, сбрасывает участников и статус раунда.
func (r *GameRepository) ResetGameRoundFull(ctx context.Context, tournamentID, gameID uuid.UUID, gameType string) (matchesDeleted, participantsReset, ratingHistoryDeleted int64, err error) {
	err = r.db.RunInTx(ctx, func(tx *sqlx.Tx) error {
		// 0. Проверяем наличие выполняющихся матчей (running)
		var runningCount int
		if txErr := tx.GetContext(ctx, &runningCount, `
			SELECT COUNT(*) FROM matches
			WHERE tournament_id = $1 AND game_type = $2 AND status = 'running'
		`, tournamentID, gameType); txErr != nil {
			return errors.Wrap(txErr, "failed to check running matches")
		}
		if runningCount > 0 {
			return errors.ErrValidation.WithMessage("cannot reset: there are matches currently running")
		}

		// 1. Удаляем историю рейтингов для этой игры (через связь с matches)
		result, txErr := tx.ExecContext(ctx, `
			DELETE FROM rating_history rh
			WHERE rh.tournament_id = $1
			AND rh.match_id IN (
				SELECT id FROM matches WHERE tournament_id = $1 AND game_type = $2
			)
		`, tournamentID, gameType)
		if txErr != nil {
			return errors.Wrap(txErr, "failed to delete rating history")
		}
		ratingHistoryDeleted, _ = result.RowsAffected()

		// 2. Удаляем матчи
		result, txErr = tx.ExecContext(ctx, `
			DELETE FROM matches
			WHERE tournament_id = $1 AND game_type = $2
		`, tournamentID, gameType)
		if txErr != nil {
			return errors.Wrap(txErr, "failed to delete matches")
		}
		matchesDeleted, _ = result.RowsAffected()

		// 3. Сбрасываем рейтинги участников (через связь с programs)
		result, txErr = tx.ExecContext(ctx, `
			UPDATE tournament_participants tp
			SET rating = 1000, wins = 0, losses = 0, draws = 0
			FROM programs p
			WHERE tp.program_id = p.id
			AND tp.tournament_id = $1
			AND p.game_id = $2
		`, tournamentID, gameID)
		if txErr != nil {
			return errors.Wrap(txErr, "failed to reset participants")
		}
		participantsReset, _ = result.RowsAffected()

		// 4. Сбрасываем номер раунда
		result, txErr = tx.ExecContext(ctx, `
			UPDATE tournament_games
			SET current_round = 0, round_completed = false, round_completed_at = NULL
			WHERE tournament_id = $1 AND game_id = $2
		`, tournamentID, gameID)
		if txErr != nil {
			return errors.Wrap(txErr, "failed to reset game round")
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return errors.ErrNotFound.WithMessage("tournament game not found")
		}

		return nil
	})
	return
}
