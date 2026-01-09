-- Remove round status from tournament_games
DROP INDEX IF EXISTS idx_tournament_games_round_status;
ALTER TABLE tournament_games DROP COLUMN IF EXISTS round_completed;
ALTER TABLE tournament_games DROP COLUMN IF EXISTS round_completed_at;
ALTER TABLE tournament_games DROP COLUMN IF EXISTS current_round;
