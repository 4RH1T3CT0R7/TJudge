-- Fix materialized views to support CONCURRENT refresh
-- CONCURRENTLY requires at least one UNIQUE index

-- Add unique index on program_id for global leaderboard
CREATE UNIQUE INDEX IF NOT EXISTS idx_leaderboard_global_program_unique
    ON leaderboard_global(program_id);

-- Add unique index on (tournament_id, program_id) for tournament leaderboard
CREATE UNIQUE INDEX IF NOT EXISTS idx_leaderboard_tournament_unique
    ON leaderboard_tournament(tournament_id, program_id);
