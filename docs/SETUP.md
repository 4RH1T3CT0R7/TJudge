# Настройка и развёртывание TJudge

## Быстрый старт

### Требования

| Компонент | Версия | Назначение |
|-----------|--------|------------|
| Docker | 20+ | Контейнеризация |
| Docker Compose | 2+ | Оркестрация |
| Go | 1.24+ | Локальная разработка |
| Node.js | 20+ | Фронтенд разработка |
| Make | — | Команды сборки |

### Запуск (Docker Compose)

```bash
# Клонирование и настройка
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
cp .env.example .env

# Запуск всех сервисов
docker-compose up -d

# Проверка
docker-compose ps
curl http://localhost:8080/health
```

**Доступные сервисы:**

| Сервис | URL | Описание |
|--------|-----|----------|
| Веб-приложение | http://localhost:8080 | Основной интерфейс |
| API | http://localhost:8080/api/v1 | REST API |
| Grafana | http://localhost:3000 | Мониторинг (admin/admin) |
| Prometheus | http://localhost:9092 | Метрики |
| Loki | http://localhost:3100 | Логи |

---

## Локальная разработка

### Настройка окружения

```bash
# 1. Запуск инфраструктуры
docker-compose up -d postgres redis

# 2. Установка зависимостей
go mod download
cd web && npm install && cd ..

# 3. Применение миграций
make migrate-up

# 4. Запуск API (терминал 1)
make run-api

# 5. Запуск воркера (терминал 2)
make run-worker

# 6. Запуск фронтенда в dev-режиме (терминал 3)
cd web && npm run dev
```

### Команды Make

| Команда | Описание |
|---------|----------|
| `make dev` | API с hot reload (air) |
| `make run-api` | Запуск API сервера |
| `make run-worker` | Запуск воркера |
| `make test` | Unit тесты (~970 в internal/ + table-driven subtests) |
| `make test-race` | Тесты с детектором гонок |
| `make test-coverage` | Тесты с покрытием |
| `make lint` | Линтер (golangci-lint) |
| `make build` | Сборка бинарников |
| `make docker-build` | Сборка Docker образов |
| `make migrate-up` | Применить миграции |
| `make migrate-down` | Откатить миграции |
| `make admin EMAIL=x@y.z` | Назначить администратора |
| `make benchmark` | Бенчмарки производительности |
| `make benchmark-interpret` | Бенчмарки с анализом |
| `make test-load` | Нагрузочные тесты |

### Работа с фронтендом

```bash
cd web
npm run dev        # Режим разработки (http://localhost:5173)
npm run build      # Сборка для встраивания в Go
npm run lint       # Линтинг
npm run preview    # Предпросмотр сборки
```

**Стек фронтенда:**
- React 19
- TypeScript 5.9
- Vite 7.2
- Tailwind CSS 4.1
- Zustand 5.0 (state management)
- React Query 5.90

> После `npm run build` запустите `go build` для встраивания в бинарник.

---

## Конфигурация

### Переменные окружения (.env)

> **Примечание:** Конфигурация загружается через `godotenv` + `os.Getenv()` (см. `internal/config/config.go`).
> Файл `config.example.yaml` существует в корне проекта как справочник по структуре, но загрузка YAML **не реализована** -- используются только переменные окружения.
> Для секретов в production поддерживается суффикс `_FILE` (Docker secrets) для переменных `DB_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET`.

