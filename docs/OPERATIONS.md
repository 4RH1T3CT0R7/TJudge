# TJudge - эксплуатация

Документ для оператора: развёртывание, обновление, диагностика и восстановление. Рассчитан на self-hosted single-node. Для кластерного деплоя смотрите `deployments/k8s/` (экспериментальный режим).

## 1. Требования

- Linux x86_64, Docker 24+, Docker Compose v2.
- 4 ГБ RAM, 20 ГБ диска (под БД и резервные копии).
- Домен с A-записью на IP сервера.
- Открытые порты 80 и 443.

## 2. Первичная настройка

```bash
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge

./scripts/init-secrets.sh      # создаст ./secrets/{db_password,redis_password,jwt_secret}
chmod 600 secrets/*

cp .env.production.example .env.production
```

Минимум для `.env.production`:

```ini
ENVIRONMENT=production
BASE_URL=https://tjudge.example.com
JWT_SECRET=<случайные 48+ байт; openssl rand -hex 48>
DB_PASSWORD=<crypto-random>
REDIS_PASSWORD=<crypto-random>
CORS_ALLOWED_ORIGINS=https://tjudge.example.com
WEBSOCKET_ALLOWED_ORIGINS=https://tjudge.example.com
RATE_LIMIT_ENABLED=true
```

`JWT_SECRET` не должен содержать плейсхолдеры (`CHANGE_ME`, `secret`, `password` и подобные): при старте в prod код сразу упадёт с ошибкой.

## 3. Первый запуск

```bash
# TLS: сертификат Let's Encrypt.
./scripts/init-ssl.sh tjudge.example.com admin@example.com

# Сборка и запуск.
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# Миграции применяются автоматически сервисом migrate.

# Проверка health.
curl https://tjudge.example.com/health    # ожидаем "OK"

# Назначение первого админа.
docker exec -it tjudge-api ./tjudge-admin promote admin@example.com
```

## 4. Резервное копирование

Автоматический backup включается профилем compose:

```bash
docker compose -f docker-compose.prod.yml --profile backup up -d backup
```

По умолчанию backup раз в сутки в volume `backups_data` (prod) или `./backups` (self-hosted), retention 30 дней. Ручной запуск: `./scripts/backup.sh ./backups`.

Проверка: `docker logs tjudge-backup` через сутки.

## 5. Мониторинг

```bash
docker compose -f docker-compose.prod.yml --profile monitoring up -d prometheus grafana loki alertmanager
```

Grafana на `https://tjudge.example.com:3000` (или за reverse-proxy). Учётные данные задаются в `.env.production`:

```
GF_ADMIN_USER=admin
GF_ADMIN_PASSWORD=<replace>
```

## 6. Чеклист перед открытием на публику

- [ ] `JWT_SECRET` не короче 32 байт, не из blacklist-плейсхолдеров (автопроверка при старте).
- [ ] `.env.production` не в git: `git ls-files .env.production` пусто.
- [ ] TLS настроен, HTTP-трафика нет, HSTS выставлен.
- [ ] `CORS_ALLOWED_ORIGINS` содержит только ваши домены без `*`.
- [ ] `WEBSOCKET_ALLOWED_ORIGINS` задан (в prod fail-closed).
- [ ] Worker запущен не от root: `docker compose ps worker` показывает `user=1000`.
- [ ] `RATE_LIMIT_ENABLED=true`.
- [ ] Backup-сервис работает: `docker ps | grep tjudge-backup`.
- [ ] В CI прошли `gosec`, `npm audit`, `Trivy`; HIGH-находки исправлены.
- [ ] Первый админ назначен, пароль надёжный.

## 7. Обновление (blue-green)

```bash
./scripts/blue-green-deploy.sh <new-tag>
./scripts/smoke-test.sh              # проверяем готовность нового стека
./scripts/switch-traffic.sh          # переключаем nginx-upstream
# После ~5 минут мониторинга:
./scripts/blue-green-deploy.sh cleanup
```

Откат: `./scripts/rollback.sh`.

### 7.1 Очистка диска после релизов

`scripts/deploy.sh` и `scripts/blue-green-deploy.sh` автоматически вызывают `cleanup_old_images` после успешного деплоя. Функция делает три вещи:

1. Удаляет **dangling-образы** (безымянные слои, оставшиеся после rebuild).
2. Для каждого репозитория `tjudge-{api,worker,executor,migrate,cli}` **оставляет N последних тегов** (по умолчанию 3, переопределяется через `TJUDGE_IMAGE_KEEP`), остальные сносит.
3. Чистит **build-кэш** старше 7 дней.

