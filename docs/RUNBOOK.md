# TJudge — Operations Runbook

Документ для оператора на дежурстве. Процедуры расположены в порядке "от самого частого к самому редкому".

## 1. Быстрая диагностика

```bash
# Health-check API
curl -sf http://localhost:8080/health            # → "OK"
curl -sfH "Authorization: Bearer <admin-jwt>" \
    http://localhost:8080/api/v1/system/health   # → JSON со статусом

# Метрики
curl -s http://localhost:8080/metrics | grep -E '^(tjudge_|go_goroutines)'

# Последние логи API
docker logs --tail=200 tjudge-api

# Размер очередей и dead-letter
docker exec -it tjudge-redis redis-cli -n 0 llen queue:high
docker exec -it tjudge-redis redis-cli -n 0 llen queue:dead_letter
```

## 2. Инциденты

### 2.1 API падает / perpetual restart

1. Проверить логи: `docker logs --tail=300 tjudge-api`. Частые причины:
   - `JWT_SECRET must be at least 32 bytes` → обновите секрет (см. §5).
   - `database connection refused` → проверьте Postgres (§2.3).
   - `panic: send on closed channel` → **должно быть починено в P0.4**; если видите — откройте issue.
2. Рестарт: `docker compose restart api`.
3. Если рестарт не помогает — откат: `./scripts/rollback.sh`.

### 2.2 Очередь матчей растёт (Prometheus: `tjudge_queue_size{priority="high"} > 1000`)

1. Проверить количество воркеров: `curl -s localhost:9090/metrics | grep tjudge_worker_pool_size`.
2. Увеличить `WORKER_MAX` и перезапустить worker:
   ```bash
   echo "WORKER_MAX=50" >> .env
   docker compose up -d worker
   ```
3. Если матчи падают: `docker logs --tail=300 tjudge-worker | grep ERROR`. Частая причина — `tjudge-cli` образ отсутствует. Пересобрать: `docker compose build tjudge-cli`.

### 2.3 Postgres недоступен

1. `docker ps | grep postgres` — контейнер запущен?
2. Если нет: `docker compose up -d postgres` и ждать healthcheck'а.
3. Если запущен но недоступен: `docker exec -it tjudge-postgres psql -U tjudge -c 'SELECT 1'`.
4. Диск переполнен? `df -h /var/lib/docker`. При 100% — удалить старые backup-ы и/или partition-ы.
5. Crash-loop → восстановление из backup (см. §3).

### 2.4 Redis недоступен

API автоматически переключается на fallback rate-limiter (P1.5, 0.5× основного лимита). Очередь матчей **не работает** при падении Redis.
1. Перезапустить: `docker compose restart redis`.
2. Если данные очередей потеряны — запустить recovery-worker, он переставит pending-матчи в очередь (P0.5).

### 2.5 WebSocket шторм подключений

- Метрика `tjudge_queue_deadletter_size` растёт → poison-сообщения; проверить deserializer.
- Клиент flood: в client.go включён per-client rate limit (P1.6, 10 msg/sec). Лимитируемые клиенты получают close 1008.
- Hub panic не должен случаться после P0.4; если случился — собрать core-dump и заглянуть в закрытия каналов.

### 2.6 Admin-действия надо расследовать (аудит)

```bash
curl -sH "Authorization: Bearer <admin-jwt>" \
  'http://localhost:8080/api/v1/admin/audit?limit=500' | jq
```

Таблица: `SELECT * FROM audit_log ORDER BY created_at DESC LIMIT 200;`

## 3. Резервное копирование и восстановление

### 3.1 Backup

Автоматический (P1.14): включается профилем compose:
```bash
docker compose --profile backup up -d backup
```
По умолчанию каждые 24ч в `backups_data` volume (prod) или `./backups` (selfhosted). Retention 30 дней.

Ручной: `./scripts/backup.sh ./backups`.

### 3.2 Восстановление из backup

```bash
# 1. Остановить API и worker (чтобы не было конкурентных записей)
docker compose stop api worker

# 2. Восстановить БД (пример: последний backup)
LATEST=$(ls -t backups/tjudge_*.sql.gz | head -1)
gunzip -c "$LATEST" | docker exec -i tjudge-postgres psql -U tjudge tjudge

# 3. Запустить миграции (на случай если backup старее текущих миграций)
docker compose up migrate

# 4. Запустить api и worker
docker compose up -d api worker
```

### 3.3 Point-in-time recovery

Не настроен (WAL-archiving не включён). Для prod рекомендуется настроить `pgbackrest` или `wal-g` с S3-хранилищем — отдельная задача в P2.

## 4. Деплой

См. [PRODUCTION_DEPLOY.md](PRODUCTION_DEPLOY.md).

Быстрый rollback:
```bash
./scripts/rollback.sh
```

## 5. Частые задачи

### 5.1 Ротация JWT_SECRET

1. Сгенерировать: `openssl rand -hex 48`.
2. Обновить secret (Docker secrets или env-var в deploy).
3. `docker compose up -d api` (rolling, если replicas>1).
4. **Все существующие сессии инвалидируются** — пользователи должны перелогиниться.

### 5.2 Назначить админа

```bash
make admin EMAIL=foo@bar.com
```

### 5.3 Миграция БД

```bash
make migrate-up       # apply pending
make migrate-down     # откат последней (dev only!)
make migrate-create NAME=add_foo  # шаблон
```

### 5.4 Сброс password (без email)

```sql
-- 1. Сгенерировать bcrypt-hash (cost 12, как в auth.Service):
-- SELECT crypt('newpassword', gen_salt('bf', 12));
UPDATE users SET password_hash = '$2a$12$...' WHERE email = 'foo@bar.com';
```

## 6. Известные ограничения (design)

- ELO rating update — delta-based (P1.13), параллельные матчи одного участника могут давать snapshot-based deltas; для строгой сериализации нужен advisory lock (в P2 backlog).
- Docker-in-Docker worker монтирует `docker.sock` read-only с non-root user'ом (P0.8); на хосте должна существовать docker-group с совпадающим GID.
- Password reset использует LogMailer по-умолчанию — reset-ссылки пишутся в лог. Настройте SMTP_HOST/USER/PASSWORD/FROM/USE_TLS env для реальной отправки.
