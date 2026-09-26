# TJudge

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

Турнирная платформа для соревнований программных ботов по теории игр.
Команды пишут программы-стратегии (Python, C/C++, Go, Rust, Java, JavaScript,
Ruby, PHP, Lua), система компилирует их в песочнице, гоняет round-robin матчи
в изолированных Docker-контейнерах, ведёт лидерборды по очкам и транслирует
результаты в реальном времени через WebSocket.

Стек: Go 1.26 / PostgreSQL 15 / Redis 7 / React 19.
Разработка — [BMSTU ITSTech](https://github.com/bmstu-itstech) (МГТУ им. Баумана).

<img src="docs/media/demo.gif" width="100%" alt="Интерфейс TJudge: главная, игры, рейтинг, матчи по раундам, правила игры">

## Быстрый старт

```bash
git clone https://github.com/4RH1T3CT0R7/TJudge.git
cd TJudge
cp .env.example .env
make docker-up        # создаёт внешнюю сеть monitoring и поднимает docker compose
```

Первый запуск собирает образы (api, worker, исполнитель матчей `tjudge-cli`,
песочница компиляции `tjudge-builder`) — несколько минут. Миграции БД
применяются автоматически.

После запуска:

| Сервис | URL |
|--------|-----|
| Веб-приложение и API | http://localhost:8080 |
| Метрики api / worker | http://localhost:9090/metrics, http://localhost:9091/metrics |
| Grafana, Prometheus | http://localhost:3000 (admin/admin), http://localhost:9092 — после `make monitoring-up` |

Назначение администратора (сначала зарегистрируйтесь через веб-интерфейс):

```bash
make admin EMAIL=your-email@example.com
```

После назначения — выйдите и войдите заново.

Деплой на свой сервер (профили по железу):

```bash
make deploy              # автоопределение профиля
make deploy-weak         # 2 ядра, 4 ГБ RAM
make deploy-medium       # 4 ядра, 8 ГБ RAM
make deploy-strong       # 8+ ядер, 16+ ГБ RAM
```

Подробнее — [docs/OPERATIONS.md](docs/OPERATIONS.md).

## Игры

Пять игр, каждая исполняется через [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli) (Rust).
Правила с протоколами взаимодействия — в веб-интерфейсе и в [руководстве](docs/USER_GUIDE.md).

| Игра | Идентификатор |
|------|---------------|
| Дилемма заключённого | `dilemma` |
| Перетягивание каната | `tug_of_war` |
| Дилемма путешественника | `travelers_dilemma` |
| Общественное благо | `public_goods` |
| Аукцион двойной цены | `dollar_auction` |

## Архитектура

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
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
| Frontend | React 19, TypeScript, Vite, Tailwind CSS 4, TanStack Query, Zustand |
| API Server | Go 1.26, Chi Router, JWT, WebSocket |
| Domain Events | In-process Event Bus, между репликами — Redis pub/sub `tjudge:events` |
| Worker Pool | Go, автомасштабирование от `WORKER_MIN` до `WORKER_MAX` (по умолчанию число ядер), приоритетная очередь |
| Database | PostgreSQL 15 (миграции 000001–000045), живые лидерборды по партиционированным `matches` |
| Cache/Queue | Redis 7 — кэш турниров и лидерборда, очереди матчей и компиляции, распределённые локи, rate limiting |
| Monitoring | Prometheus, Grafana, Alertmanager, Pushgateway (`make monitoring-up`); прод-стек — отдельное репо infra-monitoring |
| Executor | Компиляция в песочнице `tjudge-builder`, матчи в [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli) (Rust), оба без сети |

## Разработка

```bash
cp .env.example .env                   # DB_PORT=5433 под dev-compose
docker network create monitoring       # один раз, compose ждёт внешнюю сеть
docker compose up -d postgres redis    # БД и кэш
(cd web && npm ci && npm run build)    # фронт встраивается в бинарник, без сборки go build падает
make docker-build-executor docker-build-builder   # образы песочниц для воркера
make migrate-up                        # миграции
# для make run-worker в .env: PROGRAMS_PATH и HOST_PROGRAMS_PATH - один абсолютный путь

# Запуск (в разных терминалах)
make run-api                           # API сервер
make run-worker                        # воркер
cd web && npm run dev                  # фронтенд с hot reload, http://localhost:5173
```

Основные команды:

| Категория | Команда | Описание |
|-----------|---------|----------|
| Запуск | `make run-api` / `make run-worker` | API сервер / воркер |
| | `make dev` | API с hot reload (air) |
| Тесты | `make test` / `make test-race` | Unit / с детектором гонок |
| | `make test-coverage` | С HTML-отчётом покрытия |
| | `RUN_INTEGRATION=true make test-integration` | Интеграционные (PostgreSQL + Redis) |
| | `make test-e2e` | End-to-end (запущенный сервер) |
| Сборка | `make build` / `make docker-build` | Бинарники / Docker образы |
| Качество | `make lint` / `make fmt` / `make security` | golangci-lint / формат / gosec + govulncheck |
| БД | `make migrate-up` / `make migrate-down` | Применить все / откатить одну миграцию |
| | `make admin EMAIL=...` | Назначить администратора |
| Бэкапы | `make backup` / `make restore BACKUP=...` | Создать / восстановить бэкап БД |
| Диагностика | `make status` / `make doctor` | Статус в терминале / глубокая проверка |

Тесты: unit рядом с кодом, integration (`-tags=integration`: репозитории
`internal/storage` и `tests/integration`, нужны БД и Redis), e2e и security
(`tests/e2e`, `tests/security`, нужен запущенный API). Подробнее — [docs/SETUP.md](docs/SETUP.md).

CI/CD (GitHub Actions): `ci` на push в main и PR (фронтенд, npm audit, vet, линт,
govulncheck, миграции up→down→up, тесты с -race, интеграционные, e2e и security),
`nightly` (полный цикл компиляция → матч на dev-compose) и `release` по тегу `v*` —
сборка образов, выкладка на сервер, проверка запущенной версии и пост-деплойный doctor.

## API

Основные эндпоинты (полный справочник — [docs/openapi.yaml](docs/openapi.yaml)):

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/v1/auth/register` | Регистрация |
| `POST` | `/api/v1/auth/login` | Авторизация (JWT) |
| `POST` | `/api/v1/auth/refresh` | Обновление токена |
| `GET` | `/api/v1/auth/me` | Текущий пользователь |
| `GET` | `/api/v1/tournaments` | Список турниров |
| `POST` | `/api/v1/tournaments` | Создать турнир |
| `POST` | `/api/v1/tournaments/:id/start` | Запустить турнир |
| `GET` | `/api/v1/tournaments/:id/leaderboard` | Лидерборд турнира (программа команды в каждой игре) |
| `GET` | `/api/v1/tournaments/:id/cross-game-leaderboard` | Кросс-игровой лидерборд команд |
| `GET` | `/api/v1/tournaments/:id/active-game` | Текущая активная игра |
| `POST` | `/api/v1/teams` | Создать команду |
| `POST` | `/api/v1/teams/join` | Вступить в команду по инвайт-коду |
| `POST` | `/api/v1/programs` | Загрузить программу (multipart) |
| `GET` | `/api/v1/games` | Список игр |
| `GET` | `/api/v1/system/health` | Здоровье системы (admin) |
| `WS` | `/api/v1/ws/tournaments/:id` | Real-time обновления |

## Документация

| Документ | Описание |
|----------|----------|
| [docs/USER_GUIDE.md](docs/USER_GUIDE.md) | Участие в турнирах, программы, рейтинг, админка, добавление игры |
| [docs/SETUP.md](docs/SETUP.md) | Локальная разработка, окружение, схема БД |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Прод и self-hosted: деплой, runbook, бэкапы, мониторинг |
| [docs/openapi.yaml](docs/openapi.yaml) | Полный справочник REST API (из него генерируются типы фронта: `npm run generate:api`) |

## Лицензия

MIT License. См. [LICENSE](LICENSE).
