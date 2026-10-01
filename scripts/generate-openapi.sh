#!/usr/bin/env bash
# The OpenAPI document (apps/api/openapi/openapi.yaml) is maintained by hand
# next to the handlers. This script verifies it:
#   1. the contract tests check that every route is documented and vice versa,
#      that all $refs resolve and that mutating operations document CSRF;
#   2. if @redocly/cli is available (npx), the document is linted and an HTML
#      reference is rendered to docs/api-reference.html.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
(cd "$root/apps/api" && go test ./tests/ -run 'OpenAPI|CSRF' -count=1)
if [[ "${SKIP_REDOCLY:-}" != "1" ]] && command -v npx >/dev/null 2>&1; then
  npx --yes @redocly/cli@latest lint "$root/apps/api/openapi/openapi.yaml" || echo "warning: redocly lint reported issues"
  npx --yes @redocly/cli@latest build-docs "$root/apps/api/openapi/openapi.yaml" -o "$root/docs/api-reference.html" || true
fi
echo "OpenAPI document verified."
