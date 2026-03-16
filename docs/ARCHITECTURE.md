# Архитектура TJudge

## Обзор

TJudge построен на принципах Clean Architecture с чётким разделением на слои:

1. **API Server** (`cmd/api`) — HTTP/WebSocket эндпоинты, middleware, маршрутизация
2. **Worker Pool** (`cmd/worker`) — обработка матчей с автомасштабированием
3. **Domain** (`internal/domain`) — бизнес-логика, не зависящая от инфраструктуры
4. **Events** (`internal/events`) — синхронная шина событий для декаплинга side-effects
5. **Infrastructure** (`internal/infrastructure`) — PostgreSQL, Redis, Docker, файловое хранилище

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Frontend   │────▶│  API Server │────▶│  PostgreSQL  │
│  (React)    │◀────│   (Go/Chi)  │◀────│              │
└─────────────┘     └──────┬──────┘     └──────────────┘
       ▲                   │
       │ WebSocket         │ Event Bus
       └───────────────────┤
                     ┌─────▼─────┐
                     │   Redis   │
                     │ кэш+очередь│
                     └─────┬─────┘
                           │
               ┌───────────┼───────────┐
               ▼           ▼           ▼
         ┌─────────┐ ┌─────────┐ ┌─────────┐
         │ Worker  │ │ Worker  │ │ Worker  │
         └────┬────┘ └────┬────┘ └────┬────┘
              │           │           │
         ┌────▼───────────▼───────────▼────┐
         │    Docker контейнеры            │
         │    (tjudge-cli, Rust)           │
         └─────────────────────────────────┘
```

---

## Структура проекта

```
tjudge/
├── cmd/
│   ├── api/              # Точка входа API сервера
│   ├── worker/           # Точка входа Worker сервиса
│   ├── migrations/       # Инструмент миграций БД
│   └── benchmark/        # Интерпретатор бенчмарков
├── internal/
│   ├── api/              # HTTP слой
│   │   ├── handlers/     #   Обработчики запросов
│   │   ├── httputil/     #   Общие HTTP-утилиты (WriteJSON, WriteError)
│   │   ├── middleware/   #   Auth, rate limiting, CORS, CSRF, logging
│   │   ├── batch/        #   Batch API
│   │   └── routes.go     #   Определение маршрутов
│   ├── config/           # Конфигурация приложения (env vars через godotenv)
│   ├── domain/           # Бизнес-логика (чистый слой)
│   │   ├── auth/         #   JWT, логин, права доступа
│   │   ├── rating/       #   Расчёт ELO рейтингов
│   │   ├── tournament/   #   Управление турнирами, round-robin
│   │   ├── team/         #   Управление командами
│   │   ├── game/         #   Управление играми
│   │   └── models.go     #   Доменные сущности
│   ├── events/           # Шина событий (Domain Events)
│   │   ├── bus.go        #   Bus interface, SyncBus, NoopBus
│   │   ├── events.go     #   Типы событий (structs)
│   │   └── handlers/     #   Обработчики событий
│   │       ├── cache.go  #     Инвалидация кэша
│   │       └── broadcast.go  # WebSocket рассылка
│   ├── infrastructure/   # Внешние сервисы
│   │   ├── cache/        #   Redis кэширование + distributed locks
│   │   ├── db/           #   PostgreSQL репозитории
│   │   ├── executor/     #   Docker исполнитель матчей
│   │   ├── queue/        #   Приоритетная очередь (Redis)
│   │   └── storage/      #   Файловое хранилище программ
│   ├── websocket/        # Real-time обновления (WebSocket Hub)
│   ├── worker/           # Пул воркеров с автомасштабированием
│   └── web/              # Встроенный фронтенд (embed.go)
├── pkg/                  # Общие утилиты (без зависимости на internal)
│   ├── errors/           #   AppError, предопределённые ошибки
│   ├── logger/           #   Структурированное логирование (zap)
│   ├── metrics/          #   Prometheus метрики
│   ├── pagination/       #   Курсорная пагинация
│   └── validator/        #   Валидация входных данных
├── web/                  # React фронтенд (React 19, TypeScript, Tailwind CSS 4)
├── migrations/           # SQL миграции (29 шт.)
├── tests/
│   ├── e2e/              # End-to-end тесты (18 тестов)
│   ├── integration/      # Интеграционные тесты (PostgreSQL + Redis)
│   ├── benchmark/        # Бенчмарки производительности
│   ├── load/             # Нагрузочные тесты
│   ├── performance/      # Тесты специфических сценариев
│   └── chaos/            # Хаос-тесты устойчивости
├── deployments/          # Prometheus, Grafana, Loki, K8s конфиги
├── scripts/              # Деплой, бэкапы, blue-green
├── docker/               # Dockerfiles
└── docs/                 # Документация
```

---

## Потоки данных

### Турнирный жизненный цикл

```
1. Админ создаёт турнир
   API → TournamentService.Create → DB
   → Event: TournamentCreated → [Cache.Set]

