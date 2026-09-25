-- actor_id объявлен NOT NULL вместе с ON DELETE SET NULL: удаление пользователя
-- с записями аудита падало на not-null. аудит должен переживать удаление актора
ALTER TABLE audit_log ALTER COLUMN actor_id DROP NOT NULL;
