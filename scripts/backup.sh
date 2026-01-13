#!/bin/bash
set -euo pipefail

# TJudge Database Backup Script
# Usage: ./scripts/backup.sh [backup_dir]

BACKUP_DIR="${1:-${BACKUP_DIR:-./backups}}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-tjudge-postgres}"
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

# Create backup directory if not exists
mkdir -p "$BACKUP_DIR"

# Generate timestamp for backup file
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/tjudge_${TIMESTAMP}.sql.gz"

log_info "Starting database backup..."
log_info "Container: $POSTGRES_CONTAINER"
log_info "Database: $DB_NAME"
log_info "Output: $BACKUP_FILE"

# Check if container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
    log_error "Container '$POSTGRES_CONTAINER' is not running!"
    log_info "Available containers:"
    docker ps --format '  {{.Names}}'
    exit 1
fi

# Perform backup
log_info "Dumping database..."
if docker exec "$POSTGRES_CONTAINER" pg_dump -U "$DB_USER" "$DB_NAME" 2>/dev/null | gzip > "$BACKUP_FILE"; then
    # Verify backup was created and has content
    if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
        SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
        log_info "Backup completed successfully: $BACKUP_FILE ($SIZE)"
    else
        log_error "Backup file is empty or missing!"
        rm -f "$BACKUP_FILE"
        exit 1
    fi
else
    log_error "Backup failed!"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# Cleanup old backups
log_info "Cleaning up backups older than ${RETENTION_DAYS} days..."
DELETED=$(find "$BACKUP_DIR" -name "tjudge_*.sql.gz" -mtime +"$RETENTION_DAYS" -delete -print | wc -l)
if [ "$DELETED" -gt 0 ]; then
    log_info "Deleted $DELETED old backup(s)"
fi

# List current backups
log_info "Current backups:"
if ls "$BACKUP_DIR"/tjudge_*.sql.gz 1>/dev/null 2>&1; then
    ls -lh "$BACKUP_DIR"/tjudge_*.sql.gz | awk '{print "  " $9 " (" $5 ")"}'
    TOTAL=$(ls "$BACKUP_DIR"/tjudge_*.sql.gz 2>/dev/null | wc -l)
    log_info "Total backups: $TOTAL"
else
    log_warn "No backups found"
fi

log_info "Backup process completed."