2. Админ добавляет игры к турниру
   API → TournamentService.AddGame → DB (tournament_games)

3. Команды регистрируются и загружают программы
   API → TeamService.Create → DB (teams, team_members)
   API → ProgramService.Upload → Storage + DB (programs)

4. Команды присоединяются к турниру (загружают программу)
   API → TournamentService.Join → Distributed Lock → DB
   → Event: ParticipantJoined → [Cache.Invalidate, Leaderboard.UpdateRating]

5. Админ стартует раунд игры
   API → TournamentService.StartGameRound → Round-robin пары → Queue
   → Event: TournamentStarted → [Cache.Invalidate, WS.Broadcast]
   → Event: MatchesCreated → [WS.Broadcast]

6. Воркеры обрабатывают матчи
   Worker → Queue.Dequeue → Docker Executor → tjudge-cli
   → RatingService.ProcessMatchResult → DB (rating_history)
   → Event: MatchResultProcessed → [Leaderboard.Update, WS.Broadcast]

7. Админ завершает турнир
   API → TournamentService.Complete → DB
   → Event: TournamentCompleted → [Cache.Invalidate, WS.Broadcast]
```

### Исполнение матча (детально)

```
                        ┌──────────────┐
                        │ Redis Queue  │
                        │ (prioritized)│
                        └──────┬───────┘
                               │ Dequeue
                        ┌──────▼───────┐
                        │    Worker    │
                        │  Processor   │
                        └──────┬───────┘
                               │
                    ┌──────────▼──────────┐
                    │   Docker Executor   │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │  tjudge-cli   │  │
                    │  │  (Rust)       │  │
                    │  │               │  │
                    │  │  Program1.py  │  │
                    │  │  vs           │  │
                    │  │  Program2.py  │  │
                    │  └───────┬───────┘  │
                    │          │ stdout   │
                    └──────────┼──────────┘
                               │ Parse result
                    ┌──────────▼──────────┐
                    │  Rating Service     │
                    │  (ELO calculation)  │
                    │                     │
                    │  → DB update        │
                    │  → Event publish    │
                    └─────────────────────┘
```

---

## Domain Events

### Проблема

Сервисы (Tournament, Rating) напрямую вызывали cache invalidation, leaderboard update, WebSocket broadcast. Каждый метод "помнил" обо всех side-effects. Добавление нового side-effect требовало правки всех методов во всех сервисах.

### Решение

In-process синхронная шина событий (`internal/events/`). Сервисы эмитят события, обработчики реагируют. Side-effects декаплены от бизнес-логики.

### Архитектура

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────────────┐
│ TournamentService│───▶│   SyncBus    │───▶│ TournamentCacheHandler│
│                 │     │   (events)   │     │ LeaderboardCacheHandler│
│ RatingService   │───▶│              │───▶│ BroadcastHandler      │
└─────────────────┘     └──────────────┘     └─────────────────────┘
      Publish                                     Handle
```

### Bus interface

```go
type Handler interface {
    Handle(ctx context.Context, event any) error
}

type Bus interface {
    Publish(ctx context.Context, event any)
    Subscribe(handler Handler, eventTypes ...any)
}
```

**SyncBus** — основная реализация. `Publish` вызывает подписчиков синхронно. Ошибки обработчиков логируются (ERROR), но не прерывают цепочку — side-effects не блокируют основной flow.

**NoopBus** — для unit-тестов (ничего не делает).

### Типы событий

| Событие | Кто публикует | Данные |
|---------|---------------|--------|
| `TournamentCreated` | TournamentService.Create | Tournament object |
| `TournamentStarted` | TournamentService.Start | TournamentID, Status |
| `TournamentCompleted` | TournamentService.Complete | TournamentID, Status |
| `TournamentDeleted` | TournamentService.Delete | TournamentID |
| `ParticipantJoined` | TournamentService.Join | TournamentID, ProgramID, InitialRating |
| `MatchesCreated` | TournamentService.ScheduleMatches | TournamentID, ProgramID, MatchCount |
| `MatchResultProcessed` | RatingService.ProcessMatchResult | TournamentID, MatchID, Program IDs, Ratings, Winner |
| `GameRoundReset` | GameRoundHandler.ResetGameRound | TournamentID, GameID |

