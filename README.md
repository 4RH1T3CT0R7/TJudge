# TJudge

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go)
![React](https://img.shields.io/badge/React-19+-61DAFB?style=for-the-badge&logo=react)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791?style=for-the-badge&logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7+-DC382D?style=for-the-badge&logo=redis)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)

**Турнирная платформа для соревнований программных ботов по теории игр**

[Быстрый старт](#быстрый-старт) •
[Игры](#игры) •
[Возможности](#возможности) •
[Архитектура](#архитектура) •
[Разработка](#разработка) •
[API](#api) •
[Документация](#документация)

</div>

---

## О проекте

TJudge -- система, в которой команды пишут программы-стратегии и выставляют их друг против друга в классических играх теории игр. Матчи исполняются в изолированных Docker-контейнерах, рейтинги рассчитываются по ELO, а результаты транслируются в реальном времени через WebSocket.

Проект разрабатывается в **BMSTU ITSTech** (МГТУ им. Баумана).

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

После назначения -- выйдите и войдите заново.

---

## Игры

В системе реализовано 5 игр. Каждая проверяет разные подходы к принятию решений: от простого выбора "предать или сотрудничать" до сложного управления ресурсами.

| Игра | Идентификатор | Суть |
|------|---------------|------|
| **Дилемма заключённого** | `prisoners_dilemma` | Сотрудничать (`COOPERATE`) или предать (`DEFECT`)? Взаимная кооперация приносит по 5 очков, предательство кооператора -- 10, но взаимное предательство -- лишь по 1. Классика теории игр, где побеждают стратегии, умеющие прощать. |
| **Перетягивание каната** | `tug_of_war` | У каждого игрока 100 единиц энергии на весь матч. Каждый раунд оба решают, сколько потратить -- кто потратил больше, получает очко. Главный вопрос: когда ударить всеми силами, а когда экономить? |
| **Дилемма путешественника** | `travelers_dilemma` | Оба игрока называют число от 2 до 100. Оба получают меньшее из двух, но назвавший меньше получает бонус +2, а назвавший больше -- штраф -2. Равновесие Нэша -- назвать 2, но кооперативные стратегии регулярно побеждают. |
| **Общественное благо** | `public_goods` | У каждого по 20 токенов. Вложенные токены попадают в общий пул, умножаются на 1.5 и делятся поровну. Вкладывать выгодно для группы, но искушение быть "безбилетником" велико. |
| **Аукцион двойной цены** | `dollar_auction` | Поочередные ставки за приз в 100 очков. Подвох: оба платят свои ставки, но приз получает только победитель. Ловушка невозвратных затрат затягивает -- ставки могут превысить стоимость приза. |

Игры исполняются через [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli) (Rust). Правила с протоколами взаимодействия доступны в веб-интерфейсе и в [руководстве пользователя](docs/USER_GUIDE.md).

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
| Игры | Добавление игр с правилами (Markdown), множитель очков, кросс-игровой лидерборд |
| Раунды | Управление раундами по играм -- запуск, пауза, завершение, автозапуск по таймеру |
| Мониторинг | Grafana дашборды, метрики Prometheus, агрегированные логи |

---

## Архитектура

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│  Frontend   │────▶│     API     │────▶│  PostgreSQL   │
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
| API Server | Go 1.26, Chi Router, JWT, WebSocket |
| Domain Events | In-process Event Bus -- декаплинг side-effects (кэш, broadcast) от бизнес-логики |
| Worker Pool | Go, автомасштабирование 10-1000, приоритетная очередь |
| Database | PostgreSQL 15 (41 миграция), живые лидерборды по партиционированным matches |
| Cache/Queue | Redis 7, кэширование турниров/лидерборда, очередь матчей, rate limiting |
| Monitoring | Prometheus, Grafana, Loki, Promtail, Alertmanager |
| Executor | Docker-изолированный [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli) (Rust) |

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
| **Тесты** | `make test` | Unit тесты |
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

### Тестирование

В проекте **1600+ тестовых функций** (Go) и vitest-тесты фронтенда:

| Уровень | Описание |
|---------|----------|
| Unit (~970) | Бизнес-логика, handlers, middleware, cache, worker, websocket |
| DB Integration (~200 subtests) | PostgreSQL репозитории через `testify/suite` |
| E2E (24) | HTTP API через запущенный сервер |

Также: бенчмарки производительности, нагрузочные тесты, хаос-тесты.

Подробнее о покрытии по пакетам: [docs/SETUP.md](docs/SETUP.md)

### CI/CD

GitHub Actions: фронтенд (eslint, vitest, сборка артефактом), lint, unit тесты, race detector, сборка, security scan, интеграционные, E2E и хаос-тесты; nightly - полный match-flow с Docker, перф-гейт (benchstat), restore-тест бэкапа.

---

## API

### Основные эндпоинты

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/v1/auth/register` | Регистрация |
| `POST` | `/api/v1/auth/login` | Авторизация (JWT) |
| `POST` | `/api/v1/auth/refresh` | Обновление токена |
| `GET` | `/api/v1/auth/me` | Текущий пользователь |
| `GET` | `/api/v1/tournaments` | Список турниров |
| `POST` | `/api/v1/tournaments` | Создать турнир |
| `POST` | `/api/v1/tournaments/:id/join` | Присоединиться к турниру |
| `POST` | `/api/v1/tournaments/:id/start` | Запустить турнир |
| `GET` | `/api/v1/tournaments/:id/leaderboard` | Лидерборд по игре |
| `GET` | `/api/v1/tournaments/:id/cross-game-leaderboard` | Кросс-игровой лидерборд |
| `GET` | `/api/v1/tournaments/:id/active-game` | Текущая активная игра |
| `POST` | `/api/v1/teams` | Создать команду |
| `POST` | `/api/v1/teams/join` | Присоединиться по коду |
| `POST` | `/api/v1/programs` | Загрузить программу |
| `GET` | `/api/v1/games` | Список игр |
| `GET` | `/api/v1/system/health` | Здоровье системы (admin) |
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
│   │   ├── httputil/           #   Общие HTTP-утилиты
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
│   ├── events/                 # Domain Events (шина событий)
│   │   ├── bus.go              #   SyncBus, NoopBus
│   │   ├── events.go           #   Типы событий
│   │   └── handlers/           #   Cache, Broadcast обработчики
│   ├── infrastructure/         # Внешние сервисы
│   │   ├── db/                 #   PostgreSQL репозитории
│   │   ├── cache/              #   Redis кэширование
│   │   ├── queue/              #   Очередь матчей
│   │   ├── storage/            #   Файловое хранилище программ
│   │   └── executor/           #   Docker исполнитель
│   ├── worker/                 # Пул воркеров
│   └── websocket/              # Real-time обновления
├── web/                        # React фронтенд
├── migrations/                 # SQL миграции (000001-000041)
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
| [docs/SETUP.md](docs/SETUP.md) | Настройка окружения и локальная разработка |
| [docs/SELF_HOSTED.md](docs/SELF_HOSTED.md) | Развёртывание на собственном сервере |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Production деплой, runbook, бэкапы, мониторинг |
| [docs/SLO.md](docs/SLO.md) | Service Level Objectives и алерты |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Детальная архитектура системы |
| [docs/API_GUIDE.md](docs/API_GUIDE.md) | REST API и WebSocket эндпоинты |
| [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md) | Схема базы данных |
| [docs/ADDING_GAMES.md](docs/ADDING_GAMES.md) | Добавление новых игр |
| [docs/PERFORMANCE_TESTING.md](docs/PERFORMANCE_TESTING.md) | Тестирование производительности |

---

## Лицензия

MIT License. См. [LICENSE](LICENSE).

---

<div align="center">

**[BMSTU ITSTech](https://github.com/bmstu-itstech)**

</div>
