-- Remove seeded games
DELETE FROM games WHERE name IN ('prisoners_dilemma', 'tug_of_war', 'good_deal', 'balance_of_universe');
