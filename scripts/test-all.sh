#!/usr/bin/env bash
# Runs all automated tests. Integration and E2E tests need PostgreSQL:
#   make dev-db   (creates reliefmesh_test and reliefmesh_e2e databases)
# Set SKIP_E2E=1 to skip the Playwright suite.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
export RELIEFMESH_TEST_DATABASE_URL="${RELIEFMESH_TEST_DATABASE_URL:-postgres://reliefmesh:reliefmesh@localhost:5432/reliefmesh_test?sslmode=disable}"
export RELIEFMESH_E2E_DATABASE_URL="${RELIEFMESH_E2E_DATABASE_URL:-postgres://reliefmesh:reliefmesh@localhost:5432/reliefmesh_e2e?sslmode=disable}"

echo "==> Go unit and integration tests"
(cd "$root/apps/api" && go test -race -count=1 ./...)
echo "==> JavaScript unit tests"
(cd "$root" && pnpm -r --if-present test)
if [[ "${SKIP_E2E:-}" != "1" ]]; then
  echo "==> Playwright end-to-end tests"
  (cd "$root" && pnpm --filter @reliefmesh/web build)
  (cd "$root/apps/web" && E2E_START_SERVER=1 pnpm exec playwright test)
fi
echo "All tests passed."
