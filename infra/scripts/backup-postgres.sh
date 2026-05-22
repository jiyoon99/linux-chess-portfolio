#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
STAMP="$(date +%Y%m%d-%H%M%S)"

mkdir -p "$BACKUP_DIR"
docker compose exec -T postgres pg_dump -U chess chess | gzip > "$BACKUP_DIR/chess-$STAMP.sql.gz"
find "$BACKUP_DIR" -type f -name 'chess-*.sql.gz' -mtime +14 -delete