```bash
# ─── Окружение ────────────────────────────────────────
ENVIRONMENT=development        # development | production

# ─── API Server ───────────────────────────────────────
API_PORT=8080                  # Порт HTTP сервера
BASE_URL=http://localhost:8080 # Базовый URL (для ссылок-приглашений и др.)
READ_TIMEOUT=30s               # Таймаут чтения запроса
WRITE_TIMEOUT=30s              # Таймаут записи ответа
SHUTDOWN_TIMEOUT=10s           # Таймаут graceful shutdown

# ─── PostgreSQL ───────────────────────────────────────
DB_HOST=localhost
DB_PORT=5432                   # Внутренний порт PostgreSQL (по умолчанию 5432).
                               # Docker Compose маппит 5432 -> 5433 на хосте,
                               # поэтому при локальной разработке используйте DB_PORT=5433
DB_USER=tjudge
DB_PASSWORD=secret             # Поддерживает Docker secrets: DB_PASSWORD_FILE
DB_NAME=tjudge
DB_SSLMODE=disable             # disable | require | verify-full
DB_MAX_CONNECTIONS=50          # Максимальное количество соединений
DB_MAX_IDLE=10                 # Максимальное количество idle-соединений
DB_MAX_LIFETIME=1h             # Максимальное время жизни соединения

# ─── Redis ────────────────────────────────────────────
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=                # Поддерживает Docker secrets: REDIS_PASSWORD_FILE
REDIS_DB=0                     # Номер базы данных Redis
REDIS_POOL_SIZE=100            # Размер пула соединений

# ─── Worker Pool ──────────────────────────────────────
WORKER_MIN=10                  # Минимальное количество воркеров (по умолчанию 10)
WORKER_MAX=1000                # Максимальное количество воркеров (по умолчанию 1000)
WORKER_QUEUE_SIZE=10000        # Размер внутренней очереди
WORKER_TIMEOUT=90s             # Таймаут обработки матча
WORKER_RETRY_ATTEMPTS=3        # Количество повторных попыток
WORKER_RETRY_DELAY=5s          # Задержка между попытками

# ─── Executor (Docker-контейнер для матчей) ───────────
EXECUTOR_DOCKER_IMAGE=tjudge-cli:latest
EXECUTOR_TIMEOUT=60s           # Таймаут выполнения матча
EXECUTOR_CPU_QUOTA=100000      # Лимит CPU (микросекунды на 100ms)
EXECUTOR_MEMORY_LIMIT=536870912  # Лимит памяти: 512MB
EXECUTOR_PIDS_LIMIT=100        # Лимит процессов в контейнере
EXECUTOR_NETWORK_DISABLED=true # Отключить сеть в контейнере
EXECUTOR_DEFAULT_ITERATIONS=100 # Количество итераций по умолчанию

# ─── JWT (ОБЯЗАТЕЛЬНО измените в production!) ─────────
JWT_SECRET=your-secret-key-minimum-32-characters  # Поддерживает Docker secrets: JWT_SECRET_FILE
JWT_ACCESS_TTL=24h             # Время жизни access-токена (по умолчанию 24 часа)
JWT_REFRESH_TTL=168h           # Время жизни refresh-токена (по умолчанию 7 дней)

# ─── Хранилище программ ──────────────────────────────
PROGRAMS_PATH=/data/programs   # Путь хранения загруженных программ
HOST_PROGRAMS_PATH=            # Путь на хосте для Docker-in-Docker (если пусто, используется PROGRAMS_PATH)
MAX_FILE_SIZE=10485760         # Максимальный размер файла: 10MB

# ─── CORS ─────────────────────────────────────────────
CORS_ALLOWED_ORIGINS=http://localhost:3000
CORS_MAX_AGE=3600              # Время кэширования preflight-запросов (секунды)

# ─── Rate Limiting ────────────────────────────────────
RATE_LIMIT_ENABLED=false       # Отключён по умолчанию для разработки
RATE_LIMIT_RPM=100             # Запросов в минуту
RATE_LIMIT_BURST=200           # Допустимый burst

# ─── Логирование ─────────────────────────────────────
LOG_LEVEL=info                 # debug, info, warn, error
LOG_FORMAT=json                # json, console
LOG_OUTPUT=stdout              # stdout, stderr, путь к файлу
LOG_ASYNC=true                 # Асинхронное логирование (рекомендуется для production)

# ─── Метрики ─────────────────────────────────────────
METRICS_ENABLED=true
METRICS_PORT=9090
METRICS_PATH=/metrics
```

---

## Production деплой

### Управление секретами

**Development:** Переменные окружения в `.env`

**Production:** Docker Secrets

```bash
# Создание секретов
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

### Запуск в production

```bash
# Загрузка образов
docker-compose -f docker-compose.prod.yml pull

# Запуск
docker-compose -f docker-compose.prod.yml up -d

