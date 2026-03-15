-- Remove 3 new games: travelers_dilemma, public_goods, dollar_auction
DELETE FROM games WHERE name IN ('travelers_dilemma', 'public_goods', 'dollar_auction');
