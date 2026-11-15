# School Management – Attendance Service

Small Go 1.26 service for classes, students and attendance, deployed locally with Docker Compose
behind an HTTPS reverse proxy.
Stack: `net/http`, SQLite (`modernc.org/sqlite`, pure Go), Caddy, GitHub Actions + GHCR.

Docs: [architecture](docs/architecture.md) · [runbook](docs/runbook.md) · [evidence](docs/evidence/)

## Quick start (clean machine)

Requirements: Docker with Compose v2, `curl`, `make` (optional), Go 1.26 only if you want to run tests natively.

```bash
cp .env.example .env              # optional; defaults work
./scripts/deploy.sh v1.0.0        # builds image, starts proxy + api, waits until healthy, exports the TLS CA
open https://localhost:8443       # dashboard (your browser will warn until you trust deploy/caddy-root.crt)
curl --cacert deploy/caddy-root.crt https://localhost:8443/healthz
```

Sign in to the dashboard with a demo access code, e.g. `teacher-ananya-token`.

| Access code (Bearer token) | User | Access |
|---|---|---|
| `admin-suresh-token` | Principal | read all classes, no writes |
| `teacher-ananya-token` | Ananya Sharma | Class 10-A, 9-A |
| `teacher-rajesh-token` | Rajesh Verma | Class 10-A, 10-B |
| `teacher-priya-token` | Priya Nair | Class 9-B, 8-A |
| `teacher-vikram-token` | Vikram Singh Rathore | Class 8-B |

These are **synthetic demo credentials** for the seed data (stored hashed with SHA-256). For anything
real set `SEED_DATA=false`; do not reuse them.

## Ports and networks

| Component | Port | Exposure |
|---|---|---|
| Caddy proxy (HTTPS) | host `8443` → container `443` (`HTTPS_PORT`) | published to the host |
| API | `8080` | private `backend` network only (`internal: true`, no internet, not published) |
| SQLite | file `/data/school.db` in volume `school-data` | not network accessible |

## Configuration (environment variables)

| Variable | Default | Purpose |
|---|---|---|
| `HTTPS_PORT` | `8443` | host port for HTTPS |
| `APP_VERSION` | `dev` | image tag; `deploy.sh`/`rollback.sh` manage it |
| `IMAGE_REPO` | `school-api` | set to `ghcr.io/<owner>/<repo>` to pull CI-built images |
| `SEED_DATA` | `true` | load synthetic data into an **empty** database |
| `LOG_LEVEL` | `info` | `info` or `debug` (debug includes probe requests) |
| `DB_PATH`, `ADDR` | `/data/school.db`, `:8080` | set in the image/compose |

No secrets are committed. `.env`, `backups/` and the exported CA certificate are git-ignored.

## API

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz`, `/readyz` | liveness (with version) / database check |
| GET | `/api/me` | current user |
| GET | `/api/classes` | accessible classes |
| GET | `/api/classes/{id}/students` | students in a class |
| POST | `/api/classes/{id}/attendance` | record `{student_id, date, session, status}` |
| GET | `/api/classes/{id}/attendance?date=YYYY-MM-DD` | records for a date |
| PUT | `/api/classes/{id}/attendance/{attendanceId}` | change `{status}` |

`status`: `PRESENT`/`ABSENT`; `session`: `MORNING`/`AFTERNOON`. Duplicates are rejected by a SQLite
UNIQUE constraint (HTTP 409). Only assigned teachers can write (admin gets 403). Creations and status
changes go to `audit_logs` in the same transaction.

## Operations cheat sheet

| Task | Command |
|---|---|
| Deploy / upgrade (auto backup, auto rollback if unhealthy) | `./scripts/deploy.sh v1.1.0` |
| Roll back to previous version | `./scripts/rollback.sh` |
| Backup now | `./scripts/backup.sh` → `backups/school-<time>.db` (+ `.sha256`) |
| Restore | `./scripts/restore.sh backups/school-<time>.db` |
| Logs | `docker compose logs -f api` (JSON), `docker compose logs proxy` |
| Failure/recovery drill | `./scripts/drill.sh \| tee docs/evidence/drill-$(date +%F).txt` |
| Local CI checks | `make check` (fmt, vet, staticcheck, tests), `make vuln`, `make scan` |

### Rollback procedure

1. `deploy.sh` takes a backup, then switches versions, and rolls back automatically if the new version is not healthy within 90 s.
2. Manual: `./scripts/rollback.sh` (or `./scripts/rollback.sh v1.0.0`). Images are tagged per version, so no rebuild is needed.
3. The schema only uses `CREATE TABLE IF NOT EXISTS`, so versions are data-compatible. If a release ever changes data incompatibly, also run `./scripts/restore.sh` with the backup taken by `deploy.sh`.

Evidence: [deploy and rollback](docs/evidence/deploy-rollback-2026-11-14.txt),
[failure drill + restore](docs/evidence/drill-2026-11-14.txt),
[tests and scans](docs/evidence/scans-and-tests-2026-11-14.txt).

## CI/CD (GitHub Actions → GHCR)

`.github/workflows/ci.yml` runs on pull requests, pushes to `main` and `v*` tags:

1. **checks**: `gofmt`, `go vet`, `staticcheck`, `go test -race`, `govulncheck`.
2. **image** (after checks): build, Trivy scan (HIGH/CRITICAL fail the build), then on non-PR events push to
   `ghcr.io/<owner>/<repo>` tagged `sha-<commit>`, the semver (`1.2.3` for tag `v1.2.3`) and `latest` (main). The
   digest is written to the run summary. Only this job has `packages: write`; it uses the built-in `GITHUB_TOKEN`.

### Setting it up (one time)

1. Create an empty GitHub repository, then:
   ```bash
   git init -b main && git add -A && git commit -m "Initial commit"
   git remote add origin git@github.com:<you>/<repo>.git && git push -u origin main
   ```
2. Open the **Actions** tab: the first run starts automatically. Screenshot it for the submission.
3. Release: `git tag v1.0.0 && git push origin v1.0.0`. The run publishes `ghcr.io/<you>/<repo>:1.0.0`; copy the tag and digest from the run summary.
4. To deploy the CI-built image locally: `docker login ghcr.io` (with a personal access token, `read:packages`), then
   set `IMAGE_REPO=ghcr.io/<you>/<repo>` in `.env` and run `./scripts/deploy.sh 1.0.0`.
   (Package visibility is private by default; make it public in the package settings to skip the login.)

## Tests

```bash
go test ./...        # or: make test
```

Integration tests (`tests/api_test.go`) use `httptest` and cover auth, role and class-assignment
checks, 400/403/404/409 responses, concurrent duplicate attendance (exactly one 201), and audit logging.

## Known limitations

- Local deployment only; TLS uses Caddy's local CA (`tls internal`), not a public certificate.
- Single SQLite writer, no HA. Backups are manual or taken at deploy time (schedule `scripts/backup.sh` with cron if needed).
- `docker kill` is treated by Docker as a manual stop, so it does not trigger the restart policy; real crashes do (seen in the corrupted-database drill).
- No metrics endpoint (stretch goal); logs and health checks are provided.
- Demo tokens are fixed synthetic values; there is no user-management endpoint.
