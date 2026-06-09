-- Статус жизненного цикла программы для асинхронной компиляции.
--
-- Раньше компиляция выполнялась синхронно в HTTP-хендлере загрузки на хосте
-- API-процесса (без таймаута и песочницы). Теперь загрузка создаёт программу
-- в статусе 'compiling', задача уходит в Redis-очередь, worker компилирует
-- в Docker-песочнице и переводит программу в 'ready' или 'failed'.
--
-- ready     - программа готова к матчам (скомпилирована / синтаксис проверен);
-- compiling - в очереди на компиляцию или компилируется;
-- failed    - компиляция/проверка синтаксиса не прошла (см. error_message).

ALTER TABLE programs ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'ready';

-- Backfill: существующие программы с ошибкой компиляции считаем failed.
UPDATE programs SET status = 'failed' WHERE error_message IS NOT NULL;

ALTER TABLE programs ADD CONSTRAINT programs_status_check
    CHECK (status IN ('compiling', 'ready', 'failed'));

-- Планировщик матчей выбирает только ready-программы.
CREATE INDEX IF NOT EXISTS idx_programs_status ON programs (status) WHERE status != 'ready';
