-- Транзакционный outbox для пост-обработки результатов матчей.
--
-- Проблема: результат матча (UPDATE matches) и обновление рейтингов
-- (rating_history + tournament_participants) выполнялись в разных
-- транзакциях. Падение процесса между ними молча теряло рейтинг -
-- матч завершён, а ELO не пересчитан (фиксировалось только в логе).
--
-- Решение: задача «обработать рейтинг» записывается в outbox в одной
-- транзакции с результатом матча (MatchRepository.UpdateResultWithOutbox).
-- Worker обновляет рейтинг сразу (fast path) и помечает задачу done;
-- OutboxDispatcher периодически подбирает зависшие pending-задачи
-- (краш между транзакциями, ошибка БД) и доводит их до конца.

CREATE TABLE IF NOT EXISTS match_outbox (
    id          BIGSERIAL PRIMARY KEY,
    match_id    UUID NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'rating_update',
    status      TEXT NOT NULL DEFAULT 'pending',  -- pending | done | error
    attempts    INT  NOT NULL DEFAULT 0,
    last_error  TEXT,
    claimed_at  TIMESTAMPTZ,                      -- lease: защита от двойного клейма
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

-- Частичный индекс: диспетчер сканирует только pending-задачи.
CREATE INDEX IF NOT EXISTS idx_match_outbox_pending
    ON match_outbox (created_at)
    WHERE status = 'pending';

-- Быстрый поиск задачи по матчу для fast-path пометки done.
CREATE INDEX IF NOT EXISTS idx_match_outbox_match
    ON match_outbox (match_id)
    WHERE status = 'pending';
