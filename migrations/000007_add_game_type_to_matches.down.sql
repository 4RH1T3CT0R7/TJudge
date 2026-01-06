-- Drop game_type column from matches table
DROP INDEX IF EXISTS idx_matches_game_type;
ALTER TABLE matches DROP COLUMN game_type;
