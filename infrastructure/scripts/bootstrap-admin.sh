#!/usr/bin/env bash
# Creates the first ReliefMesh administrator in the production Compose stack.
#
# Usage: ./bootstrap-admin.sh [username] [display name]
#
# You are asked for a password (not echoed). Leave it empty to generate a
# temporary password that must be changed at first sign-in. The command
# refuses to run if an active administrator already exists.
set -euo pipefail
cd "$(dirname "$0")/../compose"

username="${1:-admin}"
display="${2:-Administrator}"
compose=(docker compose -f docker-compose.yml)

read -r -s -p "Password for '${username}' (empty = generate temporary password): " password
echo
if [[ -n "$password" ]]; then
  read -r -s -p "Repeat password: " repeat
  echo
  if [[ "$password" != "$repeat" ]]; then
    echo "Passwords do not match." >&2
    exit 1
  fi
fi

# The password is passed on stdin so it never appears in the process list or
# shell history.
printf '%s' "$password" | "${compose[@]}" exec -T api reliefmesh-api bootstrap-admin \
  --username "$username" --display-name "$display"
