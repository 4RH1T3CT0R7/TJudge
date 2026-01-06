# TJudge - High-Load Tournament System

Высоконагруженная система для проведения турниров по программированию игр. Поддерживает автоматизированное проведение матчей между программами-ботами, расчёт рейтингов и управление турнирами.

## Возможности

- **Высокая производительность**: обработка 100+ матчей в секунду
- **Масштабируемость**: горизонтальное масштабирование воркеров
- **Изоляция**: безопасное выполнение пользовательского кода в Docker контейнерах
- **Рейтинговая система**: расчёт рейтингов по системе ELO
- **Real-time обновления**: WebSocket для мгновенного получения результатов
- **Мониторинг**: интеграция с Prometheus и Grafana

## Архитектура

```
API Server ─┬─> PostgreSQL (данные)
            ├─> Redis (кэш, очереди)
            └─> Worker Pool ──> Docker (изоляция tjudge-cli)
```

## Стек технологий

- **Backend**: Go 1.21+
- **Database**: PostgreSQL 15+
- **Cache**: Redis 7+
- **Monitoring**: Prometheus, Grafana
- **Containerization**: Docker
- **API**: REST + WebSocket

## Быстрый старт

### Предварительные требования

- Go 1.21 или выше
- Docker и Docker Compose
- Make (опционально, для удобства)
- PostgreSQL 15+ (или через Docker)
- Redis 7+ (или через Docker)

### Установка

1. Клонируйте репозиторий:
```bash
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
```

2. Установите зависимости:
```bash
make deps
# или
go mod download
```

3. Создайте файл конфигурации:
```bash
cp config.example.yaml config.yaml
# Отредактируйте config.yaml под ваши нужды
```

4. Создайте `.env` файл:
```bash
cat > .env << EOF
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=tjudge
DB_PASSWORD=secret
DB_NAME=tjudge

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# JWT
JWT_SECRET=your-secret-key-change-in-production

# Server
API_PORT=8080
WORKER_COUNT=10
EOF
```

### Запуск через Docker Compose (рекомендуется)

```bash
# Запустить все сервисы
make docker-up

# Применить миграции
make migrate-up

# Посмотреть логи
make docker-logs

# Остановить сервисы
make docker-down
```

API будет доступен на `http://localhost:8080`

### Локальный запуск (для разработки)

1. Запустите PostgreSQL и Redis:
```bash
docker-compose up -d postgres redis
```

2. Примените миграции:
```bash
make migrate-up
```

3. Запустите API сервер:
```bash
make run-api
```

4. В другом терминале запустите воркер:
```bash
make run-worker
```

## Разработка

### Структура проекта

```
tjudge/
├── cmd/                    # Точки входа приложений
│   ├── api/               # API сервер
│   ├── worker/            # Worker процесс
│   └── migrations/        # CLI для миграций
├── internal/              # Внутренняя логика
│   ├── api/              # HTTP handlers, middleware, routes
│   ├── domain/           # Бизнес-логика (auth, tournament, match, rating)
│   ├── infrastructure/   # DB, cache, queue, executor
│   ├── worker/           # Worker pool
│   └── config/           # Конфигурация
├── pkg/                  # Переиспользуемые пакеты
│   ├── logger/          # Структурированное логирование
│   ├── metrics/         # Prometheus метрики
│   ├── errors/          # Кастомные ошибки
│   └── validator/       # Валидация данных
├── migrations/           # SQL миграции
├── tests/               # Тесты
│   ├── integration/    # Интеграционные тесты
│   ├── e2e/           # E2E тесты
│   └── load/          # Нагрузочные тесты (k6)
├── docker/             # Dockerfiles
└── deployments/        # Конфигурации для развертывания
```

### Основные команды

```bash
# Помощь
make help

# Сборка
make build

# Тесты
make test                  # Все тесты
make test-race            # С детектором гонок
make test-coverage        # С покрытием
make test-integration     # Интеграционные
make test-e2e            # E2E тесты

# Линтинг
make lint

# Форматирование
make fmt

# Запуск
make run-api             # API сервер
make run-worker          # Worker
make dev                 # С hot reload (air)

# Docker
make docker-build        # Собрать образы
make docker-up          # Запустить
make docker-down        # Остановить
make docker-logs        # Логи

# Миграции
make migrate-up         # Применить
make migrate-down       # Откатить
make migrate-create     # Создать новую

# Безопасность
make security           # Проверка уязвимостей
```

### Создание миграции

```bash
make migrate-create
# Введите имя: create_users_table
# Будут созданы файлы в migrations/
```

### Работа с Docker

Проект использует multi-stage builds для оптимизации размера образов.

