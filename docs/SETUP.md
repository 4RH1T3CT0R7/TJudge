# Настройка и развёртывание TJudge

## Требования

| Компонент | Версия | Назначение |
|-----------|--------|------------|
| Docker | 20+ | Контейнеризация |
| Docker Compose | 2+ | Оркестрация |
| Go | 1.24+ | Локальная разработка |
| Node.js | 20+ | Фронтенд |
| Make | - | Команды сборки |

## Быстрый старт (Docker Compose)

```bash
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
cp .env.example .env

docker-compose up -d
docker-compose ps
curl http://localhost:8080/health
```

| Сервис | URL |
|--------|-----|
| Веб-приложение | http://localhost:8080 |
| API | http://localhost:8080/api/v1 |
| Grafana | http://localhost:3000 (admin/admin) |
| Prometheus | http://localhost:9092 |
| Loki | http://localhost:3100 |

## Локальная разработка

```bash
# 1. Инфраструктура
docker-compose up -d postgres redis

# 2. Зависимости
go mod download
cd web && npm install && cd ..

# 3. Миграции
make migrate-up

# 4. API (терминал 1)
make run-api

# 5. Воркер (терминал 2)
make run-worker

# 6. Фронтенд (терминал 3)
cd web && npm run dev
```

Docker Compose пробрасывает PostgreSQL 5432 → 5433 на хосте, поэтому при локальной разработке используйте `DB_PORT=5433`.

### Команды Make

| Команда | Описание |
|---------|----------|
| `make dev` | API с hot reload (air) |
| `make run-api` | Запуск API сервера |
| `make run-worker` | Запуск воркера |
| `make test` | Unit-тесты (~970 в internal/) |
| `make test-race` | Тесты с детектором гонок |
| `make test-coverage` | Тесты с покрытием |
| `make lint` | Линтер (golangci-lint) |
| `make build` | Сборка бинарников |
| `make docker-build` | Сборка Docker-образов |
| `make migrate-up` | Применить миграции |
| `make migrate-down` | Откатить миграции |
| `make admin EMAIL=x@y.z` | Назначить администратора |

### Фронтенд

```bash
cd web
npm run dev        # Dev-режим (http://localhost:5173)
npm run build      # Сборка для встраивания в Go
npm run lint       # Линтинг
npm run preview    # Предпросмотр сборки
```

Стек: React 19, TypeScript 5.9, Vite 7.2, Tailwind CSS 4.1, Zustand 5.0, React Query 5.90.

После `npm run build` запустите `go build` для встраивания фронтенда в бинарник.

## Конфигурация

Загружается через `godotenv` + `os.Getenv()` (см. `internal/config/config.go`). Файл `config.example.yaml` в корне — только справочник по структуре, загрузка YAML не реализована. Для секретов в production поддерживается суффикс `_FILE` (Docker secrets) у `DB_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET`.

```bash
# ─── Окружение ───
ENVIRONMENT=development        # development | production

# ─── API Server ───
API_PORT=8080
BASE_URL=http://localhost:8080 # Для ссылок-приглашений и др.
READ_TIMEOUT=30s
WRITE_TIMEOUT=30s
SHUTDOWN_TIMEOUT=10s           # Таймаут graceful shutdown

# ─── PostgreSQL ───
DB_HOST=localhost
DB_PORT=5432                   # Локально с Docker используйте 5433 (см. выше)
DB_USER=tjudge
DB_PASSWORD=secret             # + DB_PASSWORD_FILE
DB_NAME=tjudge
DB_SSLMODE=disable             # disable | require | verify-full
DB_MAX_CONNECTIONS=50
DB_MAX_IDLE=10
DB_MAX_LIFETIME=1h

# ─── Redis ───
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=                # + REDIS_PASSWORD_FILE
REDIS_DB=0
REDIS_POOL_SIZE=100

# ─── Worker Pool ───
WORKER_MIN=10
WORKER_MAX=1000
WORKER_QUEUE_SIZE=10000
WORKER_TIMEOUT=90s
WORKER_RETRY_ATTEMPTS=3
WORKER_RETRY_DELAY=5s

# ─── Executor (Docker-контейнер для матчей) ───
EXECUTOR_DOCKER_IMAGE=tjudge-cli:latest
EXECUTOR_TIMEOUT=60s
EXECUTOR_CPU_QUOTA=100000      # Микросекунды на 100ms
EXECUTOR_MEMORY_LIMIT=536870912  # 512MB
EXECUTOR_PIDS_LIMIT=100
EXECUTOR_NETWORK_DISABLED=true
EXECUTOR_DEFAULT_ITERATIONS=100

# ─── JWT (измените в production!) ───
JWT_SECRET=your-secret-key-minimum-32-characters  # + JWT_SECRET_FILE
JWT_ACCESS_TTL=24h
JWT_REFRESH_TTL=168h           # 7 дней

# ─── Хранилище программ ───
PROGRAMS_PATH=/data/programs
HOST_PROGRAMS_PATH=            # Путь на хосте для Docker-in-Docker; пусто → PROGRAMS_PATH
MAX_FILE_SIZE=10485760         # 10MB

# ─── CORS ───
CORS_ALLOWED_ORIGINS=http://localhost:3000
CORS_MAX_AGE=3600              # Кэш preflight, секунды

# ─── Rate Limiting ───
RATE_LIMIT_ENABLED=false
RATE_LIMIT_RPM=100
RATE_LIMIT_BURST=200

# ─── Логирование ───
LOG_LEVEL=info                 # debug | info | warn | error
LOG_FORMAT=json                # json | console
LOG_OUTPUT=stdout              # stdout | stderr | путь к файлу
LOG_ASYNC=true

# ─── Метрики ───
METRICS_ENABLED=true
METRICS_PORT=9090
METRICS_PATH=/metrics
```

