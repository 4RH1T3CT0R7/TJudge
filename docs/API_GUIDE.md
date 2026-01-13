# Руководство по API

Базовый URL: `http://localhost:8080/api/v1`

## Аутентификация

Все защищённые эндпоинты требуют заголовок `Authorization: Bearer <token>`.

### Регистрация

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

### Вход

```http
POST /auth/login
Content-Type: application/json

{"email": "player1@example.com", "password": "SecurePass123!"}
```

Ответ: `200 OK` с токенами.

### Обновление токена

```http
POST /auth/refresh
Content-Type: application/json

{"refresh_token": "eyJ..."}
```

### Выход

```http
POST /auth/logout
Authorization: Bearer <token>
```

### Текущий пользователь

```http
GET /auth/me
Authorization: Bearer <token>
```

---

## Игры

### Список игр

```http
GET /games
```

Ответ:
```json
{
  "games": [
    {
      "id": "uuid",
      "slug": "prisoners_dilemma",
      "name": "Дилемма заключённого",
      "rules": "# Правила\n\n...",
      "score_multiplier": 1.0,
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

### Создание игры (админ)

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

### Получение игры

```http
GET /games/{id}
```

### Обновление игры (админ)

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

### Удаление игры (админ)

```http
DELETE /games/{id}
Authorization: Bearer <token>
```

---

## Турниры

### Список турниров

```http
GET /tournaments?status=active&limit=10&cursor=xxx
```

Параметры запроса:
- `status`: pending, active, completed
- `limit`: размер страницы (по умолчанию: 20)
- `cursor`: курсор пагинации

### Создание турнира (админ)

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

### Получение турнира

```http
GET /tournaments/{id}
```

Ответ:
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

### Добавление игры в турнир (админ)

```http
POST /tournaments/{id}/games
Authorization: Bearer <token>
Content-Type: application/json

{
  "game_id": "uuid"
}
```

### Удаление игры из турнира (админ)

```http
DELETE /tournaments/{id}/games/{game_id}
Authorization: Bearer <token>
```

### Запуск раунда игры (админ)

```http
POST /tournaments/{id}/games/{game_id}/start
Authorization: Bearer <token>
```

### Остановка раунда игры (админ)

```http
POST /tournaments/{id}/games/{game_id}/stop
Authorization: Bearer <token>
```

### Запуск турнира (админ)

```http
POST /tournaments/{id}/start
Authorization: Bearer <token>
```

### Завершение турнира (админ)

```http
POST /tournaments/{id}/complete
Authorization: Bearer <token>
```

### Таблица лидеров

```http
GET /tournaments/{id}/leaderboard?game_id=uuid&limit=100
```

Ответ:
```json
{
  "entries": [
    {
      "rank": 1,
      "team_id": "uuid",
      "team_name": "TopTeam",
      "rating": 1650,
      "wins": 10,
      "losses": 2,
      "draws": 1
    }
  ],
  "total": 50
}
```

### Матчи турнира

```http
GET /tournaments/{id}/matches?game_id=uuid&status=completed&limit=50
```

---

## Команды

### Создание команды

```http
POST /teams
Authorization: Bearer <token>
Content-Type: application/json

{
  "tournament_id": "uuid",
  "name": "Моя команда"
}
```

Ответ:
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

### Присоединение по коду

```http
POST /teams/join
Authorization: Bearer <token>
Content-Type: application/json

{
  "code": "ABC123"
}
```

### Получение команды

```http
GET /teams/{id}
Authorization: Bearer <token>
```

### Покинуть команду

```http
POST /teams/{id}/leave
Authorization: Bearer <token>
```

### Исключить участника (лидер)

```http
POST /teams/{id}/kick
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_id": "uuid"
}
```

### Регенерация кода приглашения (лидер)

```http
POST /teams/{id}/regenerate-code
Authorization: Bearer <token>
```

---

## Программы

### Загрузка программы

```http
POST /programs
Authorization: Bearer <token>
Content-Type: multipart/form-data

file: <binary>
tournament_id: "uuid"
game_id: "uuid"
name: "My Strategy"
```

Ответ:
```json
{
  "id": "uuid",
  "name": "My Strategy",
  "language": "python",
  "status": "pending",
  "created_at": "2026-01-01T00:00:00Z"
}
```

### Список программ

```http
GET /programs?tournament_id=uuid&game_id=uuid
Authorization: Bearer <token>
```

### Получение программы

```http
GET /programs/{id}
Authorization: Bearer <token>
```

### Удаление программы

```http
DELETE /programs/{id}
Authorization: Bearer <token>
```

---

## Матчи

### Получение матча

```http
GET /matches/{id}
```

Ответ:
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
  "created_at": "2026-01-01T00:00:00Z",
  "completed_at": "2026-01-01T00:01:00Z"
}
```

### Список матчей

```http
GET /matches?tournament_id=uuid&game_id=uuid&status=completed&limit=50
```

---

## WebSocket

### Подписка на турнир

```
WS /ws/tournaments/{id}?token=<jwt>
```

### Типы сообщений

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

- Настраиваемый лимит запросов в минуту
- Ответ 429 при превышении
- Заголовок `X-RateLimit-Remaining` показывает оставшуюся квоту
- Заголовок `X-RateLimit-Reset` показывает время сброса

---

## Пагинация

Курсорная пагинация:
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

### Health check

```http
GET /health
```

Ответ: `200 OK`
```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

### Метрики Prometheus

```http
GET /metrics
```

---

*Версия документации: 2.0*
*Последнее обновление: Январь 2026*
