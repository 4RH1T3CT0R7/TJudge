-- Remove performance indexes

-- Users indexes
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_created_at;

-- Programs indexes
DROP INDEX IF EXISTS idx_programs_user_id;
DROP INDEX IF EXISTS idx_programs_game_type;
DROP INDEX IF EXISTS idx_programs_user_game;
DROP INDEX IF EXISTS idx_programs_created_at;

-- Tournaments indexes
DROP INDEX IF EXISTS idx_tournaments_status;
DROP INDEX IF EXISTS idx_tournaments_game_type;
DROP INDEX IF EXISTS idx_tournaments_status_game;
DROP INDEX IF EXISTS idx_tournaments_start_time;
DROP INDEX IF EXISTS idx_tournaments_created_at;

-- Tournament participants indexes
DROP INDEX IF EXISTS idx_tournament_participants_tournament;
DROP INDEX IF EXISTS idx_tournament_participants_program;
DROP INDEX IF EXISTS idx_tournament_participants_rating;

-- Matches indexes
DROP INDEX IF EXISTS idx_matches_program1;
DROP INDEX IF EXISTS idx_matches_program2;
DROP INDEX IF EXISTS idx_matches_programs;
DROP INDEX IF EXISTS idx_matches_status_priority;
DROP INDEX IF EXISTS idx_matches_completed_at;
DROP INDEX IF EXISTS idx_matches_tournament_status;
DROP INDEX IF EXISTS idx_matches_tournament_created;

-- Rating history indexes
DROP INDEX IF EXISTS idx_rating_history_program;
DROP INDEX IF EXISTS idx_rating_history_tournament;
DROP INDEX IF EXISTS idx_rating_history_program_date;
DROP INDEX IF EXISTS idx_rating_history_tournament_date;
