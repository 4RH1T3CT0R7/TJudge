#!/bin/bash
# Entrypoint backup-контейнера. Раз в BACKUP_INTERVAL_SECONDS:
#   - pg_dump в /backups/tjudge_<время>.sql.gz;
#   - каталог программ (/data/programs без build/) в programs_<время>.tar.gz,
#     с той же меткой времени - restore.sh находит пару по ней;
#   - при заданных TELEGRAM_BOT_TOKEN и BACKUP_TELEGRAM_CHAT_ID копия каждого
#     файла до 50 МБ (лимит Bot API) уходит в Telegram. в дампе email и хеши
#     паролей, поэтому чат задаётся отдельно от алертов и только явно;
#   - файлы старше BACKUP_RETENTION_DAYS удаляются.
# Сбой пишется в лог как ERROR, следующая попытка - по расписанию.
# Разовый прогон: docker exec tjudge-backup /entrypoint.sh --once

set -euo pipefail

: "${PGHOST:?PGHOST is required}"
: "${PGUSER:?PGUSER is required}"
: "${PGDATABASE:?PGDATABASE is required}"

# Docker secrets: пароль из PGPASSWORD_FILE, сам PGPASSWORD читает libpq
if [[ -n "${PGPASSWORD_FILE:-}" && -r "${PGPASSWORD_FILE}" ]]; then
    export PGPASSWORD
    PGPASSWORD=$(< "${PGPASSWORD_FILE}")
fi

BACKUP_DIR="${BACKUP_DIR:-/backups}"
BACKUP_INTERVAL_SECONDS="${BACKUP_INTERVAL_SECONDS:-86400}"  # 24h
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
TELEGRAM_MAX_BYTES=50000000

log() {
    echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"
}

send_to_telegram() {
    local file=$1 size
    if [[ -z "${TELEGRAM_BOT_TOKEN:-}" || -z "${BACKUP_TELEGRAM_CHAT_ID:-}" ]]; then
        return 0
    fi
    size=$(stat -c '%s' "$file")
    if (( size > TELEGRAM_MAX_BYTES )); then
        log "WARN ${file} больше 50 МБ, в Telegram не отправлен: копия только на хосте"
        return 0
    fi
    if curl -sS --max-time 300 -F "chat_id=${BACKUP_TELEGRAM_CHAT_ID}" -F "document=@${file}" \
        "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendDocument" | grep -q '"ok":true'; then
        log "Telegram copy OK: ${file}"
    else
        log "ERROR Telegram copy failed: ${file}"
    fi
}

backup_once() {
    local ts dump programs
    ts=$(date -u +%Y%m%dT%H%M%SZ)
    dump="$BACKUP_DIR/tjudge_${ts}.sql.gz"
    programs="$BACKUP_DIR/programs_${ts}.tar.gz"

    log "Starting pg_dump -> ${dump}"
    if pg_dump --no-owner --no-acl "$PGDATABASE" 2>/tmp/pgdump.err | gzip > "$dump" && [[ -s "$dump" ]]; then
        log "Backup OK ($(stat -c '%s' "$dump") bytes): ${dump}"
        send_to_telegram "$dump"
    else
        log "ERROR pg_dump failed: $(cat /tmp/pgdump.err 2>/dev/null || true)"
        rm -f "$dump"
    fi

    if [[ ! -d /data/programs ]]; then
        log "ERROR /data/programs не смонтирован, программы не сохранены"
        return 0
    fi
    # build/ - временные каталоги сборки
    if tar -czf "$programs" -C /data --exclude=programs/build programs; then
        log "Programs OK ($(stat -c '%s' "$programs") bytes): ${programs}"
        send_to_telegram "$programs"
    else
        log "ERROR programs archive failed"
        rm -f "$programs"
    fi
}

cleanup_old() {
    log "Retention cleanup (>${RETENTION_DAYS} days)"
    find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'tjudge_*.sql.gz' -o -name 'programs_*.tar.gz' \) \
        -mtime +"$RETENTION_DAYS" -delete -print | while read -r f; do
        log "  removed: $f"
    done
}

mkdir -p "$BACKUP_DIR"

if [[ "${1:-}" == "--once" ]]; then
    backup_once
    cleanup_old
    exit 0
fi

trap 'exit 0' TERM INT
while true; do
    backup_once
    cleanup_old
    log "Sleeping ${BACKUP_INTERVAL_SECONDS}s until next backup"
    sleep "$BACKUP_INTERVAL_SECONDS" &
    wait $!
done