**Почему не `docker image prune -a`**: образ `tjudge-executor` запускается воркером on-demand (`internal/infrastructure/executor/executor.go:169-186`), поэтому между матчами на него **нет ни одного работающего контейнера**. Blanket-prune удалил бы его и сломал бы выполнение матчей до следующего pull'а из ghcr.io. Tag-based retention решает это и сохраняет предыдущие версии API/worker для rollback.

Если на сервере всё равно накопился мусор (длительный простой деплоя, старые проекты), ручная чистка:

```bash
# Безопасно: dangling-образы + build-кэш.
docker image prune -f
docker builder prune -af

# Посмотреть, что занимает место.
docker system df

# Увеличить/уменьшить retention для следующего деплоя.
TJUDGE_IMAGE_KEEP=5 ./scripts/blue-green-deploy.sh <version>
```

**НЕ запускать на проде:**
- `docker image prune -af` без фильтров — удалит `tjudge-executor`, следующий матч упадёт.
- `docker system prune -af --volumes` и `docker volume prune` — если контейнер `postgres` или `redis` в этот момент остановлен (например, между переключениями blue-green), volume посчитается unused и улетит вместе с БД.

Ротация логов контейнеров — в `/etc/docker/daemon.json`:

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "50m", "max-file": "3" }
}
```

После правки: `sudo systemctl restart docker`. Новые контейнеры подхватят настройку автоматически, старые — при следующем пересоздании (ближайший деплой).

Каталог `data/programs/` (переменная `HOST_PROGRAMS_PATH`) хранит загруженные программы участников и **не чистится автоматически**. Это данные, не мусор; если нужна политика retention — обсуждается отдельно.

## 8. Быстрая диагностика

### make status — всё состояние одной командой

```bash
# На сервере, рядом с docker-compose:
make status
# или с полным статусом (БД, очереди, матчи, программы, outbox):
ADMIN_TOKEN=<admin-jwt> make status
```

Команда проверяет: контейнеры, наличие образов `tjudge-cli`/`tjudge-builder`,
**не работают ли контейнеры api/worker на устаревшем образе** (образ пересобран,
но контейнер не перезапущен — главный ответ на вопрос «надо ли пересобирать»),
health API/worker'а и, при наличии `ADMIN_TOKEN`, полный статус из
`GET /api/v1/system/status`: версия сборки и аптайм, здоровье и версия миграций
PostgreSQL, Redis, размеры всех очередей (включая компиляцию и dead-letter),
матчи и программы по статусам, outbox целостности рейтингов, WebSocket-клиенты.

Тот же полный статус доступен:
- в **админ-панели** на вкладке «Система» (обновляется каждые 10 секунд);
- в **Grafana**: дашборд «TJudge — Обзор системы» (provisioning автоматический,
  профиль `monitoring` в docker-compose.selfhosted.yml);
- сырым JSON: `curl -sH "Authorization: Bearer <admin-jwt>" \
  http://localhost:8080/api/v1/system/status | jq`

### Точечные проверки

```bash
# Health API.
curl -sf http://localhost:8080/health            # "OK"
curl -sfH "Authorization: Bearer <admin-jwt>" \
    http://localhost:8080/api/v1/system/health   # JSON со статусом

# Метрики.
curl -s http://localhost:8080/metrics | grep -E '^(tjudge_|go_goroutines)'

# Последние логи API.
docker logs --tail=200 tjudge-api

# Очереди и dead-letter.
docker exec -it tjudge-redis redis-cli -n 0 llen queue:high
docker exec -it tjudge-redis redis-cli -n 0 llen queue:dead_letter
```

## 9. Инциденты

### 9.1 API падает или уходит в perpetual restart

1. Смотрим логи: `docker logs --tail=300 tjudge-api`. Частые причины:
   - `JWT_SECRET must be at least 32 bytes`: обновите секрет (см. §11.1).
   - `database connection refused`: проверяем Postgres (§9.3).
   - `panic: send on closed channel`: баг инфраструктуры, заводим issue.
2. Рестарт: `docker compose restart api`.
3. Если не помогает, откат: `./scripts/rollback.sh`.

### 9.2 Растёт очередь матчей

Триггер: `tjudge_queue_size{priority="high"} > 1000`.

