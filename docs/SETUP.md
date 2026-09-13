# Настройка и развёртывание TJudge

## Требования

Docker 20+, Docker Compose 2+, Go 1.24+, Node.js 20+ (фронтенд), Make.

## Быстрый старт (Docker Compose)

```bash
git clone https://github.com/bmstu-itstech/tjudge.git && cd tjudge
cp .env.example .env
docker-compose up -d
curl http://localhost:8080/health
```

Веб и API — http://localhost:8080 (`/api/v1`), Grafana — :3000 (admin/admin), Prometheus — :9092, Loki — :3100.

## Локальная разработка

```bash
docker-compose up -d postgres redis
go mod download && (cd web && npm install)
make migrate-up
make run-api             # терминал 1
make run-worker          # терминал 2
cd web && npm run dev    # терминал 3, http://localhost:5173
```

Compose пробрасывает PostgreSQL 5432 → 5433 на хосте, поэтому локально нужен `DB_PORT=5433`.

Фронтенд: React 19, TypeScript 5.9, Vite 7.2, Tailwind CSS 4.1, Zustand 5.0, React Query 5.90. `npm run build` собирает `web/dist`, после чего `go build` встраивает его в бинарник. Ещё есть `npm run lint` и `npm run preview`.

Основные make-таргеты: `dev` (API с hot reload через air), `run-api`, `run-worker`, `build`, `docker-build`, `migrate-up` / `migrate-down` / `migrate-create`, `lint` (golangci-lint), `admin EMAIL=x@y.z` (выдать роль админа).

## Тестирование

```bash
make test              # unit-тесты (~970 в internal/)
make test-race         # с детектором гонок
make test-coverage     # с отчётом покрытия
make test-integration  # -tags=integration, tests/integration/ — нужны postgres и redis
make test-e2e          # -tags=e2e, tests/e2e/ — нужен запущенный API
```

## Конфигурация

`godotenv` + `os.Getenv()` (`internal/config/config.go`). `config.example.yaml` — только справочник по структуре, загрузка YAML не реализована. Для секретов в production у `DB_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET` работает суффикс `_FILE` (Docker secrets).

```bash
ENVIRONMENT=development          # development | production
API_PORT=8080
BASE_URL=http://localhost:8080   # ссылки-приглашения и др.
READ_TIMEOUT=30s
WRITE_TIMEOUT=30s
SHUTDOWN_TIMEOUT=10s             # graceful shutdown
DB_HOST=localhost
DB_PORT=5432                     # локально с docker — 5433
DB_USER=tjudge
DB_PASSWORD=secret               # + DB_PASSWORD_FILE
DB_NAME=tjudge
DB_SSLMODE=disable               # disable | require | verify-full
DB_MAX_CONNECTIONS=50
DB_MAX_IDLE=10
DB_MAX_LIFETIME=1h
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=                  # + REDIS_PASSWORD_FILE
REDIS_DB=0
REDIS_POOL_SIZE=100
WORKER_MIN=10
WORKER_MAX=1000
WORKER_QUEUE_SIZE=10000
WORKER_TIMEOUT=90s
WORKER_RETRY_ATTEMPTS=3
WORKER_RETRY_DELAY=5s
EXECUTOR_DOCKER_IMAGE=tjudge-cli:latest
EXECUTOR_TIMEOUT=60s
EXECUTOR_CPU_QUOTA=100000        # микросекунды на 100ms
EXECUTOR_MEMORY_LIMIT=536870912  # 512MB
EXECUTOR_PIDS_LIMIT=100
EXECUTOR_NETWORK_DISABLED=true
EXECUTOR_DEFAULT_ITERATIONS=100
JWT_SECRET=your-secret-key-minimum-32-characters  # + JWT_SECRET_FILE; в production сменить
JWT_ACCESS_TTL=24h
JWT_REFRESH_TTL=168h             # 7 дней
PROGRAMS_PATH=/data/programs
HOST_PROGRAMS_PATH=              # путь на хосте для docker-in-docker; пусто = PROGRAMS_PATH
MAX_FILE_SIZE=10485760           # 10MB
CORS_ALLOWED_ORIGINS=http://localhost:3000
CORS_MAX_AGE=3600                # кэш preflight, секунды
RATE_LIMIT_ENABLED=false
RATE_LIMIT_RPM=100
RATE_LIMIT_BURST=200
LOG_LEVEL=info                   # debug | info | warn | error
LOG_FORMAT=json                  # json | console
LOG_OUTPUT=stdout                # stdout | stderr | путь к файлу
LOG_ASYNC=true
METRICS_ENABLED=true
METRICS_PORT=9090
METRICS_PATH=/metrics
```

## Схема БД

PostgreSQL 15. Основные таблицы (полная схема — в миграциях `migrations/`):

