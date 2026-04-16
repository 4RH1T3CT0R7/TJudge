-- P1.12: таблица audit_log для записи admin-действий.
--
-- Хранит: кто, что, над каким ресурсом, когда, с какого IP/UA.
-- retention 1 год (партиционирование не требуется при текущем объёме admin-trafic,
-- но можно добавить в P2 если записей станет > 1M).

CREATE TABLE IF NOT EXISTS audit_log (
    id           UUID PRIMARY KEY,
    actor_id     UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    actor_role   TEXT NOT NULL,
    action       TEXT NOT NULL,                -- e.g. "tournament.create", "user.promote_admin"
    target_type  TEXT NOT NULL DEFAULT '',     -- e.g. "tournament", "user", "game", ""
    target_id    TEXT NOT NULL DEFAULT '',     -- stringified UUID или произвольный идентификатор
    method       TEXT NOT NULL DEFAULT '',     -- HTTP method
    path         TEXT NOT NULL DEFAULT '',     -- request path
    status_code  INT  NOT NULL DEFAULT 0,      -- HTTP status response
    ip           TEXT NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_actor         ON audit_log(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at    ON audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_target        ON audit_log(target_type, target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action        ON audit_log(action, created_at DESC);
