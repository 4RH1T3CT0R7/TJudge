DROP TRIGGER IF EXISTS update_tournaments_updated_at ON tournaments;
DROP INDEX IF EXISTS idx_tournaments_status_game;
DROP INDEX IF EXISTS idx_tournaments_start_time;
DROP INDEX IF EXISTS idx_tournaments_game_type;
DROP INDEX IF EXISTS idx_tournaments_status;
DROP TABLE IF EXISTS tournaments;
