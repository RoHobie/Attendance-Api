#!/usr/bin/env bash
# Restore the database from a backup file: ./scripts/restore.sh backups/school-<timestamp>.db
. "$(dirname "$0")/lib.sh"
file=${1:?usage: restore.sh <backup-file>}
[ -f "$file" ] || { echo "no such file: $file" >&2; exit 1; }
[ -f "$file.sha256" ] && (cd "$(dirname "$file")" && sha256sum -c "$(basename "$file").sha256")
name=$(basename "$file")

echo "Stopping api..."
docker compose stop api
# Keep the current database in case the restore was a mistake.
docker compose run --rm tools sh -c \
  "[ -f /data/school.db ] && cp /data/school.db /backups/pre-restore-\$(date -u +%Y%m%dT%H%M%SZ).db || true"
docker compose run --rm tools sh -c \
  "rm -f /data/school.db /data/school.db-wal /data/school.db-shm && cp /backups/${name} /data/school.db && chown 65532:65532 /data/school.db"
echo "Starting api..."
docker compose up -d api proxy
wait_healthy api 90 && echo "Restore complete; api is healthy." || { echo "api did not become healthy" >&2; exit 1; }
