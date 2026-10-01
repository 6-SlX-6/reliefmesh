#!/usr/bin/env bash
# Generates a local CA and a TLS certificate for testing the PWA over HTTPS
# on a local network (service workers require HTTPS except on localhost).
#
# Usage: ./generate-dev-certs.sh [hostname] [output-dir]
# Prefers mkcert when installed; falls back to OpenSSL.
# FOR DEVELOPMENT AND TRAINING NETWORKS ONLY.
set -euo pipefail
host="${1:-reliefmesh.local}"
out="${2:-$(dirname "$0")/../caddy/certs}"
mkdir -p "$out"

if command -v mkcert >/dev/null 2>&1; then
  mkcert -cert-file "$out/$host.crt" -key-file "$out/$host.key" "$host" localhost 127.0.0.1
  echo "Certificates created with mkcert in $out"
  exit 0
fi

umask 077
openssl req -x509 -newkey rsa:3072 -sha256 -days 365 -nodes \
  -keyout "$out/dev-ca.key" -out "$out/dev-ca.crt" -subj "/CN=ReliefMesh Development CA"
openssl req -newkey rsa:2048 -nodes -keyout "$out/$host.key" -out "$out/$host.csr" -subj "/CN=$host"
printf "subjectAltName=DNS:%s,DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth\n" "$host" > "$out/$host.ext"
openssl x509 -req -in "$out/$host.csr" -CA "$out/dev-ca.crt" -CAkey "$out/dev-ca.key" -CAcreateserial \
  -out "$out/$host.crt" -days 365 -sha256 -extfile "$out/$host.ext"
rm -f "$out/$host.csr" "$out/$host.ext"
echo "Created $out/$host.crt. Install $out/dev-ca.crt as a trusted CA on test devices."
