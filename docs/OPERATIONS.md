# TJudge - эксплуатация

Runbook для single-node: деплой, диагностика, восстановление. Режимов два, команды для них различаются:

| | prod | self-hosted |
|---|---|---|
| Как выкатывается | тег `v*` → `.github/workflows/release.yml` (ssh на сервер, `~/TJudge`, образы из ghcr.io) | `make deploy*` → `scripts/quick-deploy.sh`, сборка на месте, §13 |
| Compose | `docker-compose.prod.yml` | `docker-compose.selfhosted.yml` + `--env-file config/profiles/<profile>.env` |
| Контейнеры | `tjudge-postgres-prod`, `tjudge-redis-prod`, `tjudge-nginx-prod`; api и worker по 2 реплики (`tjudge-api-1`, `tjudge-worker-1`, …) | `tjudge-postgres`, `tjudge-redis`, `tjudge-api`, `tjudge-worker`, `tjudge-nginx` |
| HTTP | nginx на `:8080`, TLS и HSTS — на внешнем прокси | nginx на 80/443, сертификат через certbot |
| Redis | с паролем из `secrets/redis_password.txt` | без пароля |
| Бэкапы | volume `tjudge_backups_data` | `./backups` |

Дальше в командах:

```bash
P() { docker compose -f docker-compose.prod.yml "$@"; }          # prod, из ~/TJudge
S() { docker compose -f docker-compose.selfhosted.yml --env-file config/profiles/medium.env "$@"; }   # self-hosted, профиль как при make deploy-*
```

## 1. Требования

Linux x86_64, Docker 24+ с Compose v2, 4 ГБ RAM, 20 ГБ диска (БД и бэкапы). Для prod — домен и прокси с TLS перед `:8080`. Для self-hosted — открытые 80 и 443.

## 2. Первичная настройка (prod)

```bash
git clone https://github.com/4RH1T3CT0R7/TJudge.git ~/TJudge && cd ~/TJudge
./scripts/init-secrets.sh      # secrets/{db_password,jwt_secret,redis_password}.txt, существующие не трогает
docker network create monitoring   # если её не создал стек мониторинга (§5)
```

Остальное compose берёт из `~/TJudge/.env`:

```ini
BASE_URL=https://tjudge.example.com              # ссылки-приглашения
CORS_ALLOWED_ORIGINS=https://tjudge.example.com
WEBSOCKET_ALLOWED_ORIGINS=https://tjudge.example.com
DOCKER_SOCK_GID=<stat -c %g /var/run/docker.sock>  # release.yml допишет сам, если строки нет
```

Пароли БД и Redis и `JWT_SECRET` в prod читаются только из `secrets/` (`*_FILE`). В `docker-compose.prod.yml` зашиты `ENVIRONMENT=production`, `RATE_LIMIT_ENABLED=true`, `WORKER_MAX=200`. Слабый `JWT_SECRET` (короче 32 байт или заглушка вроде `secret`, `CHANGE_ME`) роняет api на старте.

## 3. Первый запуск и выкладка (prod)

Штатный путь — пуш тега `v*`. `release.yml` собирает образы `ghcr.io/4rh1t3ct0r7/tjudge-{api,worker,executor,builder}:<версия>` (`migrate` собирается на сервере из checkout) и по ssh выполняет на сервере: `git pull origin main`, `init-secrets.sh`, `VERSION=<версия> P pull && P up -d --remove-orphans`, проверку, что `tjudge-api-1` работает на запрошенном образе, и `scripts/doctor.sh` (CRITICAL валит деплой). Сервис `migrate` применяет миграции до старта api и worker.

Worker на старте проверяет образы `EXECUTOR_DOCKER_IMAGE` (в prod — `ghcr.io/4rh1t3ct0r7/tjudge-executor:<версия>`) и `EXECUTOR_BUILDER_IMAGE` (в prod-compose не задан, по умолчанию `tjudge-builder:latest`). Отсутствующий образ он качает без авторизации, а при неудаче пишет `Docker image is not available` и работает дальше: матчи или сборки без образа уходят в повтор, пока образ не появится. `P pull` эти образы не тянет, поэтому на сервере они должны быть заранее:

