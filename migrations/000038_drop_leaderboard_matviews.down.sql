-- Восстановление материализованных представлений лидербордов.
-- leaderboard_global - определение из 000017, leaderboard_tournament - из 000027.

CREATE MATERIALIZED VIEW IF NOT EXISTS leaderboard_global AS
SELECT
    p.id AS program_id,
    p.name AS program_name,
    p.user_id,
    u.username,
    COALESCE(stats.total_score, 0) AS rating,
    COALESCE(stats.total_matches, 0) AS total_matches,
    COALESCE(stats.wins, 0) AS wins,
    COALESCE(stats.losses, 0) AS losses,
    COALESCE(stats.draws, 0) AS draws,
    COALESCE(stats.last_match, p.created_at) AS last_updated
FROM programs p
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
      AND m.status = 'completed'
) stats ON true
ORDER BY rating DESC, total_matches DESC;

CREATE UNIQUE INDEX idx_leaderboard_global_program ON leaderboard_global(program_id);
CREATE INDEX idx_leaderboard_global_rating ON leaderboard_global(rating DESC);
CREATE INDEX idx_leaderboard_global_user ON leaderboard_global(user_id);

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

CREATE UNIQUE INDEX idx_leaderboard_tournament_pk ON leaderboard_tournament(tournament_id, program_id);
CREATE INDEX idx_leaderboard_tournament_id ON leaderboard_tournament(tournament_id, rating DESC);

GRANT SELECT ON leaderboard_global TO PUBLIC;
GRANT SELECT ON leaderboard_tournament TO PUBLIC;
