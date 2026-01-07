-- Remove round_number from matches table
DROP INDEX IF EXISTS idx_matches_round_number;
ALTER TABLE matches DROP COLUMN IF EXISTS round_number;
