# Схема базы данных

## ER-диаграмма

```
┌─────────────┐       ┌──────────────┐       ┌─────────────────┐
│    users    │       │    teams     │       │   tournaments   │
├─────────────┤       ├──────────────┤       ├─────────────────┤
│ id (PK)     │◄──────│ leader_id    │       │ id (PK)         │
│ username    │       │ id (PK)      │◄──┐   │ name            │
│ email       │       │ tournament_id│───┼──▶│ description     │
│ password    │       │ name         │   │   │ status          │
│ role        │       │ invite_code  │   │   │ max_team_size   │
│ created_at  │       │ created_at   │   │   │ created_at      │
│ updated_at  │       └──────────────┘   │   │ started_at      │
└─────────────┘                          │   │ completed_at    │
                                         │   └─────────────────┘
┌─────────────┐                          │
│    games    │                          │
├─────────────┤       ┌──────────────────┴────────────────┐
│ id (PK)     │◄──────│          tournament_games         │
│ slug        │       ├───────────────────────────────────┤
│ name        │       │ tournament_id (FK, PK)            │
│ rules       │       │ game_id (FK, PK)                  │
│ score_mult  │       │ is_active                         │
│ created_at  │       │ round_status                      │
└─────────────┘       │ round_number                      │
                      └───────────────────────────────────┘

┌──────────────┐       ┌───────────────────┐
│   programs   │       │      matches      │
├──────────────┤       ├───────────────────┤
│ id (PK)      │◄──────│ program1_id (FK)  │
│ team_id (FK) │       │ program2_id (FK)  │
│ game_id (FK) │       │ id (PK)           │
│ name         │       │ tournament_id (FK)│
│ language     │       │ game_id (FK)      │
│ file_path    │       │ winner_id (FK)    │
│ status       │       │ status            │
│ created_at   │       │ score1, score2    │
│ updated_at   │       │ round_number      │
└──────────────┘       │ error_code        │
                       │ error_message     │
                       │ created_at        │
                       │ completed_at      │
                       │ version           │
                       └───────────────────┘

┌───────────────────┐
│   rating_history  │
├───────────────────┤
│ id (PK)           │
│ team_id (FK)      │
│ game_id (FK)      │
│ tournament_id (FK)│
│ rating            │
│ wins, losses      │
│ draws             │
│ created_at        │
└───────────────────┘
```

## Таблицы

### users

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| username | VARCHAR(50) | UNIQUE, NOT NULL | Логин |
| email | VARCHAR(255) | UNIQUE, NOT NULL | Email |
| password_hash | VARCHAR(255) | NOT NULL | Хеш bcrypt |
| role | VARCHAR(20) | DEFAULT 'user' | user, admin |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| updated_at | TIMESTAMPTZ | NOT NULL | Время обновления |

Индексы: `idx_users_username`, `idx_users_email`

### games

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| slug | VARCHAR(100) | UNIQUE, NOT NULL | Идентификатор (snake_case) |
| name | VARCHAR(200) | NOT NULL | Отображаемое название |
| rules | TEXT | | Правила игры (Markdown) |
| score_multiplier | DECIMAL(5,2) | DEFAULT 1.0 | Множитель очков |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| updated_at | TIMESTAMPTZ | NOT NULL | Время обновления |

Индексы: `idx_games_slug`

### tournaments

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| name | VARCHAR(200) | NOT NULL | Название турнира |
| description | TEXT | | Описание (Markdown) |
| status | VARCHAR(20) | NOT NULL | pending, active, completed |
| max_team_size | INT | DEFAULT 1 | Макс. участников в команде |
| max_participants | INT | | Макс. команд |
| is_perpetual | BOOLEAN | DEFAULT false | Постоянный турнир |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| started_at | TIMESTAMPTZ | | Время старта |
| completed_at | TIMESTAMPTZ | | Время завершения |
| version | INT | DEFAULT 1 | Optimistic lock |

Индексы: `idx_tournaments_status`

### tournament_games

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| tournament_id | UUID | FK → tournaments, PK | Турнир |
| game_id | UUID | FK → games, PK | Игра |
| is_active | BOOLEAN | DEFAULT true | Активна ли игра |
| round_status | VARCHAR(20) | DEFAULT 'pending' | pending, running, completed |
| round_number | INT | DEFAULT 0 | Номер текущего раунда |
| created_at | TIMESTAMPTZ | NOT NULL | Время добавления |

Первичный ключ: `(tournament_id, game_id)`

### teams

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| tournament_id | UUID | FK → tournaments | Турнир |
| name | VARCHAR(100) | NOT NULL | Название команды |
| invite_code | VARCHAR(10) | UNIQUE, NOT NULL | Код приглашения |
| leader_id | UUID | FK → users | Лидер команды |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |

Индексы: `idx_teams_tournament`, `idx_teams_invite_code`
Уникальность: `(tournament_id, name)`

### team_members

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| team_id | UUID | FK → teams, PK | Команда |
| user_id | UUID | FK → users, PK | Пользователь |
| joined_at | TIMESTAMPTZ | NOT NULL | Время присоединения |

Первичный ключ: `(team_id, user_id)`

### programs

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| team_id | UUID | FK → teams | Команда-владелец |
| game_id | UUID | FK → games | Игра |
| name | VARCHAR(100) | NOT NULL | Название программы |
| language | VARCHAR(20) | NOT NULL | python, go, cpp, java, js, rust |
| file_path | VARCHAR(500) | NOT NULL | Путь к файлу |
| status | VARCHAR(20) | DEFAULT 'pending' | pending, compiling, ready, error |
| error_message | TEXT | | Сообщение об ошибке |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| updated_at | TIMESTAMPTZ | NOT NULL | Время обновления |

