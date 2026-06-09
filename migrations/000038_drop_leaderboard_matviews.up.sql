-- Удаление материализованных представлений лидербордов.
--
-- Причина: GetLeaderboard всегда использовал «живой» CTE-запрос ради
-- актуальности данных (см. internal/infrastructure/db/tournament_leaderboard.go),
-- а matview никем не читались - при этом LeaderboardRefresher обновлял их
-- каждые 30 секунд, впустую нагружая БД. Система платила и за дорогой
-- запрос, и за бесполезный refresh.
--
-- Живой запрос переписан с OR-join на UNION ALL (использует составной
-- индекс idx_matches_tournament_game_status из 000037), refresher удалён.

DROP MATERIALIZED VIEW IF EXISTS leaderboard_tournament;
DROP MATERIALIZED VIEW IF EXISTS leaderboard_global;
