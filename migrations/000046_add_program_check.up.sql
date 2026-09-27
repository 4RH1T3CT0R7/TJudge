-- самопроверка программы после сборки: короткий нерейтинговый матч против
-- эталонного бота игры (CompileWorker). это только предупреждение команде,
-- из раундов программа не исключается.
-- NULL - проверки не было: программа старше этой миграции или окружение не
-- смогло её провести. столбцы nullable без DEFAULT - таблица не переписывается
ALTER TABLE programs ADD COLUMN IF NOT EXISTS check_status TEXT;
ALTER TABLE programs ADD COLUMN IF NOT EXISTS check_message TEXT;
ALTER TABLE programs ADD COLUMN IF NOT EXISTS checked_at TIMESTAMP;

-- DROP IF EXISTS перед ADD: повторный прогон после dirty-состояния не падает
ALTER TABLE programs DROP CONSTRAINT IF EXISTS programs_check_status_check;
ALTER TABLE programs ADD CONSTRAINT programs_check_status_check
    CHECK (check_status IN ('pending', 'ok', 'failed'));
