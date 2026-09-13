# Схема базы данных

PostgreSQL 15. Таблицы `matches` и `rating_history` партиционированы помесячно.

## ER (упрощённо)

```
users ──leader_id──< teams >──tournament_id──> tournaments
                       │                            │
                  team_members              tournament_games >── games
                                                    │
programs >──user_id/team_id/game_id      tournament_participants
   │
   ├──< matches (program1_id, program2_id) ──> tournaments
   └──< rating_history (program_id) ──> tournaments
```

## Таблицы

### users

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | PK |
| username | VARCHAR(50) | UNIQUE, NOT NULL |
| email | VARCHAR(255) | UNIQUE, NOT NULL |
| password_hash | VARCHAR(255) | NOT NULL (bcrypt) |
| role | VARCHAR(20) | DEFAULT 'user' — user, admin |
| created_at, updated_at | TIMESTAMPTZ | NOT NULL |

Индексы: `idx_users_username`, `idx_users_email`.

### games

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | PK |
| name | VARCHAR(50) | UNIQUE, NOT NULL — snake_case, `^[a-z0-9_]+$` |
| display_name | VARCHAR(255) | NOT NULL |
| rules | TEXT | Markdown |
| score_multiplier | DECIMAL(10,2) | DEFAULT 1.0 — множитель очков для лидерборда |
| created_at, updated_at | TIMESTAMP | NOT NULL |

Индексы: `idx_games_name`.

Пять игр:

| name | display_name | score_multiplier |
|------|-------------|------------------|
| `prisoners_dilemma` | Дилемма заключённого | 1.0 |
| `tug_of_war` | Перетягивание каната | 10.0 |
| `travelers_dilemma` | Дилемма путешественника | 0.05 |
| `public_goods` | Общественное благо | 0.1 |
| `dollar_auction` | Аукцион двойной цены | 1.0 |

### tournaments

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | PK |
| name | VARCHAR(200) | NOT NULL |
| description | TEXT | Markdown |
| status | VARCHAR(20) | NOT NULL — pending, active, completed |
| max_team_size | INT | DEFAULT 1 |
| max_participants | INT | макс. команд |
| is_perpetual | BOOLEAN | DEFAULT false |
| created_at | TIMESTAMPTZ | NOT NULL |
| started_at, completed_at | TIMESTAMPTZ | NULL |
| version | INT | DEFAULT 1 — optimistic lock |

Индексы: `idx_tournaments_status`.

### tournament_games

| Поле | Тип | Ограничения |
|------|-----|-------------|
| tournament_id | UUID | FK tournaments, PK |
| game_id | UUID | FK games, PK |
| is_active | BOOLEAN | DEFAULT true |
| round_status | VARCHAR(20) | DEFAULT 'pending' — pending, running, completed |
| round_number | INT | DEFAULT 0 |
| created_at | TIMESTAMPTZ | NOT NULL |

PK `(tournament_id, game_id)`. Колонки `auto_round_*` (000030) и `config` JSONB (000032) — параметры автораундов и игры на уровне турнира.

### teams

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | PK |
| tournament_id | UUID | FK tournaments |
| name | VARCHAR(100) | NOT NULL |
| invite_code | VARCHAR(10) | UNIQUE, NOT NULL |
| leader_id | UUID | FK users |
| is_disqualified, disqualified_at | — | дисквалификация (000031) |
| created_at | TIMESTAMPTZ | NOT NULL |

Индексы: `idx_teams_tournament`, `idx_teams_invite_code`. Уникальность `(tournament_id, name)`.

### team_members

PK `(team_id, user_id)`: `team_id` (FK teams), `user_id` (FK users), `joined_at` TIMESTAMPTZ NOT NULL.

### programs

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | PK |
| user_id | UUID | FK users, NOT NULL |
| name | VARCHAR(100) | NOT NULL |
| game_type | VARCHAR(50) | NOT NULL — совпадает с `games.name` |
| code_path | TEXT | NOT NULL — путь к исходнику |
| language | VARCHAR(50) | NOT NULL — python, go, cpp, java, js, rust |
| team_id | UUID | FK teams, NULL |
| tournament_id | UUID | FK tournaments, NULL |
| game_id | UUID | FK games, NULL |
| file_path | VARCHAR(500) | NULL — скомпилированный файл |
| status | TEXT | DEFAULT 'ready' — compiling, ready, failed (000040) |
| error_message | TEXT | NULL |
| version | INT | NOT NULL |
| created_at, updated_at | TIMESTAMP | NOT NULL |

