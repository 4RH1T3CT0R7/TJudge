-- Add score_multiplier to games table to balance scores across different games
-- Default 1.0 means no change, higher values increase the impact of that game

ALTER TABLE games ADD COLUMN IF NOT EXISTS score_multiplier DECIMAL(10,2) DEFAULT 1.0;

-- Rename prisoners_dilemma to dilemma to match tjudge-cli game type
-- This ensures proper JOIN between matches.game_type and games.name
UPDATE games SET name = 'dilemma' WHERE name = 'prisoners_dilemma';

-- Update tug_of_war to have a 10x multiplier to balance with dilemma
-- This is because tug_of_war scores are typically lower (rounds won) compared to
-- dilemma (accumulated payoffs per iteration)
UPDATE games SET score_multiplier = 10.0 WHERE name = 'tug_of_war';

-- Ensure dilemma uses standard multiplier
UPDATE games SET score_multiplier = 1.0 WHERE name = 'dilemma';

-- Add comment explaining the field
COMMENT ON COLUMN games.score_multiplier IS 'Multiplier applied to scores from this game for balanced leaderboard calculation. Higher values increase the game''s contribution to total score.';
