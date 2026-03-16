# Руководство по API

Базовый URL: `http://localhost:8080/api/v1`

## Содержание

- [Формат ответов](#формат-ответов)
- [Аутентификация](#аутентификация)
- [Лимиты размера тела запроса](#лимиты-размера-тела-запроса)
- [Уровни доступа](#уровни-доступа)
- [Авторизация (Auth)](#авторизация-auth)
- [Игры (Games)](#игры-games)
- [Турниры (Tournaments)](#турниры-tournaments)
- [Команды (Teams)](#команды-teams)
- [Программы (Programs)](#программы-programs)
- [Матчи (Matches)](#матчи-matches)
- [WebSocket](#websocket)
- [Система (System)](#система-system)
- [Ответы с ошибками](#ответы-с-ошибками)
- [Лимиты запросов](#лимиты-запросов)
- [Пагинация](#пагинация)
- [Системные эндпоинты](#системные-эндпоинты)

---

## Формат ответов

Все успешные ответы оборачиваются в envelope:

```json
{
  "data": { ... }
}
```

Ответы с ошибками:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Ресурс не найден"
  }
}
```

---

## Аутентификация

Все защищённые эндпоинты требуют заголовок `Authorization: Bearer <token>`.

Токен доступа (access token) получается при регистрации или входе. Срок действия ограничен. Для обновления используйте эндпоинт `/auth/refresh` с refresh-токеном.

---

## Лимиты размера тела запроса

| Тип запроса | Лимит |
|-------------|-------|
| JSON-эндпоинты | 1 МБ |
| Загрузка файлов (программы) | 10 МБ |

При превышении лимита возвращается ошибка `400 Bad Request`.

---

## Уровни доступа

В документации используются следующие обозначения:

| Обозначение | Описание |
|-------------|----------|
| :globe_with_meridians: Публичный | Доступен без авторизации |
| :key: Требует авторизации | Необходим валидный JWT-токен |
| :crown: Только администратор | Необходим JWT-токен с ролью `admin` |

Некоторые эндпоинты используют **опциональную авторизацию** (OptionalAuth): они доступны всем, но авторизованные администраторы получают расширенную информацию (например, полные сообщения об ошибках в матчах).

---

## Авторизация (Auth)

Базовый путь: `/api/v1/auth`

### POST /auth/register :globe_with_meridians: Публичный

Регистрация нового пользователя.

```http
POST /auth/register
Content-Type: application/json

{
  "username": "player1",
  "email": "player1@example.com",
  "password": "SecurePass123!"
}
```

Ответ: `201 Created`
```json
{
  "user": {"id": "uuid", "username": "player1", "role": "user"},
  "access_token": "eyJ...",
  "refresh_token": "eyJ..."
}
```

### POST /auth/login :globe_with_meridians: Публичный

Вход в систему.

```http
POST /auth/login
Content-Type: application/json

{"email": "player1@example.com", "password": "SecurePass123!"}
```

Ответ: `200 OK` с токенами (формат аналогичен регистрации).

### POST /auth/refresh :globe_with_meridians: Публичный

Обновление пары токенов.

```http
POST /auth/refresh
Content-Type: application/json

{"refresh_token": "eyJ..."}
```

Ответ: `200 OK` с новыми токенами.

### POST /auth/logout :key: Требует авторизации

Выход из системы. Добавляет токены в чёрный список.

```http
POST /auth/logout
Authorization: Bearer <token>
Content-Type: application/json

{"refresh_token": "eyJ..."}
```

Refresh-токен в теле запроса опционален. Ответ: `204 No Content`.

### GET /auth/me :key: Требует авторизации

Получение информации о текущем пользователе.

```http
GET /auth/me
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "id": "uuid",
  "username": "player1",
  "email": "player1@example.com",
  "role": "user",
  "created_at": "2026-01-01T00:00:00Z"
}
```

### PUT /auth/profile :key: Требует авторизации

Обновление профиля текущего пользователя. Позволяет сменить email и/или пароль.

```http
PUT /auth/profile
Authorization: Bearer <token>
Content-Type: application/json

{
  "email": "newemail@example.com",
  "password": "NewSecurePass123!",
  "current_password": "OldPassword123!"
}
```

Все поля опциональны. При смене пароля поле `current_password` обязательно.

Ответ: `200 OK` -- обновлённый объект пользователя.

---

## Игры (Games)

Базовый путь: `/api/v1/games`

### GET /games :globe_with_meridians: Публичный

Список всех доступных игр.

```http
GET /games?name=prisoners&limit=50&offset=0
```

Параметры запроса:
- `name` -- фильтр по имени (частичное совпадение)
- `limit` -- размер страницы (по умолчанию: 50)
- `offset` -- смещение

Ответ: `200 OK`
```json
[
  {
    "id": "uuid",
    "slug": "prisoners_dilemma",
    "name": "Дилемма заключённого",
    "rules": "# Правила\n\n...",
    "score_multiplier": 1.0,
    "created_at": "2026-01-01T00:00:00Z"
  }
]
```

### GET /games/{id} :globe_with_meridians: Публичный

Получение информации об игре по ID.

```http
GET /games/{id}
```

### GET /games/name/{name} :globe_with_meridians: Публичный

Получение игры по имени. Полезно для поиска игры без знания UUID.

```http
GET /games/name/prisoners_dilemma
```

Ответ: `200 OK` -- объект игры (формат аналогичен списку).

### POST /games :crown: Только администратор

Создание новой игры.

```http
POST /games
Authorization: Bearer <token>
Content-Type: application/json

{
  "slug": "new_game",
  "name": "Новая игра",
  "rules": "# Правила\n\nMarkdown описание...",
  "score_multiplier": 1.5
}
```

Ответ: `201 Created` -- созданный объект игры.

### PUT /games/{id} :crown: Только администратор

Обновление игры.

```http
PUT /games/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Обновлённое название",
  "rules": "# Новые правила\n\n...",
  "score_multiplier": 2.0
}
```

### DELETE /games/{id} :crown: Только администратор

Удаление игры.

```http
DELETE /games/{id}
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

---

## Турниры (Tournaments)

Базовый путь: `/api/v1/tournaments`

### Публичные эндпоинты

#### GET /tournaments :globe_with_meridians: Публичный

Список турниров с фильтрацией и пагинацией.

```http
GET /tournaments?status=active&game_type=prisoners_dilemma&limit=10&offset=0
```

Параметры запроса:
- `status` -- фильтр по статусу: `pending`, `active`, `completed`, `cancelled`
- `game_type` -- фильтр по типу игры
- `limit` -- размер страницы (по умолчанию: 50)
- `offset` -- смещение

#### GET /tournaments/{id} :globe_with_meridians: Публичный

Получение информации о турнире.

```http
GET /tournaments/{id}
```

Ответ: `200 OK`
```json
{
  "id": "uuid",
  "name": "Турнир",
  "description": "...",
  "status": "active",
  "max_team_size": 3,
  "games": [
    {
      "game_id": "uuid",
      "game_name": "Дилемма заключённого",
      "is_active": true,
      "round_status": "running",
      "round_number": 2
    }
  ],
  "teams_count": 15,
  "created_at": "2026-01-01T00:00:00Z"
}
```

#### GET /tournaments/{id}/leaderboard :globe_with_meridians: Публичный

Таблица лидеров турнира.

```http
GET /tournaments/{id}/leaderboard?limit=100
```

Ответ: `200 OK`
```json
[
  {
    "rank": 1,
    "team_id": "uuid",
    "team_name": "TopTeam",
    "rating": 1650,
    "wins": 10,
    "losses": 2,
    "draws": 1
  }
]
```

#### GET /tournaments/{id}/cross-game-leaderboard :globe_with_meridians: Публичный

Кросс-игровой рейтинг турнира. Объединяет результаты по всем играм.

```http
GET /tournaments/{id}/cross-game-leaderboard
```

Ответ: `200 OK`
```json
[
  {
    "rank": 1,
    "team_id": "uuid",
    "team_name": "TopTeam",
    "program_name": "MyBot v3",
    "game_ratings": {
      "game-uuid-1": 1650,
      "game-uuid-2": 1580
    },
    "total_rating": 3230,
    "total_wins": 18,
    "total_losses": 4,
    "total_games": 22
  }
]
```

#### GET /tournaments/{id}/matches :globe_with_meridians: Публичный

Список матчей турнира с пагинацией.

```http
GET /tournaments/{id}/matches?limit=50&offset=0
```

#### GET /tournaments/{id}/matches/rounds :globe_with_meridians: Публичный

Матчи турнира, сгруппированные по раундам. Удобно для отображения прогресса по раундам.

```http
GET /tournaments/{id}/matches/rounds
```

Ответ: `200 OK` -- массив объектов `MatchRound`, каждый содержит номер раунда и список матчей.

#### GET /tournaments/{id}/games :globe_with_meridians: Публичный

Список игр, подключённых к турниру.

```http
GET /tournaments/{id}/games
```

Ответ: `200 OK` -- массив объектов игр.

#### GET /tournaments/{id}/teams :globe_with_meridians: Публичный

Список команд, участвующих в турнире.

```http
GET /tournaments/{id}/teams
```

Ответ: `200 OK` -- массив объектов команд.

#### GET /tournaments/{id}/games/{gameId}/leaderboard :globe_with_meridians: Публичный

Таблица лидеров для конкретной игры в турнире.

```http
GET /tournaments/{id}/games/{gameId}/leaderboard?limit=100
```

Ответ: `200 OK` -- формат аналогичен общей таблице лидеров.

#### GET /tournaments/{id}/games/{gameId}/matches :globe_with_meridians: Публичный

Матчи для конкретной игры в турнире.

```http
GET /tournaments/{id}/games/{gameId}/matches?status=completed&limit=50&offset=0
```

Параметры запроса:
- `status` -- фильтр: `pending`, `running`, `completed`, `failed`
- `limit` -- размер страницы (по умолчанию: 50)
- `offset` -- смещение

#### GET /tournaments/{id}/games/status :globe_with_meridians: Публичный

Игры турнира с расширенной информацией о статусе раундов.

```http
GET /tournaments/{id}/games/status
```

Ответ: `200 OK`
```json
[
  {
    "tournament_id": "uuid",
    "game_id": "uuid",
    "game_name": "prisoners_dilemma",
    "game_display_name": "Дилемма заключённого",
    "is_active": true,
    "round_completed": false,
    "round_completed_at": null,
    "current_round": 2
  }
]
```

#### GET /tournaments/{id}/active-game :globe_with_meridians: Публичный

Текущая активная игра турнира. Возвращает `null`, если активной игры нет.

```http
GET /tournaments/{id}/active-game
```

Ответ: `200 OK` -- объект `TournamentGameWithDetails` или `null`.

### Эндпоинты с авторизацией

#### POST /tournaments/{id}/join :key: Требует авторизации

Присоединение к турниру с указанием программы.

```http
POST /tournaments/{id}/join
Authorization: Bearer <token>
Content-Type: application/json

{
  "program_id": "uuid"
}
```

Ответ: `200 OK`
```json
{"status": "joined"}
```

#### GET /tournaments/{id}/my-team :key: Требует авторизации

Получение команды текущего пользователя в турнире. Возвращает `null`, если пользователь не состоит в команде в данном турнире.

```http
GET /tournaments/{id}/my-team
Authorization: Bearer <token>
```

Ответ: `200 OK` -- объект команды или `null`.

#### POST /tournaments/{id}/games :key: Требует авторизации

Добавление игры в турнир. Доступно администраторам и создателю турнира.

```http
POST /tournaments/{id}/games
Authorization: Bearer <token>
Content-Type: application/json

{
  "game_id": "uuid"
}
```

Ответ: `204 No Content`.

### Административные эндпоинты

#### POST /tournaments :crown: Только администратор

Создание нового турнира.

```http
POST /tournaments
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Еженедельный чемпионат",
  "description": "Описание турнира (Markdown)",
  "max_team_size": 3,
  "max_participants": 100,
  "is_perpetual": false
}
```

Ответ: `201 Created` -- созданный объект турнира.

#### POST /tournaments/{id}/start :crown: Только администратор

Запуск турнира. Переводит статус в `active`.

```http
POST /tournaments/{id}/start
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{"status": "started"}
```

#### POST /tournaments/{id}/complete :crown: Только администратор

Завершение турнира. Переводит статус в `completed`.

```http
POST /tournaments/{id}/complete
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{"status": "completed"}
```

#### DELETE /tournaments/{id} :crown: Только администратор

Удаление турнира.

```http
DELETE /tournaments/{id}
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

#### POST /tournaments/{id}/matches :crown: Только администратор

Создание одиночного матча между двумя программами.

```http
POST /tournaments/{id}/matches
Authorization: Bearer <token>
Content-Type: application/json

{
  "program1_id": "uuid",
  "program2_id": "uuid",
  "priority": "medium"
}
```

Значения `priority`: `low`, `medium`, `high`. По умолчанию: `medium`.

Ответ: `201 Created` -- объект созданного матча.

#### DELETE /tournaments/{id}/games/{gameId} :crown: Только администратор

Удаление игры из турнира.

```http
DELETE /tournaments/{id}/games/{gameId}
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

#### GET /tournaments/{id}/games/{gameId}/programs :crown: Только администратор

Список программ, загруженных для конкретной игры в турнире.

```http
GET /tournaments/{id}/games/{gameId}/programs
Authorization: Bearer <token>
```

Ответ: `200 OK` -- массив объектов программ.

#### POST /tournaments/{id}/games/{gameId}/complete-round :crown: Только администратор

Пометить раунд игры как завершённый. Блокирует загрузку новых программ для этой игры.

```http
POST /tournaments/{id}/games/{gameId}/complete-round
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

#### POST /tournaments/{id}/games/{gameId}/reset-round :crown: Только администратор

Полный сброс раунда игры: удаление матчей, сброс рейтингов и статистики участников.

```http
POST /tournaments/{id}/games/{gameId}/reset-round
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "matches_deleted": 45,
  "participants_reset": 10,
  "rating_history_reset": 90
}
```

#### POST /tournaments/{id}/games/{gameId}/auto-round :crown: Только администратор

Включение или отключение автоматического запуска раундов для игры.

```http
POST /tournaments/{id}/games/{gameId}/auto-round
Authorization: Bearer <token>
Content-Type: application/json

{
  "enabled": true,
  "interval_seconds": 120
}
```

Поле `interval_seconds` должно быть в диапазоне от 10 до 3600. Обязательно при `enabled: true`.

Ответ: `200 OK`
```json
{
  "enabled": true,
  "interval_seconds": 120
}
```

#### GET /tournaments/{id}/games/{gameId}/auto-round :crown: Только администратор

Получение текущего статуса авто-раунда для игры.

```http
GET /tournaments/{id}/games/{gameId}/auto-round
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "enabled": true,
  "interval_seconds": 120,
  "last_run_at": "2026-03-17T12:00:00Z"
}
```

#### POST /tournaments/{id}/active-game :crown: Только администратор

Установка активной игры в турнире. Только одна игра может быть активной одновременно.

```http
POST /tournaments/{id}/active-game
Authorization: Bearer <token>
Content-Type: application/json

{
  "game_id": "uuid"
}
```

Ответ: `204 No Content`.

#### POST /tournaments/{id}/games/deactivate-all :crown: Только администратор

Деактивация всех игр в турнире. Снимает флаг активности со всех игр.

```http
POST /tournaments/{id}/games/deactivate-all
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

#### POST /tournaments/{id}/run-matches :crown: Только администратор

Запуск всех ожидающих матчей турнира. Добавляет их в очередь обработки.

```http
POST /tournaments/{id}/run-matches
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "status": "started",
  "enqueued": 45
}
```

#### POST /tournaments/{id}/run-game-matches :crown: Только администратор

Запуск матчей для конкретной игры в турнире.

```http
POST /tournaments/{id}/run-game-matches
Authorization: Bearer <token>
Content-Type: application/json

{
  "game_type": "prisoners_dilemma"
}
```

Ответ: `200 OK`
```json
{
  "status": "started",
  "game_type": "prisoners_dilemma",
  "enqueued": 15
}
```

#### POST /tournaments/{id}/retry-matches :crown: Только администратор

Повторный запуск всех неудачных (failed) матчей турнира.

```http
POST /tournaments/{id}/retry-matches
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "status": "retried",
  "enqueued": 3
}
```

#### POST /tournaments/{id}/programs/clear-errors :crown: Только администратор

Очистка сообщений об ошибках для всех программ в турнире. Полезно после массового исправления проблем.

```http
POST /tournaments/{id}/programs/clear-errors
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "cleared": 5,
  "message": "Очищено 5 ошибок"
}
```

---

## Команды (Teams)

Базовый путь: `/api/v1/teams`

Все эндпоинты команд требуют авторизации.

### POST /teams :key: Требует авторизации

Создание новой команды в турнире. Текущий пользователь становится лидером.

```http
POST /teams
Authorization: Bearer <token>
Content-Type: application/json

{
  "tournament_id": "uuid",
  "name": "Моя команда"
}
```

Ответ: `201 Created`
```json
{
  "id": "uuid",
  "name": "Моя команда",
  "invite_code": "ABC123",
  "leader_id": "uuid",
  "members": [
    {"id": "uuid", "username": "player1"}
  ]
}
```

### POST /teams/join :key: Требует авторизации

Вступление в команду по коду приглашения.

```http
POST /teams/join
Authorization: Bearer <token>
Content-Type: application/json

{
  "code": "ABC123"
}
```

Ответ: `200 OK` -- объект команды.

### GET /teams/{id} :key: Требует авторизации

Получение информации о команде с участниками.

```http
GET /teams/{id}
Authorization: Bearer <token>
```

### PUT /teams/{id} :key: Требует авторизации

Обновление названия команды. Доступно только лидеру команды.

```http
PUT /teams/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Новое название"
}
```

Ответ: `200 OK` -- обновлённый объект команды.

### GET /teams/{id}/members :key: Требует авторизации

Получение списка участников команды.

```http
GET /teams/{id}/members
Authorization: Bearer <token>
```

Ответ: `200 OK` -- массив объектов участников.

### POST /teams/{id}/leave :key: Требует авторизации

Покинуть команду.

```http
POST /teams/{id}/leave
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

### DELETE /teams/{id}/members/{userId} :key: Требует авторизации

Исключение участника из команды. Доступно только лидеру.

```http
DELETE /teams/{id}/members/{userId}
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

### GET /teams/{id}/invite :key: Требует авторизации

Получение кода и ссылки приглашения в команду. Доступно только лидеру.

```http
GET /teams/{id}/invite
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "code": "ABC123",
  "link": "http://localhost:8080/join?code=ABC123"
}
```

### DELETE /teams/{id} :crown: Только администратор

Удаление команды.

```http
DELETE /teams/{id}
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

---

## Программы (Programs)

Базовый путь: `/api/v1/programs`

Все эндпоинты программ требуют авторизации. Максимальный размер загружаемого файла: **10 МБ**.

### POST /programs :key: Требует авторизации

Загрузка новой программы. Поддерживает два формата: `multipart/form-data` (файл) и `application/json` (путь).

**Загрузка файла (рекомендуемый способ):**

```http
POST /programs
Authorization: Bearer <token>
Content-Type: multipart/form-data

file: <binary>
team_id: "uuid"
tournament_id: "uuid"
game_id: "uuid"
name: "My Strategy v2"
```

Поле `name` опционально -- если не указано, используется имя файла.

Сервер автоматически:
- Определяет язык по расширению файла (python, cpp, c, go, rust, java, javascript, ruby, php, lua)
- Проверяет синтаксис кода
- Компилирует программу (для компилируемых языков)
- Добавляет shebang для интерпретируемых языков (если отсутствует)
- Регистрирует программу как участника турнира
- Назначает атомарную версию

Ответ: `201 Created`
```json
{
  "id": "uuid",
  "name": "My Strategy v2",
  "language": "python",
  "version": 3,
  "error_message": null,
  "created_at": "2026-01-01T00:00:00Z"
}
```

Если обнаружена ошибка синтаксиса или компиляции, поле `error_message` содержит описание проблемы, но программа все равно сохраняется.

> **Важно:** Загрузка программ может быть заблокирована, если:
> - Турнир ещё не начался (статус не `active`)
> - Раунд игры уже завершён (в ручном режиме)
> - Выполняются матчи другой игры (в ручном режиме)
>
> В режиме авто-раунда эти ограничения снимаются.

**JSON-запрос (обратная совместимость):**

```http
POST /programs
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "My Strategy",
  "game_type": "prisoners_dilemma",
  "code_path": "strategy.py",
  "language": "python"
}
```

### GET /programs :key: Требует авторизации

Список программ текущего пользователя.

```http
GET /programs?tournament_id=uuid&game_id=uuid
Authorization: Bearer <token>
```

### GET /programs/versions :key: Требует авторизации

Список всех версий программ для команды и игры. Позволяет отслеживать историю загрузок.

```http
GET /programs/versions?team_id=uuid&game_id=uuid
Authorization: Bearer <token>
```

Параметры запроса (обязательные):
- `team_id` -- идентификатор команды
- `game_id` -- идентификатор игры

Ответ: `200 OK` -- массив объектов программ, отсортированных по версии.

### GET /programs/{id} :key: Требует авторизации

Получение информации о программе. Владелец видит свои программы, администратор -- все.

```http
GET /programs/{id}
Authorization: Bearer <token>
```

### GET /programs/{id}/download :key: Требует авторизации

Скачивание исходного файла программы. Владелец может скачивать свои программы, администратор -- любые.

```http
GET /programs/{id}/download
Authorization: Bearer <token>
```

Ответ: бинарный файл с заголовком `Content-Disposition: attachment`.

### PUT /programs/{id} :key: Требует авторизации

Обновление метаданных программы (имя, путь к коду, язык). Доступно только владельцу.

```http
PUT /programs/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Updated Strategy",
  "code_path": "new_strategy.py",
  "language": "python"
}
```

Ответ: `200 OK` -- обновлённый объект программы.

### DELETE /programs/{id} :key: Требует авторизации

Удаление программы. Доступно только владельцу. Удаляет также файл с диска.

```http
DELETE /programs/{id}
Authorization: Bearer <token>
```

Ответ: `204 No Content`.

---

## Матчи (Matches)

Базовый путь: `/api/v1/matches`

### Публичные эндпоинты (с опциональной авторизацией)

Эндпоинты списка и получения матчей используют **OptionalAuth**: они доступны без авторизации, но авторизованные пользователи получают дополнительную информацию:

- **Администраторы** видят полные сообщения об ошибках для всех матчей
- **Владельцы программы** видят полные ошибки, если их программа упала
- **Остальные** видят обезличенное сообщение: "Программа оппонента завершилась с ошибкой"

#### GET /matches :globe_with_meridians: Публичный (OptionalAuth)

Список матчей с фильтрацией.

```http
GET /matches?tournament_id=uuid&program_id=uuid&status=completed&game_type=prisoners_dilemma&limit=50&offset=0
```

Параметры запроса:
- `tournament_id` -- фильтр по турниру
- `program_id` -- фильтр по программе
- `status` -- фильтр: `pending`, `running`, `completed`, `failed`
- `game_type` -- фильтр по типу игры
- `limit` -- размер страницы (по умолчанию: 50)
- `offset` -- смещение

#### GET /matches/{id} :globe_with_meridians: Публичный (OptionalAuth)

Получение информации о матче.

```http
GET /matches/{id}
```

Ответ: `200 OK`
```json
{
  "id": "uuid",
  "tournament_id": "uuid",
  "game_id": "uuid",
  "program1": {
    "id": "uuid",
    "name": "Bot1",
    "team_name": "Team1"
  },
  "program2": {
    "id": "uuid",
    "name": "Bot2",
    "team_name": "Team2"
  },
  "winner_id": "uuid",
  "status": "completed",
  "score1": 1500,
  "score2": 1200,
  "round_number": 2,
  "error_message": null,
  "created_at": "2026-01-01T00:00:00Z",
  "completed_at": "2026-01-01T00:01:00Z"
}
```

#### GET /matches/statistics :globe_with_meridians: Публичный (OptionalAuth)

Статистика матчей. Можно фильтровать по турниру.

```http
GET /matches/statistics?tournament_id=uuid
```

Параметры запроса:
- `tournament_id` -- фильтр по турниру (опционально; без фильтра -- глобальная статистика)

Ответ: `200 OK` -- объект со статистикой (количество матчей по статусам, среднее время и т.д.).

### Административные эндпоинты

#### GET /matches/queue/stats :crown: Только администратор

Статистика очереди матчей (Redis). Показывает количество матчей в каждой очереди приоритетов.

```http
GET /matches/queue/stats
Authorization: Bearer <token>
```

Ответ: `200 OK` -- объект со статистикой очереди.

#### POST /matches/queue/clear :crown: Только администратор

Полная очистка всех очередей матчей.

```http
POST /matches/queue/clear
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{"message": "All queues cleared successfully"}
```

#### POST /matches/queue/purge :crown: Только администратор

Удаление из очереди матчей, которых нет в базе данных. Полезно для очистки "зависших" записей.

```http
POST /matches/queue/purge
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "message": "Invalid matches purged successfully",
  "purged_count": 3
}
```

---

## WebSocket

Базовый путь: `/api/v1/ws`

Все WebSocket-эндпоинты требуют авторизации. Токен передаётся через параметр запроса или subprotocol.

### GET /ws/tournaments/{id} :key: Требует авторизации

Подключение к real-time обновлениям турнира.

```
WS /ws/tournaments/{id}?token=<jwt>
```

Альтернативный способ передачи токена -- через WebSocket subprotocol:
```
Sec-WebSocket-Protocol: access_token.<jwt>
```

#### Типы сообщений

**Обновление лидерборда:**
```json
{
  "type": "leaderboard_update",
  "payload": {
    "game_id": "uuid",
    "entries": [
      {"rank": 1, "team_name": "Team1", "rating": 1650}
    ]
  }
}
```

**Обновление матча:**
```json
{
  "type": "match_update",
  "payload": {
    "match_id": "uuid",
    "status": "completed",
    "winner_id": "uuid",
    "score1": 1500,
    "score2": 1200
  }
}
```

**Обновление раунда:**
```json
{
  "type": "round_update",
  "payload": {
    "game_id": "uuid",
    "round_status": "running",
    "round_number": 3
  }
}
```

**Турнир завершён:**
```json
{
  "type": "tournament_completed",
  "payload": {
    "tournament_id": "uuid"
  }
}
```

### GET /ws/stats :key: Требует авторизации

Получение статистики WebSocket-подключений (количество активных клиентов, подключения по турнирам и т.д.).

```http
GET /ws/stats
Authorization: Bearer <token>
```

Ответ: `200 OK` -- объект со статистикой подключений.

---

## Система (System)

Базовый путь: `/api/v1/system`

Все эндпоинты доступны только администраторам.

### GET /system/metrics :crown: Только администратор

Подробные метрики сервера: CPU, память, диск, Go runtime.

```http
GET /system/metrics
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "cpu": {
    "usage_percent": 23.5,
    "cores": 8,
    "model_name": "Apple M1 Pro",
    "per_core": [15.2, 30.1, 22.0, ...]
  },
  "memory": {
    "total": 17179869184,
    "used": 8589934592,
    "free": 8589934592,
    "used_percent": 50.0
  },
  "disk": {
    "total": 500107862016,
    "used": 250053931008,
    "free": 250053931008,
    "used_percent": 50.0,
    "path": "/"
  },
  "host": {
    "hostname": "server-01",
    "platform": "darwin",
    "platform_version": "14.0",
    "os": "darwin",
    "arch": "arm64",
    "uptime": 86400
  },
  "go": {
    "version": "go1.24.0",
    "goroutines": 42,
    "heap_alloc": 10485760,
    "heap_sys": 20971520,
    "num_gc": 15,
    "gomaxprocs": 8
  },
  "temperature": [
    {"sensor_key": "CPU", "temperature": 55.0}
  ]
}
```

### GET /system/health :crown: Только администратор

Проверка здоровья системы с диагностикой ресурсов.

```http
GET /system/health
Authorization: Bearer <token>
```

Ответ: `200 OK`
```json
{
  "status": "healthy",
  "timestamp": "2026-03-17T12:00:00Z",
  "hostname": "server-01",
  "pid": 12345
}
```

Поле `status` может принимать значения: `healthy`, `warning` (при использовании памяти > 90%).

---

## Ответы с ошибками

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Неверные данные",
    "details": {"field": "email", "reason": "неверный формат"}
  }
}
```

| Код | HTTP статус | Описание |
|-----|-------------|----------|
| NOT_FOUND | 404 | Ресурс не найден |
| UNAUTHORIZED | 401 | Отсутствует/неверный токен |
| FORBIDDEN | 403 | Недостаточно прав |
| VALIDATION_ERROR | 400 | Неверные данные |
| CONFLICT | 409 | Конфликт ресурсов (напр. дубликат) |
| RATE_LIMITED | 429 | Слишком много запросов |
| INTERNAL_ERROR | 500 | Ошибка сервера |

---

## Лимиты запросов

- Настраиваемый лимит запросов в минуту (через конфигурацию)
- Ответ `429 Too Many Requests` при превышении
- Заголовок `X-RateLimit-Remaining` показывает оставшуюся квоту
- Заголовок `X-RateLimit-Reset` показывает время сброса

---

## Пагинация

API поддерживает пагинацию на основе `limit` и `offset`:

```http
GET /tournaments?limit=20&offset=40
```

- `limit` -- количество записей на странице (значение по умолчанию зависит от эндпоинта, обычно 50)
- `offset` -- смещение от начала списка

Для некоторых эндпоинтов также поддерживается курсорная пагинация:

```json
{
  "data": [...],
  "next_cursor": "eyJ...",
  "has_more": true
}
```

Передайте параметр `cursor` для получения следующей страницы.

---

## Системные эндпоинты

Эндпоинты вне `/api/v1`, доступные без авторизации.

### Health check

```http
GET /health
```

Ответ: `200 OK` с телом `OK`.

Облегчённая проверка работоспособности для балансировщиков нагрузки и мониторинга.

### Метрики Prometheus

```http
GET /metrics
```

Метрики в формате Prometheus для интеграции с системой мониторинга.

### Статические файлы

```
GET /*
```

Все неопознанные маршруты обслуживают встроенный React SPA (single-page application) с fallback на `index.html`.

---

## Сводная таблица эндпоинтов

| Метод | Путь | Доступ | Описание |
|-------|------|--------|----------|
| **Auth** | | | |
| POST | `/auth/register` | :globe_with_meridians: | Регистрация |
| POST | `/auth/login` | :globe_with_meridians: | Вход |
| POST | `/auth/refresh` | :globe_with_meridians: | Обновление токенов |
| POST | `/auth/logout` | :key: | Выход |
| GET | `/auth/me` | :key: | Текущий пользователь |
| PUT | `/auth/profile` | :key: | Обновление профиля |
| **Games** | | | |
| GET | `/games` | :globe_with_meridians: | Список игр |
| GET | `/games/{id}` | :globe_with_meridians: | Игра по ID |
| GET | `/games/name/{name}` | :globe_with_meridians: | Игра по имени |
| POST | `/games` | :crown: | Создать игру |
| PUT | `/games/{id}` | :crown: | Обновить игру |
| DELETE | `/games/{id}` | :crown: | Удалить игру |
| **Tournaments -- публичные** | | | |
| GET | `/tournaments` | :globe_with_meridians: | Список турниров |
| GET | `/tournaments/{id}` | :globe_with_meridians: | Турнир по ID |
| GET | `/tournaments/{id}/leaderboard` | :globe_with_meridians: | Лидерборд |
| GET | `/tournaments/{id}/cross-game-leaderboard` | :globe_with_meridians: | Кросс-игровой рейтинг |
| GET | `/tournaments/{id}/matches` | :globe_with_meridians: | Матчи турнира |
| GET | `/tournaments/{id}/matches/rounds` | :globe_with_meridians: | Матчи по раундам |
| GET | `/tournaments/{id}/games` | :globe_with_meridians: | Игры турнира |
| GET | `/tournaments/{id}/teams` | :globe_with_meridians: | Команды турнира |
| GET | `/tournaments/{id}/games/{gameId}/leaderboard` | :globe_with_meridians: | Лидерборд по игре |
| GET | `/tournaments/{id}/games/{gameId}/matches` | :globe_with_meridians: | Матчи по игре |
| GET | `/tournaments/{id}/games/status` | :globe_with_meridians: | Статус игр |
| GET | `/tournaments/{id}/active-game` | :globe_with_meridians: | Активная игра |
| **Tournaments -- авторизация** | | | |
| POST | `/tournaments/{id}/join` | :key: | Присоединиться |
| GET | `/tournaments/{id}/my-team` | :key: | Моя команда |
| POST | `/tournaments/{id}/games` | :key: | Добавить игру |
| **Tournaments -- администрирование** | | | |
| POST | `/tournaments` | :crown: | Создать турнир |
| POST | `/tournaments/{id}/start` | :crown: | Запустить турнир |
| POST | `/tournaments/{id}/complete` | :crown: | Завершить турнир |
| DELETE | `/tournaments/{id}` | :crown: | Удалить турнир |
| POST | `/tournaments/{id}/matches` | :crown: | Создать матч |
| DELETE | `/tournaments/{id}/games/{gameId}` | :crown: | Удалить игру из турнира |
| GET | `/tournaments/{id}/games/{gameId}/programs` | :crown: | Программы для игры |
| POST | `/tournaments/{id}/games/{gameId}/complete-round` | :crown: | Завершить раунд |
| POST | `/tournaments/{id}/games/{gameId}/reset-round` | :crown: | Сбросить раунд |
| POST | `/tournaments/{id}/games/{gameId}/auto-round` | :crown: | Настроить авто-раунд |
| GET | `/tournaments/{id}/games/{gameId}/auto-round` | :crown: | Статус авто-раунда |
| POST | `/tournaments/{id}/active-game` | :crown: | Установить активную игру |
| POST | `/tournaments/{id}/games/deactivate-all` | :crown: | Деактивировать все игры |
| POST | `/tournaments/{id}/run-matches` | :crown: | Запустить все матчи |
| POST | `/tournaments/{id}/run-game-matches` | :crown: | Запустить матчи по игре |
| POST | `/tournaments/{id}/retry-matches` | :crown: | Перезапустить неудачные |
| POST | `/tournaments/{id}/programs/clear-errors` | :crown: | Очистить ошибки программ |
| **Teams** | | | |
| POST | `/teams` | :key: | Создать команду |
| POST | `/teams/join` | :key: | Вступить по коду |
| GET | `/teams/{id}` | :key: | Получить команду |
| PUT | `/teams/{id}` | :key: | Обновить название |
| GET | `/teams/{id}/members` | :key: | Участники команды |
| POST | `/teams/{id}/leave` | :key: | Покинуть команду |
| DELETE | `/teams/{id}/members/{userId}` | :key: | Исключить участника |
| GET | `/teams/{id}/invite` | :key: | Ссылка приглашения |
| DELETE | `/teams/{id}` | :crown: | Удалить команду |
| **Programs** | | | |
| POST | `/programs` | :key: | Загрузить программу |
| GET | `/programs` | :key: | Список программ |
| GET | `/programs/versions` | :key: | Версии программ |
| GET | `/programs/{id}` | :key: | Получить программу |
| GET | `/programs/{id}/download` | :key: | Скачать программу |
| PUT | `/programs/{id}` | :key: | Обновить программу |
| DELETE | `/programs/{id}` | :key: | Удалить программу |
| **Matches** | | | |
| GET | `/matches` | :globe_with_meridians: | Список матчей |
| GET | `/matches/{id}` | :globe_with_meridians: | Получить матч |
| GET | `/matches/statistics` | :globe_with_meridians: | Статистика матчей |
| GET | `/matches/queue/stats` | :crown: | Статистика очереди |
| POST | `/matches/queue/clear` | :crown: | Очистить очередь |
| POST | `/matches/queue/purge` | :crown: | Удалить невалидные |
| **WebSocket** | | | |
| GET | `/ws/tournaments/{id}` | :key: | Real-time турнира |
| GET | `/ws/stats` | :key: | Статистика WS |
| **System** | | | |
| GET | `/system/metrics` | :crown: | Метрики сервера |
| GET | `/system/health` | :crown: | Здоровье системы |

---

*Версия документации: 4.0*
*Последнее обновление: Март 2026*