```bash
docker pull ghcr.io/4rh1t3ct0r7/tjudge-executor:<версия>        # после docker login ghcr.io, если пакет приватный
docker pull ghcr.io/4rh1t3ct0r7/tjudge-builder:<версия> && docker tag ghcr.io/4rh1t3ct0r7/tjudge-builder:<версия> tjudge-builder:latest
```

Вручную то же самое:

```bash
cd ~/TJudge && git pull
export VERSION=<версия> GITHUB_REPOSITORY_OWNER=4rh1t3ct0r7 HOST_PROGRAMS_PATH="$PWD/data/programs"
P pull && P up -d --remove-orphans
curl -s http://localhost:8080/health                 # OK
make admin EMAIL=admin@example.com                   # первый админ (сначала регистрация в UI), потом перелогин
```

## 4. Резервное копирование

Включение: `P --profile backup up -d backup` (self-hosted: `S --profile backup up -d backup`). `pg_dump` раз в сутки (`BACKUP_INTERVAL_SECONDS`), хранение 30 дней (`BACKUP_RETENTION_DAYS`), файлы `tjudge_<время>.sql.gz`. Ручной бэкап: `make backup` (self-hosted, в `./backups`) или `POSTGRES_CONTAINER=tjudge-postgres-prod ./scripts/backup.sh ./backups` (prod). Проверка через сутки: `docker logs tjudge-backup`.

## 5. Мониторинг

Два режима (`MONITORING_MODE` в `.env`, по умолчанию `standalone`). api и worker из prod-compose подключены к внешней docker-сети `monitoring` с лейблами `prometheus_job: tjudge-api / tjudge-worker`, оба режима скрейпят по этой сети. Оба стека одновременно не поднимать — порты конфликтуют.

- **standalone** — стек из репо: `make monitoring-up` (= `docker compose -f docker-compose.monitoring.yml up -d`). В self-hosted мониторинг встроен в `docker-compose.selfhosted.yml` профилем `monitoring`.
- **external** — общий стек infra-monitoring (Prometheus, Grafana, Loki, Alertmanager, Pushgateway на той же сети, авто-дискавери по docker-лейблам): `MONITORING_MODE=external` в `.env`, стек — `cd ~/infra-monitoring && docker compose up -d`.

