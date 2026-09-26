# TJudge - эксплуатация

Runbook для self-hosted single-node: деплой, диагностика, восстановление. Быстрый старт с авто-профилем железа — §13.

## 1. Требования

Linux x86_64, Docker 24+, Docker Compose v2. 4 ГБ RAM, 20 ГБ диска (БД + бэкапы). Домен с A-записью на IP сервера, открытые порты 80 и 443.

## 2. Первичная настройка

```bash
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
./scripts/init-secrets.sh      # создаст ./secrets/{db_password,redis_password,jwt_secret}
chmod 600 secrets/*
cp .env.production.example .env.production
```

Минимум для `.env.production`:

```ini
ENVIRONMENT=production
BASE_URL=https://tjudge.example.com
JWT_SECRET=<случайные 48+ байт; openssl rand -hex 48>
DB_PASSWORD=<crypto-random>
REDIS_PASSWORD=<crypto-random>
CORS_ALLOWED_ORIGINS=https://tjudge.example.com
WEBSOCKET_ALLOWED_ORIGINS=https://tjudge.example.com
RATE_LIMIT_ENABLED=true
```

`JWT_SECRET` с плейсхолдером (`CHANGE_ME`, `secret`, `password` и т.п.) уронит prod при старте — проверка вшита в код.

## 3. Первый запуск

```bash
./scripts/init-ssl.sh tjudge.example.com admin@example.com   # tls let's encrypt
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d              # миграции применит сервис migrate
curl https://tjudge.example.com/health                       # "OK"
docker exec -it tjudge-api ./tjudge-admin promote admin@example.com   # первый админ
```

## 4. Резервное копирование

Включение: `docker compose -f docker-compose.prod.yml --profile backup up -d backup`. Раз в сутки в volume `backups_data` (prod) или `./backups` (self-hosted), retention 30 дней. Ручной запуск: `./scripts/backup.sh ./backups`. Проверка через сутки: `docker logs tjudge-backup`.

## 5. Мониторинг

Два режима (`MONITORING_MODE` в `.env`, по умолчанию `standalone`). api/worker из prod-compose подключены к внешней docker-сети `monitoring` с лейблами `prometheus_job: tjudge-api / tjudge-worker` — оба режима скрейпят по этой сети, job'ы и дашборды идентичны. Оба стека одновременно не поднимать — порты конфликтуют.

- **standalone** — автономный стек TJudge: `make monitoring-up` (= `docker compose -f docker-compose.monitoring.yml up -d`).
- **external** — общий стек infra-monitoring (Prometheus+Grafana+Loki+Alertmanager+Pushgateway на той же сети, авто-дискавери по docker-лейблам), TJudge ничего не поднимает: в `.env` `MONITORING_MODE=external`, стек — `cd ~/infra-monitoring && docker compose up -d`.

