DROP INDEX IF EXISTS idx_tg_auto_round;
ALTER TABLE tournament_games
  DROP COLUMN IF EXISTS auto_round_enabled,
  DROP COLUMN IF EXISTS auto_round_interval_seconds,
  DROP COLUMN IF EXISTS auto_round_last_run_at;