Порты одинаковые: Prometheus 9092 (правила — `deployments/prometheus/alerts/`), Alertmanager 9093, Pushgateway 9094 (метрики doctor'а), Grafana 3000 (дашборды «TJudge — Обзор системы» и «TJudge — Doctor», учётка — `GF_ADMIN_USER`/`GF_ADMIN_PASSWORD`). Alertmanager по умолчанию шлёт в получатель `null`. Telegram включается по инструкции в `deployments/alertmanager/alertmanager.yml`: токен в `deployments/alertmanager/telegram_token`, chat_id и receiver раскомментировать. Если `make doctor` пишет «Prometheus НЕ скрейпит API», почти всегда виноват старый Prometheus в другой docker-сети: остановить его и поднять стек как выше.

**Guardian (только external).** В infra-monitoring входит guardian — авто-восстановление контейнеров. Лейблы в `docker-compose.prod.yml` уже стоят: nginx/api/worker — `guardian.enabled` (рестарт при unhealthy), postgres/redis — `notify-only` (БД не рестартуются: WAL-replay удлинит восстановление). Перед долгими ручными работами: `cd ~/infra-monitoring && make maintenance 1h`.

## 6. Чеклист перед открытием на публику

- [ ] `secrets/*.txt` созданы, права 600, `JWT_SECRET` не короче 32 байт (проверяется при старте).
- [ ] `.env` и `secrets/` не в git: `git ls-files .env secrets` пусто.
- [ ] TLS и HSTS на внешнем прокси, HTTP-трафика снаружи нет.
- [ ] `CORS_ALLOWED_ORIGINS` — только ваши домены, без `*`.
- [ ] `WEBSOCKET_ALLOWED_ORIGINS` — ваш домен (пусто или `*` в production пускает только свой хост).
- [ ] Worker не от root: `P exec worker id -u` → 1000.
- [ ] Бэкапы работают: `docker ps | grep tjudge-backup`.
- [ ] `make security` прошёл, HIGH-находки исправлены.
- [ ] Первый админ назначен, пароль надёжный.

## 7. Обновление

Штатное обновление — новый тег (§3): `P up -d` пересоздаёт контейнеры, а `migrate` применяет миграции до старта нового api. Пока идёт миграция, старый api ещё работает, поэтому миграции должны быть совместимы с предыдущей версией. 000042–000045 такие: добавляют nullable-столбец, меняют инвайт-коды и удаляют индексы-дубли. 000045 запускать в окно низкой нагрузки (§9.7). После 000043 капитанам pending-турниров нужно заново раздать инвайт-коды. Проверка схемы: `P run --rm migrate ./migrate version` → `dirty: false`.

Откат: `export VERSION=<прошлая версия>` и `P up -d`. Down-миграции для этого не нужны.

### 7.1 Очистка диска после релизов

`release.yml` после выкладки делает `docker image prune -f` и `docker builder prune -f`: удаляются только dangling-образы и build-кэш.

Не запускать на проде `docker image prune -af` без фильтров. Образ исполнителя матчей нужен только во время матча, в остальное время на нём нет контейнеров, и prune его удалит. Без него матчи уходят в повтор, пока worker не скачает образ заново. `docker system prune -af --volumes` и `docker volume prune` при остановленных postgres/redis удалят volume с БД.

Ротация логов контейнеров — `/etc/docker/daemon.json`:

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "50m", "max-file": "3" }
}
```

После правки `sudo systemctl restart docker`; контейнеры подхватят настройку при пересоздании. Каталог программ участников автоматически не чистится: это данные, не мусор.

## 8. Быстрая диагностика

### make status

`make status` на сервере, рядом с compose-файлами. Для полного статуса (БД, очереди, матчи, программы, outbox) — учётка админа в `.env`: `ADMIN_USER=<логин или email>`, `ADMIN_PASSWORD=<пароль>`, скрипт сам получает свежий JWT. Лучше отдельная служебная учётка (`make admin EMAIL=...`), чем личный пароль. `ADMIN_TOKEN=<jwt>` протухает за `JWT_ACCESS_TTL` (1 час).

Проверяет контейнеры, наличие образов `tjudge-cli`/`tjudge-builder`, не работают ли api/worker на устаревшем образе, health API/worker и с учёткой — `GET /api/v1/system/status`: версию сборки и аптайм, PostgreSQL и версию миграций, Redis, размеры очередей (матчи, компиляция, dead-letter), матчи и программы по статусам, зависшие `running` (дольше `WORKER_TIMEOUT` + 30s), outbox рейтингов, WebSocket-клиентов. Агрегаты кэшируются на 30 с. Тот же статус — админка, вкладка «Система»; сырой JSON — `curl -sH "Authorization: Bearer <admin-jwt>" http://localhost:8080/api/v1/system/status | jq`.

### make doctor — глубокая диагностика после деплоя

```bash
make doctor                         # терминальный отчёт + telegram при проблемах
./scripts/doctor.sh --json          # машиночитаемый вывод
DOCTOR_TELEGRAM=always make doctor  # отчёт в telegram даже когда всё ок
```

Проверяет контейнеры (включая crash-loop по RestartCount), образы, health API/worker, глубокий статус (нужны `ADMIN_USER`/`ADMIN_PASSWORD`), Prometheus (up-цели, 5xx против SLO, активные алерты), ошибки в логах api/worker за `DOCTOR_LOG_WINDOW`, диск. Вердикт: HEALTHY / DEGRADED / CRITICAL (exit 1). Отчёт уходит в Telegram (`TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`) и в Pushgateway (`PUSHGATEWAY_URL`, по умолчанию `http://localhost:9094`) для дашборда «TJudge — Doctor» и алертов `DoctorCritical`/`DoctorDegraded`/`DoctorStale`. Cron: `*/30 * * * * cd ~/TJudge && ./scripts/doctor.sh >/dev/null 2>&1`.

