#!/bin/bash
set -euo pipefail

# TJudge: проверка восстановимости бэкапа.
#
# Непротестированный бэкап - это не бэкап: скрипт разворачивает последний
# дамп в одноразовый PostgreSQL-контейнер (одной транзакцией, любая ошибка
# валит проверку), прогоняет smoke-запросы и проверяет целостность архива
# программ с той же меткой времени. Запускается вручную и в nightly.
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
    # shellcheck disable=SC2012 # нужен самый свежий по времени, имена без пробелов
    BACKUP_FILE=$(ls -t "$BACKUP_DIR"/tjudge_[0-9]*.sql.gz 2>/dev/null | head -1 || true)
fi
[ -n "$BACKUP_FILE" ] && [ -f "$BACKUP_FILE" ] || fail "Дамп не найден (BACKUP_DIR=$BACKUP_DIR)"
log "Проверяем дамп: $BACKUP_FILE ($(du -h "$BACKUP_FILE" | cut -f1))"

# 2. Одноразовый PostgreSQL
log "Поднимаем одноразовый PostgreSQL ($PG_IMAGE)..."
docker run -d --name "$TEST_CONTAINER" \
    -e POSTGRES_USER=tjudge -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tjudge \
    "$PG_IMAGE" >/dev/null

# ожидание готовности. проверка именно по tcp (-h 127.0.0.1), не unix-socket:
# во время initdb энтрипоинт поднимает временный сервер (socket-only),
# pg_isready без -h отвечает на него успешно, а затем сервер рестартует -
# psql попадал в окно рестарта и падал с connection refused
for i in $(seq 1 30); do
    if docker exec "$TEST_CONTAINER" pg_isready -h 127.0.0.1 -U tjudge >/dev/null 2>&1; then
        break
    fi
    [ "$i" -eq 30 ] && fail "PostgreSQL не поднялся за 30 секунд"
    sleep 1
done

# 3. Восстановление
log "Восстанавливаем дамп..."
RESTORE_ERR=""
if ! RESTORE_ERR=$(gunzip -c "$BACKUP_FILE" | docker exec -i "$TEST_CONTAINER" psql -U tjudge -d tjudge -q -v ON_ERROR_STOP=1 --single-transaction 2>&1 >/dev/null); then
    fail "psql завершился с ошибкой при восстановлении: $(echo "$RESTORE_ERR" | tail -3)"
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

echo "$RESULT"

# 5. Архив программ той же метки времени
PROGRAMS_FILE="$(dirname "$BACKUP_FILE")/$(basename "$BACKUP_FILE" .sql.gz | sed 's/^tjudge_/programs_/').tar.gz"
[ -f "$PROGRAMS_FILE" ] || fail "Нет архива программ $PROGRAMS_FILE"
LIST=$(tar -tzf "$PROGRAMS_FILE") || fail "Архив программ повреждён: $PROGRAMS_FILE"
log "Архив программ читается: $(grep -vc '/$' <<< "$LIST" || true) файлов"

log "Бэкап восстановим: все ключевые таблицы на месте."
