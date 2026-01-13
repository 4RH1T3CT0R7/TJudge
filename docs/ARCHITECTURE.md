# Архитектура TJudge

## Обзор

TJudge построен на принципах Clean Architecture и состоит из трёх основных компонентов:

1. **API Server** — HTTP/WebSocket эндпоинты
2. **Worker Pool** — Исполнение матчей с автомасштабированием
3. **Executor** — Изолированные Docker-контейнеры

## Структура проекта

```
tjudge/
├── cmd/
│   ├── api/          # Точка входа API сервера
│   ├── worker/       # Точка входа Worker сервиса
│   ├── migrations/   # Миграции БД
│   └── benchmark/    # Интерпретатор бенчмарков
├── internal/
│   ├── api/          # HTTP хендлеры, middleware, маршруты
│   ├── domain/       # Бизнес-логика
│   │   ├── auth/     # JWT, логин, права доступа
│   │   ├── rating/   # Расчёт ELO
│   │   ├── tournament/ # Логика турниров
│   │   ├── team/     # Логика команд
│   │   ├── game/     # Логика игр
│   │   └── models.go # Доменные сущности
│   ├── infrastructure/
│   │   ├── cache/    # Операции с Redis
│   │   ├── db/       # Репозитории PostgreSQL
│   │   ├── executor/ # Исполнение матчей в Docker
│   │   ├── queue/    # Приоритетная очередь
│   │   └── storage/  # Файловое хранилище программ
│   ├── websocket/    # Real-time обновления
│   └── worker/       # Управление пулом воркеров
├── pkg/              # Общие утилиты
│   ├── errors/       # Кастомные ошибки
│   ├── logger/       # Структурированное логирование (zap)
│   ├── metrics/      # Prometheus метрики
│   ├── pagination/   # Курсорная пагинация
│   └── validator/    # Валидация входных данных
├── deployments/      # Docker, K8s, Prometheus конфиги
└── tests/            # Интеграционные, E2E, нагрузочные, хаос-тесты
```

## Потоки данных

### Турнирный поток

```
1. Админ создаёт турнир → API валидирует → БД сохраняет
2. Админ добавляет игры → Связь tournament_games создаётся
3. Команды присоединяются → Distributed lock → Команда добавлена
4. Команды загружают программы → Программы сохраняются в storage
5. Админ стартует раунд игры → Генерация round-robin расписания → Матчи в очередь
6. Воркеры забирают → Исполнение в Docker → Результаты в БД → Обновление рейтингов
7. WebSocket рассылает обновления → Клиенты получают real-time данные
```

### Исполнение матчей

```
Очередь (Redis) → Worker → Docker Executor → Результат → БД + Кэш
     ↑                                              │
     └──────────── Retry при ошибке ────────────────┘
```

## Ключевые компоненты

### API Server (`cmd/api`)

- Chi роутер со стеком middleware
- JWT аутентификация + RBAC (роли: user, admin)
- Rate limiting (настраивается)
- CSRF защита
- Сжатие ответов (gzip)
- WebSocket для real-time обновлений

**Middleware стек:**
```
Request → Recovery → Logger → CORS → Compress → RateLimit → Auth → RBAC → Handler
```

### Worker Pool (`internal/worker`)

- Динамическое масштабирование (мин: 2, макс: 100+)
- Приоритетная очередь (HIGH → MEDIUM → LOW)
- Exponential backoff retry
- Graceful shutdown
- Recovery при панике

**Автомасштабирование:**
| Размер очереди | Действие |
|----------------|----------|
| > 100 задач | +10 воркеров |
| > 50 задач | +5 воркеров |
| < 10 задач и >50% простаивают | -5 воркеров |

### Docker Executor (`internal/infrastructure/executor`)

Ограничения безопасности:
- Сеть: отключена (`--network none`)
- Память: лимит 512MB
- CPU: 100ms на 100ms период
- Файловая система: read-only
- Таймаут: 60 сек
- Процессы: максимум 100
- Seccomp/AppArmor профили

### База данных (`internal/infrastructure/db`)

- Connection pooling (макс 100 соединений)
- Prepared statements
- Optimistic locking (поле version)
- Партиционирование таблицы matches (помесячно)
- Материализованные представления для лидербордов

**Репозитории:**
- `user_repository.go` — пользователи
- `team_repository.go` — команды
- `program_repository.go` — программы
- `tournament_repository.go` — турниры
- `game_repository.go` — игры
- `match_repository.go` — матчи
- `rating_repository.go` — рейтинги

### Кэш (`internal/infrastructure/cache`)

- Результаты матчей: TTL 24ч
- Таблицы лидеров: TTL 30 сек
- Distributed locks для конкурентности
- Прогрев кэша при старте
- Token blacklist для logout

