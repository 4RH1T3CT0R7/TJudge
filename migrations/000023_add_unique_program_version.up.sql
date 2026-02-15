-- Ensure only one version per team+game combination to prevent race conditions
-- on concurrent uploads. Duplicates must be resolved before applying this migration.
CREATE UNIQUE INDEX IF NOT EXISTS idx_programs_team_game_version
    ON programs (team_id, game_id, version)
    WHERE team_id IS NOT NULL AND game_id IS NOT NULL;
