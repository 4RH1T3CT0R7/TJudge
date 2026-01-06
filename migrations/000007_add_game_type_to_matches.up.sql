-- Add game_type column to matches table
ALTER TABLE matches ADD COLUMN game_type VARCHAR(50) NOT NULL DEFAULT 'tictactoe';

-- Add index on game_type for filtering
CREATE INDEX idx_matches_game_type ON matches(game_type);
