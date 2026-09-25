package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// playedMatch - какие матчи идут в статистику лидербордов: завершённые и форфейты
// (failed с победителем - программа соперника упала). failed без победителя - это
// сбой, а не результат. условие одно на все лидерборды, чтобы победы и total_games
// сходились между ними
const playedMatch = `(m.status = 'completed' OR (m.status = 'failed' AND m.winner IN (1, 2)))`

// GetLeaderboard - живой лидерборд турнира: строка на последнюю версию программы
// команды в каждой игре турнира, статистика - по всем версиям команды в этой игре
// (как в лидерборде игры). рейтинг это сумма очков.
// матчи разворачиваются в стороны через UNION ALL, а не JOIN с OR по program1_id/program2_id:
// OR-join ломал index scan и читал партиции matches целиком
func (r *TournamentRepository) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error) {
	query := `
		WITH latest_programs AS (
			SELECT DISTINCT ON (p.team_id, p.game_id)
				p.id AS program_id,
				p.name AS program_name,
				p.team_id,
				t.name AS team_name,
				g.name AS game_type,
				p.created_at
			FROM programs p
			INNER JOIN teams t ON t.id = p.team_id AND t.is_disqualified = false
			INNER JOIN games g ON g.id = p.game_id
			INNER JOIN tournament_games tg ON tg.tournament_id = p.tournament_id AND tg.game_id = p.game_id
			WHERE p.tournament_id = $1
			ORDER BY p.team_id, p.game_id, p.version DESC
		),
		match_sides AS (
			SELECT m.program1_id AS program_id, m.game_type,
			       m.winner = 1 AS won, m.winner = 2 AS lost, m.winner = 0 AS draw,
			       COALESCE(m.score1, 0) AS score
			FROM matches m
			WHERE m.tournament_id = $1 AND ` + playedMatch + `
			UNION ALL
			SELECT m.program2_id, m.game_type,
			       m.winner = 2, m.winner = 1, m.winner = 0,
			       COALESCE(m.score2, 0)
			FROM matches m
			WHERE m.tournament_id = $1 AND ` + playedMatch + `
		),
		team_stats AS (
			SELECT p.team_id, s.game_type,
			       COUNT(*) FILTER (WHERE s.won) AS wins,
			       COUNT(*) FILTER (WHERE s.lost) AS losses,
			       COUNT(*) FILTER (WHERE s.draw) AS draws,
			       COUNT(*) AS total_games,
			       SUM(s.score) AS total_score
			FROM match_sides s
			JOIN programs p ON p.id = s.program_id AND p.team_id IS NOT NULL
			GROUP BY p.team_id, s.game_type
		),
		program_stats AS (
			SELECT
				lp.program_id,
				lp.program_name,
				lp.team_id,
				lp.team_name,
				COALESCE(ts.wins, 0) AS wins,
				COALESCE(ts.losses, 0) AS losses,
				COALESCE(ts.draws, 0) AS draws,
				COALESCE(ts.total_games, 0) AS total_games,
				COALESCE(ts.total_score, 0) AS total_score,
				-- тай-брейк: самая ранняя из последних версий команды по всем играм
				MIN(lp.created_at) OVER (PARTITION BY lp.team_id) AS earliest_upload
			FROM latest_programs lp
			LEFT JOIN team_stats ts ON ts.team_id = lp.team_id AND ts.game_type = lp.game_type
		)
		SELECT
			ROW_NUMBER() OVER (ORDER BY total_score DESC, wins DESC, earliest_upload ASC) as rank,
			program_id,
			program_name,
			team_id,
			team_name,
			total_score as rating,
			wins,
			losses,
			draws,
			total_games
		FROM program_stats
		ORDER BY total_score DESC, wins DESC, earliest_upload ASC
		LIMIT $2
	`

	var leaderboard []*models.LeaderboardEntry

	err := r.db.QueryWithMetrics(ctx, "tournament_leaderboard", &leaderboard, query, tournamentID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament leaderboard")
	}

	return leaderboard, nil
}

