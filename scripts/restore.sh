#!/bin/bash
set -euo pipefail

# TJudge Database Restore Script
# Usage: ./scripts/restore.sh <backup_file.sql.gz>

BACKUP_FILE="${1:-}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
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

# Show usage if no backup file provided
if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    echo ""
    echo "Available backups:"
    if ls "$BACKUP_DIR"/tjudge_*.sql.gz 1>/dev/null 2>&1; then
        ls -lh "$BACKUP_DIR"/tjudge_*.sql.gz | awk '{print "  " $9 " (" $5 ", " $6 " " $7 ")"}'
    else
        echo "  No backups found in $BACKUP_DIR"
    fi
    exit 1
fi

# Check if backup file exists
if [ ! -f "$BACKUP_FILE" ]; then
    log_error "Backup file not found: $BACKUP_FILE"
    exit 1
fi

# Check if container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
    log_error "Container '$POSTGRES_CONTAINER' is not running!"
    exit 1
fi

# Warning and confirmation
echo ""
log_warn "This will REPLACE ALL DATA in database '$DB_NAME'!"
log_warn "Backup file: $BACKUP_FILE"
echo ""
read -p "Are you sure you want to continue? (type 'yes' to confirm): " confirm

if [ "$confirm" != "yes" ]; then
    log_info "Restore cancelled."
    exit 0
fi

# Create a backup of current database before restore
log_info "Creating safety backup of current database..."
SAFETY_BACKUP="$BACKUP_DIR/tjudge_pre_restore_$(date +%Y%m%d_%H%M%S).sql.gz"
if docker exec "$POSTGRES_CONTAINER" pg_dump -U "$DB_USER" "$DB_NAME" 2>/dev/null | gzip > "$SAFETY_BACKUP"; then
    log_info "Safety backup created: $SAFETY_BACKUP"
else
    log_warn "Could not create safety backup (database may be empty)"
fi

# Stop dependent services
log_info "Stopping API and Worker services..."
docker-compose stop api worker 2>/dev/null || docker compose stop api worker 2>/dev/null || true

# Terminate existing connections and drop database
log_info "Preparing database for restore..."
docker exec "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d postgres -c "
    SELECT pg_terminate_backend(pid)
    FROM pg_stat_activity
    WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();
" 2>/dev/null || true

docker exec "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS ${DB_NAME};" 2>/dev/null
docker exec "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d postgres -c "CREATE DATABASE ${DB_NAME};" 2>/dev/null

# Restore from backup
log_info "Restoring database from: $BACKUP_FILE"
if gunzip -c "$BACKUP_FILE" | docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" 2>/dev/null; then
    log_info "Database restored successfully!"
else
    log_error "Restore failed!"
    log_info "Attempting to restore from safety backup..."
    if [ -f "$SAFETY_BACKUP" ]; then
        gunzip -c "$SAFETY_BACKUP" | docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" 2>/dev/null
        log_info "Restored from safety backup."
    fi
    exit 1
fi

# Restart services
log_info "Starting API and Worker services..."
docker-compose start api worker 2>/dev/null || docker compose start api worker 2>/dev/null || true

# Verify
log_info "Verifying database connection..."
if docker exec "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
    log_info "Database is accessible."
else
    log_error "Database verification failed!"
    exit 1
fi

log_info "Restore process completed successfully!"
echo ""
log_info "You may want to delete the safety backup if everything looks good:"
echo "  rm $SAFETY_BACKUP"
