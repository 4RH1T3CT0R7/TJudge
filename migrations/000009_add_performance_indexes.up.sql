-- Add performance indexes for frequently queried fields

-- Users indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);

-- Programs indexes
CREATE INDEX IF NOT EXISTS idx_programs_user_id ON programs(user_id);
CREATE INDEX IF NOT EXISTS idx_programs_game_type ON programs(game_type);
CREATE INDEX IF NOT EXISTS idx_programs_user_game ON programs(user_id, game_type);
CREATE INDEX IF NOT EXISTS idx_programs_created_at ON programs(created_at DESC);

-- Tournaments indexes
CREATE INDEX IF NOT EXISTS idx_tournaments_status ON tournaments(status);
CREATE INDEX IF NOT EXISTS idx_tournaments_game_type ON tournaments(game_type);
CREATE INDEX IF NOT EXISTS idx_tournaments_status_game ON tournaments(status, game_type);
CREATE INDEX IF NOT EXISTS idx_tournaments_start_time ON tournaments(start_time DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_tournaments_created_at ON tournaments(created_at DESC);

-- Tournament participants indexes
CREATE INDEX IF NOT EXISTS idx_tournament_participants_tournament ON tournament_participants(tournament_id);
CREATE INDEX IF NOT EXISTS idx_tournament_participants_program ON tournament_participants(program_id);
CREATE INDEX IF NOT EXISTS idx_tournament_participants_rating ON tournament_participants(tournament_id, rating DESC);

-- Matches indexes (в дополнение к существующим партиционированным)
CREATE INDEX IF NOT EXISTS idx_matches_program1 ON matches(program1_id);
CREATE INDEX IF NOT EXISTS idx_matches_program2 ON matches(program2_id);
CREATE INDEX IF NOT EXISTS idx_matches_programs ON matches(program1_id, program2_id);
CREATE INDEX IF NOT EXISTS idx_matches_status_priority ON matches(status, priority DESC);
CREATE INDEX IF NOT EXISTS idx_matches_completed_at ON matches(completed_at DESC NULLS LAST);

-- Composite index for common queries
CREATE INDEX IF NOT EXISTS idx_matches_tournament_status ON matches(tournament_id, status);
CREATE INDEX IF NOT EXISTS idx_matches_tournament_created ON matches(tournament_id, created_at DESC);

-- Rating history indexes
CREATE INDEX IF NOT EXISTS idx_rating_history_program ON rating_history(program_id);
CREATE INDEX IF NOT EXISTS idx_rating_history_tournament ON rating_history(tournament_id);
CREATE INDEX IF NOT EXISTS idx_rating_history_program_date ON rating_history(program_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rating_history_tournament_date ON rating_history(tournament_id, created_at DESC);

-- Add comments for documentation
COMMENT ON INDEX idx_tournaments_status_game IS 'Composite index for filtering tournaments by status and game type';
COMMENT ON INDEX idx_tournament_participants_rating IS 'Index for leaderboard queries - tournament + rating DESC';
COMMENT ON INDEX idx_matches_status_priority IS 'Index for queue processing - status + priority';
COMMENT ON INDEX idx_matches_tournament_status IS 'Composite index for tournament matches by status';
