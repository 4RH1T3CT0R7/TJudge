# Руководство по деплою

## Требования

- Docker 20+
- Docker Compose 2+
- PostgreSQL 15+ (или в контейнере)
- Redis 7+ (или в контейнере)

## Разработка

```bash
# Запуск всех сервисов
docker-compose up -d

# Применение миграций
make migrate-up

# Просмотр логов
docker-compose logs -f api worker
```

## Production

### 1. Настройка секретов

```bash
mkdir -p secrets
echo "your-db-password" > secrets/db_password.txt
echo "your-jwt-secret-min-32-chars" > secrets/jwt_secret.txt
echo "your-redis-password" > secrets/redis_password.txt
chmod 600 secrets/*.txt
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
| `WORKER_MIN` | Нет | 2 | Минимум воркеров |
| `WORKER_MAX` | Нет | 10 | Максимум воркеров |
| `LOG_LEVEL` | Нет | info | debug, info, warn, error |
| `RATE_LIMIT_RPM` | Нет | 100 | Запросов в минуту |

### Рекомендации по ресурсам

| Компонент | CPU | Память | Реплики |
|-----------|-----|--------|---------|
| API | 2 ядра | 2GB | 1-3 |
| Worker | 2 ядра | 2GB | 1-5 |
| PostgreSQL | 2 ядра | 4GB | 1 |
| Redis | 1 ядро | 1GB | 1 |

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

## Масштабирование

```bash
# Увеличить количество воркеров
docker-compose up -d --scale worker=5
```

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
