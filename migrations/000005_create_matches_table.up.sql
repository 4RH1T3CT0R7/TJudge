-- Create matches table (partitioned by created_at)
CREATE TABLE IF NOT EXISTS matches (
    id UUID DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    program1_id UUID NOT NULL REFERENCES programs(id),
    program2_id UUID NOT NULL REFERENCES programs(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority VARCHAR(10) NOT NULL DEFAULT 'medium',
    score1 INT,
    score2 INT,
    winner INT,
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at),
    CONSTRAINT valid_status CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    CONSTRAINT valid_priority CHECK (priority IN ('high', 'medium', 'low')),
    CONSTRAINT valid_winner CHECK (winner IN (0, 1, 2) OR winner IS NULL)
) PARTITION BY RANGE (created_at);

-- Create partition for current month (2026-01)
CREATE TABLE matches_2026_01 PARTITION OF matches
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

-- Create partition for next month (2026-02)
CREATE TABLE matches_2026_02 PARTITION OF matches
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

-- Create indexes
CREATE INDEX idx_matches_tournament ON matches(tournament_id);
CREATE INDEX idx_matches_status ON matches(status);
CREATE INDEX idx_matches_priority_created ON matches(priority, created_at);
CREATE INDEX idx_matches_program1 ON matches(program1_id);
CREATE INDEX idx_matches_program2 ON matches(program2_id);
