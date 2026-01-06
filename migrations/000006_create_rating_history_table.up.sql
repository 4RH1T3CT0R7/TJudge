-- Create rating_history table (partitioned by created_at)
CREATE TABLE IF NOT EXISTS rating_history (
    id UUID DEFAULT gen_random_uuid(),
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    old_rating INT NOT NULL,
    new_rating INT NOT NULL,
    change INT NOT NULL,
    match_id UUID,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create partition for current month (2026-01)
CREATE TABLE rating_history_2026_01 PARTITION OF rating_history
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

-- Create partition for next month (2026-02)
CREATE TABLE rating_history_2026_02 PARTITION OF rating_history
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

-- Create indexes
CREATE INDEX idx_rating_history_program ON rating_history(program_id, created_at DESC);
CREATE INDEX idx_rating_history_tournament ON rating_history(tournament_id);
CREATE INDEX idx_rating_history_match ON rating_history(match_id);
