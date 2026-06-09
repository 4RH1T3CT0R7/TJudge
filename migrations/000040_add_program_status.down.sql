DROP INDEX IF EXISTS idx_programs_status;
ALTER TABLE programs DROP CONSTRAINT IF EXISTS programs_status_check;
ALTER TABLE programs DROP COLUMN IF EXISTS status;
