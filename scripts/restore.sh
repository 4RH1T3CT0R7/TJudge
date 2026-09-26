#!/bin/bash
set -euo pipefail

# TJudge: восстановление БД и каталога программ из бэкапа.
# Usage: ./scripts/restore.sh <tjudge_<время>.sql.gz> [programs_<время>.tar.gz]
#   Архив программ по умолчанию ищется рядом с дампом по той же метке времени.
#   prod: POSTGRES_CONTAINER=tjudge-postgres-prod
#   PROGRAMS_DIR - каталог программ на хосте (HOST_PROGRAMS_PATH из .env, иначе
#   ./data/programs). Запускать от root или uid 1000: файлы программ принадлежат ему.
#
# Дамп заливается одной транзакцией с ON_ERROR_STOP: при любой ошибке база
# пересоздаётся из страховочного дампа текущего состояния, скрипт падает.

BACKUP_FILE="${1:-}"
PROGRAMS_FILE="${2:-}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-tjudge-postgres}"
if [ -z "${PROGRAMS_DIR:-}" ] && [ -f .env ]; then
    PROGRAMS_DIR=$(sed -n 's/^HOST_PROGRAMS_PATH=//p' .env | tail -1)
fi
PROGRAMS_DIR="${PROGRAMS_DIR:-./data/programs}"
DB_NAME="${DB_NAME:-tjudge}"
DB_USER="${DB_USER:-tjudge}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1"
}

log_error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1"
}

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file.sql.gz> [programs_file.tar.gz]"
    echo ""
    echo "Available backups:"
    if ls "$BACKUP_DIR"/tjudge_*.sql.gz 1>/dev/null 2>&1; then
        ls -lh "$BACKUP_DIR"/tjudge_*.sql.gz "$BACKUP_DIR"/programs_*.tar.gz 2>/dev/null || true
    else
        echo "  No backups found in $BACKUP_DIR"
    fi
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    log_error "Backup file not found: $BACKUP_FILE"
    exit 1
fi

if [ -z "$PROGRAMS_FILE" ]; then
    candidate="$(dirname "$BACKUP_FILE")/$(basename "$BACKUP_FILE" .sql.gz | sed 's/^tjudge_/programs_/').tar.gz"
    [ -f "$candidate" ] && PROGRAMS_FILE="$candidate"
fi
if [ -n "$PROGRAMS_FILE" ] && [ ! -f "$PROGRAMS_FILE" ]; then
    log_error "Programs archive not found: $PROGRAMS_FILE"
    exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
    log_error "Container '$POSTGRES_CONTAINER' is not running!"
    exit 1
fi

# архив проверяется до того, как что-то удалено
if [ -n "$PROGRAMS_FILE" ]; then
    gzip -t "$PROGRAMS_FILE" || { log_error "Programs archive is corrupted: $PROGRAMS_FILE"; exit 1; }
fi
gzip -t "$BACKUP_FILE" || { log_error "Backup file is corrupted: $BACKUP_FILE"; exit 1; }

echo ""
log_warn "This will REPLACE ALL DATA in database '$DB_NAME'!"
log_warn "Backup file: $BACKUP_FILE"
if [ -n "$PROGRAMS_FILE" ]; then
    log_warn "Programs:    $PROGRAMS_FILE -> $PROGRAMS_DIR (текущий каталог сохраняется рядом)"
else
    log_warn "Архива программ нет: восстановится только БД, файлы программ останутся текущими"
fi
echo ""
read -r -p "Are you sure you want to continue? (type 'yes' to confirm): " confirm

if [ "$confirm" != "yes" ]; then
    log_info "Restore cancelled."
    exit 0
fi

psql_admin() {
    docker exec "$POSTGRES_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d postgres -c "$1"
}

recreate_db() {
    psql_admin "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();" >/dev/null
    psql_admin "DROP DATABASE IF EXISTS \"${DB_NAME}\";"
    psql_admin "CREATE DATABASE \"${DB_NAME}\";"
}

