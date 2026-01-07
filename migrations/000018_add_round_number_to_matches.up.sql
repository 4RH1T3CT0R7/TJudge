-- Add round_number to matches table for grouping matches by round
ALTER TABLE matches ADD COLUMN IF NOT EXISTS round_number INTEGER DEFAULT 1;

-- Create index for efficient grouping by round
CREATE INDEX IF NOT EXISTS idx_matches_round_number ON matches(tournament_id, round_number);
