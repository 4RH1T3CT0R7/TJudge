-- Add error_code column to matches table
ALTER TABLE matches ADD COLUMN IF NOT EXISTS error_code INT;

-- Create index for filtering failed matches by error code
CREATE INDEX IF NOT EXISTS idx_matches_error_code ON matches(error_code) WHERE error_code IS NOT NULL;
