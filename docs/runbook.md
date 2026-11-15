# Operations runbook

All commands run from the repository root. Services: `proxy` (Caddy, HTTPS) and `api` (Go + SQLite).

## Deploy

```bash
./scripts/deploy.sh v1.1.0
```
Takes a backup (if running), builds `school-api:v1.1.0` (or pulls it when `IMAGE_REPO` is a GHCR path), starts
the stack, waits for `healthy`, and rolls back to the previous version automatically if the new one fails.
Verify: `curl --cacert deploy/caddy-root.crt https://localhost:8443/healthz` shows the running `version`.

## Restart

| Need | Command |
|---|---|
| Restart API | `docker compose restart api` |
| Restart everything | `docker compose restart` |
| Stop / start | `docker compose stop` / `docker compose up -d` |
| Full teardown (keeps data) | `docker compose down` |

Containers use `restart: unless-stopped`, so crashes and host reboots recover automatically.
Avoid `docker compose down -v`: it **deletes the database volume**.

## Logs

```bash
docker compose logs -f api        # JSON: level, msg, request_id, method, path, status, duration_ms
docker compose logs -f proxy      # Caddy access log (JSON)
docker compose logs api | grep '"level":"ERROR"'
```
Every response carries `X-Request-ID`; find the matching line in the API log. Set `LOG_LEVEL=debug` in `.env` to include probe requests.

## Health

- `GET /healthz`: process is up (returns version). `GET /readyz`: database reachable.
- `docker compose ps` shows `healthy`/`unhealthy` (the image probes `/readyz` with `/api -healthcheck`).

## Backup and restore

```bash
./scripts/backup.sh                                  # online, consistent snapshot (SQLite VACUUM INTO + integrity check)
./scripts/restore.sh backups/school-<time>.db        # verifies checksum, stops api, replaces db, restarts, waits healthy
```
The current database is copied to `backups/pre-restore-<time>.db` first, so a restore can be undone.
`backups/` is on the host; copy it off the machine for real disaster recovery. Schedule with cron if wanted:
`0 2 * * * cd /path/to/repo && ./scripts/backup.sh`.

## Rollback

```bash
./scripts/rollback.sh            # to the previous version recorded by deploy.sh
./scripts/rollback.sh v1.0.0     # to a specific version
```
No rebuild is needed (versioned images stay on the host). Data is untouched; restore a backup only if a release changed data incompatibly.

## Incident response

| Symptom | Check | Action |
|---|---|---|
| Browser/curl gets 502 | `docker compose ps`; `docker compose logs --tail 50 api` | API down: `docker compose up -d api` |
| API `Restarting` and logs say `file is not a database` | corrupted DB file | `./scripts/restore.sh <latest backup>` |
| `unhealthy` right after a deploy | `docker compose logs api` | `./scripts/rollback.sh` |
| Certificate warning in browser | expected with Caddy's local CA | trust `deploy/caddy-root.crt` or use `curl --cacert` |
| `Bind for 0.0.0.0:8443 failed` | port in use | set `HTTPS_PORT` in `.env` |
| `403` on writes | role or class assignment | only the assigned teacher may write; check `teacher_assignments` |
| `409` on writes | duplicate student/date/session | use `PUT .../attendance/{id}` to change the status |

After any incident: confirm `/readyz`, confirm data with `GET /api/classes/{id}/attendance?date=...`, and keep the log excerpt.

## Drill (evidence for the demo)

```bash
./scripts/drill.sh | tee docs/evidence/drill-$(date +%F).txt
```
Shows: role checks (403), killed API detected (502) and recovered, corrupted database detected in logs,
then restored from backup with a marker record preserved.
