-- Композитный индекс под hot-query: выборка матчей турнира/игры с фильтром по статусу.
-- Покрывает `WHERE tournament_id=$1 AND game_type=$2 AND status IN (...)`
-- (используется в match_query.go:GetPendingByTournamentAndGame и game_repository.go).
--
-- Существующие индексы отдельно по tournament_id, game_type, status дают planner'у
-- выбор, но bitmap-сканы по трём индексам обычно медленнее одного композитного
-- на селективной таблице matches (партиционирована по created_at, размер
-- партиции растёт со временем).
CREATE INDEX IF NOT EXISTS idx_matches_tournament_game_status
    ON matches(tournament_id, game_type, status);
