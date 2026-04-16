# TJudge — Production Deployment

Развёртывание на self-hosted сервере (single-node). Для кластерного деплоя см. `deployments/k8s/` (experimental).

## 1. Предварительные требования

- Linux x86_64, Docker 24+, Docker Compose v2.
- 4 GB RAM, 20 GB диска (под БД и backup-ы).
- Домен с DNS A-записью → IP сервера.
- Открытые порты: 80 и 443.

## 2. Первичная настройка

```bash
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge

# Генерация secrets (заполните реальные значения):
./scripts/init-secrets.sh      # создаст ./secrets/{db_password,redis_password,jwt_secret}
chmod 600 secrets/*

# Конфиг
cp .env.production.example .env.production   # если пример есть; иначе скопируйте .env.example
```

Обязательно заполнить в `.env.production`:

```ini
ENVIRONMENT=production
BASE_URL=https://tjudge.example.com
JWT_SECRET=<случайные 48+ байт; openssl rand -hex 48>
DB_PASSWORD=<crypto-random>
REDIS_PASSWORD=<crypto-random>
CORS_ALLOWED_ORIGINS=https://tjudge.example.com
WEBSOCKET_ALLOWED_ORIGINS=https://tjudge.example.com
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=noreply@example.com
SMTP_PASSWORD=<smtp-password-or-app-key>
SMTP_FROM="TJudge <noreply@example.com>"
SMTP_USE_TLS=false    # true для implicit TLS (порт 465)
RATE_LIMIT_ENABLED=true
```

**Критично:** JWT_SECRET не должен содержать плейсхолдеры (`CHANGE_ME`, `secret`, `password` и т.п.) — код упадёт при boot'е в prod (P0.6).

## 3. Первый запуск

```bash
# TLS: получаем сертификат Let's Encrypt.
./scripts/init-ssl.sh tjudge.example.com admin@example.com

# Собираем и поднимаем.
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# Миграции применятся автоматически (сервис migrate).

# Проверка
curl https://tjudge.example.com/health    # → OK

# Назначить первого админа
docker exec -it tjudge-api ./tjudge-admin promote admin@example.com
```

## 4. Backup (включить сразу)

```bash
docker compose -f docker-compose.prod.yml --profile backup up -d backup
```

Проверка: `docker logs tjudge-backup` через сутки. Восстановление см. [RUNBOOK §3.2](RUNBOOK.md#32-восстановление-из-backup).

## 5. Observability (опционально)

```bash
docker compose -f docker-compose.prod.yml --profile monitoring up -d prometheus grafana loki alertmanager
```

Grafana на `https://tjudge.example.com:3000` (или за reverse-proxy'ей). Default credentials в `.env.production`:
```
GF_ADMIN_USER=admin
GF_ADMIN_PASSWORD=<replace>
```

## 6. Security-checklist перед открытием на публику

- [ ] `JWT_SECRET` — ≥32 байт, не из `CHANGE_ME` blacklist (автопроверка при boot'е).
- [ ] `.env.production` не в git: `git ls-files .env.production` возвращает пусто.
- [ ] TLS настроен (нет HTTP-трафика; HSTS выставлен — P0.3).
- [ ] CORS `AllowedOrigins` — только ваши домены (без `*`).
- [ ] `WEBSOCKET_ALLOWED_ORIGINS` задан, fail-closed в prod (P0.2).
- [ ] Worker запускается **не** от root (`docker compose ps worker → user=1000`, P0.8).
- [ ] `RATE_LIMIT_ENABLED=true`.
- [ ] Backup-service запущен (`docker ps | grep tjudge-backup`).
- [ ] SMTP настроен ИЛИ LogMailer разрешён (понимая что пользователи не смогут получить reset-letter'ы).
- [ ] `gosec`, `npm audit`, `Trivy` в CI — все HIGH blocker'ы исправлены (P1.9).
- [ ] Первый admin назначен и у него надёжный пароль.

## 7. Обновление

Стандартный путь — blue-green deploy:

```bash
./scripts/blue-green-deploy.sh <new-tag>
./scripts/smoke-test.sh              # проверка готовности нового стека
./scripts/switch-traffic.sh          # переключить nginx-upstream
# Через ~5 минут мониторинга:
./scripts/blue-green-deploy.sh cleanup
```

Rollback: `./scripts/rollback.sh`.

## 8. Scaling vertical (single-node)

- `WORKER_MAX` — максимум 200 при 4 vCPU; не увеличивайте выше `DB_MAX_CONNECTIONS * 1.5`.
- `DB_MAX_CONNECTIONS` — держите в пределах `pg_settings.max_connections - 10` для админки.
- `REDIS_POOL_SIZE` — 50-200 достаточно.

## 9. Горизонтальное масштабирование

API можно запускать в нескольких экземплярах (stateless). Worker тоже multiple-instance safe (через Redis distributed lock). Для этого перейдите на K8s — см. `deployments/k8s/` (требует доработки).
