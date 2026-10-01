#!/usr/bin/env bash
# Creates a compressed PostgreSQL backup of the production Compose stack.
#
# Usage: ./backup.sh [backup-dir]        (default: ../../backups)
#
# Environment:
#   RELIEFMESH_BACKUP_KEEP=14     number of backups to keep
#   RELIEFMESH_BACKUP_RECIPIENT   optional age/GPG recipient; if set, the dump
#                                 is encrypted (recommended: backups contain
#                                 personal data, though protected fields are
#                                 already encrypted with the data key)
#
# IMPORTANT: the data encryption keys (RELIEFMESH_DATA_ENCRYPTION_KEYS) are
# NOT part of the database. Store them separately and securely; without them
# protected fields in a backup cannot be decrypted.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
dir="${1:-$here/../../backups}"
keep="${RELIEFMESH_BACKUP_KEEP:-14}"
mkdir -p "$dir"
chmod 700 "$dir"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="$dir/reliefmesh-$stamp.sql.gz"

cd "$here/../compose"
umask 077
docker compose -f docker-compose.yml exec -T db pg_dump -U reliefmesh -d reliefmesh --no-owner --format=plain \
  | gzip -9 > "$file"

if [[ -n "${RELIEFMESH_BACKUP_RECIPIENT:-}" ]]; then
  if command -v age >/dev/null 2>&1; then
    age -r "$RELIEFMESH_BACKUP_RECIPIENT" -o "$file.age" "$file" && rm -f "$file" && file="$file.age"
  elif command -v gpg >/dev/null 2>&1; then
    gpg --batch --yes --encrypt --recipient "$RELIEFMESH_BACKUP_RECIPIENT" -o "$file.gpg" "$file" && rm -f "$file" && file="$file.gpg"
  else
    echo "warning: RELIEFMESH_BACKUP_RECIPIENT set but neither age nor gpg is installed; backup is NOT encrypted" >&2
  fi
fi

echo "Backup written: $file ($(du -h "$file" | cut -f1))"

# Rotate old backups.
ls -1t "$dir"/reliefmesh-*.sql.gz* 2>/dev/null | tail -n +"$((keep + 1))" | xargs -r rm -f
