# TJudge - эксплуатация

Runbook для single-node: деплой, диагностика, восстановление. Режимов два, команды для них различаются:

| | prod | self-hosted |
|---|---|---|
| Как выкатывается | тег `v*` → `.github/workflows/release.yml` (ssh на сервер, `~/TJudge`, образы из ghcr.io) | `make deploy*` → `scripts/quick-deploy.sh`, сборка на месте, §13 |
| Compose | `docker-compose.prod.yml` | `docker-compose.selfhosted.yml` + `--env-file .env --env-file config/profiles/<profile>.env` |
| Контейнеры | `tjudge-postgres-prod`, `tjudge-redis-prod`, `tjudge-nginx-prod`; api и worker по 2 реплики (`tjudge-api-1`, `tjudge-worker-1`, …) | `tjudge-postgres`, `tjudge-redis`, `tjudge-api`, `tjudge-worker`, `tjudge-nginx` |
| HTTP | nginx на `${NGINX_BIND:-127.0.0.1}:8080`, TLS и HSTS — на внешнем прокси | nginx на 80/443: http до выпуска сертификата, потом https (certbot) |
| Redis | с паролем из `secrets/redis_password.txt` | с паролем из `secrets/redis_password.txt` |
| Бэкапы | `./backups` (контейнер `tjudge-backup`) | `./backups` (контейнер `tjudge-backup`) |

Дальше в командах:

```bash
P() { docker compose -f docker-compose.prod.yml "$@"; }          # prod, из ~/TJudge; VERSION и пути хоста берутся из .env
S() { docker compose -f docker-compose.selfhosted.yml --env-file .env --env-file config/profiles/medium.env "$@"; }   # self-hosted, профиль как при make deploy-*
```

Явный `--env-file` отключает автозагрузку `.env`, поэтому в `S` он передаётся первым: без него api получит `BASE_URL=http://localhost`, а worker — чужой GID группы docker.

## 1. Требования

Linux x86_64, Docker 24+ с Compose v2, 4 ГБ RAM, 20 ГБ диска (БД и бэкапы). Для prod — домен и прокси с TLS перед nginx на `:8080` (по умолчанию слушает только 127.0.0.1, §2). Для self-hosted — открытые 80 и 443.

## 2. Первичная настройка (prod)

```bash
git clone https://github.com/4RH1T3CT0R7/TJudge.git ~/TJudge && cd ~/TJudge
./scripts/init-secrets.sh      # secrets/{db_password,jwt_secret,redis_password,grafana_admin_password}.txt, существующие не трогает
docker network create monitoring   # если её не создал стек мониторинга (§5)
```

Остальное compose берёт из `~/TJudge/.env`:

```ini
BASE_URL=https://tjudge.example.com   # ссылки-приглашения; CORS_ALLOWED_ORIGINS по умолчанию равен ему
#NGINX_BIND=0.0.0.0                   # только если TLS-прокси не на этом хосте (ниже)
#WORKER_MAX=4                         # воркеров на реплику, реплик 2
#TELEGRAM_BOT_TOKEN=                  # алерты и doctor (§5), с BACKUP_TELEGRAM_CHAT_ID — копии бэкапов (§4)
#BACKUP_TELEGRAM_CHAT_ID=
# деплой дописывает сам: DOCKER_SOCK_GID, HOST_PROGRAMS_PATH=$PWD/data/programs, VERSION
```

Пароли БД и Redis и `JWT_SECRET` в prod читаются только из `secrets/` (`*_FILE`). В `docker-compose.prod.yml` зашиты `ENVIRONMENT=production` и `RATE_LIMIT_ENABLED=true`. `WORKER_MAX` задаётся в `.env` на реплику (по умолчанию 4): `WORKER_MAX` × 2 реплики ≈ числу ядер хоста. Слабый `JWT_SECRET` (короче 32 байт или заглушка вроде `secret`, `CHANGE_ME`) роняет api на старте. `HOST_PROGRAMS_PATH` должен быть абсолютным: относительный путь (например, `./data/programs` из старого `.env.example`) `prepare-data.sh` отвергает, и деплой прерывается.

