-- Index for efficient tiebreak computation: latest version per team+game+tournament
CREATE INDEX IF NOT EXISTS idx_programs_team_tournament_game_version_desc
    ON programs(team_id, tournament_id, game_id, version DESC, created_at)
    WHERE team_id IS NOT NULL AND game_id IS NOT NULL;
