# TJudge

Высокопроизводительная турнирная система для соревнований программных ботов.

## Быстрый старт

```bash
# Клонирование и запуск
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
make dev

# API: http://localhost:8080
# Grafana: http://localhost:3000 (admin/admin)
```

## Возможности

- **Управление турнирами** — Round-robin турниры с рейтингом ELO
- **Исполнение матчей** — Изолированные Docker-контейнеры с лимитами ресурсов
- **Real-time обновления** — WebSocket для отслеживания прогресса турнира
- **Высокая производительность** — 100+ матчей/сек, автомасштабирование воркеров
- **Мониторинг** — Prometheus метрики, Grafana дашборды, Loki логи

## Архитектура

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Клиент    │────▶│     API     │────▶│  PostgreSQL │
└─────────────┘     └──────┬──────┘     └─────────────┘
                          │
                    ┌─────▼─────┐
                    │   Redis   │
                    │ (Очередь) │
                    └─────┬─────┘
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
        ┌─────────┐ ┌─────────┐ ┌─────────┐
        │ Worker  │ │ Worker  │ │ Worker  │
        └────┬────┘ └────┬────┘ └────┬────┘
             │           │           │
        ┌────▼────┐ ┌────▼────┐ ┌────▼────┐
        │ Docker  │ │ Docker  │ │ Docker  │
        │Executor │ │Executor │ │Executor │
        └─────────┘ └─────────┘ └─────────┘
```

## Команды

```bash
make dev          # Запуск через Docker Compose
make test         # Запуск всех тестов
make build        # Сборка бинарников
make lint         # Запуск линтера
make migrate-up   # Применение миграций
```

## Конфигурация

Основные переменные окружения:

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `DB_HOST` | localhost | Хост PostgreSQL |
| `REDIS_HOST` | localhost | Хост Redis |
| `JWT_SECRET` | — | Ключ подписи JWT (обязательно) |
| `WORKER_MIN` | 5 | Минимум воркеров |
| `WORKER_MAX` | 100 | Максимум воркеров |

Полный список в [.env.example](.env.example).

## Обзор API

| Метод | Endpoint | Описание |
|-------|----------|----------|
| POST | `/api/v1/auth/register` | Регистрация |
| POST | `/api/v1/auth/login` | Вход |
| POST | `/api/v1/tournaments` | Создание турнира |
| POST | `/api/v1/tournaments/:id/join` | Присоединение к турниру |
| POST | `/api/v1/tournaments/:id/start` | Старт турнира |
| GET | `/api/v1/tournaments/:id/leaderboard` | Таблица лидеров |
| WS | `/api/v1/ws/tournaments/:id` | Real-time обновления |

Полная документация в [API_GUIDE.md](API_GUIDE.md).

## Деплой

```bash
# Docker Compose (production)
docker-compose -f docker-compose.prod.yml up -d

# Kubernetes
kubectl apply -k deployments/kubernetes/
```

Подробности в [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md).

## Лицензия

MIT