```bash
# Собрать образы
docker build -t tjudge/api:latest -f docker/api/Dockerfile .
docker build -t tjudge/worker:latest -f docker/worker/Dockerfile .

# Запустить контейнер API
docker run -p 8080:8080 --env-file .env tjudge/api:latest
```

## API Endpoints

### Authentication
- `POST /api/v1/auth/register` - Регистрация пользователя
- `POST /api/v1/auth/login` - Вход
- `POST /api/v1/auth/refresh` - Обновление токена

### Programs
- `POST /api/v1/programs` - Загрузка программы
- `GET /api/v1/programs` - Список программ
- `GET /api/v1/programs/:id` - Детали программы
- `PUT /api/v1/programs/:id` - Обновление программы
- `DELETE /api/v1/programs/:id` - Удаление программы

### Tournaments
- `POST /api/v1/tournaments` - Создание турнира
- `GET /api/v1/tournaments` - Список турниров
- `GET /api/v1/tournaments/:id` - Детали турнира
- `POST /api/v1/tournaments/:id/join` - Регистрация в турнире
- `POST /api/v1/tournaments/:id/start` - Начало турнира
- `GET /api/v1/tournaments/:id/matches` - Матчи турнира
- `GET /api/v1/tournaments/:id/leaderboard` - Таблица лидеров

### Matches
- `GET /api/v1/matches/:id` - Детали матча

### WebSocket
- `WS /api/v1/ws/tournaments/:id` - Real-time обновления турнира

## Конфигурация

Конфигурация загружается из:
1. `config.yaml` - основная конфигурация
2. `.env` - переменные окружения (приоритет выше)

### Пример config.yaml

```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 10s

database:
  host: localhost
  port: 5432
  user: tjudge
  password: secret
  name: tjudge
  max_connections: 50
  max_idle: 10
  max_lifetime: 1h

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  pool_size: 100

worker:
  min_workers: 10
  max_workers: 1000
  queue_size: 10000
  timeout: 30s
  retry_attempts: 3

executor:
  tjudge_path: /path/to/tjudge-cli
  timeout: 30s
  cpu_quota: 100000
  memory_limit: 536870912  # 512MB
  pids_limit: 100

jwt:
  secret: your-secret-key
  access_ttl: 15m
  refresh_ttl: 7d

logging:
  level: info
  format: json
```

## Мониторинг

### Метрики (Prometheus)

Метрики доступны на `/metrics`:

- `tjudge_matches_total` - Общее количество матчей
- `tjudge_match_duration_seconds` - Длительность матча
- `tjudge_queue_size` - Размер очереди
- `tjudge_active_workers` - Активные воркеры
- `tjudge_http_requests_total` - HTTP запросы
- `tjudge_http_request_duration_seconds` - Латентность API

### Grafana

Дашборды доступны на `http://localhost:3000` (при запуске через docker-compose):

- **Overview**: общая статистика системы
- **Workers**: статистика воркеров и очередей
- **Database**: производительность БД
- **API**: метрики HTTP API

### Логи

Логи в структурированном JSON формате:

```json
{
  "level": "info",
  "ts": "2026-01-05T19:30:00.000Z",
  "msg": "match completed",
  "match_id": "123e4567-e89b-12d3-a456-426614174000",
  "duration_ms": 15234,
  "winner": 1
}
```

## Тестирование

### Unit тесты

```bash
go test ./internal/domain/rating/...
```

### Интеграционные тесты

```bash
make test-integration
```

### E2E тесты

```bash
make test-e2e
```

### Нагрузочное тестирование

```bash
make test-load
```

Используется k6 для нагрузочного тестирования. Скрипты находятся в `tests/load/`.

## Развертывание

### Docker Compose (staging/dev)

```bash
docker-compose -f docker-compose.prod.yml up -d
```

### Kubernetes (production)

```bash
kubectl apply -f deployments/k8s/namespace.yaml
kubectl apply -f deployments/k8s/
```

## Безопасность

- Все пользовательские программы выполняются в изолированных Docker контейнерах
- Ограничение ресурсов (CPU, память, процессы)
- Network isolation для контейнеров
- JWT аутентификация для API
- Rate limiting на API endpoints
- Input validation и sanitization

## Производительность

Целевые метрики:
- **Throughput**: 100+ матчей в секунду
- **API Latency**: p99 < 200ms
- **Match Execution**: < 30 секунд
- **Availability**: 99.9%

## Лицензия

MIT License

## Контрибьюторы

- [BMSTU ITSTech](https://github.com/bmstu-itstech)

## Связь

- Issues: [GitHub Issues](https://github.com/bmstu-itstech/tjudge/issues)
- Документация: [claude.md](./claude.md)
- Чек-лист разработки: [checklist.md](./checklist.md)
