-- Update leaderboard_tournament to include tiebreak by earliest program upload
-- When teams have equal total_score and wins, the team that uploaded their latest
-- program version earlier ranks higher.
-- Tiebreak = MIN(created_at) of latest-version programs across all games for the team.

-- Drop and recreate tournament leaderboard with tiebreak
DROP MATERIALIZED VIEW IF EXISTS leaderboard_tournament;

CREATE MATERIALIZED VIEW IF NOT EXISTS leaderboard_tournament AS
SELECT
    tp.tournament_id,
    tp.program_id,
    p.name AS program_name,
    p.user_id,
    u.username,
    COALESCE(stats.total_score, 0) AS rating,
    COALESCE(stats.total_matches, 0) AS total_matches,
    COALESCE(stats.wins, 0) AS wins,
    COALESCE(stats.losses, 0) AS losses,
    COALESCE(stats.draws, 0) AS draws,
    tp.created_at AS joined_at,
    COALESCE(stats.last_match, tp.created_at) AS last_updated
FROM tournament_participants tp
INNER JOIN programs p ON tp.program_id = p.id
INNER JOIN users u ON p.user_id = u.id
LEFT JOIN LATERAL (
    SELECT
        COUNT(*) AS total_matches,
        SUM(CASE
            WHEN (m.program1_id = p.id AND m.winner = 1) OR (m.program2_id = p.id AND m.winner = 2)
            THEN 1 ELSE 0
        END) AS wins,
        SUM(CASE
            WHEN (m.program1_id = p.id AND m.winner = 2) OR (m.program2_id = p.id AND m.winner = 1)
            THEN 1 ELSE 0
        END) AS losses,
        SUM(CASE WHEN m.winner = 0 THEN 1 ELSE 0 END) AS draws,
        SUM(
            CASE
                WHEN m.program1_id = p.id THEN COALESCE(m.score1, 0)
                WHEN m.program2_id = p.id THEN COALESCE(m.score2, 0)
                ELSE 0
            END
        ) AS total_score,
        MAX(m.completed_at) AS last_match
    FROM matches m
    WHERE (m.program1_id = p.id OR m.program2_id = p.id)
      AND m.tournament_id = tp.tournament_id
      AND m.status = 'completed'
) stats ON true
ORDER BY tp.tournament_id, rating DESC, wins DESC,
    -- Tiebreak: MIN of latest-version created_at across all games for the team
    -- Consistent with getLeaderboardFallback live query
    COALESCE(
        (SELECT MIN(sub_p.created_at)
         FROM (
             SELECT DISTINCT ON (p2.game_id) p2.created_at
             FROM programs p2
             WHERE p2.team_id = p.team_id
               AND p2.tournament_id = tp.tournament_id
               AND p2.team_id IS NOT NULL
             ORDER BY p2.game_id, p2.version DESC
         ) sub_p
        ),
        p.created_at
    ) ASC;

-- Recreate indexes
CREATE UNIQUE INDEX idx_leaderboard_tournament_pk ON leaderboard_tournament(tournament_id, program_id);
CREATE INDEX idx_leaderboard_tournament_id ON leaderboard_tournament(tournament_id, rating DESC);

-- Grant permissions
GRANT SELECT ON leaderboard_tournament TO PUBLIC;
