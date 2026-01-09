-- Add round status to tournament_games to track per-game round completion
ALTER TABLE tournament_games ADD COLUMN IF NOT EXISTS round_completed BOOLEAN DEFAULT FALSE;
ALTER TABLE tournament_games ADD COLUMN IF NOT EXISTS round_completed_at TIMESTAMP;
ALTER TABLE tournament_games ADD COLUMN IF NOT EXISTS current_round INT DEFAULT 0;

-- Create index for round status queries
CREATE INDEX IF NOT EXISTS idx_tournament_games_round_status ON tournament_games(tournament_id, round_completed);
