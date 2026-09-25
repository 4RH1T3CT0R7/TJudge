-- откат упадёт, если уже есть записи удалённых акторов (actor_id IS NULL):
-- удалять аудит молча откат не должен
ALTER TABLE audit_log ALTER COLUMN actor_id SET NOT NULL;