## Production деплой

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
    secrets:
      - db_password
      - jwt_secret
    environment:
      - DB_PASSWORD_FILE=/run/secrets/db_password
      - JWT_SECRET_FILE=/run/secrets/jwt_secret

secrets:
  db_password:
    file: ./secrets/db_password.txt
  jwt_secret:
    file: ./secrets/jwt_secret.txt
```

```bash
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d
docker-compose up -d --scale worker=5   # Масштабирование воркеров
curl http://localhost:8080/health
```

### Blue-Green деплой

```bash
./scripts/blue-green-deploy.sh blue   # Деплой синей версии
./scripts/switch-traffic.sh blue      # Переключение трафика
./scripts/smoke-test.sh               # Smoke-тесты
./scripts/rollback.sh                 # Откат
```

### Рекомендации по ресурсам

| Компонент | CPU | RAM | Реплики |
|-----------|-----|-----|---------|
| API | 2 ядра | 2GB | 1-3 |
| Worker | 2 ядра | 2GB | 1-10 |
| PostgreSQL | 2 ядра | 4GB | 1 |
| Redis | 1 ядро | 1GB | 1 |

## Мониторинг

Grafana (http://localhost:3000, admin/admin) — дашборды: TJudge Overview, Workers, API, Database.

Prometheus-запросы:

```promql
tjudge_queue_size{priority="high"}                                        # Матчей в очереди
tjudge_workers_active                                                     # Активных воркеров
histogram_quantile(0.99, tjudge_http_request_duration_seconds_bucket)     # HTTP p99
rate(tjudge_matches_total[5m])                                            # Обработано матчей
```

- Loki (http://localhost:3100) — логи через Promtail, доступны в Grafana.
- Alertmanager (http://localhost:9093) — алерты в `deployments/prometheus/alerts/tjudge.yml`.

## CI/CD

| Workflow | Описание |
|----------|----------|
| `ci.yml` | Линт, тесты, сборка |
| `release.yml` | Сборка образов и деплой по тегу v* |

Локальный прогон: `make lint`, `make test`, `make test-race`, `make build`, `make docker-build`.

## Устранение неполадок

| Проблема | Решение |
|----------|---------|
| `role tjudge does not exist` | Локальный PG на 5432, используйте 5433 для Docker |
| `air: command not found` | `go install github.com/air-verse/air@latest` + `export PATH=$PATH:~/go/bin` |
| `connection refused :8080` | Сервер не запущен, проверьте логи |
| `Internal server error` | Миграции не применены: `make migrate-up` |
| Матчи не обрабатываются | `docker-compose logs worker` |
| `pattern all:dist: no matching files found` | Фронтенд не собран: `cd web && npm run build` |

Диагностика:

```bash
docker-compose ps                                          # Статус контейнеров
docker-compose logs -f api worker                          # Логи
docker exec -it tjudge-postgres psql -U tjudge -d tjudge   # Подключение к БД
docker exec tjudge-redis redis-cli LLEN queue:high         # Очередь Redis
docker-compose down -v && docker-compose up -d             # Очистка и перезапуск
```

Бэкап:

```bash
# PostgreSQL
docker exec tjudge-postgres pg_dump -U tjudge tjudge > backup.sql
docker exec -i tjudge-postgres psql -U tjudge -d tjudge < backup.sql

# Redis
docker exec tjudge-redis redis-cli BGSAVE
docker cp tjudge-redis:/data/dump.rdb ./backup/
```

## Безопасность

- Не коммитьте секреты; `secrets/` и `.env` — в `.gitignore`.
- В production — Docker Secrets, JWT secret минимум 32 символа, ротация секретов.
- Настройте rate limiting (`RATE_LIMIT_RPM`).

```bash
# Сканирование зависимостей Go
go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth
# Сканирование Docker-образов
docker scan tjudge-api:latest
```
