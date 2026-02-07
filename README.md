# TJudge

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go)
![React](https://img.shields.io/badge/React-19+-61DAFB?style=for-the-badge&logo=react)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791?style=for-the-badge&logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7+-DC382D?style=for-the-badge&logo=redis)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)

**Турнирная система для соревнований программных ботов по теории игр**

[Быстрый старт](#быстрый-старт) •
[Возможности](#возможности) •
[Архитектура](#архитектура) •
[Разработка](#разработка) •
[Документация](#документация)

</div>

---

## О проекте

**TJudge** — турнирная система для проведения соревнований между программами-ботами. Команды загружают стратегии, система автоматически проводит round-robin матчи в изолированных Docker-контейнерах, рассчитывает рейтинги ELO и транслирует результаты в реальном времени через WebSocket.

### Ключевые особенности

- **Веб-интерфейс** — React SPA для управления турнирами, загрузки стратегий и отслеживания результатов
- **Командная игра** — создание команд, приглашения по коду, несколько участников на команду
- **Множество игр** — несколько игр в одном турнире с независимыми рейтингами и множителями очков
- **Real-time** — WebSocket обновления лидерборда и результатов матчей
- **Безопасное выполнение** — изоляция программ в Docker-контейнерах через [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli)
- **Автомасштабирование** — пул воркеров от 2 до 100+ с автоподстройкой под нагрузку
- **Мониторинг** — Prometheus метрики, Grafana дашборды, Loki логи, Alertmanager оповещения

---

## Быстрый старт

### Docker Compose (рекомендуется)

```bash
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
cp .env.example .env
docker-compose up -d
```

После запуска:

| Сервис | URL |
|--------|-----|
| Веб-приложение | http://localhost:8080 |
| Grafana | http://localhost:3000 (admin/admin) |
| Prometheus | http://localhost:9092 |
| Loki (логи) | http://localhost:3100 |

### Self-Hosted деплой

```bash
make deploy              # Автоопределение профиля
make deploy-weak         # 2 ядра, 4 ГБ RAM
make deploy-medium       # 4 ядра, 8 ГБ RAM
make deploy-strong       # 8+ ядер, 16+ ГБ RAM
```

Подробнее: [docs/SELF_HOSTED.md](docs/SELF_HOSTED.md)

### Назначение администратора

```bash
# Сначала зарегистрируйтесь через веб-интерфейс, затем:
make admin EMAIL=your-email@example.com
```

После назначения — выйдите и войдите заново.

---

## Возможности

### Для участников

| Функция | Описание |
|---------|----------|
| Команды | Создание команды или присоединение по коду приглашения |
| Турниры | Просмотр доступных турниров и участие |
| Программы | Загрузка стратегий (Python, Go, C++) для каждой игры турнира |
| Лидерборд | Real-time позиция в таблице лидеров с ELO рейтингом |

### Для организаторов

| Функция | Описание |
|---------|----------|
| Турниры | Создание с настройкой размера команд, лимита участников, постоянных/разовых |
| Игры | Добавление игр с правилами (Markdown), множитель очков |
| Раунды | Управление раундами по играм — запуск, пауза, завершение |
| Мониторинг | Grafana дашборды, метрики Prometheus, агрегированные логи |

---

## Архитектура

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Frontend   │────▶│     API     │────▶│  PostgreSQL  │
│  (React)    │◀────│    (Go)     │◀────│              │
└─────────────┘     └──────┬──────┘     └──────────────┘
       ▲                   │
       │ WebSocket         │
       └───────────────────┤
                     ┌─────▼─────┐
                     │   Redis   │
                     │ (очередь  │
                     │  + кэш)   │
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

| Компонент | Технологии |
|-----------|------------|
| Frontend | React 19, TypeScript, Tailwind CSS 4, Zustand |
| API Server | Go 1.24, Chi Router, JWT, WebSocket |
| Worker Pool | Go, автомасштабирование 2-100+, приоритетная очередь |
| Database | PostgreSQL 15 (22 миграции), материализованные представления для лидерборда |
| Cache/Queue | Redis 7, кэширование турниров/лидерборда, очередь матчей, rate limiting |
| Monitoring | Prometheus, Grafana, Loki, Promtail, Alertmanager |
| Executor | Docker-изолированный [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli) (Rust) |

---

## Поддерживаемые игры

| Игра | Идентификатор | Описание |
|------|---------------|----------|
| Дилемма заключённого | `prisoners_dilemma` | Классическая игра: сотрудничать (C) или предать (D). Матрица выплат 3/5/1/0 |
| Перетягивание каната | `tug_of_war` | Стратегическое распределение 100 единиц энергии за 50 раундов |

Игры реализованы в [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli). Новые игры добавляются через миграции + обновление исполнителя.

---

## Разработка

### Локальный запуск

```bash
# Зависимости
docker-compose up -d postgres redis    # БД и кэш
make migrate-up                        # Миграции

# Запуск (в разных терминалах)
make run-api                           # API сервер
make run-worker                        # Воркер
cd web && npm run dev                  # Фронтенд (hot reload)
```

### Команды

| Категория | Команда | Описание |
|-----------|---------|----------|
| **Запуск** | `make run-api` | API сервер |
| | `make run-worker` | Worker |
| | `make dev` | API с hot reload (air) |
| **Тесты** | `make test` | Unit тесты (1100+ тестов, 21 пакет) |
| | `make test-race` | С детектором гонок |
| | `make test-coverage` | С отчётом покрытия (HTML) |
| | `make test-integration` | Интеграционные (PostgreSQL + Redis) |
| | `make test-e2e` | End-to-end (запущенный сервер) |
| | `make benchmark` | Бенчмарки производительности |
| | `make test-load` | Нагрузочное тестирование |
| **Сборка** | `make build` | Бинарники (api, worker, migrate) |
| | `make docker-build` | Docker образы |
| **Качество** | `make lint` | golangci-lint |
| | `make fmt` | Форматирование кода |
| | `make security` | gosec + govulncheck |
| **БД** | `make migrate-up` | Применить миграции |
| | `make migrate-down` | Откатить миграции |
| | `make admin EMAIL=...` | Назначить администратора |
| **Бэкапы** | `make backup` | Создать бэкап БД |
| | `make restore BACKUP=...` | Восстановить из бэкапа |

### Тестовое покрытие

| Пакет | Покрытие |
|-------|----------|
| errors, logger, metrics, validator, game | 100% |
| batch, pagination, config | 93-97% |
| auth, middleware, domain models | 89-94% |
| rating, team, tournament | 82-92% |
| handlers, worker | 70-74% |
| cache, websocket, queue, storage | 53-62% |

### CI/CD

GitHub Actions: lint, unit тесты, race detector, сборка, security scan, интеграционные тесты, E2E тесты.

---

## API

### Основные эндпоинты

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/v1/auth/register` | Регистрация |
| `POST` | `/api/v1/auth/login` | Авторизация (JWT) |
| `POST` | `/api/v1/auth/refresh` | Обновление токена |
| `GET` | `/api/v1/tournaments` | Список турниров |
| `POST` | `/api/v1/tournaments` | Создать турнир |
| `POST` | `/api/v1/tournaments/:id/start` | Запустить турнир |
| `GET` | `/api/v1/tournaments/:id/leaderboard` | Лидерборд |
| `POST` | `/api/v1/teams` | Создать команду |
| `POST` | `/api/v1/teams/join` | Присоединиться по коду |
| `POST` | `/api/v1/programs` | Загрузить программу |
| `GET` | `/api/v1/games` | Список игр |
| `WS` | `/api/v1/ws/tournaments/:id` | Real-time обновления |

Полный справочник: [docs/API_GUIDE.md](docs/API_GUIDE.md)

---

## Структура проекта

```
TJudge/
├── cmd/                        # Точки входа
│   ├── api/                    #   API сервер
│   ├── worker/                 #   Worker сервис
│   ├── migrations/             #   Инструмент миграций
│   └── benchmark/              #   Интерпретатор бенчмарков
├── internal/
│   ├── api/                    # HTTP слой
│   │   ├── handlers/           #   Обработчики запросов
│   │   ├── middleware/         #   Auth, rate limiting, CORS, CSRF
│   │   ├── batch/              #   Batch API
│   │   └── routes.go           #   Маршруты
│   ├── domain/                 # Бизнес-логика
│   │   ├── auth/               #   Аутентификация, JWT
│   │   ├── tournament/         #   Турниры, round-robin
│   │   ├── rating/             #   ELO рейтинги
│   │   ├── team/               #   Команды
│   │   ├── game/               #   Игры
│   │   └── models.go           #   Доменные сущности
│   ├── infrastructure/         # Внешние сервисы
│   │   ├── db/                 #   PostgreSQL репозитории
│   │   ├── cache/              #   Redis кэширование
│   │   ├── queue/              #   Очередь матчей
│   │   ├── storage/            #   Файловое хранилище программ
│   │   └── executor/           #   Docker исполнитель
│   ├── worker/                 # Пул воркеров
│   └── websocket/              # Real-time обновления
├── web/                        # React фронтенд
├── migrations/                 # SQL миграции (22 шт.)
├── tests/
│   ├── e2e/                    # End-to-end тесты
│   ├── integration/            # Интеграционные тесты
│   ├── benchmark/              # Бенчмарки
│   ├── load/                   # Нагрузочные тесты
│   └── chaos/                  # Хаос-тесты
├── deployments/                # Prometheus, Grafana, Loki конфиги
├── scripts/                    # Деплой, бэкапы
├── docker/                     # Dockerfiles
└── docs/                       # Документация
```

---

## Документация

| Документ | Описание |
|----------|----------|
| [docs/USER_GUIDE.md](docs/USER_GUIDE.md) | Руководство пользователя и администратора |
| [docs/SETUP.md](docs/SETUP.md) | Настройка окружения, разработка, деплой |
| [docs/SELF_HOSTED.md](docs/SELF_HOSTED.md) | Развёртывание на собственном сервере |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Детальная архитектура системы |
| [docs/API_GUIDE.md](docs/API_GUIDE.md) | REST API и WebSocket эндпоинты |
| [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md) | Схема базы данных |
| [docs/PERFORMANCE_TESTING.md](docs/PERFORMANCE_TESTING.md) | Тестирование производительности |

---

## Лицензия

MIT License. См. [LICENSE](LICENSE).

---

<div align="center">

**[BMSTU ITSTech](https://github.com/bmstu-itstech)**

</div>
