# Backup and Restore

Backups are gzip-compressed PostgreSQL dumps.

## Create Backup

```bash
BACKUP_DIR=./backups ./infra/scripts/backup-postgres.sh
```

List backups:

```bash
ls -lh ./backups
```

## Verify Backup Contents

```bash
gzip -dc ./backups/chess-*.sql.gz | head
```

## Restore Into a Scratch Database

This verifies that the backup can be restored without touching the live `chess` database.

```bash
docker compose exec -T postgres psql -U chess -d postgres -c "drop database if exists chess_restore_check;"
docker compose exec -T postgres psql -U chess -d postgres -c "create database chess_restore_check;"
gzip -dc ./backups/chess-*.sql.gz | docker compose exec -T postgres psql -U chess -d chess_restore_check
docker compose exec -T postgres psql -U chess -d chess_restore_check -c "select count(*) as users from users;"
docker compose exec -T postgres psql -U chess -d postgres -c "drop database chess_restore_check;"
```

## Restore Live Database

Only do this intentionally.

```bash
docker compose stop backend
docker compose exec -T postgres psql -U chess -d postgres -c "drop database chess;"
docker compose exec -T postgres psql -U chess -d postgres -c "create database chess;"
gzip -dc ./backups/chess-YYYYMMDD-HHMMSS.sql.gz | docker compose exec -T postgres psql -U chess -d chess
docker compose start backend
```