Индексы: `idx_programs_user_id`, `idx_programs_game_type`, `idx_programs_user_game`, `idx_programs_status` (частичный, `status != 'ready'`). Уникальность `(team_id, game_id, version)`.

### matches

Партиционирована по `created_at` (помесячно, `PARTITION BY RANGE`). PK `(id, created_at)`.

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | часть PK |
| tournament_id | UUID | FK tournaments, NOT NULL |
| program1_id, program2_id | UUID | FK programs, NOT NULL |
| game_type | VARCHAR(50) | NOT NULL |
| status | VARCHAR(20) | DEFAULT 'pending' — pending, running, completed, failed, cancelled |
| priority | VARCHAR(10) | DEFAULT 'medium' — high, medium, low |
| score1, score2 | INT | NULL |
| winner | INT | NULL, CHECK (0,1,2) — 0 ничья, 1 program1, 2 program2 |
| error_message | TEXT | NULL |
| started_at, completed_at | TIMESTAMP | NULL |
| created_at | TIMESTAMP | NOT NULL — ключ партиции |

Индексы: `idx_matches_tournament`, `idx_matches_status`, `idx_matches_priority_created`, `idx_matches_program1`, `idx_matches_program2`, `idx_matches_game_type`, а также `(tournament_id, game_type, status)` из 000037 (основной индекс лидербордов).

### rating_history

Партиционирована по `created_at` (помесячно). PK `(id, created_at)`.

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | часть PK |
| program_id | UUID | FK programs, NOT NULL |
| tournament_id | UUID | FK tournaments, NOT NULL |
| old_rating, new_rating, change | INT | NOT NULL |
| match_id | UUID | NULL |
| created_at | TIMESTAMP | NOT NULL — ключ партиции |

Индексы: `idx_rating_history_program`, `idx_rating_history_tournament`, `idx_rating_history_match`, композит `(program_id, tournament_id, created_at DESC)` из 000033.

### tournament_participants

| Поле | Тип | Ограничения |
|------|-----|-------------|
| id | UUID | PK |
| tournament_id | UUID | FK tournaments, NOT NULL |
| program_id | UUID | FK programs, NOT NULL |
| rating | INT | DEFAULT 1500 — ELO |
| wins, losses, draws | INT | DEFAULT 0 |
| created_at | TIMESTAMP | NOT NULL |

Уникальность `(tournament_id, program_id)`. Индексы: `..._tournament`, `..._program`, `..._rating`.

### match_outbox (000039)

Транзакционный outbox: задача «пересчитать рейтинг» пишется в одной транзакции с результатом матча (`UpdateResultWithOutbox`). Worker обрабатывает сразу (fast path), `OutboxDispatcher` подбирает зависшие `pending`.

Ключевые поля: `id` BIGSERIAL PK, `match_id` UUID, `kind` (`rating_update`), `status` (pending/done/error), `attempts`, `last_error`, `claimed_at` (lease), `created_at`, `processed_at`. Частичный индекс `idx_match_outbox_pending` по `created_at WHERE status = 'pending'`.

### refresh_tokens / audit_log

- `refresh_tokens`: `id` PK, `user_id` FK users, `token_hash`, `expires_at`, `created_at`.
- `audit_log` (000034): админские действия (кто, что, ресурс, когда, IP/UA), retention 1 год.

## Лидерборды

Считаются живыми запросами по `matches` (`internal/infrastructure/db/tournament_leaderboard.go`): матч разворачивается в две «стороны» через `UNION ALL` (строка на каждую программу), каждая ветка использует индекс `(tournament_id, game_type, status)` из 000037 вместо OR-join. Тайбрейк при равном счёте и победах — `MIN(created_at)` последних версий программ команды.

Materialized views `leaderboard_global`/`leaderboard_tournament` (000010/000017/000027) удалены в 000038: чтение всегда шло живым запросом, а refresh каждые 30 с впустую нагружал БД.

