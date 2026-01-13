# TJudge Self-Hosted Deployment Guide

Руководство по развёртыванию TJudge на собственном сервере.

## Содержание

- [Требования](#требования)
- [Быстрый старт](#быстрый-старт)
- [Профили производительности](#профили-производительности)
- [Конфигурация](#конфигурация)
- [Бэкапы и восстановление](#бэкапы-и-восстановление)
- [Мониторинг](#мониторинг)
- [Устранение проблем](#устранение-проблем)
- [Обновление](#обновление)

---

## Требования

### Минимальные требования (профиль weak)

| Ресурс | Минимум | Рекомендуется |
|--------|---------|---------------|
| CPU | 2 ядра | 4 ядра |
| RAM | 4 ГБ | 8 ГБ |
| Диск | 20 ГБ | 50 ГБ |
| ОС | Linux, macOS | Ubuntu 22.04+ |

### Необходимое ПО

- **Docker** 24.0+ и **Docker Compose** v2
- **Git** для клонирования репозитория
- **curl** или **wget** для тестирования

### Проверка Docker

```bash
docker --version
docker compose version
```

---

## Быстрый старт

### 1. Клонирование репозитория

```bash
git clone https://github.com/bmstu-itstech/tjudge.git
cd tjudge
```

### 2. Автоматическое развёртывание

```bash
# Автоопределение профиля по характеристикам сервера
make deploy
```

Скрипт автоматически:
- Определит характеристики вашего железа
- Выберет оптимальный профиль
- Инициализирует секреты
- Соберёт и запустит все сервисы

### 3. Ручной выбор профиля

```bash
# Для слабого железа (2 ядра, 4 ГБ RAM)
make deploy-weak

# Для среднего железа (4 ядра, 8 ГБ RAM)
make deploy-medium

# Для мощного железа (8+ ядер, 16+ ГБ RAM)
make deploy-strong
```

### 4. Проверка

```bash
# Проверить статус сервисов
docker compose -f docker-compose.selfhosted.yml ps

# Проверить API
curl http://localhost:8080/health
```

---

## Профили производительности

### Обзор профилей

| Профиль | CPU | RAM | Воркеры | Память/матч | Описание |
|---------|-----|-----|---------|-------------|----------|
| **weak** | 2 | 4 ГБ | 1-3 | 256 MiB | Старый ноутбук, VPS начального уровня |
| **medium** | 4 | 8 ГБ | 2-5 | 512 MiB | Обычный сервер, рабочая станция |
| **strong** | 8+ | 16+ ГБ | 5-20 | 1 GiB | Выделенный сервер, production |

### Профиль weak (слабое железо)

**Конфигурация** `config/profiles/weak.env`:
```bash
WORKER_MIN=1
WORKER_MAX=3
EXECUTOR_MEMORY_LIMIT=268435456    # 256 MiB
EXECUTOR_CPU_QUOTA=50000           # 50% CPU
DB_MAX_CONNECTIONS=20
REDIS_POOL_SIZE=30
```

**Ограничения:**
- До 3 параллельных матчей
- Матчи выполняются медленнее
- Подходит для турниров до 50 участников

### Профиль medium (среднее железо)

**Конфигурация** `config/profiles/medium.env`:
```bash
WORKER_MIN=2
WORKER_MAX=5
EXECUTOR_MEMORY_LIMIT=536870912    # 512 MiB
EXECUTOR_CPU_QUOTA=100000          # 100% CPU
DB_MAX_CONNECTIONS=50
REDIS_POOL_SIZE=50
```

**Возможности:**
- До 5 параллельных матчей
- Стандартная скорость выполнения
- Подходит для турниров до 200 участников

### Профиль strong (мощное железо)

**Конфигурация** `config/profiles/strong.env`:
```bash
WORKER_MIN=5
WORKER_MAX=20
EXECUTOR_MEMORY_LIMIT=1073741824   # 1 GiB
EXECUTOR_CPU_QUOTA=200000          # 200% CPU
DB_MAX_CONNECTIONS=100
REDIS_POOL_SIZE=100
```

**Возможности:**
- До 20 параллельных матчей
- Быстрое выполнение сложных программ
- Подходит для крупных турниров 500+ участников

### Определение профиля

```bash
# Показать рекомендуемый профиль
make detect-profile
```

---

## Конфигурация

### Переменные окружения

Скопируйте и отредактируйте файл `.env`:

```bash
cp .env.example .env
# Отредактируйте .env
```

**Критичные переменные:**

| Переменная | Описание | Пример |
|------------|----------|--------|
| `DB_PASSWORD` | Пароль PostgreSQL | `your-secure-password` |
| `JWT_SECRET` | Секрет для JWT (мин. 32 символа) | `your-jwt-secret-min-32-chars` |
| `BASE_URL` | Публичный URL приложения | `https://tjudge.example.com` |

### Секреты

При первом запуске секреты генерируются автоматически в директории `secrets/`.

Для ручной генерации:
```bash
./scripts/init-secrets.sh
```

### Изменение профиля после развёртывания

```bash
# Остановить текущие сервисы
docker compose -f docker-compose.selfhosted.yml down

# Развернуть с новым профилем
make deploy-medium
```

---

## Бэкапы и восстановление

### Автоматические бэкапы

Добавьте в crontab для ежедневных бэкапов:

```bash
# Редактировать crontab
crontab -e

# Добавить строку (бэкап в 2:00 ночи)
0 2 * * * /path/to/tjudge/scripts/backup.sh >> /var/log/tjudge-backup.log 2>&1
```

### Ручной бэкап

```bash
# Создать бэкап
make backup

# Посмотреть список бэкапов
make backup-list
```

Бэкапы сохраняются в `./backups/` с именем `tjudge_YYYYMMDD_HHMMSS.sql.gz`.

### Восстановление

```bash
# Показать доступные бэкапы
make backup-list

# Восстановить из бэкапа
make restore BACKUP=backups/tjudge_20240115_020000.sql.gz
```

**Важно:** Восстановление остановит API и Worker, создаст safety backup текущих данных, затем восстановит из указанного файла.

### Хранение бэкапов

По умолчанию бэкапы хранятся 7 дней. Для изменения:

```bash
# В .env или профиле
BACKUP_RETENTION_DAYS=14
```

Рекомендуется также копировать бэкапы на внешнее хранилище (S3, Google Drive, etc.).

---

## Мониторинг

### Встроенный мониторинг

По умолчанию доступны:

| Сервис | URL | Описание |
|--------|-----|----------|
| API Health | `http://localhost:8080/health` | Статус API |
| Prometheus | `http://localhost:9092` | Метрики |
| Grafana | `http://localhost:3000` | Дашборды |

**Логин в Grafana:** admin / admin (смените после первого входа!)

### Включение мониторинга

Мониторинг запускается отдельным профилем:

```bash
docker compose -f docker-compose.selfhosted.yml --profile monitoring up -d
```

### Метрики API

```bash
curl http://localhost:9090/metrics
```

Ключевые метрики:
- `http_requests_total` — количество запросов
- `http_request_duration_seconds` — время ответа
- `matches_processed_total` — обработано матчей
- `worker_pool_size` — размер пула воркеров

### Логи

```bash
# Все логи
docker compose -f docker-compose.selfhosted.yml logs -f

# Только API
docker compose -f docker-compose.selfhosted.yml logs -f api

# Только Worker
docker compose -f docker-compose.selfhosted.yml logs -f worker
```

---

## Устранение проблем

### Сервис не запускается

```bash
# Проверить логи
docker compose -f docker-compose.selfhosted.yml logs api

# Проверить статус контейнеров
docker compose -f docker-compose.selfhosted.yml ps
```

### Out of Memory (OOM)

**Симптомы:** Контейнеры убиваются, в логах `OOM killed`.

**Решение:** Переключитесь на более слабый профиль:

```bash
docker compose -f docker-compose.selfhosted.yml down
make deploy-weak
```

Или уменьшите `EXECUTOR_MEMORY_LIMIT` и `WORKER_MAX` в профиле.

### Медленное выполнение матчей

**Симптомы:** Матчи долго висят в очереди.

**Решение:**
1. Увеличьте `WORKER_MAX` (если позволяет железо)
2. Уменьшите `EXECUTOR_MEMORY_LIMIT` для большего параллелизма
3. Проверьте нагрузку: `docker stats`

### Проблемы с базой данных

```bash
# Проверить подключение
docker exec tjudge-postgres psql -U tjudge -d tjudge -c "SELECT 1;"

# Посмотреть активные соединения
docker exec tjudge-postgres psql -U tjudge -d tjudge -c "SELECT count(*) FROM pg_stat_activity;"
```

### Проблемы с Redis

```bash
# Проверить Redis
docker exec tjudge-redis redis-cli ping

# Посмотреть использование памяти
docker exec tjudge-redis redis-cli info memory
```

### Очистка и перезапуск

```bash
# Полная остановка
docker compose -f docker-compose.selfhosted.yml down

# Очистка volumes (УДАЛИТ ВСЕ ДАННЫЕ!)
docker compose -f docker-compose.selfhosted.yml down -v

# Чистый запуск
make deploy
```

---

## Обновление

### Обновление до новой версии

```bash
# Создать бэкап перед обновлением
make backup

# Получить новую версию
git pull origin main

# Пересобрать и перезапустить
docker compose -f docker-compose.selfhosted.yml down
make deploy
```

### Откат на предыдущую версию

```bash
# Вернуться к предыдущему коммиту
git checkout HEAD~1

# Восстановить бэкап если нужно
make restore BACKUP=backups/tjudge_YYYYMMDD_HHMMSS.sql.gz

# Пересобрать
make deploy
```

---

## Полезные команды

```bash
# Статус сервисов
docker compose -f docker-compose.selfhosted.yml ps

# Перезапуск сервиса
docker compose -f docker-compose.selfhosted.yml restart api

# Масштабирование (если нужно больше воркеров)
docker compose -f docker-compose.selfhosted.yml up -d --scale worker=2

# Просмотр ресурсов
docker stats

# Очистка неиспользуемых образов
docker system prune -f
```

---

## Поддержка

При возникновении проблем:

1. Проверьте раздел [Устранение проблем](#устранение-проблем)
2. Посмотрите [Issues](https://github.com/bmstu-itstech/tjudge/issues)
3. Создайте новый Issue с:
   - Версией TJudge (`git rev-parse HEAD`)
   - Профилем (`make detect-profile`)
   - Логами (`docker compose logs`)