Индексы: `idx_programs_team`, `idx_programs_game`
Уникальность: `(team_id, game_id)` — одна программа на игру от команды

### matches

Партиционирована по `created_at` (помесячно).

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| tournament_id | UUID | FK → tournaments | Турнир |
| game_id | UUID | FK → games | Игра |
| program1_id | UUID | FK → programs | Первый игрок |
| program2_id | UUID | FK → programs | Второй игрок |
| winner_id | UUID | FK → programs, NULL | Победитель (null = ничья) |
| status | VARCHAR(20) | NOT NULL | pending, running, completed, failed |
| score1 | INT | | Очки первого игрока |
| score2 | INT | | Очки второго игрока |
| round_number | INT | DEFAULT 0 | Номер раунда |
| error_code | VARCHAR(50) | | Код ошибки |
| error_message | TEXT | | Сообщение об ошибке |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| started_at | TIMESTAMPTZ | | Время старта |
| completed_at | TIMESTAMPTZ | | Время завершения |
| version | INT | DEFAULT 1 | Optimistic lock |

Индексы: `idx_matches_tournament`, `idx_matches_game`, `idx_matches_status`, `idx_matches_programs`

### rating_history

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| team_id | UUID | FK → teams | Команда |
| game_id | UUID | FK → games | Игра |
| tournament_id | UUID | FK → tournaments | Турнир |
| rating | INT | DEFAULT 1500 | Текущий рейтинг ELO |
| wins | INT | DEFAULT 0 | Победы |
| losses | INT | DEFAULT 0 | Поражения |
| draws | INT | DEFAULT 0 | Ничьи |
| created_at | TIMESTAMPTZ | NOT NULL | Время записи |

Индексы: `idx_rating_team_game`, `idx_rating_tournament`

### refresh_tokens

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | ID токена |
| user_id | UUID | FK → users | Владелец |
| token_hash | VARCHAR(255) | NOT NULL | Хеш токена |
| expires_at | TIMESTAMPTZ | NOT NULL | Срок действия |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |

---

## Материализованные представления

### leaderboard_tournament

```sql
CREATE MATERIALIZED VIEW leaderboard_tournament AS
SELECT
    rh.tournament_id,
    rh.game_id,
    rh.team_id,
    t.name as team_name,
    rh.rating,
    rh.wins,
    rh.losses,
    rh.draws,
    RANK() OVER (
        PARTITION BY rh.tournament_id, rh.game_id
        ORDER BY rh.rating DESC, rh.wins DESC
    ) as rank
FROM rating_history rh
JOIN teams t ON rh.team_id = t.id
WHERE rh.id IN (
    SELECT DISTINCT ON (team_id, game_id, tournament_id)
           id
    FROM rating_history
    ORDER BY team_id, game_id, tournament_id, created_at DESC
);

CREATE UNIQUE INDEX ON leaderboard_tournament (tournament_id, game_id, team_id);
```

---

## Миграции

```bash
# Применить все миграции
make migrate-up

# Откатить последнюю миграцию
make migrate-down

# Создать новую миграцию
make migrate-create name=add_new_table

# Статус миграций
make migrate-status
```

Файлы миграций: `migrations/000001_*.sql` до `migrations/000022_*.sql`

**Структура миграций:**
```
migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_programs.up.sql
├── 000002_create_programs.down.sql
...
├── 000022_add_score_multiplier.up.sql
└── 000022_add_score_multiplier.down.sql
```

---

## Частые запросы

### Таблица лидеров

```sql
SELECT
    t.id as team_id,
    t.name as team_name,
    rh.rating,
    rh.wins,
    rh.losses,
    rh.draws
FROM rating_history rh
JOIN teams t ON rh.team_id = t.id
WHERE rh.tournament_id = $1
  AND rh.game_id = $2
  AND rh.id IN (
      SELECT DISTINCT ON (team_id) id
      FROM rating_history
      WHERE tournament_id = $1 AND game_id = $2
      ORDER BY team_id, created_at DESC
  )
ORDER BY rh.rating DESC, rh.wins DESC;
```

### Ожидающие матчи

```sql
SELECT * FROM matches
WHERE status = 'pending'
  AND tournament_id = $1
ORDER BY created_at
LIMIT 100;
```

### Программы команды

```sql
SELECT p.*, g.name as game_name, g.slug as game_slug
FROM programs p
JOIN games g ON p.game_id = g.id
WHERE p.team_id = $1
ORDER BY g.name;
```

### Команды в турнире

```sql
SELECT t.*, u.username as leader_name,
       COUNT(tm.user_id) as member_count
FROM teams t
JOIN users u ON t.leader_id = u.id
LEFT JOIN team_members tm ON t.id = tm.team_id
WHERE t.tournament_id = $1
GROUP BY t.id, u.username
ORDER BY t.created_at;
```

### Обновление материализованного представления

```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY leaderboard_tournament;
```

---

## Оптимизации

- **Connection pooling**: максимум 100 соединений
- **Prepared statements** для частых запросов
- **Партиционирование** таблицы matches (помесячно)
- **Составные индексы** для частых фильтров
- **Optimistic locking** для конкурентных обновлений
- **Материализованные представления** для лидербордов с автообновлением

---

*Версия документации: 2.0*
*Последнее обновление: Январь 2026*