// GetCrossGameLeaderboard - агрегированный рейтинг по всем играм турнира.
// рейтинг команды это сумма очков из всех матчей, очки масштабируются на score_multiplier игры
// TODO: тяжёлый запрос, закэшировать бы
func (r *TournamentRepository) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error) {
	// берутся все команды и программы в турнире со статистикой по каждой игре
	// team_id используется для связи матчей (чтобы учитывать все версии программ команды)
	query := `
		WITH latest_programs AS (
			-- последние версии программ для отображения имени
			SELECT DISTINCT ON (p.team_id, p.game_id)
				p.id as program_id,
				p.name as program_name,
				p.team_id,
				t.name as team_name,
				p.game_id,
				g.name as game_name,
				p.created_at as program_created_at
			FROM programs p
			INNER JOIN teams t ON p.team_id = t.id AND t.is_disqualified = false
			INNER JOIN games g ON p.game_id = g.id
			INNER JOIN tournament_games tg ON tg.tournament_id = p.tournament_id AND tg.game_id = p.game_id
			WHERE p.tournament_id = $1 AND p.team_id IS NOT NULL
			ORDER BY p.team_id, p.game_id, p.version DESC
		),
		match_sides AS (
			-- Матч разворачивается в две «стороны» через UNION ALL вместо
			-- OR-join: каждая ветка использует индекс по tournament_id.
			SELECT m.program1_id AS program_id, m.game_type,
			       m.winner = 1 AS won, m.winner = 2 AS lost, m.winner = 0 AS draw,
			       COALESCE(m.score1, 0) AS score
			FROM matches m
			WHERE m.tournament_id = $1 AND ` + playedMatch + `
			UNION ALL
			SELECT m.program2_id, m.game_type,
			       m.winner = 2, m.winner = 1, m.winner = 0,
			       COALESCE(m.score2, 0)
			FROM matches m
			WHERE m.tournament_id = $1 AND ` + playedMatch + `
		),
		match_stats AS (
			-- Статистика матчей для каждой программы (любой версии)
			-- Очки умножаются на score_multiplier игры для балансировки между играми
			SELECT
				p.team_id,
				g.id as game_id,
				g.name as game_name,
				COUNT(*) FILTER (WHERE s.won) as wins,
				COUNT(*) FILTER (WHERE s.lost) as losses,
				COUNT(*) FILTER (WHERE s.draw) as draws,
				COUNT(*) as total_games,
				COALESCE(SUM(s.score * COALESCE(g.score_multiplier, 1.0)), 0)::bigint as total_score
			FROM match_sides s
			JOIN programs p ON p.id = s.program_id AND p.team_id IS NOT NULL
			JOIN games g ON s.game_type = g.name
			GROUP BY p.team_id, g.id, g.name
		),
		game_stats AS (
			-- Объединяем статистику матчей с последними программами
			SELECT
				COALESCE(ms.team_id, lp.team_id) as team_id,
				COALESCE(lp.team_name, '') as team_name,
				COALESCE(lp.program_id, '00000000-0000-0000-0000-000000000000'::uuid) as program_id,
				COALESCE(lp.program_name, '') as program_name,
				COALESCE(ms.game_id, lp.game_id) as game_id,
				COALESCE(ms.game_name, lp.game_name) as game_name,
				COALESCE(ms.wins, 0) as wins,
				COALESCE(ms.losses, 0) as losses,
				COALESCE(ms.draws, 0) as draws,
				COALESCE(ms.total_games, 0) as total_games,
				COALESCE(ms.total_score, 0) as total_score,
				lp.program_created_at
			FROM latest_programs lp
			LEFT JOIN match_stats ms ON lp.team_id = ms.team_id AND lp.game_id = ms.game_id
		),
		aggregated AS (
			SELECT
				team_id,
				MAX(team_name) as team_name,
				(array_agg(program_id ORDER BY program_name))[1] as program_id,
				MAX(program_name) as program_name,
				json_object_agg(
					COALESCE(game_id::text, 'unknown'),
					json_build_object(
						'game_id', game_id,
						'game_name', game_name,
						'rating', total_score,
						'wins', wins,
						'losses', losses,
						'draws', draws,
						'total_games', total_games
					)
				) as game_ratings,
				SUM(wins) as total_wins,
				SUM(losses) as total_losses,
				SUM(total_games) as total_games,
				SUM(total_score) as total_rating,
				MIN(program_created_at) as earliest_upload
			FROM game_stats
			GROUP BY team_id
		)
		SELECT
			ROW_NUMBER() OVER (ORDER BY total_rating DESC, total_wins DESC, earliest_upload ASC) as rank,
			team_id,
			team_name,
			program_id,
			program_name,
			game_ratings,
			total_rating,
			total_wins,
			total_losses,
			total_games
		FROM aggregated
		ORDER BY total_rating DESC, total_wins DESC, earliest_upload ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cross-game leaderboard")
	}
	defer rows.Close()

	var entries []*models.CrossGameLeaderboardEntry
	for rows.Next() {
		var entry models.CrossGameLeaderboardEntry
		var gameRatingsJSON []byte

		err := rows.Scan(
			&entry.Rank,
			&entry.TeamID,
			&entry.TeamName,
			&entry.ProgramID,
			&entry.ProgramName,
			&gameRatingsJSON,
			&entry.TotalRating,
			&entry.TotalWins,
			&entry.TotalLosses,
			&entry.TotalGames,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan cross-game leaderboard entry")
		}

		// разбор game_ratings из json
		entry.GameRatings = make(map[string]models.GameRatingInfo)
		if gameRatingsJSON != nil {
			var rawRatings map[string]models.GameRatingInfo
			if err := json.Unmarshal(gameRatingsJSON, &rawRatings); err == nil {
				entry.GameRatings = rawRatings
			}
		}

		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return entries, nil
}

// GetLeaderboardByGameType - лидерборд одной игры в турнире.
// gameType это имя игры (game.name), по нему фильтруются матчи
func (r *TournamentRepository) GetLeaderboardByGameType(ctx context.Context, tournamentID uuid.UUID, gameType string, limit int) ([]*models.LeaderboardEntry, error) {
	// рейтинг на основе результатов матчей для конкретной игры
	// team_id используется для агрегации (чтобы учитывать все версии программ команды)
	query := `
		WITH latest_programs AS (
			-- последние версии программ для отображения имени
			SELECT DISTINCT ON (p.team_id)
				p.id as program_id,
				p.name as program_name,
				p.team_id,
				t.name as team_name,
				p.created_at as program_created_at
			FROM programs p
			INNER JOIN teams t ON p.team_id = t.id AND t.is_disqualified = false
			JOIN games g ON p.game_id = g.id
			WHERE p.tournament_id = $1
			  AND g.name = $2
			  AND p.team_id IS NOT NULL
			ORDER BY p.team_id, p.version DESC
		),
		match_sides AS (
			-- UNION ALL вместо OR-join: обе ветки используют составной
			-- индекс (tournament_id, game_type, status) из миграции 000037.
			SELECT m.program1_id AS program_id,
			       m.winner = 1 AS won, m.winner = 2 AS lost, m.winner = 0 AS draw,
			       COALESCE(m.score1, 0) AS score
			FROM matches m
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND ` + playedMatch + `
			UNION ALL
			SELECT m.program2_id,
			       m.winner = 2, m.winner = 1, m.winner = 0,
			       COALESCE(m.score2, 0)
			FROM matches m
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND ` + playedMatch + `
		),
		match_stats AS (
			-- Статистика матчей по team_id (учитывая все версии программ)
			SELECT
				p.team_id,
				COUNT(*) FILTER (WHERE s.won) as wins,
				COUNT(*) FILTER (WHERE s.lost) as losses,
				COUNT(*) FILTER (WHERE s.draw) as draws,
				COUNT(*) as total_games,
				COALESCE(SUM(s.score), 0) as total_score
			FROM match_sides s
			JOIN programs p ON p.id = s.program_id AND p.team_id IS NOT NULL
			GROUP BY p.team_id
		),
		combined AS (
			SELECT
				lp.program_id,
				lp.program_name,
				lp.team_id,
				lp.team_name,
				COALESCE(ms.wins, 0) as wins,
				COALESCE(ms.losses, 0) as losses,
				COALESCE(ms.draws, 0) as draws,
				COALESCE(ms.total_games, 0) as total_games,
				COALESCE(ms.total_score, 0) as total_score,
				lp.program_created_at as earliest_upload
			FROM latest_programs lp
			LEFT JOIN match_stats ms ON lp.team_id = ms.team_id
		)
		SELECT
			ROW_NUMBER() OVER (ORDER BY total_score DESC, wins DESC, earliest_upload ASC) as rank,
			program_id,
			program_name,
			team_id,
			team_name,
			total_score as rating,
			wins,
			losses,
			draws,
			total_games
		FROM combined
		ORDER BY total_score DESC, wins DESC, earliest_upload ASC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID, gameType, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get leaderboard by game type")
	}
	defer rows.Close()

	var leaderboard []*models.LeaderboardEntry
	for rows.Next() {
		var entry models.LeaderboardEntry
		err := rows.Scan(
			&entry.Rank,
			&entry.ProgramID,
			&entry.ProgramName,
			&entry.TeamID,
			&entry.TeamName,
			&entry.Rating,
			&entry.Wins,
			&entry.Losses,
			&entry.Draws,
			&entry.TotalGames,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan leaderboard entry")
		}
		leaderboard = append(leaderboard, &entry)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows iteration error")
	}

	return leaderboard, nil
}

