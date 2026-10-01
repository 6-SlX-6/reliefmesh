# Deployment

ReliefMesh runs on any Linux host with Docker: a small cloud VM, an office
server or a Raspberry Pi 4/5 (arm64 images are published) in a field
location. Hardware guidance for one organization: 2 CPU cores, 2 GB RAM,
10 GB disk.

## 1. Prerequisites

- Docker Engine with Compose v2.
- A DNS name pointing to the host and ports 80/443 reachable (for automatic
  TLS certificates). For an isolated network without public DNS, see
  [Local networks](#local-networks-without-internet).
- `openssl` for generating secrets.

## 2. Configure

```bash
git clone https://github.com/6-SlX-6/reliefmesh.git
cd reliefmesh/infrastructure/compose
cp .env.example .env
../scripts/generate-secrets.sh
```

Copy the generated values into `.env` and set `RELIEFMESH_DOMAIN` (and
optionally `ACME_EMAIL`). Protect the file: `chmod 600 .env`.

| Variable | Meaning |
| --- | --- |
| `RELIEFMESH_DOMAIN` | Public host name; also used as the allowed browser origin |
| `POSTGRES_PASSWORD` | Database password |
| `RELIEFMESH_SECRET_KEY` | Secret for CSRF tokens (min. 32 characters) |
| `RELIEFMESH_DATA_ENCRYPTION_KEYS` | `id:base64key[,old-id:old-key]` - AES-256 keys for protected fields; **back up separately** |
| `RELIEFMESH_VERSION` | Image tag |
| `RELIEFMESH_LOG_LEVEL` | `debug`, `info`, `warn`, `error` |

All API settings (see `apps/api/internal/config/config.go`):

| Variable | Default | Notes |
| --- | --- | --- |
| `RELIEFMESH_ENV` | `production` | `development` relaxes origin/cookie requirements |
| `RELIEFMESH_LISTEN_ADDR` | `:8080` | |
| `RELIEFMESH_DATABASE_URL` | - | required |
| `RELIEFMESH_PUBLIC_ORIGIN` | - | required in production, e.g. `https://relief.example.org` |
| `RELIEFMESH_EXTRA_ALLOWED_ORIGINS` | - | comma-separated additional origins |
| `RELIEFMESH_COOKIE_SECURE` | `true` for https origins | cannot be disabled in production except for localhost |
| `RELIEFMESH_SESSION_TTL` / `RELIEFMESH_SESSION_IDLE_TIMEOUT` | `24h` / `8h` | |
| `RELIEFMESH_SECRET_KEY` | - | required |
| `RELIEFMESH_DATA_ENCRYPTION_KEYS` | - | required |
| `RELIEFMESH_TRUSTED_PROXIES` | - | CIDRs whose `X-Forwarded-For` is trusted (for rate limiting) |
| `RELIEFMESH_WEB_DIR` | - | serve the built web app from the API (single-binary setups) |
| `RELIEFMESH_ALLOW_DEMO_SEED` | `false` | never enable with real data |
| `RELIEFMESH_AUTO_MIGRATE` | `true` | apply migrations at startup |
| `RELIEFMESH_LOGIN_RATE_PER_MINUTE` / `RELIEFMESH_LOGIN_BURST` | `10` / `5` | per client IP |

Every secret can be supplied as a file with a `_FILE` suffix (for example
`RELIEFMESH_SECRET_KEY_FILE=/run/secrets/secret_key`).

## 3. Start

```bash
docker compose up -d
docker compose ps        # all services healthy?
```

Caddy requests certificates automatically. Migrations run when the API starts.

## 4. Create the first administrator

```bash
../scripts/bootstrap-admin.sh admin "Jane Doe"
```

Enter a strong password (12+ characters) or leave it empty to receive a
one-time temporary password. The command refuses to run if an active
administrator exists. Then sign in at `https://<your domain>`.

## 5. First configuration

1. *Administration > Settings & notices*: set the **emergency notice** with
   the correct number(s) for your country, review the critical-urgency and
   medicine notices, decide on **exercise mode**, location precision and
   retention.
2. *Administration > Users*: create accounts. Temporary passwords are shown
   once; hand them over in person or via a trusted channel.
3. Schedule backups ([backup-and-restore.md](backup-and-restore.md)) and,
   optionally, nightly retention:

```cron
15 3 * * * cd /opt/reliefmesh/infrastructure/compose && docker compose exec -T api reliefmesh-api apply-retention
```

## Updating

```bash
cd infrastructure/compose
../scripts/backup.sh
# set RELIEFMESH_VERSION in .env to the new version
docker compose pull && docker compose up -d
```

Migrations are applied automatically. Read the release notes first.

## Key rotation

1. Generate a new key and put it **first**:
   `RELIEFMESH_DATA_ENCRYPTION_KEYS=k2:<new>,k1:<old>`.
2. Restart the API. New data is encrypted with `k2`; existing data stays
   readable with `k1`.
3. Keep the old key until all records using it have been redacted by retention
   (or re-saved). Never remove a key that is still in use.

`RELIEFMESH_SECRET_KEY` can be rotated at any time; users must reload the page
(CSRF tokens change), sessions stay valid.

## Local networks without internet

For a training room or a shelter network without public DNS:

- run the demo-style setup on HTTP only for **localhost** use, or
- create a local certificate with `infrastructure/scripts/generate-dev-certs.sh`,
  use `infrastructure/caddy/Caddyfile.local-tls.example`, and install the
  generated CA on the participants' devices. Service workers (offline mode)
  require HTTPS on any host other than `localhost`.

## Single binary without Docker

```bash
make build                                  # API binary + web app
RELIEFMESH_WEB_DIR=apps/web/.output/public \
RELIEFMESH_DATABASE_URL=... RELIEFMESH_PUBLIC_ORIGIN=https://... \
RELIEFMESH_SECRET_KEY=... RELIEFMESH_DATA_ENCRYPTION_KEYS=... \
  apps/api/bin/reliefmesh-api serve
```

Put a TLS-terminating reverse proxy in front of it.

## Hardening checklist

- [ ] HTTPS only; HSTS enabled (default Caddyfile).
- [ ] `.env` readable only by the operator; secrets not in shell history.
- [ ] Data encryption keys backed up offline, separately from database backups.
- [ ] Server updates and image updates applied regularly.
- [ ] SSH access restricted (keys only); firewall allows 80/443 only.
- [ ] Backups encrypted and restore tested.
- [ ] Demo seeding disabled (`RELIEFMESH_ALLOW_DEMO_SEED=false`).
- [ ] Audit log integrity checked periodically (*Administration > Audit log*).