### Точечные проверки

Метрики отдаёт отдельный сервер на `METRICS_PORT` (9090), на основном порту `/metrics` нет.

```bash
curl -sf http://localhost:8080/health                                                    # OK
curl -sfH "Authorization: Bearer <admin-jwt>" http://localhost:8080/api/v1/system/health  # json со статусом

# prod
P logs --tail=200 api
P exec api wget -qO- localhost:9090/metrics | grep -E '^(tjudge_|go_goroutines)'
docker exec tjudge-redis-prod redis-cli -a "$(cat secrets/redis_password.txt)" --no-auth-warning LLEN queue:high

# self-hosted
docker logs --tail=200 tjudge-api
curl -s localhost:9090/metrics | grep '^tjudge_'        # worker — :9091
docker exec tjudge-redis redis-cli LLEN queue:high
```

Очереди Redis: `queue:high`, `queue:medium`, `queue:low` (матчи), `queue:compile`, `queue:dead_letter`.

## 9. Инциденты

### 9.0 Всё упало: полный старт/стоп

После перезагрузки сервера обычно ничего делать не нужно: `restart: always`, docker поднимет всё сам. Подождать 2-3 минуты, проверить `make doctor`. Ручной полный старт (порядок важен: TJudge требует внешнюю сеть `monitoring`, её создаёт инфра-стек):

```bash
cd ~/infra-monitoring && make up                  # 1. мониторинг + сеть
cd ~/TJudge && P up -d                           # 2. tjudge
P --profile backup up -d backup                  # 3. бэкапы
make doctor                                       # 4. проверка
```

Без мониторинга достаточно `docker network create monitoring`, дальше шаг 2. Полная остановка (мониторинг первым, чтобы не спамить алертами): `cd ~/infra-monitoring && make down`, затем `cd ~/TJudge && P down`. Никогда не добавлять `-v` к `down`: удалит volumes с базой, Redis и программами. Остановить на время — `P stop` / `P start`.

### 9.1 API падает или в restart-цикле

1. `P logs --tail=300 api`. Частые причины: `JWT_SECRET must be at least 32 bytes` — обновить секрет (§11); `invalid environment` или `config validation failed` — неверное значение переменной; `database connection refused` — Postgres (§9.3); `Dirty database version` у `migrate` — §9.7.
2. `P restart api`.
3. Не помогло — откат на прошлую версию (§7).

### 9.2 Матчи не разбираются (QueueStuck)

Триггер: воркеры заняты (`sum(tjudge_active_workers) > 0`), а за 10 минут не завершился ни один матч. Большая очередь в начале раунда штатна, размер очереди (`tjudge_queue_size`) не показатель: гейдж залипает.

1. Завершения: `increase(tjudge_matches_total[10m])` в Prometheus — ноль подтверждает зависание.
2. Логи воркера: `P logs --tail=300 worker | grep -i error`. Без образов исполнителя и песочницы в логе `Docker image is not available`, матчи и сборки висят в повторе.
3. Docker daemon: `docker info` и `docker ps` на хосте отвечают быстро? Зависший daemon держит воркеры занятыми без результата.
4. Матчи, зависшие в `running` дольше `WORKER_TIMEOUT` + 30s, recovery воркера раз в минуту возвращает в `pending` и ставит в очередь. Вручную — кнопка во вкладке «Система» или `POST /api/v1/system/recovery/reset-stuck-matches` (только сброс в `pending`, в очередь их ставит recovery).
5. Матчи идут, но медленно: размер пула — `tjudge_worker_pool_size`. `WORKER_MAX` в prod зашит в `docker-compose.prod.yml`, в self-hosted задаётся в `config/profiles/<profile>.env`. Больше воркеров, чем ядер, даёт переподписку CPU и ложные таймауты программ.

### 9.3 Postgres недоступен

