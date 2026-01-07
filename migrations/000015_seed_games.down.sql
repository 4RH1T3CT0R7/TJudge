-- Remove seeded games
DELETE FROM games WHERE name IN ('dilemma', 'tug_of_war');
