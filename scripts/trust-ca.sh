#!/usr/bin/env bash
# Export Caddy's local root certificate so curl can verify https://localhost without -k.
. "$(dirname "$0")/lib.sh"
docker compose cp proxy:/data/caddy/pki/authorities/local/root.crt deploy/caddy-root.crt
echo "Saved deploy/caddy-root.crt. Test: curl --cacert deploy/caddy-root.crt https://localhost:${HTTPS_PORT}/healthz"
