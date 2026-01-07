# TJudge

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go)
![React](https://img.shields.io/badge/React-18+-61DAFB?style=for-the-badge&logo=react)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791?style=for-the-badge&logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7+-DC382D?style=for-the-badge&logo=redis)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)

**Турнирная система для соревнований программных ботов по теории игр**

[Быстрый старт](#-быстрый-старт) •
[Возможности](#-возможности) •
[Архитектура](#-архитектура) •
[API](#-api) •
[Документация](#-документация)

</div>

---

## О проекте

**TJudge** — это высоконагруженная турнирная система для проведения соревнований между программами-ботами. Система автоматизирует проведение матчей, расчёт рейтингов ELO и обеспечивает real-time отслеживание результатов.

Разработана для трека "Теория игр" хакатона Bauman Code Games.

### Ключевые особенности

- **Веб-интерфейс** — современный React SPA для управления турнирами
- **Командная игра** — создание команд, приглашения по коду
- **Множество игр** — поддержка нескольких игр в одном турнире
- **Real-time таблица** — WebSocket обновления рейтинга
- **Безопасное выполнение** — изоляция программ в Docker контейнерах
- **Высокая производительность** — 100+ матчей в секунду

---

## Быстрый старт

### Требования

- [Docker](https://docs.docker.com/get-docker/) и [Docker Compose](https://docs.docker.com/compose/install/)
- [Go 1.24+](https://golang.org/dl/) (для локальной разработки)
- [Node.js 20+](https://nodejs.org/) (для разработки фронтенда)

### Запуск через Docker Compose

```bash
# 1. Клонируйте репозиторий
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge

# 2. Скопируйте конфигурацию
cp .env.example .env

# 3. Запустите все сервисы
docker-compose up -d

# 4. Проверьте статус
docker-compose ps
```

После запуска будут доступны:

| Сервис | URL | Описание |
|--------|-----|----------|
| **Веб-приложение** | http://localhost:8080 | Основной интерфейс |
| **API** | http://localhost:8080/api/v1 | REST API |
| **Grafana** | http://localhost:3000 | Мониторинг (admin/admin) |
| **Prometheus** | http://localhost:9092 | Метрики |

### Локальная разработка

```bash
# Запустите только инфраструктуру
docker-compose up -d postgres redis

# Установите зависимости
go mod download
cd web && npm install && cd ..

# Примените миграции
make migrate-up

# Запустите API (в отдельном терминале)
make run-api

# Запустите воркер (в отдельном терминале)
make run-worker

# Запустите фронтенд в dev-режиме (в отдельном терминале)
cd web && npm run dev
```

---

## Возможности

### Для участников

| Функция | Описание |
|---------|----------|
| **Регистрация** | Создание аккаунта через email |
| **Команды** | Создание команды или присоединение по коду приглашения |
| **Турниры** | Просмотр турниров, присоединение к открытым |
| **Загрузка программ** | Отправка решения для каждой игры в турнире |
| **Результаты** | Real-time отслеживание позиции в таблице лидеров |
| **История матчей** | Просмотр результатов прошедших матчей |

### Для организаторов

| Функция | Описание |
|---------|----------|
| **Создание турнира** | Настройка названия, описания, размера команд |
| **Управление играми** | Добавление игр с правилами в Markdown |
| **Модерация** | Управление командами и участниками |
| **Запуск/завершение** | Контроль жизненного цикла турнира |

### Технические возможности

```
┌────────────────────────────────────────────────────────────────┐
│  Производительность                                            │
├────────────────────────────────────────────────────────────────┤
│  • 100+ матчей/сек при горизонтальном масштабировании          │
│  • p99 латентность API < 200ms                                 │
│  • Автомасштабирование пула воркеров (10-100)                  │
│  • Оптимизированный round-robin для больших турниров           │
└────────────────────────────────────────────────────────────────┘
```

---

## Архитектура

```
                           ┌─────────────────────────────────────┐
                           │           Веб-браузер               │
                           │    (React + Tailwind CSS SPA)       │
                           └──────────────┬──────────────────────┘
                                          │
                           ┌──────────────▼──────────────────────┐
                           │           API Server                │
                           │  (Go + Chi Router + WebSocket)      │
                           │                                     │
                           │  ┌─────────────────────────────┐    │
                           │  │     Embedded Frontend       │    │
                           │  │     (go:embed dist/)        │    │
                           │  └─────────────────────────────┘    │
                           └──────┬─────────────────┬────────────┘
                                  │                 │
                    ┌─────────────▼───┐   ┌────────▼────────────┐
                    │   PostgreSQL    │   │       Redis         │
                    │                 │   │                     │
                    │  • Пользователи │   │  • Очередь матчей   │
                    │  • Турниры      │   │  • Кэш рейтингов    │
                    │  • Команды      │   │  • Сессии           │
                    │  • Программы    │   │  • Rate limiting    │
                    │  • Матчи        │   │                     │
                    └─────────────────┘   └────────┬────────────┘
                                                   │
                           ┌───────────────────────▼──────────────┐
                           │           Worker Pool                │
                           │     (Автомасштабирование 10-100)     │
                           └───────────────────┬──────────────────┘
                                               │
                    ┌──────────────────────────┼──────────────────────────┐
                    │                          │                          │
             ┌──────▼──────┐           ┌───────▼─────┐           ┌────────▼────┐
             │   Docker    │           │   Docker    │           │   Docker    │
             │  Container  │           │  Container  │           │  Container  │
             │             │           │             │           │             │
             │ tjudge-cli  │           │ tjudge-cli  │           │ tjudge-cli  │
             │ game prog1  │           │ game prog1  │           │ game prog1  │
             │     prog2   │           │     prog2   │           │     prog2   │
             └─────────────┘           └─────────────┘           └─────────────┘
```

### Компоненты

| Компонент | Технологии | Описание |
|-----------|------------|----------|
| **Frontend** | React 18, TypeScript, Tailwind CSS, Zustand | SPA встроенный в Go binary |
| **API Server** | Go 1.24, Chi Router, WebSocket | REST API + статика |
| **Worker Pool** | Go, Redis Queue | Обработка матчей |
| **Executor** | Docker API | Изолированное выполнение программ |
| **Database** | PostgreSQL 15 | Хранение данных |
| **Cache** | Redis 7 | Очереди, кэш, сессии |
| **Monitoring** | Prometheus, Grafana, Loki | Метрики и логи |

---

## API

### Аутентификация

```bash
# Регистрация
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "player1", "email": "player1@example.com", "password": "secret123"}'

# Вход
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "player1@example.com", "password": "secret123"}'
```

### Основные эндпоинты

| Метод | Endpoint | Описание |
|-------|----------|----------|
| **Аутентификация** |||
| POST | `/api/v1/auth/register` | Регистрация |
| POST | `/api/v1/auth/login` | Вход |
| POST | `/api/v1/auth/refresh` | Обновление токена |
| GET | `/api/v1/auth/me` | Текущий пользователь |
| **Турниры** |||
| GET | `/api/v1/tournaments` | Список турниров |
| GET | `/api/v1/tournaments/:id` | Информация о турнире |
| POST | `/api/v1/tournaments` | Создание турнира |
| POST | `/api/v1/tournaments/:id/join` | Присоединение к турниру |
| POST | `/api/v1/tournaments/:id/start` | Запуск турнира |
| GET | `/api/v1/tournaments/:id/leaderboard` | Таблица лидеров |
| GET | `/api/v1/tournaments/:id/games` | Игры турнира |
| GET | `/api/v1/tournaments/:id/teams` | Команды турнира |
| **Команды** |||
| POST | `/api/v1/teams` | Создание команды |
| POST | `/api/v1/teams/join` | Присоединение по коду |
| GET | `/api/v1/teams/:id` | Информация о команде |
| POST | `/api/v1/teams/:id/leave` | Выход из команды |
| GET | `/api/v1/teams/:id/invite` | Получение ссылки-приглашения |
| **Игры** |||
| GET | `/api/v1/games` | Список игр |
| GET | `/api/v1/games/:id` | Информация об игре |
| POST | `/api/v1/games` | Создание игры (admin) |
| **Программы** |||
| GET | `/api/v1/programs` | Мои программы |
| POST | `/api/v1/programs` | Загрузка программы |
| DELETE | `/api/v1/programs/:id` | Удаление программы |
| **WebSocket** |||
| WS | `/api/v1/ws/tournaments/:id` | Real-time обновления |

### WebSocket события

```javascript
// Подключение
const ws = new WebSocket('ws://localhost:8080/api/v1/ws/tournaments/123?token=JWT_TOKEN');

// Типы сообщений
{
  "type": "leaderboard_update",
  "payload": {
    "entries": [
      {"rank": 1, "team_name": "Alpha", "rating": 1523, "wins": 10, "losses": 2}
    ]
  }
}

{
  "type": "match_completed",
  "payload": {
    "match_id": "...",
    "program1_id": "...",
    "program2_id": "...",
    "winner": 1
  }
}
```

---

## Конфигурация

### Переменные окружения

Создайте файл `.env` на основе `.env.example`:

```bash
# Окружение
ENVIRONMENT=development

# API Server
API_PORT=8080
BASE_URL=http://localhost:8080

# PostgreSQL
DB_HOST=localhost
DB_PORT=5433
DB_USER=tjudge
DB_PASSWORD=secret
DB_NAME=tjudge

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Worker Pool
WORKER_MIN=10
WORKER_MAX=100

# JWT
JWT_SECRET=your-secret-key-change-in-production
JWT_ACCESS_TTL=1h
JWT_REFRESH_TTL=168h

# Программы
PROGRAMS_PATH=/data/programs

# Логирование
LOG_LEVEL=info
LOG_FORMAT=json

# Метрики
METRICS_ENABLED=true
METRICS_PORT=9090
```

### Полный список переменных

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `ENVIRONMENT` | development | Окружение (development/production) |
| `API_PORT` | 8080 | Порт API сервера |
| `BASE_URL` | http://localhost:8080 | Базовый URL приложения |
| `DB_HOST` | localhost | Хост PostgreSQL |
| `DB_PORT` | 5433 | Порт PostgreSQL |
| `DB_USER` | tjudge | Пользователь БД |
| `DB_PASSWORD` | secret | Пароль БД |
| `DB_NAME` | tjudge | Имя базы данных |
| `DB_MAX_CONNECTIONS` | 50 | Макс. соединений с БД |
| `REDIS_HOST` | localhost | Хост Redis |
| `REDIS_PORT` | 6379 | Порт Redis |
| `REDIS_POOL_SIZE` | 100 | Размер пула соединений Redis |
| `WORKER_MIN` | 10 | Минимум воркеров |
| `WORKER_MAX` | 100 | Максимум воркеров |
| `WORKER_TIMEOUT` | 60s | Таймаут выполнения матча |
| `JWT_SECRET` | — | **Обязательно!** Секрет для JWT |
| `JWT_ACCESS_TTL` | 1h | Время жизни access токена |
| `JWT_REFRESH_TTL` | 168h | Время жизни refresh токена |
| `PROGRAMS_PATH` | /data/programs | Путь для хранения программ |
| `LOG_LEVEL` | info | Уровень логирования |
| `LOG_FORMAT` | json | Формат логов (json/console) |
| `METRICS_ENABLED` | true | Включить метрики Prometheus |
| `METRICS_PORT` | 9090 | Порт для метрик |

---

## Разработка

### Структура проекта

```
tjudge/
├── cmd/                    # Точки входа
│   ├── api/               # API сервер
│   ├── worker/            # Worker процесс
│   └── migrations/        # CLI миграций
├── internal/              # Внутренний код
│   ├── api/              # HTTP handlers, middleware
│   ├── domain/           # Бизнес-логика
│   │   ├── auth/        # Аутентификация
│   │   ├── tournament/  # Турниры
│   │   ├── team/        # Команды
│   │   ├── game/        # Игры
│   │   ├── match/       # Матчи
│   │   └── rating/      # Рейтинги
│   ├── infrastructure/   # Внешние сервисы
│   │   ├── db/          # PostgreSQL репозитории
│   │   ├── cache/       # Redis кэш
│   │   ├── queue/       # Очередь задач
│   │   ├── executor/    # Docker executor
│   │   └── storage/     # Файловое хранилище
│   ├── worker/          # Пул воркеров
│   ├── web/             # Встроенный фронтенд
│   └── config/          # Конфигурация
├── web/                  # React фронтенд
│   ├── src/
│   │   ├── api/         # API клиент
│   │   ├── components/  # React компоненты
│   │   ├── pages/       # Страницы
│   │   ├── hooks/       # React хуки
│   │   ├── store/       # Zustand store
│   │   └── types/       # TypeScript типы
│   └── ...
├── pkg/                  # Переиспользуемые пакеты
├── migrations/           # SQL миграции
├── docker/              # Dockerfiles
├── deployments/         # Конфигурации деплоя
└── docs/                # Документация
```

### Команды Make

```bash
# Справка
make help

# Зависимости
make deps              # Скачать Go зависимости

# Сборка
make build             # Собрать бинарники

# Тестирование
make test              # Все тесты
make test-race         # С детектором гонок
make test-coverage     # С отчётом покрытия
make test-integration  # Интеграционные
make test-e2e          # E2E тесты
make test-load         # Нагрузочные (k6)

# Качество кода
make lint              # Линтер (golangci-lint)
make fmt               # Форматирование
make security          # Проверка безопасности

# Запуск
make run-api           # API сервер
make run-worker        # Worker
make dev               # Hot reload (air)

# Docker
make docker-build      # Собрать образы
make docker-up         # Запустить
make docker-down       # Остановить
make docker-logs       # Логи

# Миграции
make migrate-up        # Применить
make migrate-down      # Откатить
make migrate-create    # Создать новую
```

### Работа с фронтендом

```bash
cd web

# Установка зависимостей
npm install

# Режим разработки
npm run dev            # http://localhost:5173

# Сборка для продакшена
npm run build          # Создаёт dist/

# Линтинг
npm run lint
```

**Важно:** После сборки фронтенда (`npm run build`) запустите `go build` для встраивания в Go binary.

---

## Мониторинг

### Grafana дашборды

После запуска через docker-compose доступны на http://localhost:3000 (admin/admin):

- **TJudge Overview** — общая статистика
- **Workers** — метрики пула воркеров
- **API** — HTTP запросы и латентность
- **Database** — производительность PostgreSQL

### Prometheus метрики

Доступны на http://localhost:9092:

```
# Матчи
tjudge_matches_total                # Всего матчей
tjudge_matches_active               # Активных матчей
tjudge_match_duration_seconds       # Длительность матча

# Воркеры
tjudge_workers_active               # Активных воркеров
tjudge_workers_idle                 # Простаивающих воркеров
tjudge_queue_size                   # Размер очереди

# HTTP
tjudge_http_requests_total          # HTTP запросов
tjudge_http_request_duration_seconds # Латентность
tjudge_http_requests_in_flight      # Запросов в обработке

# WebSocket
tjudge_websocket_connections        # Активных WS соединений
```

### Логи

Логи в JSON формате собираются через Loki. Запросы в Grafana:

```logql
# Ошибки API
{container_name="tjudge-api"} |= "error"

# Завершённые матчи
{container_name="tjudge-worker"} |= "match completed"

# Медленные запросы (>1s)
{container_name="tjudge-api"} | json | duration > 1s
```

---

## Деплой

### Docker Compose (staging)

```bash
# Сборка и запуск
docker-compose up -d --build

# Масштабирование воркеров
docker-compose up -d --scale worker=5
```

### Production рекомендации

1. **Секреты** — используйте Docker Secrets или Vault
2. **База данных** — managed PostgreSQL (AWS RDS, Cloud SQL)
3. **Redis** — managed Redis (ElastiCache, Memorystore)
4. **Мониторинг** — настройте алерты в Grafana
5. **Бэкапы** — автоматические бэкапы БД
6. **TLS** — настройте HTTPS через reverse proxy

---

## Документация

| Документ | Описание |
|----------|----------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Детальная архитектура |
| [docs/API_GUIDE.md](docs/API_GUIDE.md) | Полное описание API |
| [docs/DEPLOYMENT_GUIDE.md](docs/DEPLOYMENT_GUIDE.md) | Руководство по деплою |
| [docs/DEVELOPMENT_GUIDE.md](docs/DEVELOPMENT_GUIDE.md) | Руководство разработчика |
| [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md) | Схема базы данных |

---

## Лицензия

MIT License. См. [LICENSE](LICENSE).

---

## Авторы

- [BMSTU ITSTech](https://github.com/bmstu-itstech)

---

<div align="center">

**[Наверх](#tjudge)**

</div>