Порты одинаковые в обоих режимах: Prometheus 9092 (алерты из `deployments/prometheus/alerts/`), Alertmanager 9093 (Telegram: токен в `deployments/alertmanager/telegram_token`, chat_id в `alertmanager.yml`), Pushgateway 9094 (метрики doctor'а), Grafana 3000 (дашборды «TJudge — Обзор системы» и «TJudge — Doctor» провижнятся сами; учётка — `GF_ADMIN_USER`/`GF_ADMIN_PASSWORD` в `.env.production`). Если `make doctor` пишет «Prometheus НЕ скрейпит API» — почти всегда старый Prometheus в другой docker-сети не резолвит имена `api`/`worker`: остановить его и поднять стек как выше.

**Guardian (только external).** В infra-monitoring входит guardian — авто-восстановление контейнеров. Лейблы в `docker-compose.prod.yml` уже стоят: nginx/api/worker — `guardian.enabled` (рестарт при unhealthy), postgres/redis — `notify-only` (БД не рестартим: WAL-replay удлинит восстановление). Перед долгими ручными работами: `cd ~/infra-monitoring && make maintenance 1h`. Детали — README infra-monitoring.

## 6. Чеклист перед открытием на публику

- [ ] `JWT_SECRET` не короче 32 байт, не из blacklist-плейсхолдеров (автопроверка при старте).
- [ ] `.env.production` не в git: `git ls-files .env.production` пусто.
- [ ] TLS настроен, HTTP-трафика нет, HSTS выставлен.
- [ ] `CORS_ALLOWED_ORIGINS` — только ваши домены, без `*`.
- [ ] `WEBSOCKET_ALLOWED_ORIGINS` задан (в prod fail-closed).
- [ ] Worker не от root: `docker compose ps worker` показывает `user=1000`.
- [ ] `RATE_LIMIT_ENABLED=true`.
- [ ] Backup-сервис работает: `docker ps | grep tjudge-backup`.
- [ ] `make security` прошёл, HIGH-находки исправлены.
- [ ] Первый админ назначен, пароль надёжный.

## 7. Обновление (blue-green)

Blue/green-стеки миграции не применяют (сервиса `migrate` в них нет, база общая), поэтому первым шагом схема обновляется из `docker-compose.prod.yml`: `migrate` собирается из текущего checkout.

```bash
git fetch --tags && git checkout <new-tag>
docker compose -f docker-compose.prod.yml run --rm --build migrate            # миграции до переключения
docker compose -f docker-compose.prod.yml run --rm migrate ./migrate version  # dirty: false
./scripts/blue-green-deploy.sh <new-tag>
./scripts/smoke-test.sh              # готовность нового стека
./scripts/switch-traffic.sh          # переключение nginx-upstream
# после ~5 минут мониторинга:
./scripts/blue-green-deploy.sh cleanup
```

Откат: `./scripts/rollback.sh`, down-миграции для него не нужны.

Порядок «сначала миграции, потом переключение» безопасен, пока новые миграции совместимы с работающей версией: 000042–000045 добавляют nullable-столбец, меняют инвайт-коды и удаляют индексы-дубли, старый код с ними работает. Без шага миграций новый код после переключения отдаёт 500 на refresh токенов и админских ручках (`column password_changed_at does not exist`).

### 7.1 Очистка диска после релизов

`scripts/deploy.sh` и `scripts/blue-green-deploy.sh` после успешного деплоя вызывают `cleanup_old_images`: удаляет dangling-образы, для каждого `tjudge-{api,worker,executor,migrate,cli}` оставляет N последних тегов (по умолчанию 3, переопределяется `TJUDGE_IMAGE_KEEP`), чистит build-кэш старше 7 дней. Почему не `docker image prune -a`: `tjudge-executor` запускается воркером on-demand (`internal/executor/executor.go:169-186`), между матчами на него нет работающих контейнеров — blanket-prune его удалит и сломает матчи до следующего pull'а из ghcr.io. Tag-based retention заодно сохраняет прошлые версии API/worker для rollback.

Ручная чистка при накопившемся мусоре:

```bash
docker image prune -f        # безопасно: только dangling
docker builder prune -af     # build-кэш
docker system df             # что занимает место
TJUDGE_IMAGE_KEEP=5 ./scripts/blue-green-deploy.sh <version>   # другой retention
```

Не запускать на проде: `docker image prune -af` без фильтров (удалит `tjudge-executor`, следующий матч упадёт); `docker system prune -af --volumes` и `docker volume prune` (при остановленном postgres/redis, например между переключениями blue-green, volume посчитается unused и улетит вместе с БД).

Ротация логов контейнеров — `/etc/docker/daemon.json`:

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "50m", "max-file": "3" }
}
```

После правки `sudo systemctl restart docker`; старые контейнеры подхватят при пересоздании (ближайший деплой). `data/programs/` (`HOST_PROGRAMS_PATH`) — загруженные программы участников, автоматически не чистится: это данные, не мусор.

## 8. Быстрая диагностика

### make status

`make status` на сервере, рядом с docker-compose. Для полного статуса (БД, очереди, матчи, программы, outbox) — в `.env` учётка админа: `ADMIN_USER=<логин или email>`, `ADMIN_PASSWORD=<пароль>`; скрипт сам получает свежий JWT. Лучше отдельная служебная учётка (`make admin EMAIL=...`), чем личный пароль. Альтернатива `ADMIN_TOKEN=<jwt>` протухает за `JWT_ACCESS_TTL` (24ч).

Проверяет: контейнеры, наличие образов `tjudge-cli`/`tjudge-builder`, не крутятся ли api/worker на устаревшем образе (главный ответ на «надо ли пересобирать»), health API/worker и — при наличии токена — полный статус из `GET /api/v1/system/status`: версия сборки и аптайм, здоровье и версия миграций PostgreSQL, Redis, размеры всех очередей (компиляция, dead-letter), матчи и программы по статусам, outbox целостности рейтингов, WebSocket-клиенты. Тот же статус: админ-панель, вкладка «Система» (обновление каждые 10 секунд); Grafana-дашборд «TJudge — Обзор системы» (профиль `monitoring` в docker-compose.selfhosted.yml); сырой JSON — `curl -sH "Authorization: Bearer <admin-jwt>" http://localhost:8080/api/v1/system/status | jq`.