- `users` — аккаунты (username, email, password_hash, role).
- `games` — пять игр (name, rules, score_multiplier).
- `tournaments` — турниры (status: pending/active/completed, optimistic lock через `version`).
- `tournament_games` — связь турнир-игра (round_status, `config` JSONB, `auto_round_*`).
- `tournament_participants` — программы в турнире (ELO rating, wins/losses/draws).
- `teams` / `team_members` — команды с invite_code и лидером.
- `programs` — загруженные программы (status: compiling/ready/failed, уникальность `(team_id, game_id, version)`).
- `matches` — матчи; партиционирована по `created_at` помесячно, PK `(id, created_at)`.
- `rating_history` — история ELO; тоже помесячные партиции.
- `match_outbox` — outbox «пересчитать рейтинг», пишется в одной транзакции с результатом матча.
- `audit_log` — админ-аудит (refresh-токены — stateless JWT, таблицы под них нет).

Партиции `matches_YYYY_MM` / `rating_history_YYYY_MM` создаются автоматически; удаление старых — `drop_old_partitions()` при `DB_PARTITION_RETENTION_MONTHS > 0` (по умолчанию 0 — retention выключен).

Лидерборды считаются живыми запросами по `matches`: `UNION ALL` по обеим сторонам матча на индексе `(tournament_id, game_type, status)`. Materialized views удалены в 000038.

Миграции: `migrations/000001_*.sql` … `000041_*.sql`; номер 000035 пропущен намеренно (выпилен вместе с password-reset) и не переиспользуется.

```sql
-- ожидающие матчи турнира
SELECT * FROM matches WHERE status = 'pending' AND tournament_id = $1 ORDER BY created_at LIMIT 100;

-- ручное создание партиций текущего/следующего месяца
SELECT create_matches_partition_if_needed();
SELECT create_rating_history_partition_if_needed();
```

## Production

Секреты — через Docker Secrets:

```bash
mkdir -p secrets
echo "your-db-password" > secrets/db_password.txt
echo "your-jwt-secret-min-32-chars" > secrets/jwt_secret.txt
chmod 600 secrets/*.txt
```

```yaml
# docker-compose.prod.yml
services:
  api:
    secrets: [db_password, jwt_secret]
    environment: [DB_PASSWORD_FILE=/run/secrets/db_password, JWT_SECRET_FILE=/run/secrets/jwt_secret]
secrets:
  db_password: { file: ./secrets/db_password.txt }
  jwt_secret: { file: ./secrets/jwt_secret.txt }
```

```bash
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d
docker-compose up -d --scale worker=5   # масштабирование воркеров
```

Blue-green: `./scripts/blue-green-deploy.sh blue` → `./scripts/switch-traffic.sh blue` → `./scripts/smoke-test.sh`; откат — `./scripts/rollback.sh`.

Ресурсы: API и Worker — по 2 ядра / 2GB (реплик 1-3 и 1-10), PostgreSQL — 2 ядра / 4GB, Redis — 1 ядро / 1GB.

## Мониторинг и CI

Grafana (:3000, admin/admin) — дашборды TJudge Overview, Workers, API, Database. Loki (:3100) — логи через Promtail. Alertmanager (:9093) — правила в `deployments/prometheus/alerts/tjudge.yml`.

```promql
tjudge_queue_size{priority="high"}                                     # матчей в очереди
tjudge_workers_active                                                  # активных воркеров
histogram_quantile(0.99, tjudge_http_request_duration_seconds_bucket)  # http p99
rate(tjudge_matches_total[5m])                                         # обработано матчей
```

CI/CD: `ci.yml` — линт, тесты, сборка; `release.yml` — образы и деплой по тегу `v*`. Локальный прогон перед пушем: `make lint`, `make test`, `make test-race`, `make build`, `make docker-build`.

## Типовые проблемы

| Проблема | Решение |
|----------|---------|
| `role tjudge does not exist` | локальный PG занял 5432 — для Docker используйте 5433 |
| `air: command not found` | `go install github.com/air-verse/air@latest` + `~/go/bin` в PATH |
| `connection refused :8080` | сервер не запущен, смотрите логи |
| `Internal server error` | миграции не применены: `make migrate-up` |
| Матчи не обрабатываются | `docker-compose logs worker` |
| `pattern all:dist: no matching files found` | фронтенд не собран: `cd web && npm run build` |

```bash
docker-compose ps                                          # статус контейнеров
docker-compose logs -f api worker                          # логи
docker exec -it tjudge-postgres psql -U tjudge -d tjudge   # консоль бд
docker exec tjudge-redis redis-cli LLEN queue:high         # очередь redis
docker-compose down -v && docker-compose up -d             # полный сброс

# бэкап и восстановление
docker exec tjudge-postgres pg_dump -U tjudge tjudge > backup.sql
docker exec -i tjudge-postgres psql -U tjudge -d tjudge < backup.sql
docker exec tjudge-redis redis-cli BGSAVE && docker cp tjudge-redis:/data/dump.rdb ./backup/
```

## Безопасность

- Секреты не коммитить; `secrets/` и `.env` — в `.gitignore`.
- В production — Docker Secrets, JWT secret минимум 32 символа, ротация секретов.
- Rate limiting: `RATE_LIMIT_ENABLED` + `RATE_LIMIT_RPM`.
- Сканирование: `go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth` (зависимости), `docker scan tjudge-api:latest` (образы).