nginx публикуется на `${NGINX_BIND:-127.0.0.1}:8080` и берёт IP клиента из `X-Forwarded-For` только от шлюза docker-сети 172.28.0.1 (`set_real_ip_from` в `deployments/nginx/nginx.prod.conf`). TLS-прокси на этом же хосте, который ходит на `127.0.0.1:8080`, работает без настройки. Если прокси на другом хосте, в контейнере или обращается по IP хоста (`172.17.0.1` и т.п.), нужны `NGINX_BIND=0.0.0.0` в `.env` и его адрес в `set_real_ip_from`. До первой выкладки этой версии проверить, откуда прокси ходит на :8080: `ss -tnp | grep 8080` и конфиг прокси.

## 3. Первый запуск и выкладка (prod)

Штатный путь — пуш тега `v*`. `release.yml` собирает образы `ghcr.io/4rh1t3ct0r7/tjudge-{api,worker,migrate,executor,builder}:<версия>` и по ssh выполняет на сервере в `~/TJudge`:

1. `git fetch --tags --force origin && git checkout --detach <тег>`: compose, скрипты и конфиги той же версии, что и образы. Сервер всегда в detached HEAD, `git pull` там не работает.
2. `docker login ghcr.io`, затем дописывает в `.env` недостающие `DOCKER_SOCK_GID` и `HOST_PROGRAMS_PATH=$PWD/data/programs` и перезаписывает `VERSION=<версия>`.
3. `init-secrets.sh`, `P pull` и `docker pull` образов песочниц `tjudge-executor` и `tjudge-builder` той же версии (это не сервисы compose, `P pull` их не тянет).
4. `./scripts/prepare-data.sh docker-compose.prod.yml`: каталоги `HOST_PROGRAMS_PATH` и `./backups` от uid 1000 и одноразовый перенос программ из старого volume (§3.1).
5. `P up -d --remove-orphans`: `migrate` применяет миграции до старта api и worker.
6. Проверку, что `tjudge-api-1` работает на образе этой версии, пересборку включённого `backup`, prune dangling-образов и build-кэша и `scripts/doctor.sh` (CRITICAL валит деплой).

Worker на старте проверяет образы `EXECUTOR_DOCKER_IMAGE` и `EXECUTOR_BUILDER_IMAGE` (в prod — `ghcr.io/4rh1t3ct0r7/tjudge-{executor,builder}:${VERSION}`). Отсутствующий образ он качает без авторизации ghcr, а при неудаче пишет `Docker image is not available` и работает дальше: матчи или сборки уходят в повтор, пока образ не появится. Поэтому `VERSION` живёт в `.env`: без него compose берёт `:latest`, которого на сервере нет.

Вручную то же самое:

```bash
cd ~/TJudge
git fetch --tags --force origin && git checkout -q --detach v<версия>
touch .env && { [ -z "$(tail -c1 .env)" ] || echo >> .env; }   # перевод строки перед дописыванием
grep -q '^DOCKER_SOCK_GID=' .env || echo "DOCKER_SOCK_GID=$(stat -c %g /var/run/docker.sock)" >> .env
grep -q '^HOST_PROGRAMS_PATH=' .env || echo "HOST_PROGRAMS_PATH=$PWD/data/programs" >> .env
sed -i '/^VERSION=/d' .env && echo "VERSION=<версия>" >> .env
./scripts/init-secrets.sh
echo "$GHCR_TOKEN" | docker login ghcr.io -u 4rh1t3ct0r7 --password-stdin   # если пакеты приватные
P pull
docker pull ghcr.io/4rh1t3ct0r7/tjudge-executor:<версия> && docker pull ghcr.io/4rh1t3ct0r7/tjudge-builder:<версия>
./scripts/prepare-data.sh docker-compose.prod.yml
P up -d --remove-orphans
curl -s http://127.0.0.1:8080/health                 # OK
make admin EMAIL=admin@example.com                   # первый админ (сначала регистрация в UI), потом перелогин
```

### 3.1 Первая выкладка на сервер со старой версией

До этой версии prod хранил программы в volume `tjudge_programs_data`, а бэкапы — в `tjudge_backups_data`. Один раз, при первой выкладке:

0. Дамп базы на старой версии, не зависящий от скриптов: `docker exec tjudge-postgres-prod pg_dump --no-owner --no-acl -U tjudge tjudge | gzip > ~/tjudge_pre_upgrade.sql.gz`. Старые инвайт-коды после 000043 вернуть можно только из него.
1. До пуша тега проверить `.env`: `HOST_PROGRAMS_PATH` либо абсолютный, либо строки нет (деплой допишет `$PWD/data/programs`); `BASE_URL=https://<домен>` (CORS и WebSocket по умолчанию берут его, `*` по умолчанию больше нет); `NGINX_BIND` по §2; Telegram по §4 и §5.
2. Деплой. `prepare-data.sh` останавливает api и worker, копирует volume в `HOST_PROGRAMS_PATH`, сверяет число файлов, отдаёт каталог uid 1000 и ставит в volume отметку `.migrated-from-volume`, поэтому следующие деплои перенос не повторяют. Его ошибки прерывают деплой до `up`:
   - `файлы есть и в volume …, и в …: объедините их вручную` — каталог хоста уже не пуст. Файлы названы по uuid, конфликтов имён нет: скопировать недостающее и поставить отметку, затем повторить деплой.

     ```bash
     HOST_PROGRAMS_PATH=$(sed -n 's/^HOST_PROGRAMS_PATH=//p' .env | tail -1)
     docker run --rm -v tjudge_programs_data:/from -v "$HOST_PROGRAMS_PATH":/to alpine sh -c '
       cd /from && find . -type f ! -name .migrated-from-volume | while read -r f; do
         [ -e "/to/$f" ] || { mkdir -p "/to/${f%/*}" && cp -p "$f" "/to/$f"; }
       done
       chown -R 1000:1000 /to && touch /from/.migrated-from-volume'
     ```

   - `в БД программ: N, а … пуст` — таблица `programs` не пуста, а каталог пуст: не тот путь в `.env` или несмонтированный диск.
   - `HOST_PROGRAMS_PATH должен быть абсолютным путём` — исправить `.env`.
3. Руками: старые бэкапы перенести в `./backups`, где их видят `restore.sh` и ретенция контейнера бэкапа: `docker run --rm -v tjudge_backups_data:/b -v "$PWD/backups":/out alpine sh -c 'cp -p /b/* /out/'`.
4. После проверки (программы компилируются, матчи идут, в `./backups` появилась пара новых файлов): `docker volume rm tjudge_programs_data tjudge_backups_data`.

## 4. Резервное копирование

Контейнер `tjudge-backup`, включение: `P --profile backup up -d backup` (self-hosted: `S --profile backup up -d backup`). Сразу после старта и дальше раз в сутки (`BACKUP_INTERVAL_SECONDS`) он пишет в `./backups` пару файлов с общей меткой времени: `tjudge_<время>.sql.gz` (pg_dump) и `programs_<время>.tar.gz` (каталог программ). Хранение 30 дней (`BACKUP_RETENTION_DAYS`). Копия до 50 МБ уходит в Telegram, только если в `.env` заданы `TELEGRAM_BOT_TOKEN` и `BACKUP_TELEGRAM_CHAT_ID` (отдельный чат: в дампе email и хеши паролей).

Ручной бэкап — через sudo, файлы программ принадлежат uid 1000: `sudo make backup` (self-hosted) или `sudo POSTGRES_CONTAINER=tjudge-postgres-prod ./scripts/backup.sh ./backups` (prod). Он пишет ту же пару файлов и старые не удаляет. `.env` он не читает, поэтому в Telegram отправляет, только если переменные заданы в окружении. Проверка: `docker logs tjudge-backup`, `ls -lt backups | head`. При запущенном контейнере `make doctor` ставит CRITICAL, если в `./backups` нет дампа или архива программ моложе `DOCTOR_BACKUP_MAX_AGE_HOURS` (26 ч).

## 5. Мониторинг

Два режима, по умолчанию `standalone`. api и worker из prod-compose подключены к внешней docker-сети `monitoring` с лейблами `prometheus_job: tjudge-api / tjudge-worker`, оба режима скрейпят по этой сети. Оба стека одновременно не поднимать — порты конфликтуют.

