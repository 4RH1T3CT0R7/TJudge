-- P1.11: таблица одноразовых токенов для восстановления пароля.
--
-- Храним только SHA-256 хэш токена — при утечке БД нельзя использовать записи
-- для reset'а (нужен оригинальный токен из email).
-- Поле used_at гарантирует one-time-use (token тратится на первое успешное
-- обращение к /auth/password-reset/confirm).

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    requester_ip TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user    ON password_reset_tokens(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires ON password_reset_tokens(expires_at);
