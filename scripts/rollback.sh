#!/usr/bin/env bash
# Roll back to the previous version (or a given one): ./scripts/rollback.sh [version]
# Uses the image already present locally (or in GHCR); nothing is rebuilt. Data is untouched.
# If the new version changed data in a way the old one cannot read, restore a backup too.
. "$(dirname "$0")/lib.sh"
export APP_VERSION_FORCE=1
target=${1:-$(cat .previous_version 2>/dev/null || true)}
[ -n "$target" ] || { echo "no previous version recorded; pass one explicitly" >&2; exit 1; }
current=$(cat .deployed_version 2>/dev/null || echo unknown)
echo "==> Rolling back ${current} -> ${target}"
APP_VERSION=$target docker compose up -d --no-build
if wait_healthy api 90; then
  echo "$target" > .deployed_version
  echo "$current" > .previous_version
  wait_healthy proxy 60 && api "${BASE_URL}/healthz"; echo
  echo "==> Rollback complete"
else
  echo "==> ${target} did not become healthy" >&2; docker compose logs --tail 20 api >&2; exit 1
fi