### make doctor — глубокая диагностика после деплоя

```bash
make doctor                         # терминальный отчёт + telegram при проблемах
./scripts/doctor.sh --json          # машиночитаемый вывод
DOCTOR_TELEGRAM=always make doctor  # отчёт в telegram даже когда всё ок
```

Проверяет: контейнеры (включая crash-loop по RestartCount), образы и работу на устаревших образах, health API/worker, глубокий статус (БД+миграции, Redis, dead-letter, outbox, зависшая компиляция — нужны `ADMIN_USER`/`ADMIN_PASSWORD`), Prometheus (up-цели, 5xx против SLO, активные алерты — описания попадают в отчёт), ошибки в логах api/worker за `DOCTOR_LOG_WINDOW` с топом повторов, диск. Каждая проблема — с подсказкой «куда смотреть». Вердикт: HEALTHY / DEGRADED / CRITICAL (exit 1 → деплой неуспешен).

Куда отчитывается: Telegram (`TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`) — вердикт + список проблем; Grafana — дашборд «TJudge — Doctor» через Pushgateway (`PUSHGATEWAY_URL` по умолчанию `http://localhost:9094`; без pushgateway push тихо пропускается); Prometheus-алерты `DoctorCritical`/`DoctorDegraded`/`DoctorStale` (deployments/prometheus/alerts/tjudge-doctor.yml) → Telegram через Alertmanager. Запускается автоматически после деплоя: `scripts/deploy.sh` — staging гейтится по CRITICAL; blue-green — best-effort после переключения трафика. Cron для регулярной проверки: `*/30 * * * * cd /opt/tjudge && ./scripts/doctor.sh >/dev/null 2>&1`.

### Точечные проверки

```bash
curl -sf http://localhost:8080/health            # "OK"
curl -sfH "Authorization: Bearer <admin-jwt>" http://localhost:8080/api/v1/system/health   # json со статусом
curl -s http://localhost:8080/metrics | grep -E '^(tjudge_|go_goroutines)'
docker logs --tail=200 tjudge-api
docker exec -it tjudge-redis redis-cli -n 0 llen queue:high
docker exec -it tjudge-redis redis-cli -n 0 llen queue:dead_letter
```

## 9. Инциденты

### 9.0 Всё упало: полный старт/стоп

После перезагрузки сервера обычно ничего делать не нужно: `restart: always`, docker поднимет всё сам — подождать 2-3 минуты, проверить `make doctor`. Ручной полный старт (порядок важен — TJudge требует внешнюю сеть `monitoring`, её создаёт инфра-стек):

```bash
cd ~/infra-monitoring && make up                                  # 1. мониторинг + сеть
cd ~/TJudge && docker compose -f docker-compose.prod.yml up -d    # 2. tjudge
docker compose -f docker-compose.prod.yml --profile backup up -d backup  # 3. бэкапы
make doctor                                                       # 4. проверка
```

