-- Drop function
DROP FUNCTION IF EXISTS refresh_leaderboards();

-- Drop materialized views
DROP MATERIALIZED VIEW IF EXISTS leaderboard_tournament;
DROP MATERIALIZED VIEW IF EXISTS leaderboard_global;
