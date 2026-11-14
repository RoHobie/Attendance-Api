# Shared helpers. Source from other scripts.
set -euo pipefail
cd "$(dirname "$0")/.."
[ -f .env ] && set -a && . ./.env && set +a
# Default to the currently deployed version so plain `docker compose` calls do not switch tags.
if [ -z "${APP_VERSION_FORCE:-}" ] && [ -f .deployed_version ]; then APP_VERSION="$(cat .deployed_version)"; fi
export APP_VERSION="${APP_VERSION:-dev}"
HTTPS_PORT="${HTTPS_PORT:-8443}"
BASE_URL="https://localhost:${HTTPS_PORT}"

# curl against the proxy, trusting Caddy's local CA once exported (see scripts/trust-ca.sh).
api() { curl -sS --cacert deploy/caddy-root.crt --resolve "localhost:${HTTPS_PORT}:127.0.0.1" "$@"; }

wait_healthy() { # service, timeout seconds
  local svc=$1 t=${2:-90} i=0 cid
  while [ $i -lt $t ]; do
    cid=$(docker compose ps -q "$svc" 2>/dev/null || true)
    if [ -n "$cid" ] && [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null)" = healthy ]; then return 0; fi
    sleep 2; i=$((i+2))
  done
  return 1
}
