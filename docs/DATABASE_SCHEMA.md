# Схема базы данных

## ER-диаграмма

```
┌─────────────┐       ┌──────────────┐       ┌─────────────────┐
│    users    │       │   programs   │       │   tournaments   │
├─────────────┤       ├──────────────┤       ├─────────────────┤
│ id (PK)     │◄──────│ user_id (FK) │       │ id (PK)         │
│ username    │       │ id (PK)      │◄──┐   │ creator_id (FK) │
│ email       │       │ name         │   │   │ name            │
│ password    │       │ language     │   │   │ game_type       │
│ role        │       │ source_code  │   │   │ status          │
│ created_at  │       │ game_type    │   │   │ config          │
│ updated_at  │       │ is_active    │   │   │ created_at      │
└─────────────┘       │ created_at   │   │   │ started_at      │
                      │ updated_at   │   │   │ completed_at    │
                      └──────────────┘   │   └─────────────────┘
                                         │            │
                      ┌──────────────────┴────────────┘
                      │
                      ▼
              ┌───────────────────┐
              │   participants    │
              ├───────────────────┤
              │ id (PK)           │
              │ tournament_id (FK)│
              │ program_id (FK)   │
              │ rating            │
              │ wins              │
              │ losses            │
              │ draws             │
              │ joined_at         │
              └───────────────────┘
                      │
                      ▼
              ┌───────────────────┐
              │      matches      │
              ├───────────────────┤
              │ id (PK)           │
              │ tournament_id (FK)│
              │ program1_id (FK)  │
              │ program2_id (FK)  │
              │ winner_id (FK)    │
              │ status            │
              │ result            │
              │ iterations        │
              │ created_at        │
              │ started_at        │
              │ completed_at      │
              │ version           │
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

### programs

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| user_id | UUID | FK → users | Владелец |
| name | VARCHAR(100) | NOT NULL | Название |
| language | VARCHAR(20) | NOT NULL | python, go, cpp, java |
| source_code | TEXT | NOT NULL | Исходный код |
| game_type | VARCHAR(50) | NOT NULL | Тип игры (tictactoe и др.) |
| is_active | BOOLEAN | DEFAULT true | Статус активности |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| updated_at | TIMESTAMPTZ | NOT NULL | Время обновления |

Индексы: `idx_programs_user_id`, `idx_programs_game_type`

### tournaments

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| creator_id | UUID | FK → users | Создатель турнира |
| name | VARCHAR(200) | NOT NULL | Название турнира |
| game_type | VARCHAR(50) | NOT NULL | Тип игры |
| status | VARCHAR(20) | NOT NULL | pending, active, completed |
| max_participants | INT | DEFAULT 16 | Макс. участников |
| iterations_per_match | INT | DEFAULT 1000 | Игр в матче |
| config | JSONB | | Дополнительные настройки |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| started_at | TIMESTAMPTZ | | Время старта |
| completed_at | TIMESTAMPTZ | | Время завершения |
| version | INT | DEFAULT 1 | Optimistic lock |

Индексы: `idx_tournaments_status`, `idx_tournaments_game_type`

### participants

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| tournament_id | UUID | FK → tournaments | Турнир |
| program_id | UUID | FK → programs | Программа-участник |
| rating | INT | DEFAULT 1500 | Рейтинг ELO |
| wins | INT | DEFAULT 0 | Победы |
| losses | INT | DEFAULT 0 | Поражения |
| draws | INT | DEFAULT 0 | Ничьи |
| joined_at | TIMESTAMPTZ | NOT NULL | Время присоединения |

Индексы: `idx_participants_tournament`, `idx_participants_rating`
Уникальность: `(tournament_id, program_id)`

### matches

Партиционирована по `created_at` (помесячно).

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| tournament_id | UUID | FK → tournaments | Турнир |
| program1_id | UUID | FK → programs | Первый игрок |
| program2_id | UUID | FK → programs | Второй игрок |
| winner_id | UUID | FK → programs, NULL | Победитель (null = ничья) |
| status | VARCHAR(20) | NOT NULL | pending, running, completed, failed |
| result | JSONB | | Детали матча |
| iterations | INT | | Сыграно итераций |
| error_message | TEXT | | Ошибка при неудаче |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |
| started_at | TIMESTAMPTZ | | Время старта |
| completed_at | TIMESTAMPTZ | | Время завершения |
| version | INT | DEFAULT 1 | Optimistic lock |

Индексы: `idx_matches_tournament`, `idx_matches_status`, `idx_matches_programs`

### refresh_tokens

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | ID токена |
| user_id | UUID | FK → users | Владелец |
| token_hash | VARCHAR(255) | NOT NULL | Хеш токена |
| expires_at | TIMESTAMPTZ | NOT NULL | Срок действия |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |

## Миграции

```bash
# Применить все миграции
make migrate-up

# Откатить последнюю миграцию
make migrate-down

# Создать новую миграцию
make migrate-create name=add_new_table
```

Файлы миграций: `cmd/migrations/*.sql`

## Запросы

### Частые запросы

```sql
-- Таблица лидеров
SELECT p.program_id, pr.name, p.rating, p.wins, p.losses
FROM participants p
JOIN programs pr ON p.program_id = pr.id
WHERE p.tournament_id = $1
ORDER BY p.rating DESC, p.wins DESC;

-- Ожидающие матчи
SELECT * FROM matches
WHERE status = 'pending'
ORDER BY created_at
LIMIT 100;

-- Программы пользователя
SELECT * FROM programs
WHERE user_id = $1 AND is_active = true
ORDER BY updated_at DESC;
```

### Оптимизации

- Connection pooling: максимум 100 соединений
- Prepared statements для частых запросов
- Партиционирование таблицы matches (помесячно)
- Составные индексы для частых фильтров
- Optimistic locking для конкурентных обновлений
