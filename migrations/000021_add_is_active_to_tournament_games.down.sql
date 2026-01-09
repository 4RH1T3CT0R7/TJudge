-- Remove is_active column from tournament_games table
DROP INDEX IF EXISTS idx_tournament_games_single_active;
ALTER TABLE tournament_games DROP COLUMN IF EXISTS is_active;