# Масштабирование воркеров
docker-compose up -d --scale worker=5

# Проверка
curl http://localhost:8080/health
```

### Blue-Green деплой

```bash
# Деплой синей версии
./scripts/blue-green-deploy.sh blue

# Переключение трафика
./scripts/switch-traffic.sh blue

# Smoke-тесты
./scripts/smoke-test.sh

# Откат при проблемах
./scripts/rollback.sh
```

### Рекомендации по ресурсам

| Компонент | CPU | RAM | Реплики |
|-----------|-----|-----|---------|
| API | 2 ядра | 2GB | 1-3 |
| Worker | 2 ядра | 2GB | 1-10 |
| PostgreSQL | 2 ядра | 4GB | 1 |
| Redis | 1 ядро | 1GB | 1 |

---

## Мониторинг

### Grafana дашборды

http://localhost:3000 (admin/admin)

- **TJudge Overview** — общая статистика
- **Workers** — очередь, воркеры, время обработки
- **API** — запросы, латентность
- **Database** — соединения, длительность запросов

### Prometheus метрики

```promql
# Матчей в очереди
tjudge_queue_size{priority="high"}

# Активных воркеров
tjudge_workers_active

# HTTP латентность (p99)
histogram_quantile(0.99, tjudge_http_request_duration_seconds_bucket)

# Обработано матчей
rate(tjudge_matches_total[5m])
```

### Loki (логирование)

http://localhost:3100

Логи собираются через Promtail и доступны в Grafana.

### Alertmanager

http://localhost:9093

Настроенные алерты в `deployments/prometheus/alerts/tjudge.yml`.

---

## CI/CD

### GitHub Actions

| Workflow | Описание |
|----------|----------|
| `ci.yml` | Lint, Test, Build, Security scan |
| `cd.yml` | Сборка и публикация Docker образов |
| `deploy.yml` | Деплой в production |
| `release.yml` | Управление релизами |

### Запуск CI локально

```bash
# Линтинг
make lint

# Тесты
make test
make test-race

# Сборка
make build
make docker-build
```

---

## Устранение неполадок

### Частые проблемы

| Проблема | Решение |
|----------|---------|
| `role tjudge does not exist` | Локальный PG на 5432, используйте 5433 для Docker |
| `air: command not found` | `go install github.com/air-verse/air@latest` + `export PATH=$PATH:~/go/bin` |
| `connection refused :8080` | Сервер не запущен, проверьте логи |
| `Internal server error` | Миграции не применены: `make migrate-up` |
| Матчи не обрабатываются | Проверьте воркер: `docker-compose logs worker` |
| `pattern all:dist: no matching files found` | Фронтенд не собран: `cd web && npm run build` |

### Диагностика

```bash
# Статус контейнеров
docker-compose ps

# Логи сервисов
docker-compose logs -f api worker

# Подключение к БД
docker exec -it tjudge-postgres psql -U tjudge -d tjudge

# Очередь Redis
docker exec tjudge-redis redis-cli LLEN queue:high

# Очистка и перезапуск
docker-compose down -v
docker-compose up -d
```

### Резервное копирование

```bash
# PostgreSQL бэкап
docker exec tjudge-postgres pg_dump -U tjudge tjudge > backup.sql

# PostgreSQL восстановление
docker exec -i tjudge-postgres psql -U tjudge -d tjudge < backup.sql

# Redis бэкап
docker exec tjudge-redis redis-cli BGSAVE
docker cp tjudge-redis:/data/dump.rdb ./backup/
```

---

## Безопасность

### Рекомендации

1. **Никогда** не коммитьте секреты в Git
2. Добавьте `secrets/` и `.env` в `.gitignore`
3. Используйте Docker Secrets в production
4. JWT secret минимум 32 символа
5. Регулярно ротируйте секреты
6. Настройте rate limiting (`RATE_LIMIT_RPM`)

### Проверка безопасности

```bash
# Сканирование зависимостей Go
go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth

# Сканирование Docker образов
docker scan tjudge-api:latest
```

---

*Версия документации: 3.1*
*Последнее обновление: Март 2026*