load_dump() {
    gunzip -c "$1" | docker exec -i "$POSTGRES_CONTAINER" \
        psql -q -v ON_ERROR_STOP=1 --single-transaction -U "$DB_USER" -d "$DB_NAME" >/dev/null
}

STAMP=$(date +%Y%m%d_%H%M%S)
SAFETY_BACKUP="$BACKUP_DIR/tjudge_pre_restore_${STAMP}.sql.gz"
mkdir -p "$BACKUP_DIR"
log_info "Creating safety backup of current database..."
if ! docker exec "$POSTGRES_CONTAINER" pg_dump --no-owner --no-acl -U "$DB_USER" "$DB_NAME" | gzip > "$SAFETY_BACKUP"; then
    log_error "Safety backup failed, restore aborted"
    rm -f "$SAFETY_BACKUP"
    exit 1
fi
log_info "Safety backup created: $SAFETY_BACKUP"

# api, worker и backup того же compose-проекта, что и postgres, ищутся по
# меткам контейнеров: compose-файл зависит от COMPOSE_FILE, а sudo его сбрасывает.
# бэкап-контейнер монтирует тот же каталог программ: без перезапуска он так
# и архивировал бы переименованный старый каталог
project=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$POSTGRES_CONTAINER")
services=()
for svc in api worker backup; do
    while read -r name; do
        services+=("$name")
    done < <(docker ps --format '{{.Names}}' -f "label=com.docker.compose.project=$project" -f "label=com.docker.compose.service=$svc")
done
if [ ${#services[@]} -gt 0 ]; then
    log_info "Stopping ${services[*]}..."
    docker stop "${services[@]}" >/dev/null
fi

log_info "Restoring database from: $BACKUP_FILE"
recreate_db
if ! load_dump "$BACKUP_FILE"; then
    log_error "Restore failed, database is being returned to the safety backup"
    recreate_db
    if load_dump "$SAFETY_BACKUP"; then
        log_error "Database returned to its state before restore. Остановлены: docker start ${services[*]:-}"
    else
        log_error "Safety backup did not load either: $SAFETY_BACKUP. Остановлены: ${services[*]:-}"
    fi
    exit 1
fi
log_info "Database restored."

if [ -n "$PROGRAMS_FILE" ]; then
    PREVIOUS="${PROGRAMS_DIR%/}.pre_restore_${STAMP}"
    log_info "Restoring programs: $PROGRAMS_FILE (текущие -> $PREVIOUS)"
    mv "$PROGRAMS_DIR" "$PREVIOUS"
    # верхний каталог архива назван по каталогу на момент бэкапа (в контейнере
    # бэкапа - programs), поэтому он отбрасывается
    if ! { mkdir "$PROGRAMS_DIR" && tar -xzf "$PROGRAMS_FILE" -C "$PROGRAMS_DIR" --strip-components=1; }; then
        log_error "Programs restore failed, возвращается прежний каталог; БД уже восстановлена из $BACKUP_FILE"
        rm -rf "$PROGRAMS_DIR"
        mv "$PREVIOUS" "$PROGRAMS_DIR"
        exit 1
    fi
    # от root каталог создаётся с владельцем root, а api пишет в него от uid 1000
    if [ "$(id -u)" -eq 0 ]; then
        chown 1000:1000 "$PROGRAMS_DIR"
    fi
    log_info "Programs restored."
fi

if [ ${#services[@]} -gt 0 ]; then
    log_info "Starting ${services[*]}..."
    docker start "${services[@]}" >/dev/null
fi

log_info "Restore process completed successfully!"
echo ""
log_info "После проверки можно удалить страховочные копии:"
echo "  rm $SAFETY_BACKUP"
[ -n "$PROGRAMS_FILE" ] && echo "  rm -rf ${PROGRAMS_DIR%/}.pre_restore_${STAMP}"
exit 0
