package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
)

// GetLeaderboard получает таблицу лидеров турнира «живым» запросом.
// Рейтинг = сумма всех очков из всех матчей.
//
// Матчи разворачиваются в «стороны» через UNION ALL вместо
// JOIN ... ON (program1_id = p.id OR program2_id = p.id): OR-join ломал
// index scan и сканировал партиции matches целиком, а UNION ALL позволяет
// каждой ветке использовать индекс по (tournament_id, game_type, status).
func (r *TournamentRepository) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	query := `
		WITH match_sides AS (
			SELECT m.program1_id AS program_id,
			       m.winner = 1 AS won,
			       m.winner = 2 AS lost,
			       m.winner = 0 AS draw,
			       COALESCE(m.score1, 0) AS score
			FROM matches m
			WHERE m.tournament_id = $1 AND m.status = 'completed'
			UNION ALL
			SELECT m.program2_id,
			       m.winner = 2,
			       m.winner = 1,
			       m.winner = 0,
			       COALESCE(m.score2, 0)
			FROM matches m
			WHERE m.tournament_id = $1 AND m.status = 'completed'
		),
		side_stats AS (
			SELECT program_id,
			       COUNT(*) FILTER (WHERE won) as wins,
			       COUNT(*) FILTER (WHERE lost) as losses,
			       COUNT(*) FILTER (WHERE draw) as draws,
			       COUNT(*) as total_games,
			       SUM(score) as total_score
			FROM match_sides
			GROUP BY program_id
		),
		program_stats AS (
			SELECT
				p.id as program_id,
				p.name as program_name,
				t.id as team_id,
				t.name as team_name,
				COALESCE(ss.wins, 0) as wins,
				COALESCE(ss.losses, 0) as losses,
				COALESCE(ss.draws, 0) as draws,
				COALESCE(ss.total_games, 0) as total_games,
				COALESCE(ss.total_score, 0) as total_score,
				-- Tiebreak: MIN across games of latest version's created_at per team
				COALESCE(
					(SELECT MIN(sub_p.created_at)
					 FROM (
						 SELECT DISTINCT ON (p2.game_id) p2.created_at
						 FROM programs p2
						 WHERE p2.team_id = p.team_id
						   AND p2.tournament_id = $1
						   AND p2.team_id IS NOT NULL
						 ORDER BY p2.game_id, p2.version DESC
					 ) sub_p
					),
					p.created_at
				) as earliest_upload
			FROM tournament_participants tp
			JOIN programs p ON tp.program_id = p.id
			INNER JOIN teams t ON p.team_id = t.id AND t.is_disqualified = false
			LEFT JOIN side_stats ss ON ss.program_id = p.id
			WHERE tp.tournament_id = $1
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

	var leaderboard []*domain.LeaderboardEntry

	err := r.db.QueryWithMetrics(ctx, "tournament_leaderboard", &leaderboard, query, tournamentID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament leaderboard")
	}

	return leaderboard, nil
}

// GetCrossGameLeaderboard получает кросс-игровой рейтинг турнира
// Рейтинг = сумма всех очков из всех матчей
func (r *TournamentRepository) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error) {
	// Получаем все команды и программы в турнире со статистикой по каждой игре
	// Используем team_id для связи матчей (чтобы учитывать все версии программ команды)
	query := `
		WITH latest_programs AS (
			-- Получаем последние версии программ для отображения имени
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
			LEFT JOIN games g ON p.game_id = g.id
			WHERE p.tournament_id = $1 AND p.team_id IS NOT NULL
			ORDER BY p.team_id, p.game_id, p.version DESC
		),
		match_sides AS (
			-- Матч разворачивается в две «стороны» через UNION ALL вместо
			-- OR-join: каждая ветка использует индекс по tournament_id.
			SELECT m.program1_id AS program_id, m.game_type, m.status,
			       m.winner = 1 AS won, m.winner = 2 AS lost, m.winner = 0 AS draw,
			       COALESCE(m.score1, 0) AS score
			FROM matches m
			WHERE m.tournament_id = $1 AND m.status IN ('completed', 'failed')
			UNION ALL
			SELECT m.program2_id, m.game_type, m.status,
			       m.winner = 2, m.winner = 1, m.winner = 0,
			       COALESCE(m.score2, 0)
			FROM matches m
			WHERE m.tournament_id = $1 AND m.status IN ('completed', 'failed')
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
				COUNT(*) FILTER (WHERE s.draw AND s.status = 'completed') as draws,
				COUNT(*) FILTER (WHERE s.status = 'completed') as total_games,
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

	var entries []*domain.CrossGameLeaderboardEntry
	for rows.Next() {
		var entry domain.CrossGameLeaderboardEntry
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

		// Parse game ratings JSON
		entry.GameRatings = make(map[string]domain.GameRatingInfo)
		if gameRatingsJSON != nil {
			var rawRatings map[string]domain.GameRatingInfo
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

// GetLeaderboardByGameType получает таблицу лидеров для конкретной игры в турнире
// gameType - имя игры (game.name), используется для фильтрации матчей
// Рейтинг = сумма всех очков из всех матчей
func (r *TournamentRepository) GetLeaderboardByGameType(ctx context.Context, tournamentID uuid.UUID, gameType string, limit int) ([]*domain.LeaderboardEntry, error) {
	// Получаем рейтинг на основе результатов матчей для конкретной игры
	// Используем team_id для агрегации (чтобы учитывать все версии программ команды)
	query := `
		WITH latest_programs AS (
			-- Получаем последние версии программ для отображения имени
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
			SELECT m.program1_id AS program_id, m.status,
			       m.winner = 1 AS won, m.winner = 2 AS lost, m.winner = 0 AS draw,
			       COALESCE(m.score1, 0) AS score
			FROM matches m
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND m.status IN ('completed', 'failed')
			UNION ALL
			SELECT m.program2_id, m.status,
			       m.winner = 2, m.winner = 1, m.winner = 0,
			       COALESCE(m.score2, 0)
			FROM matches m
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND m.status IN ('completed', 'failed')
		),
		match_stats AS (
			-- Статистика матчей по team_id (учитывая все версии программ)
			SELECT
				p.team_id,
				COUNT(*) FILTER (WHERE s.won) as wins,
				COUNT(*) FILTER (WHERE s.lost) as losses,
				COUNT(*) FILTER (WHERE s.draw AND s.status = 'completed') as draws,
				COUNT(*) FILTER (WHERE s.status = 'completed') as total_games,
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

	var leaderboard []*domain.LeaderboardEntry
	for rows.Next() {
		var entry domain.LeaderboardEntry
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

// GetHeadToHead возвращает агрегат личных встреч всех пар команд в игре
// турнира. Обе ориентации матча (AB и BA) сливаются UNION ALL'ом: каждая
// завершённая встреча даёт две перспективы, дальше GROUP BY по паре команд.
// Дисквалифицированные команды исключаются, как и в лидербордах.
func (r *TournamentRepository) GetHeadToHead(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.HeadToHeadCell, error) {
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
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND m.status = 'completed'
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
			WHERE m.tournament_id = $1 AND m.game_type = $2 AND m.status = 'completed'
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

	var cells []*domain.HeadToHeadCell
	if err := r.db.QueryWithMetrics(ctx, "leaderboard_head_to_head", &cells, query, tournamentID, gameType); err != nil {
		return nil, errors.Wrap(err, "failed to get head-to-head")
	}
	return cells, nil
}
