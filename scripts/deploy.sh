#!/usr/bin/env bash
# Deploy a version with automatic backup and rollback:  ./scripts/deploy.sh v1.1.0
# Builds the image locally tagged with the version (or pulls it if IMAGE_REPO points at GHCR).
. "$(dirname "$0")/lib.sh"
new=${1:?usage: deploy.sh <version>}
export APP_VERSION_FORCE=1
prev=$(cat .deployed_version 2>/dev/null || true)

if [ -n "$(docker compose ps -q api 2>/dev/null)" ]; then
  echo "==> Backing up before deploy"; ./scripts/backup.sh
fi

echo "==> Deploying ${new} (previous: ${prev:-none})"
if [[ "${IMAGE_REPO:-school-api}" == ghcr.io/* ]]; then
  APP_VERSION=$new APP_VERSION_FORCE=1 docker compose pull api
else
  APP_VERSION=$new APP_VERSION_FORCE=1 docker compose build api
fi
APP_VERSION=$new APP_VERSION_FORCE=1 docker compose up -d

if wait_healthy api 90; then
  [ -n "$prev" ] && [ "$prev" != "$new" ] && echo "$prev" > .previous_version
  echo "$new" > .deployed_version
  echo "==> ${new} is healthy"
  wait_healthy proxy 60 && ./scripts/trust-ca.sh >/dev/null && api "${BASE_URL}/healthz"; echo
else
  echo "==> ${new} FAILED health check" >&2
  docker compose logs --tail 20 api >&2 || true
  if [ -n "$prev" ]; then
    echo "==> Rolling back to ${prev}" >&2
    APP_VERSION=$prev docker compose up -d
    wait_healthy api 90 && echo "==> Rolled back to ${prev}" >&2
  fi
  exit 1
fi