- **standalone** — стек из репо: `make monitoring-up` (= `docker compose -f docker-compose.monitoring.yml up -d`). В self-hosted мониторинг встроен в `docker-compose.selfhosted.yml` профилем `monitoring`.
- **external** — общий стек infra-monitoring (Prometheus, Grafana, Loki, Alertmanager, Pushgateway на той же сети, авто-дискавери по docker-лейблам): стек — `cd ~/infra-monitoring && docker compose up -d`, `make monitoring-up` здесь не нужен (с `MONITORING_MODE=external` в окружении он ничего не поднимает; `.env` Makefile не читает).

Порты одинаковые, в standalone только на 127.0.0.1 (снаружи — ssh-туннель): Prometheus 9092 (правила — `deployments/prometheus/alerts/`), Alertmanager 9093, Pushgateway 9094 (метрики doctor'а), Grafana 3000 (дашборды «TJudge — Обзор системы» и «TJudge — Doctor»; логин `GF_ADMIN_USER`, по умолчанию `admin`, пароль — в `secrets/grafana_admin_password.txt`, его создаёт `init-secrets.sh`, `make monitoring-up` вызывает его сам). Alertmanager без Telegram шлёт в получатель `null`. С `TELEGRAM_BOT_TOKEN` и `TELEGRAM_CHAT_ID` в `.env` получатель telegram добавляется сам и становится основным, файлы править не нужно. Те же переменные нужны отчётам doctor. Если `make doctor` пишет «Prometheus НЕ скрейпит API», почти всегда виноват старый Prometheus в другой docker-сети: остановить его и поднять стек как выше.

**Guardian (только external).** В infra-monitoring входит guardian — авто-восстановление контейнеров. Лейблы в `docker-compose.prod.yml` уже стоят: nginx/api/worker — `guardian.enabled` (рестарт при unhealthy), postgres/redis — `notify-only` (БД не рестартуются: WAL-replay удлинит восстановление). Перед долгими ручными работами: `cd ~/infra-monitoring && make maintenance 1h`.

## 6. Чеклист перед открытием на публику

- [ ] Каталог `secrets/` с правами 700 (файлы 644 ставит `init-secrets.sh`: сервисы в контейнерах читают их не от владельца), `JWT_SECRET` не короче 32 байт (проверяется при старте).
- [ ] `.env` и `secrets/` не в git: `git ls-files .env secrets` пусто.
- [ ] TLS и HSTS на внешнем прокси, HTTP-трафика снаружи нет, nginx на `NGINX_BIND=127.0.0.1` (или открыт только прокси).
- [ ] `BASE_URL=https://<домен>`: от него по умолчанию считается `CORS_ALLOWED_ORIGINS`.
- [ ] `CORS_ALLOWED_ORIGINS` — только ваши домены, без `*`.
- [ ] `WEBSOCKET_ALLOWED_ORIGINS`: пусто — берётся `CORS_ALLOWED_ORIGINS` (по умолчанию `BASE_URL`), `*` — только свой хост. Вход по IP или по http при `BASE_URL=https://…` получает на WebSocket 403, и фронт молча уходит в поллинг.
- [ ] Worker не от root: `P exec worker id -u` → 1000.
- [ ] Бэкапы работают: `docker ps | grep tjudge-backup`.
- [ ] `make security` прошёл, HIGH-находки исправлены.
- [ ] Первый админ назначен, пароль надёжный.

## 7. Обновление

Штатное обновление — новый тег (§3): `P up -d` пересоздаёт контейнеры, а `migrate` применяет миграции до старта нового api. Пока идёт миграция, старый api ещё работает, поэтому миграции должны быть совместимы с предыдущей версией. 000042–000045 такие: добавляют nullable-столбец, меняют инвайт-коды и удаляют индексы-дубли. 000045 запускать в окно низкой нагрузки (§9.7). После 000043 капитанам pending-турниров нужно заново раздать инвайт-коды. Проверка схемы: `P run --rm migrate ./migrate version` → `dirty: false`.

Откат — сменой `VERSION` на текущем checkout. `git checkout` тега, выпущенного до перехода на каталог программ хоста, не делать: его compose снова смонтирует volume `programs_data`, и программы, загруженные после переноса, пропадут. Down-миграции не нужны. Worker и образ матча `tjudge-executor` откатываются только вместе, это обеспечивает общая `VERSION`: новый worker со старым образом не может запустить матч (матчи возвращаются в pending), старый worker с новым образом играет без разведения ботов по uid. Образы песочниц той версии скачиваются заранее: worker тянет их без авторизации ghcr.

```bash
V=<прошлая версия>
docker pull ghcr.io/4rh1t3ct0r7/tjudge-executor:$V && docker pull ghcr.io/4rh1t3ct0r7/tjudge-builder:$V
sed -i "s/^VERSION=.*/VERSION=$V/" .env
P up -d --no-deps api worker
```

`VERSION` правится в `.env`, иначе следующий ручной `P up -d` снова поднимет плохой релиз. Пока там прошлая версия, её `migrate` не знает новых миграций (у ранних версий нет и образа `tjudge-migrate`), поэтому `P run --rm migrate` и `P up -d` без `--no-deps` падают на migrate: сервисы перезапускать через `P up -d --no-deps <сервис>`, вперёд — новым тегом.

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

Проверяет контейнеры (включая crash-loop по RestartCount), образы, health API/worker, глубокий статус (нужны `ADMIN_USER`/`ADMIN_PASSWORD`), Prometheus (up-цели, 5xx против SLO, активные алерты), ошибки в логах api/worker за `DOCTOR_LOG_WINDOW` (уровень error; ответы 4xx api пишет как warn), диск, свежесть бэкапов (§4). Вердикт: HEALTHY / DEGRADED / CRITICAL (exit 1). Отчёт уходит в Telegram (`TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`) и в Pushgateway (`PUSHGATEWAY_URL`, по умолчанию `http://localhost:9094`) для дашборда «TJudge — Doctor» и алертов `DoctorCritical`/`DoctorDegraded`/`DoctorStale`. Cron: `*/30 * * * * cd ~/TJudge && ./scripts/doctor.sh >/dev/null 2>&1`.

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
docker exec tjudge-redis redis-cli -a "$(cat secrets/redis_password.txt)" --no-auth-warning LLEN queue:high
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

Триггер: воркеры заняты (`sum(tjudge_active_workers) > 0`), а за 10 минут не завершился ни один матч. Большая очередь в начале раунда штатна. Размер очереди — `sum(max by (priority) (tjudge_queue_size{job="tjudge-worker"}))`: гейдж api хранит значение на момент последней постановки и не информативен.

1. Завершения: `increase(tjudge_matches_total[10m])` в Prometheus — ноль подтверждает зависание.
2. Логи воркера: `P logs --tail=300 worker | grep -i error`. Без образов исполнителя и песочницы в логе `Docker image is not available`, матчи и сборки висят в повторе.
3. Docker daemon: `docker info` и `docker ps` на хосте отвечают быстро? Зависший daemon держит воркеры занятыми без результата.
4. Матчи, зависшие в `running` дольше `WORKER_TIMEOUT` + 30s, recovery воркера раз в минуту возвращает в `pending` и ставит в очередь. Вручную — кнопка во вкладке «Система» или `POST /api/v1/system/recovery/reset-stuck-matches` (только сброс в `pending`, в очередь их ставит recovery).
5. Матчи идут, но медленно: размер пула — `tjudge_worker_pool_size`. `WORKER_MAX` в prod задаётся в `.env` на реплику (по умолчанию 4, реплик 2), в self-hosted — в `config/profiles/<profile>.env`. Больше воркеров, чем ядер, даёт переподписку CPU и ложные таймауты программ.

### 9.3 Postgres недоступен

1. `docker ps | grep postgres` — контейнер запущен?
2. Нет: `P up -d postgres`, ждать healthcheck.
3. Запущен, но недоступен: `docker exec -it tjudge-postgres-prod psql -U tjudge -c 'SELECT 1'` (self-hosted — `tjudge-postgres`).
4. Диск: `df -h /var/lib/docker`. При переполнении удалить старые бэкапы и партиции.
5. Crash-loop — восстановление из бэкапа (§10).

### 9.4 Redis недоступен

Rate limiter api переключается на запасной in-memory лимит (вдвое строже основного); очереди матчей и компиляции без Redis не работают. Перезапуск: `P restart redis`. Если очереди потеряны, recovery воркера раз в минуту заново ставит `pending`-матчи в очередь. Программы, застрявшие в `compiling`, возвращает в очередь кнопка во вкладке «Система» (`POST /api/v1/system/recovery/requeue-compiling`).

### 9.5 Dead-letter и WebSocket

- Алерт `DeadLetterGrowing` (больше 10 записей в dead-letter за 10 минут, `tjudge_queue_deadletter_push_total`, размер — `tjudge_queue_deadletter_size`): задачи, которые не удалось разобрать. Причину искать в логах worker'а, очистка — кнопка во вкладке «Система» (`POST /api/v1/system/recovery/clear-dead-letter`).
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

`scripts/restore.sh` делает страховочный дамп текущей базы, останавливает api, worker и backup (ищет их по compose-проекту контейнера postgres, `COMPOSE_FILE` не нужен), пересоздаёт базу и заливает дамп. Архив `programs_<время>.tar.gz` с той же меткой он подхватывает сам и заменяет им каталог программ, прежний остаётся рядом как `<каталог>.pre_restore_<время>`. Потом сервисы запускаются обратно. Запуск через sudo: файлы программ принадлежат uid 1000. Бэкапы старой версии лежат в volume (§3.1), дамп без архива программ восстанавливает только БД.

```bash
# self-hosted
sudo make restore BACKUP=backups/tjudge_<время>.sql.gz
S run --rm migrate && S up -d api worker      # если бэкап старше текущих миграций

# prod
sudo POSTGRES_CONTAINER=tjudge-postgres-prod ./scripts/restore.sh backups/tjudge_<время>.sql.gz
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
- Матч-контейнер: без сети, корень только на чтение, два бота под случайной парой uid из 20000–59999 без capabilities. На бота — `(EXECUTOR_PIDS_LIMIT - 4) / 2` процессов и потоков (weak 23, medium и prod 48, strong 98), файлы до 32 МБ, общий `/tmp` 64 МБ. При userns-remap на хосте диапазон subuid должен покрывать uid до 60000.
- Seccomp-профиль матчей (`deployments/security/seccomp-executor.json`, allowlist) ни в одном compose не включён. Включение: смонтировать файл в worker и задать `EXECUTOR_SECCOMP_PROFILE=<путь в контейнере>`. Запрещённый профилем syscall засчитывается как ошибка программы, поэтому сначала прогнать матчи на всех 10 языках.

## 13. Профили железа и быстрый self-hosted старт

Профили задают число воркеров, лимиты памяти/CPU на матч и пулы БД/Redis (`config/profiles/{weak,medium,strong}.env`):

| Профиль | CPU | RAM | WORKER_MAX | Память/матч | Для кого |
|---------|-----|-----|------------|-------------|----------|
| weak | 2 | 4 ГБ | 3 | 256 MiB | старый ноут, начальный VPS; турниры до ~50 участников |
| medium | 4 | 8 ГБ | 4 | 512 MiB | обычный сервер; до ~200 участников |
| strong | 8+ | 16+ ГБ | 8 | 1 GiB | выделенный сервер; 500+ участников |

Быстрый старт (миграции применяет сервис `migrate`):

```bash
git clone https://github.com/4RH1T3CT0R7/TJudge.git && cd TJudge
make detect-profile       # рекомендуемый профиль по железу
make deploy               # авто: определит железо, соберёт и поднимет
# либо вручную: make deploy-weak | make deploy-medium | make deploy-strong
S ps
docker exec tjudge-api wget -qO- localhost:8080/health   # OK
```

Без сертификата сайт работает по http на 80. С `DOMAIN=<домен>` в `.env` (DNS указывает на сервер, 80 открыт) certbot выпускает сертификат Let's Encrypt (почта для регистрации — `CERTBOT_EMAIL`, необязательна), и nginx в течение 5 минут сам переходит на https с редиректом с 80. Продление — раз в 12 часов, тоже само. После перехода — `BASE_URL=https://<домен>` в `.env` и `S up -d api`, иначе CORS и WebSocket ждут http-origin.

`make deploy*` вызывают `scripts/quick-deploy.sh <profile>`. Он один раз дописывает в `.env` `HOST_PROGRAMS_PATH` (абсолютный путь к `data/programs`) и `DOCKER_GID` (группа `docker.sock`), ручные команды `S` берут их оттуда. Смена профиля — `S down` и заново нужным `make deploy-*`.
