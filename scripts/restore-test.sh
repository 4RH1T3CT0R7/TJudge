#!/bin/bash
set -euo pipefail

# TJudge: проверка восстановимости бэкапа.
#
# Непротестированный бэкап - это не бэкап: скрипт разворачивает последний
# дамп в одноразовый PostgreSQL-контейнер и прогоняет smoke-запросы.
# Запускается вручную или nightly-CI (.github/workflows/nightly.yml).
#
# Usage: ./scripts/restore-test.sh [backup_file]
#   Без аргумента берётся самый свежий дамп из $BACKUP_DIR (./backups).

BACKUP_DIR="${BACKUP_DIR:-./backups}"
BACKUP_FILE="${1:-}"
TEST_CONTAINER="tjudge-restore-test-$$"
PG_IMAGE="${PG_IMAGE:-postgres:15-alpine}"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $1"; }
fail() { echo -e "${RED}[$(date '+%H:%M:%S')] FAIL:${NC} $1"; cleanup; exit 1; }

cleanup() {
    docker rm -f "$TEST_CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# 1. Находим дамп
if [ -z "$BACKUP_FILE" ]; then
    BACKUP_FILE=$(ls -t "$BACKUP_DIR"/tjudge_*.sql.gz 2>/dev/null | head -1 || true)
fi
[ -n "$BACKUP_FILE" ] && [ -f "$BACKUP_FILE" ] || fail "Дамп не найден (BACKUP_DIR=$BACKUP_DIR)"
log "Проверяем дамп: $BACKUP_FILE ($(du -h "$BACKUP_FILE" | cut -f1))"

# 2. Одноразовый PostgreSQL
log "Поднимаем одноразовый PostgreSQL ($PG_IMAGE)..."
docker run -d --name "$TEST_CONTAINER" \
    -e POSTGRES_USER=tjudge -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tjudge \
    "$PG_IMAGE" >/dev/null

# Ждём готовности
for i in $(seq 1 30); do
    if docker exec "$TEST_CONTAINER" pg_isready -U tjudge >/dev/null 2>&1; then
        break
    fi
    [ "$i" -eq 30 ] && fail "PostgreSQL не поднялся за 30 секунд"
    sleep 1
done

# 3. Восстановление
log "Восстанавливаем дамп..."
if ! gunzip -c "$BACKUP_FILE" | docker exec -i "$TEST_CONTAINER" psql -U tjudge -d tjudge -q -v ON_ERROR_STOP=0 >/dev/null 2>&1; then
    fail "psql завершился с ошибкой при восстановлении"
fi

# 4. Smoke-запросы: ключевые таблицы существуют и читаются
log "Прогоняем smoke-запросы..."
SMOKE_SQL="
SELECT 'users', COUNT(*) FROM users;
SELECT 'tournaments', COUNT(*) FROM tournaments;
SELECT 'programs', COUNT(*) FROM programs;
SELECT 'matches', COUNT(*) FROM matches;
SELECT 'rating_history', COUNT(*) FROM rating_history;
"
if ! RESULT=$(docker exec -i "$TEST_CONTAINER" psql -U tjudge -d tjudge -t -v ON_ERROR_STOP=1 <<< "$SMOKE_SQL" 2>&1); then
    fail "Smoke-запросы упали: $RESULT"
fi

echo "$RESULT" | sed 's/^/  /'
log "Бэкап восстановим: все ключевые таблицы на месте."
