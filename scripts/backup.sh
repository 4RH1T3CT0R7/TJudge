#!/bin/bash
set -euo pipefail

# TJudge: ручной бэкап - дамп БД и архив каталога программ с общей меткой
# времени (restore.sh находит пару по ней). Файлы программ принадлежат
# uid 1000, запускать от него или от root.
# Usage: ./scripts/backup.sh [backup_dir]
#   POSTGRES_CONTAINER=tjudge-postgres-prod для prod, PROGRAMS_DIR - каталог программ

BACKUP_DIR="${1:-${BACKUP_DIR:-./backups}}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-tjudge-postgres}"
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

mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/tjudge_${TIMESTAMP}.sql.gz"
PROGRAMS_FILE="$BACKUP_DIR/programs_${TIMESTAMP}.tar.gz"

log_info "Starting database backup..."
log_info "Container: $POSTGRES_CONTAINER"
log_info "Database: $DB_NAME"
log_info "Output: $BACKUP_FILE"

if ! docker ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
    log_error "Container '$POSTGRES_CONTAINER' is not running!"
    log_info "Available containers:"
    docker ps --format '  {{.Names}}'
    exit 1
fi

log_info "Dumping database..."
if docker exec "$POSTGRES_CONTAINER" pg_dump --no-owner --no-acl -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_FILE" \
    && [ -s "$BACKUP_FILE" ]; then
    log_info "Backup completed successfully: $BACKUP_FILE ($(du -h "$BACKUP_FILE" | cut -f1))"
else
    log_error "Backup failed!"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# build/ - временные каталоги сборки
log_info "Archiving programs: $PROGRAMS_DIR"
if tar -czf "$PROGRAMS_FILE" -C "$(dirname "$PROGRAMS_DIR")" --exclude="$(basename "$PROGRAMS_DIR")/build" "$(basename "$PROGRAMS_DIR")"; then
    log_info "Programs archived: $PROGRAMS_FILE ($(du -h "$PROGRAMS_FILE" | cut -f1))"
else
    log_error "Programs archive failed (права на $PROGRAMS_DIR?)"
    rm -f "$PROGRAMS_FILE"
    exit 1
fi

# Off-site копия в Telegram (TELEGRAM_BOT_TOKEN + TELEGRAM_CHAT_ID, env или .env).
# Лимит Bot API на документ - 50MB, больший файл остаётся только локально.
send_to_telegram() {
    local file="$1"

    if [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ]; then
        log_warn "TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID не заданы - off-site копия $file не отправлена"
        return 0
    fi

    local size_bytes
    size_bytes=$(wc -c < "$file")
    if [ "$size_bytes" -gt 50000000 ]; then
        log_warn "$file больше 50MB (лимит Telegram Bot API) - off-site копия не отправлена."
        return 0
    fi

    log_info "Отправка $file в Telegram..."
    local response
    if response=$(curl -sS --max-time 120 \
        -F "chat_id=${TELEGRAM_CHAT_ID}" \
        -F "document=@${file}" \
        -F "caption=TJudge backup $(date '+%Y-%m-%d %H:%M:%S') ($(du -h "$file" | cut -f1 | tr -d ' '))" \
        "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendDocument"); then
        if echo "$response" | grep -q '"ok":true'; then
            log_info "Отправлено в Telegram: $file"
        else
            log_error "Telegram отклонил документ: $(echo "$response" | head -c 300)"
        fi
    else
        log_error "Не удалось отправить $file в Telegram (сеть/таймаут)"
    fi
}

send_to_telegram "$BACKUP_FILE"
send_to_telegram "$PROGRAMS_FILE"

log_info "Cleaning up backups older than ${RETENTION_DAYS} days..."
DELETED=$(find "$BACKUP_DIR" -maxdepth 1 \( -name "tjudge_*.sql.gz" -o -name "programs_*.tar.gz" \) -mtime +"$RETENTION_DAYS" -delete -print | wc -l)
if [ "$DELETED" -gt 0 ]; then
    log_info "Deleted $DELETED old backup file(s)"
fi

log_info "Backup process completed."
