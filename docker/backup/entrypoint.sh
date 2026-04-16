#!/bin/bash
# P1.14: entrypoint backup-контейнера. Раз в BACKUP_INTERVAL_SECONDS
# вызывает pg_dump через PGHOST/PGUSER/PGPASSWORD.
# При падении дампа — ждёт следующей итерации (не падаем, но alert в лог).

set -euo pipefail

: "${PGHOST:?PGHOST is required}"
: "${PGUSER:?PGUSER is required}"
: "${PGDATABASE:?PGDATABASE is required}"

# Поддержка Docker secrets: если задан PGPASSWORD_FILE — читаем пароль из него.
# Сам PGPASSWORD экспортируется для libpq.
if [[ -n "${PGPASSWORD_FILE:-}" && -r "${PGPASSWORD_FILE}" ]]; then
    export PGPASSWORD
    PGPASSWORD=$(< "${PGPASSWORD_FILE}")
fi

BACKUP_DIR="${BACKUP_DIR:-/backups}"
BACKUP_INTERVAL_SECONDS="${BACKUP_INTERVAL_SECONDS:-86400}"  # 24h
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"

log() {
    echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"
}

mkdir -p "$BACKUP_DIR"

while true; do
    TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
    OUT="$BACKUP_DIR/tjudge_${TIMESTAMP}.sql.gz"

    log "Starting pg_dump -> ${OUT}"
    if pg_dump --no-owner --no-acl "$PGDATABASE" 2>/tmp/pgdump.err | gzip > "$OUT"; then
        SIZE=$(stat -c '%s' "$OUT" 2>/dev/null || stat -f '%z' "$OUT")
        if [[ -s "$OUT" ]]; then
            log "Backup OK (${SIZE} bytes): ${OUT}"
        else
            log "ERROR empty backup, removing ${OUT}"
            rm -f "$OUT"
        fi
    else
        log "ERROR pg_dump failed: $(cat /tmp/pgdump.err 2>/dev/null || true)"
        rm -f "$OUT"
    fi

    # Retention: удаляем файлы старше RETENTION_DAYS дней.
    log "Retention cleanup (>$RETENTION_DAYS days)"
    find "$BACKUP_DIR" -name 'tjudge_*.sql.gz' -mtime +"$RETENTION_DAYS" -type f -delete -print | while read -r f; do
        log "  removed: $f"
    done

    log "Sleeping ${BACKUP_INTERVAL_SECONDS}s until next backup"
    sleep "$BACKUP_INTERVAL_SECONDS"
done
