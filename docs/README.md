# Документация TJudge

Добро пожаловать в документацию турнирной системы TJudge!

## Содержание

### Для пользователей

- [Быстрый старт](../README.md#-быстрый-старт) — запуск системы за 5 минут
- [Возможности](../README.md#-возможности) — обзор функционала

### Для разработчиков

| Документ | Описание |
|----------|----------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Архитектура системы, компоненты, взаимодействия |
| [API_GUIDE.md](API_GUIDE.md) | Полное описание REST API и WebSocket |
| [DEVELOPMENT_GUIDE.md](DEVELOPMENT_GUIDE.md) | Настройка окружения разработки |
| [DATABASE_SCHEMA.md](DATABASE_SCHEMA.md) | Схема базы данных, миграции |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Правила контрибьютинга |

### Для DevOps

| Документ | Описание |
|----------|----------|
| [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md) | Деплой в Docker/Kubernetes |
| [SECRETS_MANAGEMENT.md](SECRETS_MANAGEMENT.md) | Управление секретами |

---

## Быстрые ссылки

### Запуск

```bash
# Docker Compose (рекомендуется)
docker-compose up -d

# Локально
make run-api      # API сервер
make run-worker   # Worker
cd web && npm run dev  # Фронтенд
```

### Основные URL

| Сервис | URL | Описание |
|--------|-----|----------|
| Веб-приложение | http://localhost:8080 | Основной интерфейс |
| API | http://localhost:8080/api/v1 | REST API |
| Grafana | http://localhost:3000 | Мониторинг |
| Prometheus | http://localhost:9092 | Метрики |

### Полезные команды

```bash
make help          # Справка по командам
make test          # Запуск тестов
make lint          # Проверка кода
make migrate-up    # Применить миграции
make docker-logs   # Просмотр логов
```

---

## Структура проекта

```
tjudge/
├── cmd/                    # Точки входа приложений
│   ├── api/               # API сервер
│   ├── worker/            # Worker для обработки матчей
│   └── migrations/        # CLI для миграций
├── internal/              # Внутренний код
│   ├── api/              # HTTP handlers, middleware, routes
│   ├── domain/           # Бизнес-логика
│   ├── infrastructure/   # Внешние сервисы (DB, Redis, Docker)
│   ├── worker/           # Пул воркеров
│   ├── web/              # Встроенный фронтенд (embed)
│   └── config/           # Конфигурация
├── web/                  # React фронтенд
│   ├── src/             # Исходный код
│   └── dist/            # Собранное приложение
├── pkg/                  # Переиспользуемые пакеты
├── migrations/           # SQL миграции
├── docker/              # Dockerfiles
├── deployments/         # Конфигурации деплоя
│   ├── grafana/        # Дашборды Grafana
│   ├── prometheus/     # Конфиг Prometheus
│   ├── loki/           # Конфиг Loki
│   └── alertmanager/   # Алерты
└── docs/                # Документация
```

---

## Технологии

### Backend

| Технология | Версия | Назначение |
|------------|--------|------------|
| Go | 1.24+ | Язык программирования |
| Chi | v5 | HTTP роутер |
| pgx | v5 | PostgreSQL драйвер |
| go-redis | v9 | Redis клиент |
| zap | — | Структурированное логирование |
| gorilla/websocket | — | WebSocket |

### Frontend

| Технология | Версия | Назначение |
|------------|--------|------------|
| React | 18+ | UI фреймворк |
| TypeScript | 5+ | Типизация |
| Tailwind CSS | 4 | Стилизация |
| Vite | 7 | Сборщик |
| Zustand | — | Стейт менеджмент |
| React Query | — | Работа с данными |

### Инфраструктура

| Технология | Версия | Назначение |
|------------|--------|------------|
| PostgreSQL | 15+ | База данных |
| Redis | 7+ | Кэш, очереди |
| Docker | — | Контейнеризация |
| Prometheus | — | Метрики |
| Grafana | — | Визуализация |
| Loki | — | Логи |

---

## Архитектура кратко

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Клиент    │────▶│     API     │────▶│  PostgreSQL │
│  (React)    │◀────│   (Go)      │◀────│             │
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
        │ (Go)    │ │ (Go)    │ │ (Go)    │
        └────┬────┘ └────┬────┘ └────┬────┘
             │           │           │
        ┌────▼────────────▼───────────▼────┐
        │         Docker Containers        │
        │  (изолированное выполнение)      │
        └──────────────────────────────────┘
```

### Поток данных

1. **Пользователь** загружает программу через веб-интерфейс
2. **API сервер** сохраняет программу и создаёт задачи на матчи
3. **Redis** хранит очередь матчей
4. **Worker** забирает матч из очереди
5. **Docker Executor** запускает матч в изолированном контейнере
6. **Результат** сохраняется в PostgreSQL
7. **WebSocket** уведомляет клиентов об обновлении рейтинга

---

## Поддержка

- **Issues**: [GitHub Issues](https://github.com/bmstu-itstech/tjudge/issues)
- **Discussions**: [GitHub Discussions](https://github.com/bmstu-itstech/tjudge/discussions)

---

## Лицензия

MIT License