Без мониторинга: достаточно `docker network create monitoring`, дальше шаг 2. Полная остановка (мониторинг первым, чтобы не спамить алертами в Telegram): `cd ~/infra-monitoring && make down`, затем `cd ~/TJudge && docker compose -f docker-compose.prod.yml down`. Никогда не добавлять `-v` к `down` — удалит volumes: базу, Redis, загруженные программы. «Остановить на время» — `docker compose stop` / `start`.

### 9.1 API падает или в perpetual restart

1. `docker logs --tail=300 tjudge-api`. Частые причины: `JWT_SECRET must be at least 32 bytes` — обновить секрет (§11); `database connection refused` — Postgres (§9.3); `panic: send on closed channel` — баг инфраструктуры, заводить issue.
2. `docker compose restart api`.
3. Не помогло — откат: `./scripts/rollback.sh`.

### 9.2 Матчи не разбираются (QueueStuck)

Триггер: воркеры заняты (`sum(tjudge_active_workers) > 0`), а за 10 минут не завершился ни один матч. Большая очередь в начале раунда штатна, размер очереди (`tjudge_queue_size`) не показатель: гейдж залипает.

1. Завершения: `increase(tjudge_matches_total[10m])` в Prometheus — ноль подтверждает зависание.
2. Логи воркера: `docker logs --tail=300 tjudge-worker | grep ERROR`. Частая причина — нет образа `tjudge-cli`; пересобрать: `docker compose build tjudge-cli`.
3. Docker daemon: `docker info` и `docker ps` на хосте отвечают быстро? Зависший daemon держит воркеры занятыми без результата.
4. Матчи идут, но медленно: размер пула `curl -s localhost:9090/metrics | grep tjudge_worker_pool_size`; увеличить `WORKER_MAX` в `.env` и `docker compose up -d worker`.

### 9.3 Postgres недоступен

1. `docker ps | grep postgres` — контейнер запущен?
2. Нет: `docker compose up -d postgres`, ждать healthcheck.
3. Запущен, но недоступен: `docker exec -it tjudge-postgres psql -U tjudge -c 'SELECT 1'`.
4. Диск: `df -h /var/lib/docker`. При переполнении удалить старые backup-ы и партиции.
5. Crash-loop: восстановление из backup (§10).

### 9.4 Redis недоступен

API сам переключается на fallback rate-limiter (0.5× от основного лимита); очередь матчей без Redis не работает. Перезапустить: `docker compose restart redis`. Если данные очередей потеряны — запустить recovery-worker: он переставит pending-матчи в очередь.

### 9.5 WebSocket-шторм подключений

Растёт `tjudge_queue_deadletter_size` — возможно poison-сообщения, проверить deserializer. Клиентский flood: в `client.go` включён per-client rate limit (10 msg/sec), лимитируемые получают close 1008.

### 9.6 Расследование admin-действий

`curl -sH "Authorization: Bearer <admin-jwt>" 'http://localhost:8080/api/v1/admin/audit?limit=500' | jq`, в БД — `SELECT * FROM audit_log ORDER BY created_at DESC LIMIT 200;`.

### 9.7 Миграция в состоянии DIRTY

Упавшая миграция оставляет схему в `dirty`, и каждый следующий `migrate up` падает с `Dirty database version N. Fix and force version.`; api и worker в prod-compose ждут `migrate` и не стартуют. 000045 падает так намеренно: DROP INDEX на партиционированных таблицах ждёт лок не дольше 5 с (`lock_timeout`), и транзакция откатывается целиком. Лок держат pg_dump бэкапа, трафик старого цвета при blue-green и долгие запросы лидербордов.

