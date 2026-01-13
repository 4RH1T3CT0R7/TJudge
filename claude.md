# TJudge - Турнирная система для теории игр

## Обзор проекта

TJudge — высокопроизводительная турнирная система для соревнований по программированию в теории игр. Программы-стратегии соревнуются друг с другом в различных играх (Дилемма заключённого, Перетягивание каната и др.).

**Стек:** Go 1.24, PostgreSQL 15, Redis 7, Docker, React 19 + Tailwind CSS 4

## Архитектура

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Frontend   │────▶│     API     │────▶│   Worker    │
│  (React)    │     │   Server    │     │    Pool     │
└─────────────┘     └─────────────┘     └─────────────┘
                           │                   │
                           ▼                   ▼
                    ┌─────────────┐     ┌─────────────┐
                    │ PostgreSQL  │     │ tjudge-cli  │
                    │   + Redis   │     │  (Docker)   │
                    └─────────────┘     └─────────────┘
```

### Компоненты

| Компонент | Путь | Описание |
|-----------|------|----------|
| API Server | `cmd/api/` | REST API + WebSocket, JWT авторизация |
| Worker | `cmd/worker/` | Обработка матчей, автомасштабируемый пул |
| Frontend | `web/` | React SPA, встраивается в Go бинарник |
| tjudge-cli | Внешний | Rust исполнитель игр (Docker образ) |

## Ключевые файлы

### Точки входа
- `cmd/api/main.go` — Запуск API сервера, инъекция зависимостей
- `cmd/worker/main.go` — Запуск пула воркеров
- `cmd/migrations/main.go` — Запуск миграций БД
- `cmd/benchmark/main.go` — Интерпретатор бенчмарков

### Доменная логика
- `internal/domain/models.go` — Основные сущности (Tournament, Match, Program, User, Team, Game)
- `internal/domain/tournament/service.go` — Управление турнирами, генерация round-robin
- `internal/domain/auth/service.go` — Сервис аутентификации
- `internal/domain/rating/elo.go` — Расчёт рейтингов ELO
- `internal/domain/team/service.go` — Управление командами
- `internal/domain/game/service.go` — Управление играми

### Инфраструктура
- `internal/infrastructure/db/` — PostgreSQL репозитории
- `internal/infrastructure/cache/` — Слой кэширования Redis
- `internal/infrastructure/queue/` — Очередь матчей (Redis)
- `internal/infrastructure/executor/executor.go` — Docker исполнитель матчей
- `internal/worker/pool.go` — Пул воркеров с автомасштабированием

### API
- `internal/api/routes.go` — Определение маршрутов
- `internal/api/handlers/` — HTTP обработчики
- `internal/api/middleware/` — Авторизация, rate limiting, логирование

### Frontend
- `web/src/` — Исходный код React приложения
- `internal/web/embed.go` — Встраивание фронтенда в бинарник

## Схема базы данных

### Основные таблицы
```sql
users                    -- Аккаунты пользователей (username, email, password_hash, role)
teams                    -- Команды (name, tournament_id, invite_code, leader_id)
programs                 -- Загруженные программы (team_id, code, language, status)
games                    -- Определения игр (slug, name, rules, score_multiplier)
tournaments              -- Определения турниров (name, description, status, max_team_size)
tournament_games         -- Связь турниров и игр (tournament_id, game_id, is_active, round_status)
matches                  -- Записи матчей (tournament_id, game_id, program1_id, program2_id, result)
rating_history           -- История рейтингов (team_id, game_id, rating, delta)
```

### Материализованные представления
```sql
leaderboard_global       -- Глобальные рейтинги
leaderboard_tournament   -- Рейтинги по турнирам
```

Миграции: `migrations/000001_*.sql` до `migrations/000022_*.sql`

## Поток выполнения матчей

1. **Создание турнира**: Админ создаёт турнир, добавляет игры
2. **Регистрация команд**: Команды присоединяются, загружают программы для каждой игры
3. **Генерация матчей**: Round-robin пары (1 матч на пару)
4. **Очередь**: Матчи добавляются в приоритетную очередь Redis
5. **Обработка воркерами**:
   - Воркер берёт матч из очереди
   - Запускает Docker контейнер с tjudge-cli
   - Передаёт программы через монтирование volume
   - tjudge-cli выполняет итерации (параметр `-i`)
6. **Обработка результатов**: Обновление ELO рейтингов, WebSocket рассылка

```go
// Генерация матчей (упрощённо из tournament/service.go)
for i := 0; i < len(participants); i++ {
    for j := i + 1; j < len(participants); j++ {
        // 1 матч на пару, итерации внутри tjudge-cli
        createMatch(participants[i], participants[j])
    }
}
```

## Конфигурация

### Переменные окружения
```bash
# База данных
DB_HOST=localhost
DB_PORT=5432
DB_USER=tjudge
DB_PASSWORD=secret
DB_NAME=tjudge

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# JWT
JWT_SECRET=your-32-char-secret-minimum