1. `docker ps | grep postgres` — контейнер запущен?
2. Нет: `P up -d postgres`, ждать healthcheck.
3. Запущен, но недоступен: `docker exec -it tjudge-postgres-prod psql -U tjudge -c 'SELECT 1'` (self-hosted — `tjudge-postgres`).
4. Диск: `df -h /var/lib/docker`. При переполнении удалить старые бэкапы и партиции.
5. Crash-loop — восстановление из бэкапа (§10).

### 9.4 Redis недоступен

Rate limiter api переключается на запасной in-memory лимит (вдвое строже основного); очереди матчей и компиляции без Redis не работают. Перезапуск: `P restart redis`. Если очереди потеряны, recovery воркера раз в минуту заново ставит `pending`-матчи в очередь. Программы, застрявшие в `compiling`, возвращает в очередь кнопка во вкладке «Система» (`POST /api/v1/system/recovery/requeue-compiling`).

### 9.5 Dead-letter и WebSocket

- Растёт `tjudge_queue_deadletter_size` (алерт `DeadLetterGrowing`): задачи, которые не удалось обработать. Причину искать в логах worker'а, очистка — кнопка во вкладке «Система» (`POST /api/v1/system/recovery/clear-dead-letter`).
- Клиентский flood по WebSocket: у каждого клиента лимит 10 сообщений в секунду (burst 20), нарушитель получает close 1008.

### 9.6 Расследование admin-действий

`curl -sH "Authorization: Bearer <admin-jwt>" 'http://localhost:8080/api/v1/admin/audit?limit=500' | jq`, в БД — `SELECT * FROM audit_log ORDER BY created_at DESC LIMIT 200;`. В аудит попадают `/system/*`, админские ручки турниров, игр, команд и очереди матчей, а также `POST /tournaments/{id}/games`.

### 9.7 Миграция в состоянии DIRTY

Упавшая миграция оставляет схему в `dirty`, и каждый следующий `migrate up` падает с `Dirty database version N. Fix and force version.`; api и worker в prod-compose ждут `migrate` и не стартуют. 000045 падает так намеренно: DROP INDEX на партиционированных таблицах ждёт лок не дольше 5 с (`lock_timeout`), и транзакция откатывается целиком. Лок держат pg_dump бэкапа, трафик старого api и долгие запросы лидербордов.

```bash
P run --rm migrate ./migrate version   # Current version: 45 (dirty: true)
P run --rm migrate ./migrate force 44  # 000045 откатилась целиком
# повтор в окно низкой нагрузки, когда не идёт pg_dump бэкапа
P run --rm migrate
P up -d
```

Для другой миграции N `force N-1` верен, только если её изменения не применились: файл выполняется одной транзакцией, так что при ошибке это обычно так, но стоит сверить схему.

## 10. Восстановление из бэкапа

`scripts/restore.sh` делает страховочный дамп, пересоздаёт базу и заливает бэкап.

```bash
# self-hosted
make restore BACKUP=backups/tjudge_<время>.sql.gz
S run --rm migrate && S up -d api worker      # если бэкап старше текущих миграций

# prod: бэкапы лежат в volume, сначала копия на хост
P stop api worker
mkdir -p backups && docker run --rm -v tjudge_backups_data:/b -v "$PWD/backups":/out alpine sh -c 'cp "$(ls -t /b/tjudge_*.sql.gz | head -1)" /out/'
POSTGRES_CONTAINER=tjudge-postgres-prod ./scripts/restore.sh backups/tjudge_<время>.sql.gz
P up -d                                        # migrate догонит схему, потом старт api и worker
```

Point-in-time recovery не настроен: WAL-archiving выключен. Для него нужен `pgbackrest` или `wal-g` с S3-хранилищем.

## 11. Частые задачи

