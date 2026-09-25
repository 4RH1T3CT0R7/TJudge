-- индексы-дубли: каждый либо совпадает с другим индексом, либо является его
-- префиксом, поэтому планировщик обходится без него, а запись в matches
-- (вставка раунда, смена статуса, результат) платит за каждый лишний индекс.
-- idx_matches_game_type и idx_matches_priority_created сюда не входят: они не
-- дубли, их неиспользуемость надо сначала подтвердить по pg_stat_user_indexes
--
-- DROP INDEX на партиционированных matches и rating_history нельзя сделать
-- CONCURRENTLY: он берёт ACCESS EXCLUSIVE на родителя и все партиции. в очереди за
-- долгим запросом такой лок заблокировал бы все новые запросы к matches, поэтому
-- ожидание ограничено: миграция падает через 5с (транзакция откатывается целиком),
-- после чего её повторяют в окно низкой нагрузки. golang-migrate выполняет файл
-- одним запросом, то есть одной транзакцией, поэтому SET LOCAL действует до конца файла
SET LOCAL lock_timeout = '5s';

-- matches: префиксы составных индексов
DROP INDEX IF EXISTS idx_matches_tournament;   -- (tournament_id) - префикс idx_matches_tournament_status и др.
DROP INDEX IF EXISTS idx_matches_program1;     -- (program1_id) - префикс idx_matches_programs
DROP INDEX IF EXISTS idx_matches_status;       -- (status) - префикс idx_matches_status_priority

-- rating_history
DROP INDEX IF EXISTS idx_rating_history_program_date; -- то же, что idx_rating_history_program
DROP INDEX IF EXISTS idx_rating_history_tournament;   -- (tournament_id) - префикс idx_rating_history_tournament_date

-- обычные индексы поверх UNIQUE-ограничений, у которых уже есть свой индекс
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_games_name;
DROP INDEX IF EXISTS idx_teams_code;
DROP INDEX IF EXISTS idx_tournaments_code;
