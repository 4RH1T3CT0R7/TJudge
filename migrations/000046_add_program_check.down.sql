ALTER TABLE programs DROP CONSTRAINT IF EXISTS programs_check_status_check;
ALTER TABLE programs DROP COLUMN IF EXISTS checked_at;
ALTER TABLE programs DROP COLUMN IF EXISTS check_message;
ALTER TABLE programs DROP COLUMN IF EXISTS check_status;
