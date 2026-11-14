#!/usr/bin/env bash
# Failure and recovery drill. Prints evidence; capture it with:
#   ./scripts/drill.sh | tee docs/evidence/drill-$(date +%F).txt
# Requires the stack to be running (./scripts/deploy.sh <version>).
. "$(dirname "$0")/lib.sh"

TEACHER="Authorization: Bearer teacher-ananya-token"
step() { printf '\n=== %s | %s\n' "$(date -u +%H:%M:%SZ)" "$*"; }
status() { docker compose ps --format 'table {{.Service}}\t{{.Status}}'; }
[ -f deploy/caddy-root.crt ] || ./scripts/trust-ca.sh >/dev/null

step "0. Baseline: health, readiness and containers"
api "${BASE_URL}/healthz"; echo
api "${BASE_URL}/readyz"; echo
status

step "1. Access control: unassigned teacher (Vikram, Class 8-B) cannot write to Class 10-A"
api -o /dev/null -w 'HTTP %{http_code}\n' -X POST "${BASE_URL}/api/classes/5/attendance" \
  -H "Authorization: Bearer teacher-vikram-token" -H 'Content-Type: application/json' \
  -d '{"student_id":81,"date":"2026-11-13","session":"MORNING","status":"PRESENT"}'
echo "admin (non-teacher) write:"
api -o /dev/null -w 'HTTP %{http_code}\n' -X POST "${BASE_URL}/api/classes/5/attendance" \
  -H "Authorization: Bearer admin-suresh-token" -H 'Content-Type: application/json' \
  -d '{"student_id":81,"date":"2026-11-13","session":"MORNING","status":"PRESENT"}'

step "2. Failure A: the api container is killed (simulated crash)"
docker compose kill api
sleep 1
echo "Detection - through the proxy while api is down:"
api -o /dev/null -w 'HTTP %{http_code}\n' "${BASE_URL}/readyz" || true
status
echo "Recovery - bring the service back:"
docker compose up -d api
wait_healthy api 60 && echo "api recovered" || echo "api did NOT recover"
api "${BASE_URL}/readyz"; echo
echo "Last log lines:"; docker compose logs --tail 3 api

step "3. Write a marker record, then take a backup"
marker_date=$(date -u +%F)
api -w '\nHTTP %{http_code}\n' -X POST "${BASE_URL}/api/classes/5/attendance" -H "$TEACHER" \
  -H 'Content-Type: application/json' \
  -d "{\"student_id\":82,\"date\":\"${marker_date}\",\"session\":\"AFTERNOON\",\"status\":\"ABSENT\"}"
count() { api -H "$TEACHER" "${BASE_URL}/api/classes/5/attendance?date=${marker_date}" | grep -o '"session":"AFTERNOON"' | wc -l; }
echo "Afternoon records for ${marker_date} before disaster: $(count)"
./scripts/backup.sh
latest=$(ls -t backups/school-*.db | head -1)

step "4. Failure B: database file corrupted"
docker compose stop api
docker compose run --rm tools sh -c "head -c 4096 /dev/urandom > /data/school.db; rm -f /data/school.db-wal /data/school.db-shm"
docker compose up -d api
sleep 12
status
echo "Detection (api cannot open the database):"
docker compose logs --tail 3 api
api -o /dev/null -w 'readyz via proxy: HTTP %{http_code}\n' "${BASE_URL}/readyz" || true

step "5. Recovery: restore ${latest}"
./scripts/restore.sh "$latest"
echo "Afternoon records for ${marker_date} after restore: $(count)"
api "${BASE_URL}/readyz"; echo
status

step "Drill complete"
