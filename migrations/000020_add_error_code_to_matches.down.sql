-- Remove error_code column from matches table
DROP INDEX IF EXISTS idx_matches_error_code;
ALTER TABLE matches DROP COLUMN IF EXISTS error_code;
