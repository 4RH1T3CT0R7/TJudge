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
[Документация](#документация)

</div>

---

## О проекте

**TJudge** — турнирная система для проведения соревнований между программами-ботами. Автоматизирует проведение матчей, расчёт рейтингов ELO и обеспечивает real-time отслеживание результатов.

### Ключевые особенности

- **Веб-интерфейс** — React SPA для управления турнирами
- **Командная игра** — создание команд, приглашения по коду
- **Множество игр** — несколько игр в одном турнире с независимыми рейтингами
- **Real-time** — WebSocket обновления лидерборда
- **Безопасность** — изоляция выполнения в Docker контейнерах
- **Производительность** — 100+ матчей/сек

---

## Быстрый старт

```bash
# Клонирование и запуск
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
cp .env.example .env
docker-compose up -d

# Проверка
curl http://localhost:8080/health
```

| Сервис | URL |
|--------|-----|
| Веб-приложение | http://localhost:8080 |
| Grafana | http://localhost:3000 (admin/admin) |
| Prometheus | http://localhost:9092 |

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
| Команды | Создание или присоединение по коду приглашения |
| Турниры | Просмотр доступных турниров и участие |
| Программы | Загрузка стратегий для каждой игры турнира |
| Результаты | Real-time позиция в таблице лидеров |

### Для организаторов

| Функция | Описание |
|---------|----------|
| Создание турнира | Название, описание, размер команд |
| Управление играми | Добавление игр с правилами (Markdown), множитель очков |
| Контроль раундов | Запуск, приостановка, завершение по играм |
| Мониторинг | Grafana дашборды, метрики Prometheus |

---

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

| Компонент | Технологии |
|-----------|------------|
| Frontend | React 19, TypeScript, Tailwind CSS 4, Zustand |
| API Server | Go 1.24, Chi Router, WebSocket, JWT |
| Worker Pool | Go, автомасштабирование 2-100+ |
| Database | PostgreSQL 15, Redis 7 |
| Monitoring | Prometheus, Grafana, Loki, Alertmanager |

---

## Документация

| Документ | Описание |
|----------|----------|
| **[docs/USER_GUIDE.md](docs/USER_GUIDE.md)** | Полное руководство пользователя и администратора |
| [docs/SETUP.md](docs/SETUP.md) | Настройка окружения, разработка, деплой |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Детальная архитектура системы |
| [docs/API_GUIDE.md](docs/API_GUIDE.md) | REST API и WebSocket эндпоинты |
| [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md) | Схема базы данных |
| [docs/PERFORMANCE_TESTING.md](docs/PERFORMANCE_TESTING.md) | Тестирование производительности |

---

## Разработка

```bash
# Локальная разработка
docker-compose up -d postgres redis
make migrate-up
make run-api      # Терминал 1
make run-worker   # Терминал 2
cd web && npm run dev  # Терминал 3
```

### Основные команды

| Команда | Описание |
|---------|----------|
| `make run-api` | API сервер |
| `make run-worker` | Worker |
| `make test` | Unit тесты |
| `make test-race` | Тесты с детектором гонок |
| `make lint` | Линтер |
| `make build` | Сборка бинарников |
| `make docker-build` | Docker образы |
| `make benchmark` | Бенчмарки производительности |

Подробнее: [docs/SETUP.md](docs/SETUP.md)

---

## Поддерживаемые игры

| Игра | Идентификатор | Описание |
|------|---------------|----------|
| Дилемма заключённого | `prisoners_dilemma` | Классическая игра теории игр |
| Перетягивание каната | `tug_of_war` | Стратегическое распределение ресурсов |

Игры реализованы в [tjudge-cli](https://github.com/bmstu-itstech/tjudge-cli).

---

## Лицензия

MIT License. См. [LICENSE](LICENSE).

---

<div align="center">

**[BMSTU ITSTech](https://github.com/bmstu-itstech)**

</div>
