# Схема базы данных

## ER-диаграмма

```
┌─────────────┐       ┌──────────────┐       ┌─────────────────┐
│    users    │       │    teams     │       │   tournaments   │
├─────────────┤       ├──────────────┤       ├─────────────────┤
│ id (PK)     │◄──────│ leader_id    │       │ id (PK)         │
│ username    │       │ id (PK)      │◄──┐   │ name            │
│ email       │       │ tournament_id│───┼──▶│ game_type       │
│ password    │       │ name         │   │   │ status          │
│ role        │       │ invite_code  │   │   │ max_participants│
│ created_at  │       │ created_at   │   │   │ created_at      │
│ updated_at  │       └──────────────┘   │   │ updated_at      │
└─────────────┘                          │   └─────────────────┘
                                         │
┌────────────────┐                       │
│     games     │                       │
├────────────────┤    ┌──────────────────┴────────────────┐
│ id (PK)        │◄───│       tournament_games            │
│ name           │    ├───────────────────────────────────┤
│ display_name   │    │ tournament_id (FK, PK)            │
│ rules          │    │ game_id (FK, PK)                  │
│ score_mult     │    │ is_active                         │
│ created_at     │    │ round_completed                   │
│ updated_at     │    │ current_round                     │
└────────────────┘    └───────────────────────────────────┘

┌──────────────────┐       ┌───────────────────┐
│    programs      │       │      matches      │
├──────────────────┤       ├───────────────────┤
│ id (PK)          │◄──────│ program1_id (FK)  │
│ user_id (FK)     │       │ program2_id (FK)  │
│ team_id (FK)     │       │ id (PK)           │
│ tournament_id(FK)│       │ tournament_id (FK)│
│ game_id (FK)     │       │ game_type         │
│ name             │       │ status            │
│ game_type        │       │ priority          │
│ language         │       │ score1, score2    │
│ version          │       │ winner            │
│ created_at       │       │ error_message     │
│ updated_at       │       │ created_at        │
└──────────────────┘       │ completed_at      │
                           └───────────────────┘

┌───────────────────┐       ┌───────────────────────────┐
│   rating_history  │       │  tournament_participants  │
├───────────────────┤       ├───────────────────────────┤
│ id (PK)           │       │ id (PK)                   │
│ program_id (FK)   │       │ tournament_id (FK)        │
│ tournament_id (FK)│       │ program_id (FK)           │
│ old_rating        │       │ rating                    │
│ new_rating        │       │ wins, losses, draws       │
│ change            │       │ created_at                │
│ match_id (FK)     │       └───────────────────────────┘
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
| name | VARCHAR(50) | UNIQUE, NOT NULL | Идентификатор (snake_case, `^[a-z0-9_]+$`) |
| display_name | VARCHAR(255) | NOT NULL | Отображаемое название |
| rules | TEXT | | Правила игры (Markdown) |
| score_multiplier | DECIMAL(10,2) | DEFAULT 1.0 | Множитель очков для балансировки лидерборда |
| created_at | TIMESTAMP | NOT NULL | Время создания |
| updated_at | TIMESTAMP | NOT NULL | Время обновления |

Индексы: `idx_games_name`

**Доступные игры (5 шт.):**

| name | display_name | score_multiplier | Описание |
|------|-------------|------------------|----------|
| `prisoners_dilemma` | Дилемма заключённого | 1.0 | Классическая игра: COOPERATE или DEFECT |
| `tug_of_war` | Перетягивание каната | 10.0 | Управление энергией, одновременные ставки |
| `travelers_dilemma` | Дилемма путешественника | 0.05 | Заявки в диапазоне [L, U] с бонусом/штрафом |
| `public_goods` | Общественное благо | 0.1 | Вклад в общий пул с множителем |
| `dollar_auction` | Аукцион двойной цены | 1.0 | Поочерёдные ставки, оба платят |

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
| tournament_id | UUID | FK к tournaments, PK | Турнир |
| game_id | UUID | FK к games, PK | Игра |
| is_active | BOOLEAN | DEFAULT true | Активна ли игра |
| round_status | VARCHAR(20) | DEFAULT 'pending' | pending, running, completed |
| round_number | INT | DEFAULT 0 | Номер текущего раунда |
| created_at | TIMESTAMPTZ | NOT NULL | Время добавления |

Первичный ключ: `(tournament_id, game_id)`

### teams

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| tournament_id | UUID | FK к tournaments | Турнир |
| name | VARCHAR(100) | NOT NULL | Название команды |
| invite_code | VARCHAR(10) | UNIQUE, NOT NULL | Код приглашения |
| leader_id | UUID | FK к users | Лидер команды |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |

Индексы: `idx_teams_tournament`, `idx_teams_invite_code`
Уникальность: `(tournament_id, name)`

### team_members

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| team_id | UUID | FK к teams, PK | Команда |
| user_id | UUID | FK к users, PK | Пользователь |
| joined_at | TIMESTAMPTZ | NOT NULL | Время присоединения |

Первичный ключ: `(team_id, user_id)`

### programs

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| user_id | UUID | FK к users, NOT NULL | Автор программы |
| name | VARCHAR(100) | NOT NULL | Название программы |
| game_type | VARCHAR(50) | NOT NULL | Тип игры (совпадает с `games.name`) |
| code_path | TEXT | NOT NULL | Путь к исходному коду |
| language | VARCHAR(50) | NOT NULL | python, go, cpp, java, js, rust |
| team_id | UUID | FK к teams, NULL | Команда-владелец |
| tournament_id | UUID | FK к tournaments, NULL | Турнир |
| game_id | UUID | FK к games, NULL | Ссылка на игру |
| file_path | VARCHAR(500) | NULL | Путь к скомпилированному файлу |
| error_message | TEXT | NULL | Сообщение об ошибке |
| version | INT | NOT NULL | Версия программы |
| created_at | TIMESTAMP | NOT NULL | Время создания |
| updated_at | TIMESTAMP | NOT NULL | Время обновления |

Индексы: `idx_programs_user_id`, `idx_programs_game_type`, `idx_programs_user_game`
Уникальность: `(team_id, game_id, version)` - уникальная версия программы на игру от команды

### matches

Партиционирована по `created_at` (помесячно). Партиции создаются автоматически функцией `create_matches_partition_if_needed()`.

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK (совместный с created_at) | Уникальный идентификатор |
| tournament_id | UUID | FK к tournaments, NOT NULL | Турнир |
| program1_id | UUID | FK к programs, NOT NULL | Первый игрок |
| program2_id | UUID | FK к programs, NOT NULL | Второй игрок |
| game_type | VARCHAR(50) | NOT NULL | Тип игры (совпадает с `games.name`) |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'pending' | pending, running, completed, failed |
| priority | VARCHAR(10) | NOT NULL, DEFAULT 'medium' | high, medium, low |
| score1 | INT | NULL | Очки первого игрока |
| score2 | INT | NULL | Очки второго игрока |
| winner | INT | NULL, CHECK (0, 1, 2) | 0 = ничья, 1 = program1, 2 = program2 |
| error_message | TEXT | NULL | Сообщение об ошибке |
| started_at | TIMESTAMP | NULL | Время старта |
| completed_at | TIMESTAMP | NULL | Время завершения |
| created_at | TIMESTAMP | NOT NULL | Время создания (ключ партиционирования) |

Первичный ключ: `(id, created_at)`

Индексы: `idx_matches_tournament`, `idx_matches_status`, `idx_matches_priority_created`, `idx_matches_program1`, `idx_matches_program2`, `idx_matches_game_type`

### rating_history

Партиционирована по `created_at` (помесячно). Партиции создаются автоматически функцией `create_rating_history_partition_if_needed()`.

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK (совместный с created_at) | Уникальный идентификатор |
| program_id | UUID | FK к programs, NOT NULL | Программа |
| tournament_id | UUID | FK к tournaments, NOT NULL | Турнир |
| old_rating | INT | NOT NULL | Рейтинг до матча |
| new_rating | INT | NOT NULL | Рейтинг после матча |
| change | INT | NOT NULL | Изменение рейтинга (дельта) |
| match_id | UUID | NULL | Матч, вызвавший изменение |
| created_at | TIMESTAMP | NOT NULL | Время записи (ключ партиционирования) |

Первичный ключ: `(id, created_at)`

Индексы: `idx_rating_history_program`, `idx_rating_history_tournament`, `idx_rating_history_match`

### tournament_participants

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | Уникальный идентификатор |
| tournament_id | UUID | FK к tournaments, NOT NULL | Турнир |
| program_id | UUID | FK к programs, NOT NULL | Программа участника |
| rating | INT | NOT NULL, DEFAULT 1500 | Текущий рейтинг ELO |
| wins | INT | NOT NULL, DEFAULT 0 | Победы |
| losses | INT | NOT NULL, DEFAULT 0 | Поражения |
| draws | INT | NOT NULL, DEFAULT 0 | Ничьи |
| created_at | TIMESTAMP | NOT NULL | Время регистрации |

Уникальность: `(tournament_id, program_id)`
Индексы: `idx_tournament_participants_tournament`, `idx_tournament_participants_program`, `idx_tournament_participants_rating`

### refresh_tokens

| Поле | Тип | Ограничения | Описание |
|------|-----|-------------|----------|
| id | UUID | PK | ID токена |
| user_id | UUID | FK к users | Владелец |
| token_hash | VARCHAR(255) | NOT NULL | Хеш токена |
| expires_at | TIMESTAMPTZ | NOT NULL | Срок действия |
| created_at | TIMESTAMPTZ | NOT NULL | Время создания |

---

## Материализованные представления

### leaderboard_tournament

Обновлено в миграции 000027 с поддержкой тайбрейка по времени загрузки программы.

```sql
CREATE MATERIALIZED VIEW leaderboard_tournament AS
SELECT
    tp.tournament_id,
    tp.program_id,
    p.name AS program_name,
    p.user_id,
    u.username,
    COALESCE(stats.total_score, 0) AS rating,
    COALESCE(stats.total_matches, 0) AS total_matches,
    COALESCE(stats.wins, 0) AS wins,
    COALESCE(stats.losses, 0) AS losses,
    COALESCE(stats.draws, 0) AS draws,
    tp.created_at AS joined_at,
    COALESCE(stats.last_match, tp.created_at) AS last_updated
