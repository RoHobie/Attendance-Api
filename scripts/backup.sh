#!/usr/bin/env bash
# Take a verified snapshot of the live database into ./backups/ (no downtime).
. "$(dirname "$0")/lib.sh"
mkdir -p backups
ts=$(date -u +%Y%m%dT%H%M%SZ)
name="school-${ts}.db"
# Clear any stale snapshot (VACUUM INTO refuses to overwrite), then snapshot the live database.
docker compose run --rm tools sh -c "rm -f /data/.snapshot.db"
docker compose exec -T api /api -backup /data/.snapshot.db >/dev/null
# The api image has no shell, so the helper container moves the snapshot out and hands it to you.
docker compose run --rm -e HUID="$(id -u)" -e HGID="$(id -g)" tools sh -c \
  "cp /data/.snapshot.db /backups/${name} && rm -f /data/.snapshot.db && chown \$HUID:\$HGID /backups/${name} \
   && cd /backups && sha256sum ${name} > ${name}.sha256 && chown \$HUID:\$HGID ${name}.sha256"
echo "Backup written: backups/${name}"
cat "backups/${name}.sha256"