# Worker
WORKER_MIN=2
WORKER_MAX=10

# Сервер
SERVER_PORT=8080
LOG_LEVEL=info
```

## Команды разработки

```bash
# Настройка
make docker-up          # Запуск PostgreSQL, Redis
make migrate-up         # Применить миграции

# Разработка
make run-api            # Запуск API сервера
make run-worker         # Запуск воркера

# Тестирование
make test               # Unit тесты
make test-race          # С детектором гонок
make benchmark          # Бенчмарки производительности
make benchmark-interpret # Бенчмарки с анализом

# Сборка
make build              # Сборка бинарников
make docker-build       # Сборка Docker образов

# Утилиты
make lint               # Запуск golangci-lint
make admin EMAIL=x@y.z  # Назначить администратора
```

## API эндпоинты

### Авторизация
- `POST /api/v1/auth/register` — Регистрация
- `POST /api/v1/auth/login` — Вход (возвращает JWT)
- `POST /api/v1/auth/refresh` — Обновление токена
- `GET /api/v1/auth/me` — Текущий пользователь

### Турниры
- `GET /api/v1/tournaments` — Список турниров
- `POST /api/v1/tournaments` — Создать турнир (админ)
- `GET /api/v1/tournaments/:id` — Получить турнир
- `POST /api/v1/tournaments/:id/start` — Запустить турнир
- `GET /api/v1/tournaments/:id/leaderboard` — Таблица лидеров

### Команды
- `POST /api/v1/teams` — Создать команду
- `POST /api/v1/teams/join` — Присоединиться по коду
- `GET /api/v1/teams/:id` — Получить команду
- `POST /api/v1/teams/:id/leave` — Покинуть команду

### Игры
- `GET /api/v1/games` — Список игр
- `POST /api/v1/games` — Создать игру (админ)
- `GET /api/v1/games/:id` — Получить игру
- `PUT /api/v1/games/:id` — Обновить игру

### Программы
- `POST /api/v1/programs` — Загрузить программу
- `GET /api/v1/programs` — Список программ пользователя
- `GET /api/v1/programs/:id` — Детали программы

### WebSocket
- `WS /api/v1/ws/tournaments/:id` — Real-time обновления (лидерборд, результаты матчей)

## Тестирование

### Unit тесты
Расположены рядом с исходными файлами: `*_test.go`

### Бенчмарки
`tests/benchmark/` — Бенчмарки производительности

```bash
make benchmark-interpret  # Запуск с интерпретацией
go run ./cmd/benchmark -standards  # Показать ожидаемые значения
```

### Интеграционные/E2E тесты
`tests/integration/`, `tests/e2e/`

## Стандарты производительности

| Операция | Ожидаемое | Категория |
|----------|-----------|-----------|
| Health Endpoint | 50µs | API |
| Список турниров | 5ms | API |
| Лидерборд | 10ms | API |
| Добавление в очередь | 500µs | Очередь |
| Создание матча (БД) | 2ms | БД |
| Worker Pool (100 матчей) | 100ms | Worker |

Запустите `make benchmark-interpret` для полного анализа.

## Docker

### Образы
- `tjudge-api` — API сервер
- `tjudge-worker` — Worker сервис
- `tjudge-cli` — Исполнитель игр (Rust)

### Сервисы docker-compose.yml
- `api` — API сервер
- `worker` — Пул воркеров
- `postgres` — База данных
- `redis` — Кэш + Очередь
- `prometheus` — Метрики
- `grafana` — Дашборды
- `loki` — Логирование
- `alertmanager` — Оповещения

## CI/CD

### GitHub Actions
- `.github/workflows/ci.yml` — Lint, Test, Build, Security scan
- `.github/workflows/cd.yml` — Сборка и публикация Docker образов
- `.github/workflows/deploy.yml` — Деплой в production
- `.github/workflows/release.yml` — Управление релизами

### Деплой
Docker Compose деплой с blue-green скриптами в `scripts/`.

## Частые проблемы

### "package requires newer Go version"
Убедитесь, что Go 1.24+ установлен. Проверьте `go version`.

### "pattern all:dist: no matching files found"
Фронтенд не собран. Выполните `cd web && npm run build` или `make docker-build`.

### "resource temporarily unavailable" в worker
Уменьшите количество воркеров: `WORKER_MIN=2 WORKER_MAX=5`

### Проблемы подключения к БД
Проверьте `DB_HOST`, `DB_PORT`, `DB_PASSWORD` в `.env`.

## Структура файлов

```
TJudge/
├── cmd/                    # Точки входа
│   ├── api/                # API сервер
│   ├── worker/             # Worker сервис
│   ├── migrations/         # Инструмент миграций
│   └── benchmark/          # Интерпретатор бенчмарков
├── internal/
│   ├── api/                # HTTP слой
│   │   ├── handlers/       # Обработчики запросов
│   │   ├── middleware/     # Middleware
│   │   └── routes.go       # Определение маршрутов
│   ├── domain/             # Бизнес-логика
│   │   ├── auth/           # Аутентификация
│   │   ├── tournament/     # Сервис турниров
│   │   ├── rating/         # Расчёт ELO
│   │   ├── team/           # Сервис команд
│   │   ├── game/           # Сервис игр
│   │   └── models.go       # Доменные сущности
│   ├── infrastructure/     # Внешние сервисы
│   │   ├── db/             # PostgreSQL
│   │   ├── cache/          # Redis
│   │   ├── queue/          # Очередь матчей
│   │   └── executor/       # Docker исполнитель
│   ├── worker/             # Пул воркеров
│   ├── websocket/          # Real-time обновления
│   └── web/                # Встроенный фронтенд
├── web/                    # React фронтенд
├── migrations/             # SQL миграции
├── docker/                 # Dockerfiles
├── tests/                  # Тестовые наборы
│   ├── benchmark/          # Тесты производительности
│   ├── integration/        # Интеграционные тесты
│   ├── e2e/                # End-to-end тесты
│   ├── load/               # Нагрузочные тесты
│   ├── performance/        # Тесты производительности
│   └── chaos/              # Хаос-тесты
├── scripts/                # Скрипты деплоя
├── deployments/            # Конфиги развёртывания
├── docs/                   # Документация
├── docker-compose.yml      # Локальная разработка
└── Makefile                # Команды сборки
```

## Полезные запросы

```sql
-- Проверка ожидающих матчей
SELECT COUNT(*) FROM matches WHERE status = 'pending';