```bash
docker compose -f docker-compose.prod.yml run --rm migrate ./migrate version   # Current version: 45 (dirty: true)
docker compose -f docker-compose.prod.yml run --rm migrate ./migrate force 44  # 000045 откатилась целиком
# повтор в окно низкой нагрузки, когда не идёт pg_dump бэкапа
docker compose -f docker-compose.prod.yml run --rm migrate
docker compose -f docker-compose.prod.yml up -d
```

Для другой миграции N `force N-1` верен, только если её изменения не применились: файл выполняется одной транзакцией, так что при ошибке это обычно так, но стоит сверить схему.

## 10. Восстановление из backup

```bash
docker compose stop api worker                 # 1. остановить конкурентные записи
LATEST=$(ls -t backups/tjudge_*.sql.gz | head -1)
gunzip -c "$LATEST" | docker exec -i tjudge-postgres psql -U tjudge tjudge   # 2. восстановить бд
docker compose up migrate                      # 3. миграции, если backup старше текущих
docker compose up -d api worker                # 4. запустить обратно
```

Point-in-time recovery не настроен: WAL-archiving выключен. Для prod — `pgbackrest` или `wal-g` с S3-хранилищем.

## 11. Частые задачи

- **Ротация JWT_SECRET:** сгенерировать `openssl rand -hex 48`; обновить secret (Docker secrets или env-переменная); `docker compose up -d api` (rolling при `replicas>1`). Все сессии инвалидируются, пользователи перелогиниваются.
- **Назначение админа:** `make admin EMAIL=foo@bar.com`.
- **Миграции БД:** `make migrate-up` — применить pending; `make migrate-down` — откат последней (только dev); `make migrate-create NAME=add_foo` — шаблон новой.
- **Сброс пароля:** `UPDATE users SET password_hash = '$2a$12$...' WHERE email = 'foo@bar.com';` — bcrypt-hash cost 12, как в auth.Service: `SELECT crypt('newpassword', gen_salt('bf', 12));`.

## 12. Масштабирование и ограничения

- Вертикально: `WORKER_MAX` — максимум 200 при 4 vCPU, не выше `DB_MAX_CONNECTIONS * 1.5`; `DB_MAX_CONNECTIONS` — в пределах `pg_settings.max_connections - 10` (запас на админские сессии); `REDIS_POOL_SIZE` 50-200 достаточно.
- Горизонтально: API stateless, несколько экземпляров ок; worker безопасен в multi-instance через Redis distributed lock.
- Обновление ELO delta-based: параллельные матчи одного участника могут давать snapshot-based deltas; для строгой сериализации нужен advisory lock.
- Docker-in-Docker worker монтирует `docker.sock` read-only с non-root пользователем: на хосте должна существовать docker-group с совпадающим GID.

## 13. Профили железа и быстрый self-hosted старт

Профили задают число воркеров, лимиты памяти/CPU на матч и пулы БД/Redis (`config/profiles/{weak,medium,strong}.env`):

| Профиль | CPU | RAM | WORKER_MAX | Память/матч | Для кого |
|---------|-----|-----|------------|-------------|----------|
| weak | 2 | 4 ГБ | 3 | 256 MiB | старый ноут, начальный VPS; турниры до ~50 участников |
| medium | 4 | 8 ГБ | 5 | 512 MiB | обычный сервер; до ~200 участников |
| strong | 8+ | 16+ ГБ | 20 | 1 GiB | выделенный сервер; 500+ участников |

Быстрый старт (self-hosted compose, миграции применяет сервис `migrate`):

```bash
git clone https://github.com/bmstu-itstech/tjudge.git && cd tjudge
make detect-profile       # рекомендуемый профиль по железу
make deploy               # авто: определит железо, создаст секреты, соберёт и поднимет
# либо вручную: make deploy-weak | make deploy-medium | make deploy-strong
docker compose -f docker-compose.selfhosted.yml ps
curl http://localhost:8080/health                    # "OK"
```

`make deploy*` вызывают `scripts/quick-deploy.sh <profile>`. Смена профиля — `docker compose -f docker-compose.selfhosted.yml down` и заново нужным `make deploy-*`.
