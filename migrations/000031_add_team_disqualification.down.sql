DROP INDEX IF EXISTS idx_teams_disqualified;

ALTER TABLE teams DROP COLUMN IF EXISTS disqualified_at;
ALTER TABLE teams DROP COLUMN IF EXISTS is_disqualified;

ALTER TABLE matches DROP CONSTRAINT IF EXISTS valid_status;
ALTER TABLE matches ADD CONSTRAINT valid_status CHECK (status IN ('pending', 'running', 'completed', 'failed'));