// GetHeadToHead - матрица личных встреч всех пар команд в игре турнира.
// обе ориентации матча (AB и BA) сливаются через UNION ALL: одна встреча даёт
// две перспективы, потом всё группируется по паре команд; дисквалифицированные исключаются
func (r *TournamentRepository) GetHeadToHead(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*models.HeadToHeadCell, error) {
	query := `
		WITH sides AS (
			SELECT p1.team_id AS team_id,
			       p2.team_id AS opponent_id,
			       (m.winner = 1)::int AS win,
			       (m.winner = 2)::int AS loss,
			       (m.winner = 0)::int AS draw,
			       COALESCE(m.score1, 0) AS score_for,
			       COALESCE(m.score2, 0) AS score_against
			FROM matches m
			JOIN programs p1 ON p1.id = m.program1_id
			JOIN programs p2 ON p2.id = m.program2_id
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND ` + playedMatch + `
			UNION ALL
			SELECT p2.team_id,
			       p1.team_id,
			       (m.winner = 2)::int,
			       (m.winner = 1)::int,
			       (m.winner = 0)::int,
			       COALESCE(m.score2, 0),
			       COALESCE(m.score1, 0)
			FROM matches m
			JOIN programs p1 ON p1.id = m.program1_id
			JOIN programs p2 ON p2.id = m.program2_id
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND ` + playedMatch + `
		)
		SELECT s.team_id,
		       t.name  AS team_name,
		       s.opponent_id,
		       ot.name AS opponent_name,
		       SUM(s.win)           AS wins,
		       SUM(s.loss)          AS losses,
		       SUM(s.draw)          AS draws,
		       SUM(s.score_for)     AS score_for,
		       SUM(s.score_against) AS score_against
		FROM sides s
		JOIN teams t  ON t.id  = s.team_id     AND NOT t.is_disqualified
		JOIN teams ot ON ot.id = s.opponent_id AND NOT ot.is_disqualified
		GROUP BY s.team_id, t.name, s.opponent_id, ot.name
		ORDER BY team_name, opponent_name
	`

	var cells []*models.HeadToHeadCell
	if err := r.db.QueryWithMetrics(ctx, "leaderboard_head_to_head", &cells, query, tournamentID, gameType); err != nil {
		return nil, errors.Wrap(err, "failed to get head-to-head")
	}
	return cells, nil
}
