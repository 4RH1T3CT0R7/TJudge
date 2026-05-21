# Документация TJudge

## Для пользователей

| Документ | Описание |
|----------|----------|
| **[USER_GUIDE.md](USER_GUIDE.md)** | Полное руководство: участие в турнирах, написание стратегий, правила игр |

## Для разработчиков

| Документ | Описание |
|----------|----------|
| [SETUP.md](SETUP.md) | Настройка окружения, локальная разработка |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Архитектура системы, Domain Events, компоненты, потоки данных |
| [API_GUIDE.md](API_GUIDE.md) | REST API эндпоинты, WebSocket протокол, формат ответов |
| [DATABASE_SCHEMA.md](DATABASE_SCHEMA.md) | Схема БД, таблицы, миграции, полезные запросы |
| [ADDING_GAMES.md](ADDING_GAMES.md) | Как добавить новую игру в систему |
| [PERFORMANCE_TESTING.md](PERFORMANCE_TESTING.md) | Бенчмарки, нагрузочные тесты, метрики, профилирование |

## Для эксплуатации

| Документ | Описание |
|----------|----------|
| [SELF_HOSTED.md](SELF_HOSTED.md) | Self-hosted развёртывание с Docker Compose |
| [OPERATIONS.md](OPERATIONS.md) | Production деплой, runbook, бэкапы, мониторинг, инциденты |
| [SLO.md](SLO.md) | Service Level Objectives, алерты, error budget |
| [ROADMAP.md](ROADMAP.md) | Отложенные задачи, что решили не делать сейчас и почему |

## Справочники API

| Файл | Описание |
|----------|----------|
| [openapi.yaml](openapi.yaml) | OpenAPI 3 спецификация |
| [swagger/](swagger/) | Swagger UI для просмотра API |

## Быстрые ссылки

```bash
# Запуск
docker-compose up -d

# Локальная разработка
make run-api          # API сервер
make run-worker       # Worker
cd web && npm run dev # Фронтенд

# Тестирование
make test             # Unit тесты
make test-race        # С детектором гонок
make lint             # Линтер
make benchmark        # Бенчмарки
```

| URL | Сервис |
|-----|--------|
| http://localhost:8080 | Веб-приложение |
| http://localhost:8080/api/v1 | REST API |
| http://localhost:3000 | Grafana (admin/admin) |
| http://localhost:9092 | Prometheus |

## Архитектура (обзор)

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Frontend   │────▶│     API     │────▶│  PostgreSQL  │
│  (React)    │◀────│  (Go/Chi)   │◀────│              │
└─────────────┘     └──────┬──────┘     └──────────────┘
                           │ Event Bus
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
         │      Docker (tjudge-cli)        │
         └─────────────────────────────────┘
```

Подробнее: [ARCHITECTURE.md](ARCHITECTURE.md)

## Технологии

| Backend | Frontend | Инфраструктура |
|---------|----------|----------------|
| Go 1.26 | React 19 | PostgreSQL 15 |
| Chi Router | TypeScript | Redis 7 |
| WebSocket | Tailwind CSS 4 | Docker |
| JWT + RBAC | Zustand | Prometheus/Grafana/Loki |
| Event Bus | Vite 7.2 | Alertmanager |
