# TJudge - Руководство разработчика

## Быстрый старт

### 1. Требования

- Go 1.24+
- Docker & Docker Compose
- Make

### 2. Запуск окружения разработки

```bash
# Запустить базу данных и кэш
docker compose up -d postgres redis

# Применить миграции
make migrate-up

# Запустить API сервер с hot reload
make dev
```

Сервер доступен по адресу: http://localhost:8080

### 3. Проверка работоспособности

```bash
curl http://localhost:8080/health
# Вывод: OK
```

---

## Команды Make

| Команда | Описание |
|---------|----------|
| `make dev` | Запуск API с hot reload (air) |
| `make build` | Сборка всех бинарников |
| `make test` | Запуск unit тестов |
| `make test-integration` | Запуск интеграционных тестов |
| `make test-e2e` | Запуск E2E тестов |
| `make lint` | Запуск линтера (golangci-lint) |
| `make migrate-up` | Применить миграции БД |
| `make migrate-down` | Откатить миграции |
| `make docker-up` | Запустить все сервисы в Docker |
| `make docker-down` | Остановить Docker сервисы |
| `make clean` | Удалить артефакты сборки |

---

## API Эндпоинты

### Health Check
```
GET /health
```

### Аутентификация

```bash
# Регистрация нового пользователя
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "email": "player1@example.com",
    "password": "SecurePass123!"
  }'

# Ответ:
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "user": {
    "id": "uuid",
    "username": "player1",
    "email": "player1@example.com"
  }
}
```

```bash
# Вход
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "password": "SecurePass123!"
  }'
```

```bash
# Обновление токена
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "eyJ..."}'
```

```bash
# Получить текущего пользователя (требуется авторизация)
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>"
```

```bash
# Выход
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

### Турниры

```bash
# Список турниров (публичный)
curl http://localhost:8080/api/v1/tournaments
curl "http://localhost:8080/api/v1/tournaments?game_type=tictactoe&status=active&limit=10"

# Получить турнир по ID (публичный)
curl http://localhost:8080/api/v1/tournaments/{id}

# Таблица лидеров (публичный)
curl http://localhost:8080/api/v1/tournaments/{id}/leaderboard

# Матчи турнира (публичный)
curl http://localhost:8080/api/v1/tournaments/{id}/matches
```

```bash
# Создать турнир (требуется авторизация)
curl -X POST http://localhost:8080/api/v1/tournaments \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Мой турнир",
    "description": "Описание турнира",
    "game_type": "tictactoe",
    "max_participants": 16
  }'
```

```bash
# Присоединиться к турниру (требуется авторизация)
curl -X POST http://localhost:8080/api/v1/tournaments/{id}/join \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"program_id": "uuid-вашей-программы"}'
```

```bash
# Запустить турнир (требуется авторизация, только организатор)
curl -X POST http://localhost:8080/api/v1/tournaments/{id}/start \
  -H "Authorization: Bearer <access_token>"
```

### Программы (боты)

Все эндпоинты программ требуют авторизации.

```bash
# Создать программу
curl -X POST http://localhost:8080/api/v1/programs \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Мой бот",
    "code_path": "path/to/code",
    "language": "python",
    "game_type": "tictactoe"
  }'
```

```bash
# Список ваших программ
curl http://localhost:8080/api/v1/programs \
  -H "Authorization: Bearer <access_token>"

# Получить программу по ID
curl http://localhost:8080/api/v1/programs/{id} \
  -H "Authorization: Bearer <access_token>"

# Обновить программу
curl -X PUT http://localhost:8080/api/v1/programs/{id} \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "Новое имя бота"}'

# Удалить программу
curl -X DELETE http://localhost:8080/api/v1/programs/{id} \
  -H "Authorization: Bearer <access_token>"
```

### Матчи

```bash
# Список матчей
curl http://localhost:8080/api/v1/matches
curl "http://localhost:8080/api/v1/matches?tournament_id={id}&status=completed"

# Получить матч по ID
curl http://localhost:8080/api/v1/matches/{id}

# Статистика
curl http://localhost:8080/api/v1/matches/statistics
```

### WebSocket (обновления в реальном времени)

```javascript
// Подключение к обновлениям турнира
const ws = new WebSocket('ws://localhost:8080/api/v1/ws/tournaments/{tournament_id}');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Обновление:', data);
};
```

---

## Конфигурация

Конфигурация загружается из переменных окружения. Создайте файл `.env`:

```env
# Окружение
ENVIRONMENT=development

# API Сервер
API_PORT=8080
READ_TIMEOUT=30s
WRITE_TIMEOUT=30s

# База данных (PostgreSQL)
DB_HOST=localhost
DB_PORT=5433          # 5433 чтобы избежать конфликта с локальным PostgreSQL
DB_USER=tjudge
DB_PASSWORD=secret
DB_NAME=tjudge
DB_MAX_CONNECTIONS=10

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT
JWT_SECRET=ваш-секретный-ключ-измените-в-продакшене
JWT_ACCESS_TTL=1h
JWT_REFRESH_TTL=168h