### Обработчики

**TournamentCacheHandler** — инвалидация кэша турниров:

| Событие | Действие |
|---------|----------|
| TournamentCreated | `tournamentCache.Set(tournament)` |
| TournamentStarted | `tournamentCache.Invalidate(id)` |
| TournamentCompleted | `tournamentCache.Invalidate(id)` |
| TournamentDeleted | `tournamentCache.Invalidate(id)` + `leaderboardCache.Clear(id)` |
| ParticipantJoined | `tournamentCache.Invalidate(id)` |
| GameRoundReset | `tournamentCache.Invalidate(id)` + `leaderboardCache.Clear(id)` |

**LeaderboardCacheHandler** — обновление кэша лидерборда:

| Событие | Действие |
|---------|----------|
| ParticipantJoined | `leaderboardCache.UpdateRating(tid, pid, initialRating)` + `InvalidateFullLeaderboard(tid)` |
| MatchResultProcessed | `leaderboardCache.UpdateRating` x2 + `InvalidateFullLeaderboard(tid)` |
| GameRoundReset | `leaderboardCache.Clear(tid)` |

**BroadcastHandler** — WebSocket рассылка:

| Событие | WebSocket тип |
|---------|---------------|
| TournamentStarted | `tournament_update` |
| TournamentCompleted | `tournament_update` |
| MatchesCreated | `matches_created` |
| MatchResultProcessed | `match_result` |

### Что НЕ является событием

| Операция | Причина |
|----------|---------|
| Cache-aside чтение (GetByID → cache.Get/Set) | Read-path, не side-effect |
| Leaderboard чтение (GetLeaderboard → cache.Get/Set + singleflight) | Read-path |
| Очередь матчей (EnqueueBatch) | Основная бизнес-логика |
| Запись в БД | Основная бизнес-логика |

### Wiring

**API Server** (`cmd/api/main.go`):
```go
eventBus := events.NewSyncBus(log)

// Все обработчики
eventBus.Subscribe(cacheHandler, TournamentCreated{}, TournamentStarted{}, ...)
eventBus.Subscribe(leaderboardHandler, ParticipantJoined{}, MatchResultProcessed{}, ...)
eventBus.Subscribe(broadcastHandler, TournamentStarted{}, MatchResultProcessed{}, ...)

tournamentService := tournament.NewService(..., eventBus, ...)
ratingService := rating.NewService(repo, eventBus, log)
```

**Worker** (`cmd/worker/main.go`):
```go
eventBus := events.NewSyncBus(log)

// Worker: только cache handlers (нет WebSocket)
eventBus.Subscribe(leaderboardHandler, MatchResultProcessed{})

ratingService := rating.NewService(repo, eventBus, log)
```

---

## Ключевые компоненты

### API Server (`cmd/api`)

Chi роутер со стеком middleware:

```
Request → RequestID → RealIP → Logger → Recoverer → SecureHeaders → Compress
  → SmartTimeout → RateLimit → CORS → [MaxBodySize] → [Auth/OptionalAuth] → [RBAC] → Handler
```

**Глобальные middleware** (применяются ко всем запросам):

| Middleware | Файл | Описание |
|------------|------|----------|
| RequestID | chi (встроенный) | Генерация уникального ID запроса |
| RealIP | chi (встроенный) | Определение реального IP за прокси |
| Logger | chi (встроенный) | Логирование запросов |
| Recoverer | chi (встроенный) | Восстановление после паник |
| SecureHeaders | `security.go` | Заголовки безопасности (X-Frame-Options, CSP, HSTS, X-Content-Type-Options и др.) |
| Compress | `compress.go` | gzip-сжатие ответов |
| SmartTimeout | `timeout.go` | Адаптивный таймаут по типу операции (5с кэш, 10с обычные, 15с БД, 30с тяжёлые, без таймаута WebSocket) |
| RateLimit | `ratelimit.go` | Rate limiting с Redis + in-memory fallback (2x лимит при падении Redis) |
| CORS | go-chi/cors | Cross-origin resource sharing, настройки из конфига |

**Per-route middleware** (применяются к группам маршрутов):

| Middleware | Файл | Описание |
|------------|------|----------|
| MaxBodySize | `bodylimit.go` | Ограничение размера тела запроса (1MB для JSON, 10MB для загрузки программ) |
| Auth | `auth.go` | Обязательная JWT аутентификация (Bearer token или WebSocket subprotocol) |
| OptionalAuth | `auth.go` | Опциональная аутентификация (показывает больше информации администраторам) |
| RBAC | `rbac.go` | Проверка ролей (user, admin) |

