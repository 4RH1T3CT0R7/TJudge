-- Add new fields to tournaments table
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS code VARCHAR(8) UNIQUE;
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS max_team_size INT DEFAULT 1;
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS is_permanent BOOLEAN DEFAULT FALSE;
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS creator_id UUID REFERENCES users(id);

-- Create tournament_games junction table (many-to-many)
CREATE TABLE IF NOT EXISTS tournament_games (
    tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tournament_id, game_id)
);

-- Create indexes
CREATE INDEX idx_tournament_games_tournament ON tournament_games(tournament_id);
CREATE INDEX idx_tournament_games_game ON tournament_games(game_id);
CREATE INDEX idx_tournaments_code ON tournaments(code);
CREATE INDEX idx_tournaments_creator ON tournaments(creator_id);

-- Generate codes for existing tournaments
UPDATE tournaments
SET code = generate_unique_code(6)
WHERE code IS NULL;

-- Make code NOT NULL after populating
ALTER TABLE tournaments ALTER COLUMN code SET NOT NULL;
