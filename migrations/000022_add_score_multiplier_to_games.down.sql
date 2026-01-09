-- Revert prisoners_dilemma name change
UPDATE games SET name = 'prisoners_dilemma' WHERE name = 'dilemma';

-- Remove score_multiplier from games table
ALTER TABLE games DROP COLUMN IF EXISTS score_multiplier;
