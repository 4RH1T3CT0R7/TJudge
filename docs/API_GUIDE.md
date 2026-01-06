# Руководство по API

Базовый URL: `https://api.tjudge.example.com/api/v1`

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
  "user": {"id": "uuid", "username": "player1"},
  "access_token": "eyJ...",
  "refresh_token": "eyJ..."
}
```

### Вход
```http
POST /auth/login
Content-Type: application/json

{"username": "player1", "password": "SecurePass123!"}
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

## Программы

### Создание программы
```http
POST /programs
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "MyBot",
  "language": "python",
  "source_code": "print(input())",
  "game_type": "tictactoe"
}
```

### Список моих программ
```http
GET /programs
Authorization: Bearer <token>
```

### Получение/Обновление/Удаление программы
```http
GET    /programs/:id
PUT    /programs/:id
DELETE /programs/:id
```

## Турниры

### Создание турнира
```http
POST /tournaments
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Еженедельный чемпионат",
  "game_type": "tictactoe",
  "max_participants": 16,
  "iterations_per_match": 1000
}
```

### Список турниров
```http
GET /tournaments?status=active&game_type=tictactoe&limit=10&cursor=xxx
```

Параметры запроса:
- `status`: pending, active, completed
- `game_type`: фильтр по игре
- `limit`: размер страницы (по умолчанию: 20)
- `cursor`: курсор пагинации

### Получение турнира
```http
GET /tournaments/:id
```

### Присоединение к турниру
```http
POST /tournaments/:id/join
Authorization: Bearer <token>
Content-Type: application/json

{"program_id": "uuid"}
```

### Старт турнира
```http
POST /tournaments/:id/start
Authorization: Bearer <token>
```
Требуется: создатель турнира или админ.

### Таблица лидеров
```http
GET /tournaments/:id/leaderboard
```
Ответ:
```json
{
  "entries": [
    {"rank": 1, "program_name": "TopBot", "rating": 1650, "wins": 10, "losses": 2}
  ]
}
```

### Матчи турнира
```http
GET /tournaments/:id/matches?status=completed&limit=50
```

## Матчи

### Получение матча
```http
GET /matches/:id
```

### Список матчей
```http
GET /matches?tournament_id=uuid&status=completed
```

### Статистика матчей
```http
GET /matches/statistics
```

## WebSocket

### Подписка на турнир
```
WS /ws/tournaments/:id
Авторизация через query: ?token=<jwt>
```

Получаемые сообщения:
```json
{"type": "tournament_started", "payload": {...}}
{"type": "match_completed", "payload": {"match_id": "...", "winner_id": "..."}}
{"type": "tournament_completed", "payload": {...}}
```

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
| CONFLICT | 409 | Конфликт ресурсов |
| RATE_LIMITED | 429 | Слишком много запросов |
| INTERNAL_ERROR | 500 | Ошибка сервера |

## Лимиты запросов

- 100 запросов/минуту на IP
- Ответ 429 при превышении
- Заголовок `X-RateLimit-Remaining` показывает оставшуюся квоту

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
