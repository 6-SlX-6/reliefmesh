#!/usr/bin/env bash
# Runs all static checks: Go formatting/vet, ESLint, TypeScript.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root/apps/api"
echo "==> gofmt"
unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then echo "Unformatted Go files:"; echo "$unformatted"; exit 1; fi
echo "==> go vet"
go vet ./...
if command -v staticcheck >/dev/null 2>&1; then echo "==> staticcheck"; staticcheck ./...; fi
cd "$root"
echo "==> eslint"
pnpm --filter @reliefmesh/web lint
echo "==> typecheck"
pnpm -r --if-present typecheck
echo "All checks passed."