- **Ротация JWT_SECRET:** `openssl rand -hex 48 > secrets/jwt_secret.txt`, затем `P up -d --force-recreate api`. Все сессии инвалидируются, пользователи перелогиниваются.
- **Назначение админа:** `make admin EMAIL=foo@bar.com` (находит контейнер `tjudge-postgres*`), затем перелогин.
- **Миграции:** в prod — `P run --rm migrate` (up), `./migrate version`, `./migrate force N`; локально — `make migrate-up`, `make migrate-down` (откат одной). Новая миграция: `make migrate-create` (спросит имя, нужен CLI golang-migrate).
- **Сброс пароля:** bcrypt cost 12, как в auth.Service. `password_changed_at` отзывает выданные раньше refresh-токены.

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
UPDATE users SET password_hash = crypt('NewPassw0rd', gen_salt('bf', 12)), password_changed_at = NOW()
WHERE email = 'foo@bar.com';
```

## 12. Масштабирование и ограничения

- `WORKER_MAX` по умолчанию равен числу ядер: каждый воркер держит матч-контейнер с квотой CPU, переподписка даёт ложные таймауты программ. Пул БД по умолчанию считается от `WORKER_MAX` (не больше 100), явный `DB_MAX_CONNECTIONS` держать в пределах `max_connections - 10` на все реплики вместе. Пул Redis автоматически не меньше `WORKER_MAX` + 20.
- API stateless, несколько реплик работают. События турнира расходятся по репликам через Redis pub/sub `tjudge:events`.
- Worker безопасен в нескольких репликах: матчи берутся из общей очереди, компиляция идёт под локом `lock:compile:<id>`, планирование турнира — под локом `tournament:schedule:<id>`. На старте worker удаляет свои и осиротевшие контейнеры по метке `tjudge.owner=<hostname>`, поэтому hostname у реплик должен различаться.
- `WORKER_TIMEOUT` у api и worker должен совпадать: от него считается порог зависшего матча в recovery и в статусе.
- Рейтинг применяется ровно один раз: outbox-задача и строки участников блокируются в одной транзакции.
- Worker монтирует `docker.sock` и работает не от root: на хосте нужна группа docker с GID из `DOCKER_SOCK_GID` (prod) или `DOCKER_GID` (self-hosted).

## 13. Профили железа и быстрый self-hosted старт

Профили задают число воркеров, лимиты памяти/CPU на матч и пулы БД/Redis (`config/profiles/{weak,medium,strong}.env`):

| Профиль | CPU | RAM | WORKER_MAX | Память/матч | Для кого |
|---------|-----|-----|------------|-------------|----------|
| weak | 2 | 4 ГБ | 3 | 256 MiB | старый ноут, начальный VPS; турниры до ~50 участников |
| medium | 4 | 8 ГБ | 5 | 512 MiB | обычный сервер; до ~200 участников |
| strong | 8+ | 16+ ГБ | 20 | 1 GiB | выделенный сервер; 500+ участников |

Быстрый старт (миграции применяет сервис `migrate`):

```bash
git clone https://github.com/4RH1T3CT0R7/TJudge.git && cd TJudge
make detect-profile       # рекомендуемый профиль по железу
make deploy               # авто: определит железо, соберёт и поднимет
# либо вручную: make deploy-weak | make deploy-medium | make deploy-strong
S ps
docker exec tjudge-api wget -qO- localhost:8080/health   # OK
```

nginx отдаёт только HTTPS (80 редиректит) и без сертификата для `DOMAIN` (по умолчанию `localhost`) не стартует. `./scripts/init-ssl.sh <домен>` кладёт временный самоподписанный сертификат, команды получения настоящего через certbot — в шапке скрипта.

Worker проверяет на старте, что `WORKER_TIMEOUT` не меньше `EXECUTOR_TIMEOUT` + 20s, и при несогласованных таймаутах не запускается (`WORKER_TIMEOUT (…) must be at least EXECUTOR_TIMEOUT (…) + 20s` в логах). Если в профиле или в `docker-compose.selfhosted.yml` таймауты равны (60s/60s), перед `make deploy*` нужно добавить в профиль `WORKER_TIMEOUT=90s`, для weak с `EXECUTOR_TIMEOUT=90s` — `WORKER_TIMEOUT=120s`.

`make deploy*` вызывают `scripts/quick-deploy.sh <profile>`. Смена профиля — `S down` и заново нужным `make deploy-*`.
