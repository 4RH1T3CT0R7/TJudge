-- Add disqualification fields to teams
ALTER TABLE teams ADD COLUMN is_disqualified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE teams ADD COLUMN disqualified_at TIMESTAMP;

-- Allow 'cancelled' status for matches cancelled by disqualification
ALTER TABLE matches DROP CONSTRAINT IF EXISTS valid_status;
ALTER TABLE matches ADD CONSTRAINT valid_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled'));

-- Index for fast disqualification lookups
CREATE INDEX idx_teams_disqualified ON teams(tournament_id) WHERE is_disqualified = true;
