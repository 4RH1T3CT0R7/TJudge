-- Add auto-round mode columns to tournament_games
ALTER TABLE tournament_games
  ADD COLUMN IF NOT EXISTS auto_round_enabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS auto_round_interval_seconds INT NOT NULL DEFAULT 60,
  ADD COLUMN IF NOT EXISTS auto_round_last_run_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_tg_auto_round
  ON tournament_games(tournament_id) WHERE auto_round_enabled = true;
