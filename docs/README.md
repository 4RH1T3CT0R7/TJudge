# Документация TJudge

## Для пользователей

| Документ | Описание |
|----------|----------|
| **[USER_GUIDE.md](USER_GUIDE.md)** | Полное руководство: участие в турнирах, написание стратегий, правила игр |

## Для разработчиков

| Документ | Описание |
|----------|----------|
| [SETUP.md](SETUP.md) | Настройка окружения, локальная разработка, деплой |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Архитектура системы, компоненты, потоки данных |
| [API_GUIDE.md](API_GUIDE.md) | REST API эндпоинты, WebSocket |
| [DATABASE_SCHEMA.md](DATABASE_SCHEMA.md) | Схема БД, миграции |

## Быстрые ссылки

```bash
# Запуск
docker-compose up -d

# Локальная разработка
make run-api      # API сервер
make run-worker   # Worker
cd web && npm run dev  # Фронтенд

# Тестирование
make test
make lint
```

| URL | Сервис |
|-----|--------|
| http://localhost:8080 | Веб-приложение |
| http://localhost:8080/api/v1 | REST API |
| http://localhost:3000 | Grafana (admin/admin) |

## Архитектура

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Frontend   │────▶│     API     │────▶│  PostgreSQL │
│  (React)    │◀────│    (Go)     │◀────│             │
└─────────────┘     └──────┬──────┘     └─────────────┘
                          │
                    ┌─────▼─────┐
                    │   Redis   │
                    │ (очередь) │
                    └─────┬─────┘
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
        ┌─────────┐ ┌─────────┐ ┌─────────┐
        │ Worker  │ │ Worker  │ │ Worker  │
        └────┬────┘ └────┬────┘ └────┬────┘
             │           │           │
        ┌────▼───────────▼───────────▼────┐
        │      Docker (tjudge-cli)        │
        └─────────────────────────────────┘
```

## Технологии

| Backend | Frontend | Инфраструктура |
|---------|----------|----------------|
| Go 1.24 | React 18 | PostgreSQL 15 |
| Chi Router | TypeScript | Redis 7 |
| WebSocket | Tailwind CSS 4 | Docker |
| pgx/sqlx | Zustand | Prometheus/Grafana |