# Логирование
LOG_LEVEL=debug       # debug, info, warn, error
LOG_FORMAT=console    # console, json

# Rate Limiting
RATE_LIMIT_ENABLED=false
```

---

## Частые проблемы и решения

### 1. "role tjudge does not exist"

**Проблема:** Локальный PostgreSQL работает на порту 5432, конфликтуя с Docker.

**Решение:**
```bash
# Проверить что на порту 5432
lsof -i :5432

# Если работает локальный postgres, используйте порт 5433 для Docker
# В docker-compose.yml:
ports:
  - "5433:5432"

# В .env:
DB_PORT=5433
```

### 2. "air: command not found"

**Проблема:** Air не в PATH.

**Решение:**
```bash
# Air установлен в ~/go/bin
export PATH=$PATH:~/go/bin

# Или добавьте в .zshrc / .bashrc
echo 'export PATH=$PATH:~/go/bin' >> ~/.zshrc
```

### 3. Миграции не работают

**Проблема:** База данных не запущена или неверные credentials.

**Решение:**
```bash
# Проверить что postgres работает
docker compose ps

# Проверить подключение
docker exec tjudge-postgres pg_isready -U tjudge

# Перезапустить контейнеры
docker compose down -v
docker compose up -d postgres redis
```

### 4. "connection refused" на localhost:8080

**Проблема:** Сервер не запущен или упал.

**Решение:**
```bash
# Проверить что занимает порт 8080
lsof -i :8080

# Проверить логи сервера в терминале где запущен make dev
# Искать FATAL или ERROR сообщения
```

### 5. "Internal server error" на API запросах

**Проблема:** Обычно отсутствуют таблицы в БД.

**Решение:**
```bash
# Запустить миграции
make migrate-up

# Проверить статус миграций
docker exec tjudge-postgres psql -U tjudge -d tjudge -c "\dt"
```

### 6. Docker контейнеры unhealthy

**Проблема:** Сервис не смог запуститься.

**Решение:**
```bash
# Проверить логи
docker compose logs postgres
docker compose logs redis

# Полный сброс
docker compose down -v
docker compose up -d postgres redis
```

---

## Структура проекта

```
TJudge/
├── cmd/
│   ├── api/            # Точка входа API сервера
│   ├── worker/         # Точка входа фонового воркера
│   └── migrations/     # Инструмент миграций
├── internal/
│   ├── api/            # HTTP handlers, routes, middleware
│   ├── config/         # Загрузка конфигурации
│   ├── domain/         # Бизнес-логика (auth, tournament, rating)
│   ├── infrastructure/ # Реализации DB, cache, queue
│   ├── websocket/      # WebSocket hub и клиенты
│   └── worker/         # Обработка фоновых задач
├── pkg/                # Общие пакеты (logger, errors, metrics)
├── migrations/         # SQL файлы миграций
├── tests/
│   ├── integration/    # Интеграционные тесты
│   ├── e2e/            # End-to-end тесты
│   └── chaos/          # Chaos/stress тесты
├── docker/             # Dockerfiles
├── deployments/        # Kubernetes, Prometheus конфиги
└── docs/               # Документация
```

---

## Тестирование

```bash
# Unit тесты
make test

# С покрытием
make test-coverage

# Интеграционные тесты (требуется запущенная БД)
RUN_INTEGRATION=true make test-integration

# E2E тесты (требуется запущенный сервер)
make test-e2e

# Линтер
make lint
```

---

## Метрики и мониторинг

Эндпоинт метрик: http://localhost:9090/metrics

Доступные метрики:
- `tjudge_http_requests_total` - Количество HTTP запросов
- `tjudge_http_request_duration_seconds` - Латентность запросов
- `tjudge_matches_total` - Обработанные матчи
- `tjudge_match_duration_seconds` - Время выполнения матча
- `tjudge_queue_size` - Размер очереди по приоритетам
- `tjudge_active_workers` - Количество активных воркеров
- `tjudge_cache_hits_total` / `tjudge_cache_misses_total` - Статистика кэша

---

## Полный Docker деплой

Для запуска всего в Docker:

```bash
# Запустить все сервисы
docker compose up -d

# Сервисы:
# - postgres (5433)
# - redis (6379)
# - api (8080)
# - worker
# - prometheus (9090)
# - grafana (3000)
```

Grafana: http://localhost:3000 (admin/admin)

---

## Веб-интерфейс и админка

**ВАЖНО:** Текущая версия проекта - это **только API (бэкенд)**.

Веб-интерфейс и админ-панель **не реализованы**.

Для полноценного веб-приложения необходимо создать:
1. **Frontend** (React/Vue/Next.js) - пользовательский интерфейс
2. **Admin Panel** - панель администратора для управления турнирами и пользователями

Если требуется фронтенд, его нужно разработать отдельно, подключив к существующему API.
