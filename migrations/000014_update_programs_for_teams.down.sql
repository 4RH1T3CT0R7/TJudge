-- Remove indexes
DROP INDEX IF EXISTS idx_programs_team_game_version;
DROP INDEX IF EXISTS idx_programs_team_game;
DROP INDEX IF EXISTS idx_programs_tournament_game;
DROP INDEX IF EXISTS idx_programs_game;
DROP INDEX IF EXISTS idx_programs_tournament;
DROP INDEX IF EXISTS idx_programs_team;

-- Remove new columns from programs
ALTER TABLE programs DROP COLUMN IF EXISTS team_id;
ALTER TABLE programs DROP COLUMN IF EXISTS tournament_id;
ALTER TABLE programs DROP COLUMN IF EXISTS game_id;
ALTER TABLE programs DROP COLUMN IF EXISTS file_path;
ALTER TABLE programs DROP COLUMN IF EXISTS error_message;
ALTER TABLE programs DROP COLUMN IF EXISTS version;