- Единый формат ответов через `httputil.WriteJSON` (envelope `{"data": ...}`)
- WebSocket Hub для real-time обновлений
- CSRF middleware (`csrf.go`) не активирован: JWT через Authorization header, не cookies

### Worker Pool (`internal/worker`)

- Динамическое масштабирование (мин: 10, макс: 1000 по умолчанию)
- Приоритетная очередь (HIGH → MEDIUM → LOW)
- Exponential backoff retry
- Graceful shutdown + recovery при панике

**Автомасштабирование:**

| Размер очереди | Действие |
|----------------|----------|
| > 100 задач | +10 воркеров |
| > 50 задач | +5 воркеров |
| < 10 задач и >50% простаивают | -5 воркеров |

### Docker Executor (`internal/infrastructure/executor`)

Sandbox-ограничения для безопасного исполнения пользовательских программ:

| Ресурс | Ограничение |
|--------|-------------|
| Сеть | `--network none` (отключена) |
| Память | 512MB лимит |
| CPU | 100ms на 100ms период |
| Файловая система | read-only |
| Таймаут | 60 секунд |
| Процессы | максимум 100 |
| Безопасность | Seccomp + AppArmor профили |

### Cache (`internal/infrastructure/cache`)

Многоуровневая система кэширования на Redis:

| Компонент | TTL | Описание |
|-----------|-----|----------|
| `tournament_cache.go` | 5 мин | Кэш данных турниров (JSON) |
| `leaderboard_cache.go` | 30 сек | Sorted set + JSON кэш лидерборда |
| `match_cache.go` | 24ч | Результаты матчей |
| `ratelimiter.go` | настр. | Rate limiting per-IP |
| `token_blacklist.go` | TTL токена | Blacklist JWT при logout |
| `distributed_lock.go` | настр. | Mutex для конкурентных операций |
| `warmer.go` | — | Прогрев кэша при старте |

### Файловое хранилище (`internal/infrastructure/storage`)

Управление файлами загруженных программ:

- Хранение исходного кода программ на файловой системе (`/data/programs` по умолчанию)
- Ограничение размера файла (10MB по умолчанию, настраивается)
- Поддержка `HostProgramsPath` для Docker-in-Docker окружений
- Генерация уникальных путей через UUID
- Безопасная работа с файлами: проверка путей, атомарная запись

### Batch API (`internal/api/batch`)

Пакетное выполнение нескольких API запросов в одном HTTP вызове:

- Объединение до N запросов в один HTTP запрос (настраивается через `MaxRequests`)
- Параллельное выполнение с индивидуальными таймаутами (`RequestTimeout`)
- Фильтрация по разрешённым методам и путям (`AllowedMethods`, `AllowedPaths`)
- Каждый запрос в пакете содержит `id`, `method`, `path`, `headers`, `body`
- Ответ содержит массив результатов с сохранением `id` для сопоставления

### Конфигурация (`internal/config`)

Централизованная загрузка конфигурации из переменных окружения:

- Загрузка через `godotenv` (из `.env` файла при наличии)
- Структуры конфигурации: Server, Database, Redis, Worker, Executor, Storage, JWT, Logging, Metrics, CORS, RateLimit
- Значения по умолчанию для всех параметров
- Валидация конфигурации при загрузке

### База данных (`internal/infrastructure/db`)

- Connection pooling (макс 100 соединений)
- Prepared statements для частых запросов
- Optimistic locking (поле `version`) для конкурентных обновлений
- Партиционирование таблицы `matches` (помесячно)
- Материализованные представления для лидербордов

---

## Конкурентность и безопасность

| Операция | Механизм защиты |
|----------|-----------------|
| Присоединение к турниру | Distributed lock (Redis) |
| Старт раунда турнира | Distributed lock + optimistic lock |
| Обработка результатов матчей | Atomic DB updates + event bus |
| WebSocket broadcast | RWMutex в Hub |
| Записи в БД | PostgreSQL транзакции |
| Rate limiting | Redis (основной) + in-memory fallback |

---

## Масштабирование

### Горизонтальное

- **API**: Stateless серверы за балансировщиком
- **Workers**: Масштабирование по размеру очереди (автомасштабирование)
- **БД**: Read replicas (опционально)
- **Redis**: Cluster mode (опционально)

### Вертикальное

