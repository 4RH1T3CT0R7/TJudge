DROP TRIGGER IF EXISTS update_programs_updated_at ON programs;
DROP INDEX IF EXISTS idx_programs_user_game;
DROP INDEX IF EXISTS idx_programs_game_type;
DROP INDEX IF EXISTS idx_programs_user_id;
DROP TABLE IF EXISTS programs;
