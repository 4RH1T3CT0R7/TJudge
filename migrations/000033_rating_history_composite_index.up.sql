-- P1.2: composite index (program_id, tournament_id, created_at DESC) для rating_history.
--
-- Существующий idx_rating_history_program_date покрывает (program_id, created_at DESC),
-- но запрос "последний рейтинг программы в турнире" фильтрует дополнительно по
-- tournament_id, что приводит к bitmap-scan + filter. Новый композитный индекс
-- делает lookup O(log N) без filter.
--
-- Типичные запросы, которые ускорит:
--   SELECT new_rating FROM rating_history
--   WHERE program_id = ? AND tournament_id = ?
--   ORDER BY created_at DESC LIMIT 1;
--
-- CREATE INDEX CONCURRENTLY нельзя в транзакции golang-migrate, поэтому
-- используется обычный CREATE INDEX IF NOT EXISTS. Для уже-заполненных
-- prod-таблиц рекомендуется применить вручную с CONCURRENTLY через psql.
CREATE INDEX IF NOT EXISTS idx_rating_history_program_tournament_date
    ON rating_history(program_id, tournament_id, created_at DESC);