- Worker pool автомасштабируется 10 → 1000 по нагрузке (настраивается через `WORKER_MIN`/`WORKER_MAX`)
- Connection pool БД настраивается через `DB_MAX_CONNECTIONS`

---

## Доменные сущности

```go
// internal/domain/models.go — основные сущности

type User struct {
    ID, Username, Email, PasswordHash, Role string
}

type Tournament struct {
    ID              uuid.UUID
    Name            string
    Description     string
    Status          TournamentStatus    // "pending" | "active" | "completed"
    MaxTeamSize     int
    MaxParticipants *int                // nil = без ограничений
    CreatorID       *uuid.UUID
    IsPerpetual     bool
    Version         int                 // optimistic lock
    Games           []TournamentGame
}

type Team struct {
    ID, TournamentID, Name, InviteCode string
    LeaderID uuid.UUID
    Members  []User
}

type Program struct {
    ID, TeamID, GameID           uuid.UUID
    Name, Language, FilePath     string
    Status                       string  // "pending" | "ready" | "error"
}

type Match struct {
    ID, TournamentID, GameID     uuid.UUID
    Program1ID, Program2ID       uuid.UUID
    Winner                       *int    // 1, 2, 0 (ничья), nil (не завершён)
    Status                       string  // "pending" | "running" | "completed" | "failed"
    Score1, Score2               int
    Version                      int
}

type Game struct {
    ID              uuid.UUID
    Slug            string              // "prisoners_dilemma"
    Name            string
    Rules           string              // Markdown
    ScoreMultiplier float64
}
```

### Поддерживаемые игры

| Slug | Название | Тип | Описание |
|------|----------|-----|----------|
| `prisoners_dilemma` | Дилемма заключённого | Одновременная | Классическая игра: сотрудничать (C) или предать (D). Матрица выплат 0/1/3/5 |
| `tug_of_war` | Перетягивание каната | Одновременная | Распределение энергии (100 ед.), смещение каната, 50 раундов |
| `travelers_dilemma` | Дилемма путешественника | Одновременная | Заявки в диапазоне [2, 100], бонус/штраф R=2 за меньшую/большую заявку |
| `public_goods` | Общественное благо | Одновременная | Вклад в общий пул (20 токенов), множитель m=1.5, дилемма безбилетника |
| `dollar_auction` | Аукцион двойной цены | Поочерёдная | Торги за приз P=100, оба платят свои ставки, ловушка эскалации |

---

## Обработка ошибок

```go
// pkg/errors — типизированные ошибки с HTTP кодами
var (
    ErrNotFound         // 404
    ErrUnauthorized     // 401
    ErrForbidden        // 403
    ErrValidation       // 400
    ErrConflict         // 409
    ErrInternal         // 500
    ErrRateLimitExceeded // 429
)
```

Все API ответы оборачиваются в единый формат:

```json
// Успех
{"data": { ... }}

// Ошибка
{"error": {"code": "NOT_FOUND", "message": "..."}}
```

---

## WebSocket протокол

### Подключение

```
WS /api/v1/ws/tournaments/{id}?token=<jwt>
```

### Типы сообщений

| Тип | Описание | Источник |
|-----|----------|----------|
| `leaderboard_update` | Обновление рейтинга/позиции | BroadcastHandler |
| `match_result` | Результат матча | BroadcastHandler (MatchResultProcessed) |
| `matches_created` | Новые матчи созданы | BroadcastHandler (MatchesCreated) |
| `tournament_update` | Изменение статуса турнира | BroadcastHandler |
| `round_update` | Изменение статуса раунда | Прямой вызов |

---

## Метрики (Prometheus)

```promql
# HTTP
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

---

## Тестирование

| Уровень | Расположение | Количество | Описание |
|---------|-------------|------------|----------|
| Unit | `*_test.go` рядом с исходниками | ~1200 | Бизнес-логика, handlers, middleware, cache, worker |
| DB Integration | `tests/integration/` | ~60 | PostgreSQL репозитории (RUN_INTEGRATION=true) |
| E2E | `tests/e2e/` | 18 | HTTP API через запущенный сервер |
| Benchmark | `tests/benchmark/` | — | Производительность компонентов |
| Load | `tests/load/` | — | Нагрузочное тестирование API |
| Chaos | `tests/chaos/` | — | Устойчивость к сбоям |

```bash
make test               # Unit тесты (~1200)
make test-race          # С детектором гонок
make test-integration   # DB интеграционные
make test-e2e           # End-to-end
make benchmark          # Бенчмарки
```

---

*Версия документации: 3.1*
*Последнее обновление: Март 2026*
