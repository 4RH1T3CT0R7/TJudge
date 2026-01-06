# Архитектура

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
│   └── migrations/   # Миграции БД
├── internal/
│   ├── api/          # HTTP хендлеры, middleware, маршруты
│   ├── domain/       # Бизнес-логика
│   │   ├── auth/     # JWT, логин, права доступа
│   │   ├── rating/   # Расчёт ELO
│   │   └── tournament/ # Логика турниров
│   ├── infrastructure/
│   │   ├── cache/    # Операции с Redis
│   │   ├── db/       # Репозитории PostgreSQL
│   │   ├── executor/ # Исполнение матчей в Docker
│   │   └── queue/    # Приоритетная очередь
│   ├── websocket/    # Real-time обновления
│   └── worker/       # Управление пулом воркеров
├── pkg/              # Общие утилиты
│   ├── errors/       # Кастомные ошибки
│   ├── logger/       # Структурированное логирование
│   ├── metrics/      # Prometheus метрики
│   └── validator/    # Валидация входных данных
├── deployments/      # Docker, K8s конфиги
└── tests/            # Интеграционные, E2E, хаос-тесты
```

## Потоки данных

### Турнирный поток
```
1. Пользователь создаёт турнир → API валидирует → БД сохраняет
2. Пользователи присоединяются → Distributed lock → Участник добавлен
3. Админ стартует турнир → Генерация round-robin расписания → Матчи в очередь
4. Воркеры забирают → Исполнение в Docker → Результаты в БД → Обновление рейтингов
5. WebSocket рассылает обновления → Клиенты получают real-time данные
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
- JWT аутентификация + RBAC
- Rate limiting (100 запросов/мин)
- WebSocket для real-time обновлений

### Worker Pool (`internal/worker`)
- Динамическое масштабирование (мин: 5, макс: 100+)
- Приоритетная очередь (HIGH → MEDIUM → LOW)
- Exponential backoff retry
- Graceful shutdown

### Docker Executor (`internal/infrastructure/executor`)
Ограничения безопасности:
- Сеть: отключена
- Память: лимит 256MB
- CPU: 1 ядро
- Файловая система: read-only
- Таймаут: 60 сек
- Seccomp/AppArmor профили

### База данных (`internal/infrastructure/db`)
- Connection pooling (макс 100)
- Prepared statements
- Optimistic locking (поле version)
- Партиционирование таблицы matches

### Кэш (`internal/infrastructure/cache`)
- Результаты матчей: TTL 24ч
- Таблицы лидеров: TTL 30 сек
- Distributed locks для конкурентности
- Прогрев кэша при старте

## Конкурентность

| Операция | Защита |
|----------|--------|
| Присоединение к турниру | Distributed lock |
| Старт турнира | Distributed lock + optimistic lock |
| Обработка матчей | Atomic counters |
| WebSocket broadcast | RWMutex |
| Запись в БД | Транзакции |

## Масштабирование

### Горизонтальное
- API: Stateless, масштабирование репликами
- Workers: Масштабирование по размеру очереди
- БД: Read replicas (опционально)
- Redis: Cluster mode (опционально)

### Вертикальное
- Worker pool автомасштабируется 5→100+ по нагрузке
- Connection pool БД настраивается динамически

## Метрики

Основные Prometheus метрики:
- `tjudge_http_requests_total` — Количество API запросов
- `tjudge_http_request_duration_seconds` — Гистограмма латентности
- `tjudge_queue_size` — Ожидающие матчи
- `tjudge_active_workers` — Текущие воркеры
- `tjudge_matches_total` — Обработанные матчи
- `tjudge_cache_hits_total` — Попадания в кэш

## Обработка ошибок

```
pkg/errors определяет:
- ErrNotFound (404)
- ErrUnauthorized (401)
- ErrForbidden (403)
- ErrValidation (400)
- ErrConflict (409)
- ErrInternal (500)
```

Все ошибки оборачиваются с контекстом для отладки.