-- Лидерборд турнира
SELECT * FROM leaderboard_tournament WHERE tournament_id = 'uuid';

-- Обновление материализованных представлений
REFRESH MATERIALIZED VIEW CONCURRENTLY leaderboard_global;

-- Программы команды с рейтингами
SELECT p.name, rh.rating, rh.wins, rh.losses
FROM programs p
JOIN rating_history rh ON p.team_id = rh.team_id
WHERE p.team_id = 'uuid';

-- Команды в турнире
SELECT t.name, t.invite_code, u.username as leader
FROM teams t
JOIN users u ON t.leader_id = u.id
WHERE t.tournament_id = 'uuid';
```

## Мониторинг

- Prometheus: `http://localhost:9092`
- Grafana: `http://localhost:3000` (admin/admin)
- API Метрики: `http://localhost:8080/metrics`
- Loki (логи): `http://localhost:3100`

## Документация

| Документ | Описание |
|----------|----------|
| `docs/SETUP.md` | Настройка, разработка, деплой |
| `docs/USER_GUIDE.md` | Руководство пользователя, правила игр |
| `docs/ARCHITECTURE.md` | Детальная архитектура |
| `docs/API_GUIDE.md` | Полный справочник API |
| `docs/DATABASE_SCHEMA.md` | Схема базы данных |
| `docs/PERFORMANCE_TESTING.md` | Тестирование производительности |

## Ссылки

- API Server: `http://localhost:8080`
- WebSocket: `ws://localhost:8080/api/v1/ws/tournaments/:id`
- PostgreSQL: `localhost:5433` (в Docker)
- Redis: `localhost:6379`
