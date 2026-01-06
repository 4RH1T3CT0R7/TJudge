# Руководство по деплою

## Требования

- Docker 20+
- Docker Compose 2+ (или Kubernetes 1.25+)
- PostgreSQL 15+ (или в контейнере)
- Redis 7+ (или в контейнере)

## Разработка

```bash
# Запуск всех сервисов
make dev

# Или вручную
docker-compose up -d

# Применение миграций
make migrate-up

# Просмотр логов
docker-compose logs -f api worker
```

## Production (Docker Compose)

### 1. Настройка секретов

```bash
mkdir -p secrets
echo "your-db-password" > secrets/db_password.txt
echo "your-jwt-secret-min-32-chars" > secrets/jwt_secret.txt
echo "your-redis-password" > secrets/redis_password.txt
```

### 2. Деплой

```bash
# Загрузка образов
docker-compose -f docker-compose.prod.yml pull

# Запуск сервисов
docker-compose -f docker-compose.prod.yml up -d

# Проверка здоровья
curl http://localhost:8080/health
```

### 3. Blue-Green деплой

```bash
# Деплой новой версии в неактивное окружение
./scripts/blue-green-deploy.sh v1.2.0

# Smoke-тесты
./scripts/smoke-test.sh

# Переключение трафика
./scripts/switch-traffic.sh

# Откат при необходимости
./scripts/rollback.sh
```

## Production (Kubernetes)

### 1. Создание Namespace и Secrets

```bash
kubectl create namespace tjudge

# Создание секретов (замените значения!)
kubectl create secret generic tjudge-secrets \
  --from-literal=DB_PASSWORD=your-password \
  --from-literal=JWT_SECRET=your-jwt-secret \
  --from-literal=REDIS_PASSWORD=your-redis-password \
  -n tjudge
```

### 2. Деплой

```bash
# Применение всех манифестов
kubectl apply -k deployments/kubernetes/

# Или по отдельности
kubectl apply -f deployments/kubernetes/namespace.yaml
kubectl apply -f deployments/kubernetes/configmap.yaml
kubectl apply -f deployments/kubernetes/postgres.yaml
kubectl apply -f deployments/kubernetes/redis.yaml
kubectl apply -f deployments/kubernetes/api.yaml
kubectl apply -f deployments/kubernetes/worker.yaml
kubectl apply -f deployments/kubernetes/ingress.yaml
kubectl apply -f deployments/kubernetes/hpa.yaml
```

### 3. Проверка

```bash
kubectl get pods -n tjudge
kubectl get svc -n tjudge
kubectl logs -f deployment/tjudge-api -n tjudge
```

## Конфигурация

### Переменные окружения

| Переменная | Обязательно | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `DB_HOST` | Да | — | Хост PostgreSQL |
| `DB_PORT` | Нет | 5432 | Порт PostgreSQL |
| `DB_USER` | Да | — | Пользователь БД |
| `DB_PASSWORD` | Да | — | Пароль БД |
| `DB_NAME` | Да | — | Имя БД |
| `REDIS_HOST` | Да | — | Хост Redis |
| `REDIS_PASSWORD` | Нет | — | Пароль Redis |
| `JWT_SECRET` | Да | — | Ключ подписи JWT (мин. 32 символа) |
| `WORKER_MIN` | Нет | 5 | Минимум воркеров |
| `WORKER_MAX` | Нет | 100 | Максимум воркеров |
| `LOG_LEVEL` | Нет | info | debug, info, warn, error |
| `RATE_LIMIT_RPM` | Нет | 100 | Запросов в минуту |

### Рекомендации по ресурсам

| Компонент | CPU | Память | Реплики |
|-----------|-----|--------|---------|
| API | 2 ядра | 2GB | 3+ |
| Worker | 4 ядра | 4GB | 5+ |
| PostgreSQL | 4 ядра | 8GB | 1 (+ реплики) |
| Redis | 2 ядра | 4GB | 1 (+ sentinel) |

## Мониторинг

### Prometheus эндпоинты

- API: `http://api:9090/metrics`
- Worker: `http://worker:9090/metrics`

### Grafana дашборды

Преднастроенные дашборды:
- **Overview**: частота запросов, ошибки, латентность
- **Workers**: размер очереди, пропускная способность, время обработки
- **Database**: соединения, длительность запросов
- **Cache**: hit rate, использование памяти

### Алерты

Настроенные алерты:
- API/Worker недоступен
- Высокий процент ошибок (>5%)
- Высокая латентность (p99 >1с)
- Размер очереди >1000
- Память >80%

## Масштабирование

### Ручное масштабирование

```bash
# Docker Compose
docker-compose up -d --scale worker=10

# Kubernetes
kubectl scale deployment tjudge-worker --replicas=10 -n tjudge
```

### Автомасштабирование (K8s)

HPA настроен для:
- API: 3-20 реплик, целевая загрузка CPU 70%
- Worker: 5-50 реплик, целевая загрузка CPU 60% + размер очереди

## Резервное копирование

### PostgreSQL

```bash
# Бэкап
pg_dump -h localhost -U tjudge tjudge > backup.sql

# Восстановление
psql -h localhost -U tjudge tjudge < backup.sql
```

### Redis

```bash
# Запуск бэкапа
redis-cli BGSAVE

# Копирование RDB файла
cp /data/dump.rdb /backup/
```

## Устранение неполадок

### API не запускается
```bash
# Проверка логов
docker logs tjudge-api
kubectl logs deployment/tjudge-api -n tjudge

# Частые проблемы:
# - Ошибка подключения к БД: проверьте DB_HOST, учётные данные
# - Ошибка подключения к Redis: проверьте REDIS_HOST
# - JWT secret слишком короткий: минимум 32 символа
```

### Воркеры не обрабатывают задачи
```bash
# Проверка размера очереди
redis-cli LLEN queue:high

# Проверка логов воркера
docker logs tjudge-worker

# Частые проблемы:
# - Docker socket не смонтирован
# - Образ исполнителя не загружен
```

### Высокая латентность
```bash
# Проверка метрик
curl http://localhost:9090/metrics | grep duration

# Частые причины:
# - Пул соединений БД исчерпан
# - Память Redis заполнена
# - Недостаточно воркеров
```