## Миграции

```bash
make migrate-up                       # применить все
make migrate-down                     # откатить последнюю
make migrate-create name=add_table    # создать новую
make migrate-status                   # статус
```

Файлы: `migrations/000001_*.sql` … `migrations/000041_*.sql`. Номер 000035 пропущен намеренно (удалён вместе с password-reset) и не переиспользуется.

Ключевые миграции 000023–000041:

| # | Название | Суть |
|---|----------|------|
| 000023 | `add_unique_program_version` | уникальность `(team_id, game_id, version)` — защита от гонок при загрузке |
| 000024 | `add_auto_partition_function` | `create_matches_partition_if_needed()` |
| 000025 | `add_rating_history_auto_partition` | `create_rating_history_partition_if_needed()` |
| 000026 | `add_tiebreak_index` | `idx_programs_team_tournament_game_version_desc` |
| 000027 | `update_leaderboard_views_tiebreak` | пересоздание matview с тайбрейком |
| 000028 | `seed_new_games` | +3 игры: `travelers_dilemma`, `public_goods`, `dollar_auction` |
| 000029 | `update_game_rules` | правила и протоколы всех 5 игр |
| 000030 | `add_auto_round` | `auto_round_*` в `tournament_games` |
| 000031 | `add_team_disqualification` | `is_disqualified`/`disqualified_at`; статус `cancelled` у матчей |
| 000032 | `add_game_config` | `config` JSONB в `tournament_games` |
| 000033 | `rating_history_composite_index` | `(program_id, tournament_id, created_at DESC)` |
| 000034 | `audit_log` | таблица админ-аудита |
| 000036 | `fk_cascade_audit` | явные `ON DELETE` для FK (matches, teams, audit_log) |
| 000037 | `matches_composite_index` | `(tournament_id, game_type, status)` — индекс лидербордов |
| 000038 | `drop_leaderboard_matviews` | удаление matview лидербордов |
| 000039 | `add_match_outbox` | таблица `match_outbox` |
| 000040 | `add_program_status` | колонка `programs.status` (compiling/ready/failed) |
| 000041 | `add_partition_retention` | `drop_old_partitions(parent, retention_months)`; retention по умолчанию выключен, включается `PARTITION_RETENTION_MONTHS > 0` |

## Автопартиционирование

`matches` и `rating_history` партиционированы по `created_at` помесячно. Партиции создаются автоматически (миграции 000024/000025) для текущего и следующего месяца; при отсутствии — динамически через `EXECUTE format(...)`. Именование: `matches_YYYY_MM`, `rating_history_YYYY_MM`.

```sql
SELECT create_matches_partition_if_needed();
SELECT create_rating_history_partition_if_needed();
```

Удаление старых партиций — `drop_old_partitions(parent, retention_months)` (000041), вызывается горутиной обслуживания (`internal/infrastructure/db/db.go`) при `PARTITION_RETENTION_MONTHS > 0`.

## Частые запросы

Ожидающие матчи турнира:

```sql
SELECT * FROM matches
WHERE status = 'pending' AND tournament_id = $1
ORDER BY created_at LIMIT 100;
```

Программы команды:

```sql
SELECT p.*, g.name AS game_name
FROM programs p
JOIN games g ON p.game_id = g.id
WHERE p.team_id = $1
ORDER BY g.name;
```

Команды в турнире с числом участников:

```sql
SELECT t.*, u.username AS leader_name, COUNT(tm.user_id) AS member_count
FROM teams t
JOIN users u ON t.leader_id = u.id
LEFT JOIN team_members tm ON t.id = tm.team_id
WHERE t.tournament_id = $1
GROUP BY t.id, u.username
ORDER BY t.created_at;
```

## Оптимизации

- Connection pooling: максимум 100 соединений.
- Prepared statements для частых запросов.
- Автопартиционирование `matches` и `rating_history` с автосозданием партиций.
- Составные индексы для частых фильтров; тайбрейк-индекс для лидерборда.
- Уникальность версий программ против гонок при загрузке.
- Живые лидерборды через `UNION ALL` на индексе `(tournament_id, game_type, status)`.
