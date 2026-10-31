SET LOCAL lock_timeout = '5s';

ALTER TABLE tournaments DROP COLUMN IF EXISTS team_names_locked;
