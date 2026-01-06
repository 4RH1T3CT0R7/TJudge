-- Create programs table
CREATE TABLE IF NOT EXISTS programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    game_type VARCHAR(50) NOT NULL,
    code_path TEXT NOT NULL,
    language VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_programs_user_id ON programs(user_id);
CREATE INDEX idx_programs_game_type ON programs(game_type);
CREATE INDEX idx_programs_user_game ON programs(user_id, game_type);

-- Create trigger
CREATE TRIGGER update_programs_updated_at
    BEFORE UPDATE ON programs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
