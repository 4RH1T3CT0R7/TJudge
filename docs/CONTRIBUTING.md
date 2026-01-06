# Участие в разработке

## Настройка окружения

```bash
# Клонирование
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge

# Установка инструментов
make install-tools

# Запуск зависимостей
docker-compose up -d postgres redis

# Локальный запуск
go run cmd/api/main.go
go run cmd/worker/main.go
```

## Стиль кода

- Следуйте [Effective Go](https://go.dev/doc/effective_go)
- Используйте `gofmt` и `golangci-lint`
- Функции не длиннее 50 строк
- Пишите тесты для нового кода

## Структура проекта

```
internal/           # Приватный код
├── api/           # HTTP хендлеры
├── domain/        # Бизнес-логика (без внешних зависимостей)
├── infrastructure/# Внешние сервисы (БД, Redis, Docker)
├── websocket/     # Real-time обновления
└── worker/        # Обработка матчей

pkg/               # Общие утилиты
tests/             # Интеграционные, E2E, хаос-тесты
```

## Внесение изменений

### 1. Создание ветки

```bash
git checkout -b feature/your-feature
# или
git checkout -b fix/your-fix
```

### 2. Написание кода

- Доменная логика: `internal/domain/`
- API эндпоинты: `internal/api/handlers/`
- Инфраструктура: `internal/infrastructure/`

### 3. Тестирование

```bash
# Unit-тесты
make test

# Интеграционные (требуется Docker)
make test-integration

# Линтер
make lint
```

### 4. Коммит

```bash
# Формат: тип(область): сообщение
git commit -m "feat(tournament): добавлен double elimination"
git commit -m "fix(executor): исправлена обработка таймаута"
git commit -m "docs(api): обновлены примеры авторизации"
```

Типы: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

### 5. Pull Request

- Целевая ветка: `main`
- Включите: описание, план тестирования, breaking changes
- Дождитесь прохождения CI
- Запросите ревью

## Тестирование

| Тип | Расположение | Команда |
|-----|--------------|---------|
| Unit | `*_test.go` | `make test` |
| Интеграционные | `tests/integration/` | `make test-integration` |
| E2E | `tests/e2e/` | `make test-e2e` |
| Хаос | `tests/chaos/` | `make test-chaos` |

## Типичные задачи

### Добавление API эндпоинта

1. Определите хендлер в `internal/api/handlers/`
2. Добавьте маршрут в `internal/api/routes.go`
3. Напишите тесты
4. Обновите `docs/API_GUIDE.md`

### Добавление доменной логики

1. Реализуйте в `internal/domain/`
2. Держите код чистым (без вызовов БД/Redis)
3. Тщательно тестируйте

### Добавление инфраструктуры

1. Реализуйте в `internal/infrastructure/`
2. Напишите интеграционные тесты
3. Обновите Docker Compose при необходимости

## Вопросы?

- Issues: [GitHub Issues](https://github.com/bmstu-itstech/tjudge/issues)
- Обсуждения: [GitHub Discussions](https://github.com/bmstu-itstech/tjudge/discussions)