**Компоненты:**
- `cache.go` — основной кэш
- `leaderboard_cache.go` — кэш лидербордов
- `match_cache.go` — кэш матчей
- `ratelimiter.go` — rate limiting
- `token_blacklist.go` — blacklist JWT
- `distributed_lock.go` — распределённые блокировки
- `warmer.go` — прогрев кэша

## Конкурентность

| Операция | Защита |
|----------|--------|
| Присоединение к команде | Distributed lock |
| Старт раунда турнира | Distributed lock + optimistic lock |
| Обработка матчей | Atomic counters |
| WebSocket broadcast | RWMutex |
| Запись в БД | Транзакции |

## Масштабирование

### Горизонтальное

- **API**: Stateless, масштабирование репликами (за балансировщиком)
- **Workers**: Масштабирование по размеру очереди
- **БД**: Read replicas (опционально)
- **Redis**: Cluster mode (опционально)

### Вертикальное

- Worker pool автомасштабируется 2→100+ по нагрузке
- Connection pool БД настраивается динамически

## Доменные сущности

```go
// Основные сущности (internal/domain/models.go)

type User struct {
    ID           uuid.UUID
    Username     string
    Email        string
    PasswordHash string
    Role         string // "user" | "admin"
}

type Team struct {
    ID           uuid.UUID
    TournamentID uuid.UUID
    Name         string
    InviteCode   string
    LeaderID     uuid.UUID
    Members      []User
}

type Game struct {
    ID              uuid.UUID
    Slug            string // "prisoners_dilemma"
    Name            string
    Rules           string // Markdown
    ScoreMultiplier float64
}

type Tournament struct {
    ID          uuid.UUID
    Name        string
    Description string
    Status      string // "pending" | "active" | "completed"
    MaxTeamSize int
    Games       []TournamentGame
}

type TournamentGame struct {
    TournamentID uuid.UUID
    GameID       uuid.UUID
    IsActive     bool
    RoundStatus  string // "pending" | "running" | "completed"
    RoundNumber  int
}

type Program struct {
    ID       uuid.UUID
    TeamID   uuid.UUID
    GameID   uuid.UUID
    Name     string
    Language string
    FilePath string
    Status   string // "pending" | "compiling" | "ready" | "error"
}

type Match struct {
    ID           uuid.UUID
    TournamentID uuid.UUID
    GameID       uuid.UUID
    Program1ID   uuid.UUID
    Program2ID   uuid.UUID
    WinnerID     *uuid.UUID
    Status       string // "pending" | "running" | "completed" | "failed"
    Score1       int
    Score2       int
    RoundNumber  int
    ErrorCode    *string
    ErrorMessage *string
}
```

## Метрики

Основные Prometheus метрики:

```promql
# HTTP метрики
tjudge_http_requests_total{method, path, status}
tjudge_http_request_duration_seconds{method, path}

# Очередь
tjudge_queue_size{priority}
tjudge_queue_wait_time_seconds{priority}

# Воркеры
tjudge_active_workers
tjudge_worker_pool_size

# Матчи
tjudge_matches_total{status, game_type}
tjudge_match_duration_seconds{game_type}
tjudge_matches_in_progress

# Кэш
tjudge_cache_hits_total{cache_type}
tjudge_cache_misses_total{cache_type}

# База данных
tjudge_db_query_duration_seconds{query_type}
tjudge_db_connections{state}
```

## Обработка ошибок

```go
// pkg/errors определяет:
var (
    ErrNotFound     = errors.New("not found")           // 404
    ErrUnauthorized = errors.New("unauthorized")        // 401
    ErrForbidden    = errors.New("forbidden")           // 403
    ErrValidation   = errors.New("validation error")    // 400
    ErrConflict     = errors.New("conflict")            // 409
    ErrInternal     = errors.New("internal error")      // 500
)
```

Все ошибки оборачиваются с контекстом для отладки.

## WebSocket протокол

### Подключение

```
WS /api/v1/ws/tournaments/{id}?token=<jwt>
```

### Типы сообщений

```json
// Обновление лидерборда
{
    "type": "leaderboard_update",
    "payload": {
        "game_id": "uuid",
        "entries": [
            {"rank": 1, "team_name": "Team1", "rating": 1650}
        ]
    }
}

// Обновление матча
{
    "type": "match_update",
    "payload": {
        "match_id": "uuid",
        "status": "completed",
        "score1": 1500,
        "score2": 1200
    }
}

// Обновление статуса раунда
{
    "type": "round_update",
    "payload": {
        "game_id": "uuid",
        "round_status": "running",
        "round_number": 2
    }
}
```

---

*Версия документации: 2.0*
*Последнее обновление: Январь 2026*
