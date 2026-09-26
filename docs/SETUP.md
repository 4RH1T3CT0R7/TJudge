# Настройка и разработка TJudge

Прод и self-hosted развёртывание — в [OPERATIONS.md](OPERATIONS.md).

## Требования

Docker 24+ с Compose v2 (`docker compose`), Go 1.26+, Node.js 22, Make.

## Быстрый старт (dev-compose)

```bash
git clone https://github.com/4RH1T3CT0R7/TJudge.git && cd TJudge
cp .env.example .env
make docker-up                      # создаёт сеть monitoring и поднимает docker-compose.yml
curl http://localhost:8080/health   # OK
```

Compose собирает api, worker, `tjudge-cli` и `tjudge-builder`, миграции применяет сервис `migrate`. Веб и API — http://localhost:8080 (`/api/v1`), метрики api — :9090/metrics, worker — :9091/metrics. Мониторинг (Prometheus :9092, Grafana :3000 admin/admin, Alertmanager :9093, Pushgateway :9094) поднимается отдельно: `make monitoring-up`. Сеть `monitoring` объявлена внешней: без `make docker-up` её нужно создать руками (`docker network create monitoring`).

## Локальная разработка

```bash
docker network create monitoring            # один раз
docker compose up -d postgres redis
go mod download && (cd web && npm ci && npm run build)
make docker-build-executor docker-build-builder
make migrate-up
make run-api                                # терминал 1
make run-worker                             # терминал 2
cd web && npm run dev                       # терминал 3, http://localhost:5173
```

- Compose отдаёт PostgreSQL на 5433 (`DB_PORT=5433` уже в `.env.example`).
- `npm run build` кладёт фронт в `internal/web/dist`, а `go build` встраивает его через `go:embed`. Без сборки `go build`, `go vet` и `go test` падают с `pattern all:dist: no matching files found`.
- Воркер запускает контейнеры через Docker SDK и монтирует в них файлы программ с хоста. Когда api и worker идут локально, в `.env` `PROGRAMS_PATH` и `HOST_PROGRAMS_PATH` задают одним абсолютным путём, например `/path/to/TJudge/data/programs`.
- На старте worker проверяет образы `EXECUTOR_DOCKER_IMAGE` и `EXECUTOR_BUILDER_IMAGE`. Отсутствующий образ он пытается скачать, а если не вышло, пишет ошибку `Docker image is not available` и работает дальше: матчи или сборки без образа повторяются, пока он не появится.

Фронтенд: React 19, TypeScript 5.9, Vite 7, Tailwind CSS 4, TanStack Query 5, Zustand 5. Скрипты `web/`: `dev`, `build`, `lint`, `test` (vitest), `generate:api`. Типы API в `web/src/api/generated` генерируются из `docs/openapi.yaml`. После правки спеки нужен `npm run generate:api`, иначе CI упадёт на проверке свежести.

Make-таргеты разработки: `dev` (API с hot reload через air), `run-api`, `run-worker`, `build`, `docker-build`, `docker-build-executor`, `docker-build-builder`, `lint` (golangci-lint v2.11.4 с тегами integration, e2e, security), `fmt`, `security` (gosec + govulncheck), `migrate-up`, `migrate-down` (откат одной миграции), `migrate-create` (спрашивает имя интерактивно, нужен CLI golang-migrate), `admin EMAIL=x@y.z`, `create-user EMAIL= USERNAME= PASSWORD= [ADMIN=1]`.

## Тестирование

```bash
make test                     # unit
make test-race                # unit с детектором гонок
TZ=UTC RUN_INTEGRATION=true make test-integration   # -tags=integration: ./internal/storage/... и ./tests/integration/... (-p 1)
make test-e2e                 # -tags=e2e, нужен запущенный API
go test -tags=security ./tests/security/...        # authn/authz и лимиты, нужен запущенный API
E2E_FULL_CYCLE=true go test -tags=e2e -run TestE2E_FullCycle ./tests/e2e/   # компиляция и матч, нужны worker и образы песочниц
```

