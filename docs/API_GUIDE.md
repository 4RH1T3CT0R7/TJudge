# Руководство по API

Базовый URL: `http://localhost:8080/api/v1`

Полный справочник запросов и ответов (схемы, поля, примеры) — `docs/openapi.yaml`
и Swagger UI по `/swagger/*` (под админом). Ниже — только конвенции и карта эндпоинтов.

## Конвенции

### Формат ответов

Успех оборачивается в envelope `data`, ошибка — в `error` (соответствует `AppError`
с полями `code`/`message` и опциональным `details`):

```json
{ "data": { ... } }
```

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Неверные данные",
    "details": {"field": "email", "reason": "неверный формат"}
  }
}
```

Коды ошибок:

| code | HTTP | Когда |
|------|------|-------|
| `NOT_FOUND` | 404 | Ресурс не найден |
| `UNAUTHORIZED` | 401 | Отсутствует/неверный токен |
| `FORBIDDEN` | 403 | Недостаточно прав |
| `VALIDATION_ERROR` | 400 | Неверные данные / тело больше лимита |
| `CONFLICT` | 409 | Конфликт (например, дубликат) |
| `RATE_LIMITED` | 429 | Слишком много запросов |
| `INTERNAL_ERROR` | 500 | Ошибка сервера |

### Заголовки

- `X-Request-ID` — возвращается на каждый запрос (сквозной идентификатор для логов/трейсинга).
- `Idempotency-Key` — поддерживается на `POST /tournaments` и `POST /programs`.
  Повторный запрос с тем же ключом не создаёт дубль.
- CORS отдаёт `Link`, `X-Request-ID`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.

### Лимиты тела

| Тип | Лимит |
|-----|-------|
| JSON-эндпоинты | 1 МБ |
| Загрузка программ (`/programs`) | 10 МБ |

Превышение → `400 Bad Request`.

### Rate limit

Включается конфигом (`RATE_LIMIT_ENABLED`, `RATE_LIMIT_RPM`). При срабатывании — `429`,
остаток квоты и время сброса в заголовках `X-RateLimit-Remaining` / `X-RateLimit-Reset`.

### Пагинация

Два способа, зависят от эндпоинта:

- limit/offset: `GET /tournaments?limit=20&offset=40` (`limit` по умолчанию обычно 50).
- курсорная: ответ содержит `next_cursor` и `has_more`; следующая страница — параметром `cursor`.

## Аутентификация

JWT в заголовке `Authorization: Bearer <token>`.

- `access_token` и `refresh_token` выдаются при `POST /auth/register` и `POST /auth/login`.
- Access-токен короткоживущий; обновляется через `POST /auth/refresh` по refresh-токену.
- `POST /auth/logout` кладёт токен в blacklist — дальше он не принимается.
- CSRF-защиты нет: токен ходит в заголовке, куки не используются.

WebSocket-эндпоинты берут токен из query `?token=<jwt>` либо из subprotocol
`Sec-WebSocket-Protocol: access_token.<jwt>`.

## Уровни доступа

- **публичный** — без авторизации.
- **токен** — нужен валидный JWT.
- **опц. токен** — доступно всем, но админ с токеном видит больше (например, полные
  сообщения об ошибках матчей).
- **админ** — JWT с ролью `admin`; при заданном `AdminChecker` роль сверяется с БД.

## Эндпоинты

Пути указаны относительно `/api/v1`.

| Метод | Путь | Доступ | Что делает |
|-------|------|--------|------------|
| **Auth** | | | |
| POST | `/auth/register` | публичный | Регистрация, выдаёт токены |
| POST | `/auth/login` | публичный | Вход, выдаёт токены |
| POST | `/auth/refresh` | публичный | Обновление токенов по refresh |
| POST | `/auth/logout` | токен | Выход (токен в blacklist) |
| GET | `/auth/me` | токен | Текущий пользователь |
| PUT | `/auth/profile` | токен | Обновить профиль |
| **Games** | | | |
| GET | `/games` | публичный | Список игр (кэш 60с, ETag) |
| GET | `/games/{id}` | публичный | Игра по ID (кэш 60с) |
| GET | `/games/name/{name}` | публичный | Игра по имени (кэш 60с) |
| POST | `/games` | админ | Создать игру |
| PUT | `/games/{id}` | админ | Обновить игру |
| DELETE | `/games/{id}` | админ | Удалить игру |
| **Tournaments — чтение** | | | |
| GET | `/tournaments` | публичный | Список турниров |
| GET | `/tournaments/{id}` | публичный | Турнир по ID |
| GET | `/tournaments/{id}/leaderboard` | публичный | Кросс-игровой лидерборд |
| GET | `/tournaments/{id}/cross-game-leaderboard` | публичный | Агрегированный рейтинг |
| GET | `/tournaments/{id}/matches` | публичный | Матчи турнира |
| GET | `/tournaments/{id}/matches/rounds` | публичный | Матчи, сгруппированные по раундам |
| GET | `/tournaments/{id}/games` | публичный | Игры турнира |
| GET | `/tournaments/{id}/games/status` | публичный | Игры со статусом раунда |
| GET | `/tournaments/{id}/teams` | публичный | Команды турнира |
| GET | `/tournaments/{id}/active-game` | публичный | Активная игра |
| GET | `/tournaments/{id}/games/{gameId}/leaderboard` | публичный | Лидерборд по игре |
| GET | `/tournaments/{id}/games/{gameId}/head-to-head` | публичный | Матрица личных встреч |
| GET | `/tournaments/{id}/games/{gameId}/matches` | публичный | Матчи по игре |
| GET | `/tournaments/{id}/programs/{programId}/rating-history` | публичный | История ELO программы |
| **Tournaments — участие** | | | |
| POST | `/tournaments/{id}/join` | токен | Присоединиться к турниру |
| GET | `/tournaments/{id}/my-team` | токен | Моя команда в турнире |
| POST | `/tournaments/{id}/games` | токен | Добавить игру (админ или создатель) |
| **Tournaments — админ** | | | |
| POST | `/tournaments` | админ | Создать турнир (Idempotency-Key) |
| POST | `/tournaments/{id}/start` | админ | Запустить турнир |
| POST | `/tournaments/{id}/complete` | админ | Завершить турнир |
| DELETE | `/tournaments/{id}` | админ | Удалить турнир |
| POST | `/tournaments/{id}/matches` | админ | Создать матч |
| DELETE | `/tournaments/{id}/games/{gameId}` | админ | Убрать игру из турнира |
| GET | `/tournaments/{id}/games/{gameId}/programs` | админ | Программы для игры |
| GET | `/tournaments/{id}/programs/download-zip` | админ | Скачать все программы zip'ом |
| POST | `/tournaments/{id}/games/{gameId}/complete-round` | админ | Пометить раунд завершённым |
| POST | `/tournaments/{id}/games/{gameId}/reset-round` | админ | Сбросить раунд |
| POST | `/tournaments/{id}/games/{gameId}/auto-round` | админ | Настроить авто-раунд |
| GET | `/tournaments/{id}/games/{gameId}/auto-round` | админ | Статус авто-раунда |
| POST | `/tournaments/{id}/active-game` | админ | Установить активную игру |
| POST | `/tournaments/{id}/games/deactivate-all` | админ | Деактивировать все игры |
| POST | `/tournaments/{id}/run-matches` | админ | Запустить все матчи |
| POST | `/tournaments/{id}/run-game-matches` | админ | Запустить матчи по игре |
| POST | `/tournaments/{id}/retry-matches` | админ | Перезапустить неудачные |
| POST | `/tournaments/{id}/programs/clear-errors` | админ | Очистить ошибки программ |
| **Teams** | | | |
| POST | `/teams` | токен | Создать команду |
| POST | `/teams/join` | токен | Вступить по invite-коду |
| GET | `/teams/{id}` | токен | Команда по ID |
| PUT | `/teams/{id}` | токен | Переименовать |
| GET | `/teams/{id}/members` | токен | Участники |
| POST | `/teams/{id}/leave` | токен | Покинуть команду |
| DELETE | `/teams/{id}/members/{userId}` | токен | Исключить участника (лидер) |
| GET | `/teams/{id}/invite` | токен | Ссылка-приглашение |
| DELETE | `/teams/{id}` | админ | Удалить команду |
| POST | `/teams/{id}/disqualify` | админ | Дисквалифицировать |
| POST | `/teams/{id}/restore` | админ | Восстановить |
| **Programs** (тело до 10 МБ) | | | |
| POST | `/programs` | токен | Загрузить программу (Idempotency-Key) |
| GET | `/programs` | токен | Список программ пользователя |
| GET | `/programs/versions` | токен | Версии программ команды |
| GET | `/programs/{id}` | токен | Программа по ID |
| GET | `/programs/{id}/download` | токен | Скачать исходник |
| PUT | `/programs/{id}` | токен | Обновить программу |
| DELETE | `/programs/{id}` | токен | Удалить программу |
| **Matches** | | | |
| GET | `/matches` | опц. токен | Список матчей с фильтрами |
| GET | `/matches/statistics` | опц. токен | Статистика матчей |
| GET | `/matches/{id}` | опц. токен | Матч по ID |
| GET | `/matches/queue/stats` | админ | Статистика очереди |
| POST | `/matches/queue/clear` | админ | Очистить очередь |
| POST | `/matches/queue/purge` | админ | Удалить невалидные матчи |
| **WebSocket** | | | |
| GET | `/ws/tournaments/{id}` | токен | Real-time обновления турнира |
| GET | `/ws/stats` | токен | Статистика WS-подключений |
| **System** (все админ) | | | |
| GET | `/system/metrics` | админ | Метрики сервера (CPU, память, диск, Go runtime) |
| GET | `/system/health` | админ | Здоровье системы (`healthy`/`warning`) |
| GET | `/system/status` | админ | Полный статус (если включён) |
| POST | `/system/recovery/outbox-retry` | админ | Ретрай ошибок outbox |
| POST | `/system/recovery/requeue-compiling` | админ | Перезапустить зависшую компиляцию |
| POST | `/system/recovery/reset-stuck-matches` | админ | Сбросить зависшие матчи |
| POST | `/system/recovery/clear-dead-letter` | админ | Очистить dead-letter |
| **Admin** | | | |
| GET | `/admin/audit` | админ | Audit-лог |

Эндпоинты `recovery/*`, `system/status` и `admin/audit` подключаются только если
соответствующий обработчик задан при сборке сервера.

### Вне `/api/v1`

| Метод | Путь | Доступ | Что делает |
|-------|------|--------|------------|
| GET | `/health` | публичный | Легкий liveness-чек, тело `OK` |
| GET | `/swagger/*` | админ | Swagger UI |
| ANY | `/debug/pprof/*` | админ | pprof-профилирование |
| GET | `/*` | публичный | Встроенный React SPA, fallback на `index.html` |

Prometheus-метрики отдаёт отдельный сервер на `METRICS_PORT`, а не этот роутер.

## WebSocket

Подключение: `WS /api/v1/ws/tournaments/{id}?token=<jwt>`. Сервер шлёт JSON-сообщения
вида `{"type": ..., "payload": {...}}`. Типы:

| type | Когда | Ключевые поля payload |
|------|-------|-----------------------|
| `matches_created` | Запланирована пачка матчей | `program_id`, `matches_count` |
| `match_result` | Матч сыгран, рейтинг пересчитан | `match_id`, `program1_id`, `program2_id`, `new_rating1`, `new_rating2`, `winner` |
| `program_update` | Программа скомпилирована | `program_id`, `team_id`, `status`, `error_message` |

Набор полей `match_result` — замороженный контракт фронтенда.
