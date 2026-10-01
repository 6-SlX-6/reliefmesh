#!/usr/bin/env bash
# Starts a disposable ReliefMesh stack for end-to-end tests:
#   * resets the database given in RELIEFMESH_E2E_DATABASE_URL (DESTRUCTIVE),
#   * loads the "flood" exercise scenario,
#   * serves the built web app (.output/public) from the API on :8080.
# Build the web app first: pnpm --filter @reliefmesh/web build
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
api="$here/../../api"
web_public="$here/../.output/public"
: "${RELIEFMESH_E2E_DATABASE_URL:?set RELIEFMESH_E2E_DATABASE_URL to a disposable database}"

if [[ ! -f "$web_public/index.html" ]]; then
  echo "web app not built: run 'pnpm --filter @reliefmesh/web build' first" >&2
  exit 1
fi

psql "$RELIEFMESH_E2E_DATABASE_URL" -q -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'

export RELIEFMESH_DATABASE_URL="$RELIEFMESH_E2E_DATABASE_URL"
export RELIEFMESH_ENV=development
export RELIEFMESH_PUBLIC_ORIGIN="${E2E_BASE_URL:-http://localhost:8080}"
export RELIEFMESH_SECRET_KEY="e2e-secret-e2e-secret-e2e-secret-e2e-secret"
export RELIEFMESH_DATA_ENCRYPTION_KEYS="e2e:$(printf '%032d' 0 | base64)"
export RELIEFMESH_ALLOW_DEMO_SEED=true
export RELIEFMESH_LOGIN_RATE_PER_MINUTE=600
export RELIEFMESH_LOGIN_BURST=200
export RELIEFMESH_WEB_DIR="$web_public"
export RELIEFMESH_LOG_LEVEL=warn

cd "$api"
go build -o "$here/.reliefmesh-api" ./cmd/reliefmesh-api
"$here/.reliefmesh-api" seed-demo --scenario flood
exec "$here/.reliefmesh-api" serve
