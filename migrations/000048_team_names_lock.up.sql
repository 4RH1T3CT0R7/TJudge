-- запрет переименования команд капитанами, включает админ одной кнопкой.
-- DEFAULT false на PG11+ таблицу не переписывает, но ACCESS EXCLUSIVE на
-- tournaments нужен, поэтому ожидание лока ограничено, как в 000045
-- (docs/OPERATIONS.md §9.7)
SET LOCAL lock_timeout = '5s';
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS team_names_locked BOOLEAN NOT NULL DEFAULT false;
