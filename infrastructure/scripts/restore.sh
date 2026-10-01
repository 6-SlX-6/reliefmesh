#!/usr/bin/env bash
# Restores a PostgreSQL backup into the production Compose stack.
#
# Usage: ./restore.sh path/to/reliefmesh-YYYYmmddTHHMMSSZ.sql.gz
#
# DESTRUCTIVE: the current database content is replaced. The API is stopped
# during the restore. The data encryption keys in .env must be the ones that
# were active when the backup was taken.
set -euo pipefail
file="${1:?usage: restore.sh <backup.sql.gz>}"
[[ -f "$file" ]] || { echo "No such file: $file" >&2; exit 1; }
cd "$(dirname "$0")/../compose"
compose=(docker compose -f docker-compose.yml)

echo "This replaces ALL data in the ReliefMesh database with: $file"
read -r -p "Type RESTORE to continue: " answer
[[ "$answer" == "RESTORE" ]] || { echo "Aborted."; exit 1; }

case "$file" in
  *.age) decrypt=(age -d "$file") ;;
  *.gpg) decrypt=(gpg --decrypt "$file") ;;
  *) decrypt=(cat "$file") ;;
esac

"${compose[@]}" stop api
"${compose[@]}" exec -T db psql -U reliefmesh -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS reliefmesh WITH (FORCE);" -c "CREATE DATABASE reliefmesh OWNER reliefmesh;"
"${decrypt[@]}" | gunzip | "${compose[@]}" exec -T db psql -U reliefmesh -d reliefmesh -v ON_ERROR_STOP=1 -q
"${compose[@]}" start api
echo "Restore complete. Verify the audit log integrity in Administration > Audit log."
