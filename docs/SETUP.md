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
| `make test` | Unit тесты |
| `make test-race` | Тесты с детектором гонок |
| `make test-coverage` | Тесты с покрытием |
| `make lint` | Линтер (golangci-lint) |
| `make build` | Сборка бинарников |
| `make docker-build` | Сборка Docker образов |
| `make migrate-up` | Применить миграции |
| `make migrate-down` | Откатить миграции |
| `make admin EMAIL=x@y.z` | Назначить администратора |

### Работа с фронтендом

```bash
cd web
npm run dev        # Режим разработки (http://localhost:5173)
npm run build      # Сборка для встраивания в Go
npm run lint       # Линтинг
```

> После `npm run build` запустите `go build` для встраивания в бинарник.

---

## Конфигурация

### Переменные окружения (.env)

```bash
# Окружение
ENVIRONMENT=development

# API Server
API_PORT=8080
BASE_URL=http://localhost:8080

# PostgreSQL
DB_HOST=localhost
DB_PORT=5433          # 5433 чтобы избежать конфликта с локальным PG
DB_USER=tjudge
DB_PASSWORD=secret
DB_NAME=tjudge
DB_MAX_CONNECTIONS=50

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_POOL_SIZE=100

# Worker Pool
WORKER_MIN=10
WORKER_MAX=100
WORKER_TIMEOUT=60s

# JWT (ОБЯЗАТЕЛЬНО измените в production!)
JWT_SECRET=your-secret-key-minimum-32-characters
JWT_ACCESS_TTL=1h
JWT_REFRESH_TTL=168h

# Хранилище программ
PROGRAMS_PATH=/data/programs

# Логирование
LOG_LEVEL=info        # debug, info, warn, error
LOG_FORMAT=json       # json, console

# Метрики
METRICS_ENABLED=true
METRICS_PORT=9090
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

---

## Устранение неполадок

### Частые проблемы

| Проблема | Решение |
|----------|---------|
| `role tjudge does not exist` | Локальный PG на 5432, используйте 5433 для Docker |
| `air: command not found` | `export PATH=$PATH:~/go/bin` |
| `connection refused :8080` | Сервер не запущен, проверьте логи |
| `Internal server error` | Миграции не применены: `make migrate-up` |
| Матчи не обрабатываются | Проверьте воркер: `docker-compose logs worker` |

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
pg_dump -h localhost -U tjudge tjudge > backup.sql

# Redis бэкап
redis-cli BGSAVE
cp /data/dump.rdb /backup/
```

---

## Безопасность

1. **Никогда** не коммитьте секреты в Git
2. Добавьте `secrets/` и `.env` в `.gitignore`
3. Используйте Docker Secrets в production
4. JWT secret минимум 32 символа
5. Регулярно ротируйте секреты
6. Настройте rate limiting (`RATE_LIMIT_RPM`)
