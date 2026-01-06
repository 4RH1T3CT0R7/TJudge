-- Add new fields to programs table for team support
ALTER TABLE programs ADD COLUMN IF NOT EXISTS team_id UUID REFERENCES teams(id) ON DELETE SET NULL;
ALTER TABLE programs ADD COLUMN IF NOT EXISTS tournament_id UUID REFERENCES tournaments(id) ON DELETE CASCADE;
ALTER TABLE programs ADD COLUMN IF NOT EXISTS game_id UUID REFERENCES games(id);
ALTER TABLE programs ADD COLUMN IF NOT EXISTS file_path TEXT;
ALTER TABLE programs ADD COLUMN IF NOT EXISTS error_message TEXT;
ALTER TABLE programs ADD COLUMN IF NOT EXISTS version INT DEFAULT 1;

-- Create indexes
CREATE INDEX idx_programs_team ON programs(team_id);
CREATE INDEX idx_programs_tournament ON programs(tournament_id);
CREATE INDEX idx_programs_game ON programs(game_id);
CREATE INDEX idx_programs_tournament_game ON programs(tournament_id, game_id);
CREATE INDEX idx_programs_team_game ON programs(team_id, game_id);

-- Add constraint for version uniqueness per team+game
CREATE UNIQUE INDEX idx_programs_team_game_version ON programs(team_id, game_id, version)
    WHERE team_id IS NOT NULL AND game_id IS NOT NULL;
