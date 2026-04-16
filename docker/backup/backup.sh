#!/bin/bash
# One-off backup — для ручного запуска внутри контейнера.
# Периодический запуск делает entrypoint.sh.
set -euo pipefail

TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT="/backups/tjudge_${TIMESTAMP}.sql.gz"
pg_dump --no-owner --no-acl "$PGDATABASE" | gzip > "$OUT"
echo "Backup: $OUT"