- Интеграционные, e2e и security тесты по умолчанию подключаются к БД dev-compose: `DB_HOST=localhost`, `DB_PORT=5433`, `DB_NAME=tjudge`, `DB_USER=tjudge`, `DB_PASSWORD=secret`; Redis — `localhost:6379`. API для e2e и security — `E2E_API_URL` (по умолчанию http://localhost:8080). E2E повышают своего пользователя до админа прямо в БД, поэтому `DB_*` должны смотреть в базу этого API.
- Интеграционные тесты чистят за собой таблицы по шаблонам имён. Для прогона лучше отдельная база (`DB_NAME=tjudge_test` плюс `go run ./cmd/migrations up`), в CI так и сделано. Postgres и процесс тестов должны работать в UTC.
- Unit-тесты используют `testify` и ручные моки (`mock.Mock`), Redis в unit-тестах — miniredis.

## Конфигурация

Загрузка — `internal/config/config.go`: `.env` через godotenv, затем переменные окружения. Полный список с комментариями — [`.env.example`](../.env.example). Правила, которые проверяются на старте:

- Разбор строгий: неверное целое, bool (только формы `strconv.ParseBool`) или длительность без единиц (`90`, а не `90s`) останавливает старт.
- `WORKER_TIMEOUT` меньше `EXECUTOR_TIMEOUT` + 20s на старте поднимается до этого значения, оба больше нуля. `WORKER_TIMEOUT` у api и worker должен совпадать: от него считается порог зависшего матча (`WORKER_TIMEOUT` + 30s) в recovery и в `/system/status`.
- `EXECUTOR_MEMORY_LIMIT` не меньше 6 МиБ, `EXECUTOR_DEFAULT_ITERATIONS` и `EXECUTOR_COMPILE_WORKERS` не меньше 1, `TRUSTED_PROXIES` — валидные CIDR.
- В production (`ENVIRONMENT=production`) `JWT_SECRET` не короче 32 байт и не из списка заглушек.
- Секреты `DB_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET` можно передать файлом: `DB_PASSWORD_FILE` и т.д. (Docker secrets).

Неочевидные дефолты: `WORKER_MAX` = число ядер, `WORKER_MIN` = min(2, `WORKER_MAX`), пул БД считается от `WORKER_MAX` (не больше 100), пул Redis не меньше `WORKER_MAX` + 20, `JWT_ACCESS_TTL=1h`, `JWT_REFRESH_TTL=168h`, `RATE_LIMIT_ENABLED=false`, `EXECUTOR_COMPILE_WORKERS=2`. `EXECUTOR_SECCOMP_PROFILE` — путь к JSON-профилю (`deployments/security/seccomp-executor.json`), битый файл роняет старт worker'а. `EXECUTOR_APPARMOR_PROFILE` — имя профиля, загруженного на хосте (`deployments/security/apparmor-executor`). Трейсинг включается `OTEL_EXPORTER_OTLP_ENDPOINT`.

IP клиента для лимитов, аудита и логов определяет `middleware.RealIP`. Если `TRUSTED_PROXIES` пуст, от соседа из loopback или приватной сети берётся только `X-Real-IP`. Если список задан, `X-Forwarded-For` разбирается справа налево по нему. `WEBSOCKET_ALLOWED_ORIGINS` (или `CORS_ALLOWED_ORIGINS`, если первая пуста): в production пусто или `*` пускает только свой хост.

## Схема БД

PostgreSQL 15. Основные таблицы (полная схема — в `migrations/`):

- `users` — аккаунты (username, email, password_hash, role, password_changed_at).
- `games` — игры (name, display_name, rules в Markdown, score_multiplier).
- `tournaments` — турниры (status: pending/active/completed, optimistic lock через `version`).
- `tournament_games` — связь турнир-игра (round_status, `config` JSONB, `auto_round_*`).
- `tournament_participants` — программы в турнире (ELO rating, wins/losses/draws).
- `teams` / `team_members` — команды с инвайт-кодом (`code`) и лидером.
- `programs` — загруженные программы (status: compiling/ready/failed, уникальность `(team_id, game_id, version)`).
- `matches` — матчи; партиционирована по `created_at` помесячно, PK `(id, created_at)`.
- `rating_history` — история ELO; тоже помесячные партиции.
- `match_outbox` — outbox «пересчитать рейтинг», пишется в одной транзакции с результатом матча.
- `audit_log` — админ-аудит (refresh-токены — stateless JWT, таблицы под них нет).

Партиции `matches_YYYY_MM` / `rating_history_YYYY_MM` создаются автоматически; удаление старых — `drop_old_partitions()` при `DB_PARTITION_RETENTION_MONTHS > 0` (по умолчанию 0 — retention выключен).

Лидерборды считаются живыми запросами по `matches`: `UNION ALL` по обеим сторонам матча на индексе `(tournament_id, game_type, status)`. Materialized views удалены в 000038.

Миграции: `migrations/000001_*.sql` … `000045_*.sql`; номер 000035 пропущен намеренно (выпилен вместе с password-reset) и не переиспользуется. `go run ./cmd/migrations up|down|version|force N`: `down` откатывает ровно одну миграцию. Последние:

- 000042 — `users.password_changed_at`: смена пароля отзывает выписанные раньше refresh-токены.
- 000043 — новые инвайт-коды (8 hex-символов) у команд pending-турниров: старые коды и ссылки `/join/:code` перестают работать, капитанам нужно заново раздать коды.
- 000044 — `audit_log.actor_id` nullable: удаление пользователя не ломается о записи аудита.
- 000045 — удаление индексов-дублей; при занятом локе падает через 5 с, восстановление — OPERATIONS §9.7.

```sql
-- ожидающие матчи турнира
SELECT * FROM matches WHERE status = 'pending' AND tournament_id = $1 ORDER BY created_at LIMIT 100;

-- ручное создание партиций текущего/следующего месяца
SELECT create_matches_partition_if_needed();
SELECT create_rating_history_partition_if_needed();
```

## Мониторинг и CI

`make monitoring-up` поднимает `docker-compose.monitoring.yml`: Prometheus (:9092, правила в `deployments/prometheus/alerts/tjudge-slo.yml` и `tjudge-doctor.yml`, тест правил — `promtool test rules deployments/prometheus/tjudge-slo.test.yml`), Alertmanager (:9093, по умолчанию получатель `null`, Telegram включается в `deployments/alertmanager/alertmanager.yml`), Pushgateway (:9094) и Grafana (:3000, дашборды «TJudge — Обзор системы» и «TJudge — Doctor»).

```promql
sum(tjudge_queue_size)                                                  # матчей в очередях
sum(tjudge_active_workers)                                              # занятых воркеров
histogram_quantile(0.99, sum by (le) (rate(tjudge_http_request_duration_seconds_bucket[5m])))  # http p99
sum(rate(tjudge_matches_total[5m]))                                     # завершённых матчей
```

CI (`.github/workflows/`): `ci.yml` на push в main и PR — фронтенд (npm ci, свежесть openapi-кодгена, lint, test, build), npm audit, Go (vet и golangci-lint с тегами integration, e2e, security; govulncheck; миграции up→down→up; `go test -race`; интеграционные с `-p 1`; e2e и security против `bin/api`). `nightly.yml` — ежедневно в 03:00 UTC и вручную, полный цикл на dev-compose (`E2E_FULL_CYCLE=true`). `release.yml` — образы и деплой по тегу `v*`. Зависимости обновляет Dependabot (`.github/dependabot.yml`).

Перед пушем: `make lint`, `make test-race`, `make build`, в `web/` — `npm run lint && npm test && npm run build`.

## Типовые проблемы

| Проблема | Решение |
|----------|---------|
| `pattern all:dist: no matching files found` | фронтенд не собран: `cd web && npm ci && npm run build` |
| `network monitoring declared as external, but could not be found` | `docker network create monitoring` или `make docker-up` |
| `role tjudge does not exist` | локальный PG занял 5432 — для Docker используйте 5433 |
| `invalid environment: ...` на старте | опечатка в `.env`: число, bool или длительность без единиц |
| `air: command not found` | `go install github.com/air-verse/air@latest` + `~/go/bin` в PATH |
| `Internal server error` | миграции не применены: `make migrate-up` |
| Программы висят в `compiling` / матчи не идут | логи worker'а: `docker compose logs worker`; нет образа `tjudge-builder` или `tjudge-cli` |

```bash
docker compose ps                                           # статус контейнеров
docker compose logs -f api worker                           # логи
docker exec -it tjudge-postgres psql -U tjudge -d tjudge    # консоль бд
docker exec tjudge-redis redis-cli LLEN queue:high          # очередь (ещё queue:medium, queue:low, queue:compile, queue:dead_letter)
docker compose down -v && make docker-up                    # полный сброс вместе с данными
```

## Безопасность

- Секреты не коммитить: `secrets/` и `.env` в `.gitignore`.
- В production — Docker Secrets (`*_FILE`), `JWT_SECRET` от 32 байт, `RATE_LIMIT_ENABLED=true`.
- Сканирование: `make security` (gosec + govulncheck), `npm audit` в `web/`.