FROM tournament_participants tp
INNER JOIN programs p ON tp.program_id = p.id
INNER JOIN users u ON p.user_id = u.id
LEFT JOIN LATERAL (
    -- Агрегация статистики матчей для участника
    SELECT COUNT(*) AS total_matches,
           SUM(CASE WHEN ... THEN 1 ELSE 0 END) AS wins,
           SUM(CASE WHEN ... THEN 1 ELSE 0 END) AS losses,
           SUM(CASE WHEN m.winner = 0 THEN 1 ELSE 0 END) AS draws,
           SUM(...) AS total_score,
           MAX(m.completed_at) AS last_match
    FROM matches m
    WHERE (m.program1_id = p.id OR m.program2_id = p.id)
      AND m.tournament_id = tp.tournament_id
      AND m.status = 'completed'
) stats ON true
ORDER BY tp.tournament_id, rating DESC, wins DESC,
    -- Тайбрейк: MIN(created_at) последних версий программ
    COALESCE(
        (SELECT MIN(sub_p.created_at) FROM (...) sub_p),
        p.created_at
    ) ASC;
```

Индексы: `idx_leaderboard_tournament_pk (tournament_id, program_id)`, `idx_leaderboard_tournament_id (tournament_id, rating DESC)`

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

Файлы миграций: `migrations/000001_*.sql` до `migrations/000036_*.sql`. Номер 000035 пропущен намеренно и не должен переиспользоваться в новых миграциях.

**Структура миграций:**
```
migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_programs.up.sql
├── 000002_create_programs.down.sql
...
├── 000028_seed_new_games.up.sql
├── 000028_seed_new_games.down.sql
├── 000029_update_game_rules.up.sql
├── 000029_update_game_rules.down.sql
├── 000030_add_auto_round.up.sql
├── 000030_add_auto_round.down.sql
├── 000031_add_team_disqualification.up.sql
├── 000031_add_team_disqualification.down.sql
├── 000032_add_game_config.up.sql
├── 000032_add_game_config.down.sql
├── 000033_rating_history_composite_index.up.sql
├── 000033_rating_history_composite_index.down.sql
├── 000034_audit_log.up.sql
├── 000034_audit_log.down.sql
├── 000036_fk_cascade_audit.up.sql
└── 000036_fk_cascade_audit.down.sql
```

**Миграции 023-036 (подробности):**

| Миграция | Название | Описание |
|----------|----------|----------|
| 000023 | `add_unique_program_version` | Уникальное ограничение на версию программы (`team_id` + `game_id` + `version`). Предотвращает гонки при конкурентной загрузке. |
| 000024 | `add_auto_partition_function` | Функция `create_matches_partition_if_needed()` для автоматического создания помесячных партиций таблицы `matches`. |
| 000025 | `add_rating_history_auto_partition` | Функция `create_rating_history_partition_if_needed()` для автоматического создания помесячных партиций таблицы `rating_history`. |
| 000026 | `add_tiebreak_index` | Составной индекс `idx_programs_team_tournament_game_version_desc` для эффективного вычисления тайбрейка в лидерборде. |
| 000027 | `update_leaderboard_views_tiebreak` | Пересоздание materialized view `leaderboard_tournament` с сортировкой по тайбрейку (при равном рейтинге и числе побед приоритет у ранее загрузивших программу). |
| 000028 | `seed_new_games` | Добавление 3 новых игр: `travelers_dilemma`, `public_goods`, `dollar_auction`. |
| 000029 | `update_game_rules` | Обновление правил и протоколов взаимодействия для всех 5 игр (исправления форматов, очков, протоколов). |
| 000030 | `add_auto_round` | Колонки в `tournament_games` для режима автоматических раундов (`auto_round_enabled`, `auto_round_interval_seconds`, `auto_round_last_run_at`). |
| 000031 | `add_team_disqualification` | Поля `is_disqualified`, `disqualified_at` в `teams`; разрешён статус `cancelled` для матчей, отменённых дисквалификацией. |
| 000032 | `add_game_config` | Колонка `config` (JSONB) в `tournament_games` для хранения параметров игры на уровне турнира (итерации, множитель очков, кастомные параметры). |
| 000033 | `rating_history_composite_index` | Композитный индекс `(program_id, tournament_id, created_at DESC)` для `rating_history`. Ускоряет выборку последнего рейтинга программы в рамках турнира. |
| 000034 | `audit_log` | Таблица `audit_log` для записи админских действий: кто, что, над каким ресурсом, когда, с какого IP/UA. Retention 1 год. |
| 000036 | `fk_cascade_audit` | Явные политики `ON DELETE` для внешних ключей (были неявные `NO ACTION`/`RESTRICT`); нормализует поведение каскадов для `matches`, `teams`, `audit_log`. |

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

## Автопартиционирование

Таблицы `matches` и `rating_history` партиционированы по `created_at` (помесячно, `PARTITION BY RANGE`). Для автоматического создания партиций используются две функции, добавленные в миграциях 000024 и 000025.

### create_matches_partition_if_needed()

Создаёт партиции таблицы `matches` для текущего и следующего месяца. Именование партиций: `matches_YYYY_MM`.

```sql
-- Вызов вручную (при необходимости)
SELECT create_matches_partition_if_needed();
```

### create_rating_history_partition_if_needed()

Аналогичная функция для таблицы `rating_history`. Именование партиций: `rating_history_YYYY_MM`.

```sql
-- Вызов вручную (при необходимости)
SELECT create_rating_history_partition_if_needed();
```

**Принцип работы:** каждая функция проверяет существование партиции для текущего месяца и следующего (опережение на 1 месяц). Если партиция отсутствует, она создаётся динамически через `EXECUTE format(...)`. Это предотвращает ошибки `INSERT` при истечении заранее созданных партиций.

---

## Оптимизации

- **Connection pooling**: максимум 100 соединений
- **Prepared statements** для частых запросов
- **Автопартиционирование** таблиц `matches` и `rating_history` (помесячно, с автосозданием партиций)
- **Составные индексы** для частых фильтров
- **Тайбрейк-индекс** для эффективного разрешения одинаковых рейтингов в лидерборде
- **Уникальное ограничение версий** программ для предотвращения гонок при загрузке
- **Материализованные представления** для лидербордов с поддержкой `REFRESH CONCURRENTLY`

---

*Версия документации: 4.0*
*Последнее обновление: Март 2026*