1. Проверяем размер пула воркеров: `curl -s localhost:9090/metrics | grep tjudge_worker_pool_size`.
2. Увеличиваем `WORKER_MAX` и перезапускаем воркер:
   ```bash
   echo "WORKER_MAX=50" >> .env
   docker compose up -d worker
   ```
3. Если матчи падают: `docker logs --tail=300 tjudge-worker | grep ERROR`. Частая причина - отсутствует образ `tjudge-cli`. Пересобрать: `docker compose build tjudge-cli`.

### 9.3 Postgres недоступен

1. `docker ps | grep postgres` - контейнер запущен?
2. Если нет: `docker compose up -d postgres`, ждём healthcheck.
3. Если запущен, но недоступен: `docker exec -it tjudge-postgres psql -U tjudge -c 'SELECT 1'`.
4. Проверить диск: `df -h /var/lib/docker`. При переполнении удалить старые backup-ы и партиции.
5. Crash-loop: восстановление из backup (§10).

### 9.4 Redis недоступен

API автоматически переключается на fallback rate-limiter (0.5× от основного лимита). Очередь матчей при падении Redis не работает.

1. Перезапустить: `docker compose restart redis`.
2. Если данные очередей потеряны, запустить recovery-worker: он переставит pending-матчи в очередь.

### 9.5 WebSocket-шторм подключений

- Метрика `tjudge_queue_deadletter_size` растёт - возможно poison-сообщения; проверить deserializer.
- Клиентский flood: в `client.go` включён per-client rate limit (10 msg/sec). Лимитируемые клиенты получают close 1008.

### 9.6 Расследование admin-действий

```bash
curl -sH "Authorization: Bearer <admin-jwt>" \
  'http://localhost:8080/api/v1/admin/audit?limit=500' | jq
```

В БД: `SELECT * FROM audit_log ORDER BY created_at DESC LIMIT 200;`.

## 10. Восстановление из backup

```bash
# 1. Остановить API и worker, чтобы не было конкурентных записей.
docker compose stop api worker

# 2. Восстановить БД (пример: последний backup).
LATEST=$(ls -t backups/tjudge_*.sql.gz | head -1)
gunzip -c "$LATEST" | docker exec -i tjudge-postgres psql -U tjudge tjudge

# 3. Прогнать миграции на случай, если backup старше текущих.
docker compose up migrate

# 4. Запустить api и worker.
docker compose up -d api worker
```

Point-in-time recovery пока не настроен: WAL-archiving не включён. Для prod рекомендуется `pgbackrest` или `wal-g` с S3-хранилищем.

## 11. Частые задачи

### 11.1 Ротация JWT_SECRET

1. Сгенерировать: `openssl rand -hex 48`.
2. Обновить secret (Docker secrets или env-переменная).
3. `docker compose up -d api` (rolling при `replicas>1`).
4. Все существующие сессии инвалидируются, пользователи должны перелогиниться.

### 11.2 Назначение админа

```bash
make admin EMAIL=foo@bar.com
```

### 11.3 Миграции БД

```bash
make migrate-up                      # применить pending
make migrate-down                    # откат последней (только dev)
make migrate-create NAME=add_foo     # шаблон новой миграции
```

### 11.4 Сброс пароля пользователя

```sql
-- bcrypt-hash (cost 12, как в auth.Service):
-- SELECT crypt('newpassword', gen_salt('bf', 12));
UPDATE users SET password_hash = '$2a$12$...' WHERE email = 'foo@bar.com';
```

## 12. Масштабирование

### 12.1 Вертикальное (single-node)

- `WORKER_MAX`: максимум 200 при 4 vCPU; не ставить выше `DB_MAX_CONNECTIONS * 1.5`.
- `DB_MAX_CONNECTIONS`: держим в пределах `pg_settings.max_connections - 10` с запасом на админские сессии.
- `REDIS_POOL_SIZE`: 50-200 достаточно.

### 12.2 Горизонтальное

API stateless, запускается в нескольких экземплярах. Worker безопасен в multi-instance режиме через Redis distributed lock. Переход в K8s: см. `deployments/k8s/` (требует доработки).

## 13. Известные ограничения

- Обновление ELO идёт delta-based: параллельные матчи одного участника могут давать snapshot-based deltas. Для строгой сериализации нужен advisory lock.
- Docker-in-Docker worker монтирует `docker.sock` read-only с non-root пользователем. На хосте должна существовать docker-group с совпадающим GID.
