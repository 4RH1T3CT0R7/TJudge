-- момент последней смены пароля: refresh-токены, выписанные раньше, отзываются
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;
