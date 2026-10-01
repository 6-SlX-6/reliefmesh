#!/usr/bin/env bash
# Prints freshly generated secrets for infrastructure/compose/.env.
set -euo pipefail
echo "POSTGRES_PASSWORD=$(openssl rand -base64 33 | tr -d '/+=' | head -c 40)"
echo "RELIEFMESH_SECRET_KEY=$(openssl rand -base64 48 | tr -d '\n')"
echo "RELIEFMESH_DATA_ENCRYPTION_KEYS=k$(date -u +%Y%m%d):$(openssl rand -base64 32)"
