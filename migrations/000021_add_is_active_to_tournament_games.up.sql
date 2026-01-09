-- Add is_active column to tournament_games table
-- Only one game can be active at a time per tournament
ALTER TABLE tournament_games ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT false;

-- Create partial unique index to ensure only one active game per tournament
CREATE UNIQUE INDEX IF NOT EXISTS idx_tournament_games_single_active
ON tournament_games (tournament_id)
WHERE is_active = true;
