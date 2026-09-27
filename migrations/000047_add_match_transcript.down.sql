SET LOCAL lock_timeout = '5s';

ALTER TABLE matches DROP COLUMN IF EXISTS transcript;
