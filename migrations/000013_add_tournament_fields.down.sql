-- Remove tournament_games table
DROP TABLE IF EXISTS tournament_games;

-- Remove indexes
DROP INDEX IF EXISTS idx_tournaments_code;
DROP INDEX IF EXISTS idx_tournaments_creator;

-- Remove new columns from tournaments
ALTER TABLE tournaments DROP COLUMN IF EXISTS code;
ALTER TABLE tournaments DROP COLUMN IF EXISTS description;
ALTER TABLE tournaments DROP COLUMN IF EXISTS max_team_size;
ALTER TABLE tournaments DROP COLUMN IF EXISTS is_permanent;
ALTER TABLE tournaments DROP COLUMN IF EXISTS creator_id;
