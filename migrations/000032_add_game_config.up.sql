ALTER TABLE tournament_games ADD COLUMN IF NOT EXISTS config JSONB DEFAULT '{}';
COMMENT ON COLUMN tournament_games.config IS 'Game-specific configuration (iterations, score_multiplier, custom params)';
