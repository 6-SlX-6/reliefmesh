# ReliefMesh

**Offline-first, privacy-preserving coordination for local disaster-response exercises and community aid.**

> **Emergency notice:** If there is immediate danger, contact your local emergency services. In the EU, call 112 where available. **ReliefMesh is not an emergency dispatch service.**
>
> ReliefMesh is intended for authorized preparedness exercises, local aid coordination and non-emergency community support. It never notifies authorities, never dispatches or recommends emergency responders and never performs medical triage.

ReliefMesh is a self-hosted web application for community organizations, volunteer groups, temporary shelters, mutual-aid networks and civil-protection training teams. It helps authorized coordinators and volunteers answer one question:

**Who needs what, where, how urgently, who is handling it, and has the request actually been resolved?**

---

## Contents

- [Highlights](#highlights)
- [What ReliefMesh is not](#what-reliefmesh-is-not)
- [Screens and roles](#screens-and-roles)
- [Quick start (demo)](#quick-start-demo)
- [Production deployment](#production-deployment)
- [Development](#development)
- [Testing](#testing)
- [Architecture](#architecture)
- [Documentation](#documentation)
- [Project status and roadmap](#project-status-and-roadmap)
- [Contributing, security and license](#contributing-security-and-license)

## Highlights

| Area | What you get |
| --- | --- |
| **Structured requests and offers** | 15 categories (drinking water, shelter, transport, translation, medicine pickup, ...), quantities, needed-by time, accessibility needs, human-readable references (`RM-2026-0001042`, `OF-2026-0000012`). |
| **Manual coordination** | Coordinators review, verify, prioritize and assign. Allocation from offers is manual and **concurrency-safe**: over-allocation is impossible, even under simultaneous assignments (row locks + database constraint). |
| **Lifecycle you can trust** | Server-enforced state machines for requests (11 statuses), offers and assignments. Delivery never resolves a request automatically - a coordinator confirms that the need was actually met. |
| **Immutable audit trail** | Every status change, assignment, access to protected data and admin action is recorded in an append-only, **hash-chained** audit log with integrity verification. |
| **Privacy by design** | Location modes from "none" to "protected exact"; exact addresses and contact details are **encrypted at rest (AES-256-GCM)** and only revealed through an audited endpoint - visible to the data subject in their request history. Volunteers see only what their task needs. No tracking, no analytics, no third-party requests. |
| **Offline-first PWA** | Create requests, offers, notes and status changes without a connection. Changes queue in IndexedDB, survive reloads and synchronize idempotently when the server is reachable again. Conflicts are shown, **never discarded silently**. |
| **Built for stress** | Large touch targets, high contrast (WCAG AA verified in tests), clear status wording with icons and text, keyboard navigation, screen-reader labels, mobile bottom navigation. |
| **Safety guardrails** | Always-visible, per-deployment configurable emergency notice; explicit acknowledgement before critical requests; medicine pickup is logistics-only (no free-text medical data); no routing and no "safe destination" claims; exercise-mode banner. |
| **Self-hosted** | Go API + PostgreSQL + static PWA. Docker Compose with Caddy (automatic TLS). No vendor cloud, no API keys, no telemetry. |

## What ReliefMesh is not

- not an emergency call system, and not a replacement for emergency services, authorities or a professional emergency operations center;
- not a dispatch system for police, fire brigade, ambulance, rescue services or military units;
- not a medical system: no diagnosis, no triage, no medical advice;
- not a public-safety decision system and not a tool for coordinating dangerous activities;
- not a social network: no public posting, no public maps, no sharing features.

These boundaries are enforced in the product (see [docs/emergency-disclaimer.md](docs/emergency-disclaimer.md)).

## Screens and roles

| Role | Can do | Cannot do |
| --- | --- | --- |
| **Requester** | Create requests, follow their status, add updates | See other requests, offers, volunteer identities or internal notes |
| **Volunteer** | Work on tasks assigned to them, record handover, offer resources | Browse requests; open contact details or exact locations unless granted for an active assignment |
| **Coordinator** | Review, verify, prioritize, assign, close requests; manage offers; internal notes; dashboard | Manage users or instance settings |
| **Organization manager** | Everything a coordinator can, plus volunteer availability and non-personal reports | Change security settings |
| **Administrator** | Users and roles, password resets, notices, categories, retention, audit log, demo data | Read operational personal data (administration is separate from coordination) |

All permissions are enforced by the API; the interface only mirrors them. See [docs/role-model.md](docs/role-model.md).

## Quick start (demo)

Requirements: Docker with Compose v2.

```bash
git clone https://github.com/6-SlX-6/reliefmesh.git
cd reliefmesh
docker compose -f infrastructure/compose/docker-compose.demo.yml up -d --build
```

Open <http://localhost:8080> and sign in with one of the demo accounts (password `reliefmesh-demo-exercise`):

| Username | Role |
| --- | --- |
| `demo-coordinator` | Coordinator |
| `demo-manager` | Organization manager |
| `demo-volunteer-1`, `demo-volunteer-2`, `demo-volunteer-3` | Volunteers |
| `demo-requester-1`, `demo-requester-2` | Requesters |
| `demo-admin` | Administrator |

The demo loads the fictional [flood exercise](examples/flood-exercise/README.md). Administrators can load the [shelter](examples/shelter-exercise/README.md) and [power outage](examples/power-outage-exercise/README.md) scenarios under *Administration > Data & retention*.

> The demo uses public secrets and passwords. **Never enter real personal data into a demo instance.**

## Production deployment

Short version (details in [docs/deployment.md](docs/deployment.md)):

```bash
cd infrastructure/compose
cp .env.example .env
../scripts/generate-secrets.sh        # paste the output into .env, set RELIEFMESH_DOMAIN
docker compose up -d                  # PostgreSQL, API, web, Caddy with automatic HTTPS
../scripts/bootstrap-admin.sh admin "Your Name"
```

Then sign in, change the temporary password if one was generated, and:

1. set the **emergency notice for your country** (*Administration > Settings & notices*);
2. decide whether the instance runs in **exercise mode**;
3. create coordinator, volunteer and requester accounts;
4. schedule [backups](docs/backup-and-restore.md) and store the data encryption key separately.

## Development

Requirements: Go 1.26+, Node.js 20.18+ with pnpm 10, PostgreSQL 16 (or Docker).

```bash
pnpm install                      # JavaScript workspace (web app + shared packages)
make dev-db                       # PostgreSQL in Docker (creates test databases too)
make api-run                      # API on http://localhost:8080 (applies migrations)
make api-seed                     # optional: load the flood exercise scenario
make web-dev                      # web app on http://localhost:3000, proxies /api
```

Create an administrator for local development:

```bash
cd apps/api
RELIEFMESH_DATABASE_URL=postgres://reliefmesh:reliefmesh@localhost:5432/reliefmesh?sslmode=disable \
  go run ./cmd/reliefmesh-api bootstrap-admin --username admin --also-coordinator
```

Useful commands: `make help`, `make lint`, `make test`, `make test-e2e`, `make openapi`.

### Repository layout

```
apps/api            Go API (chi, pgx, Argon2id, AES-GCM), migrations, OpenAPI, tests
apps/web            Nuxt 3 PWA (Tailwind, Pinia, Dexie, Workbox), unit + Playwright tests
packages/           shared-types, api-client, ui-tokens (TypeScript workspace packages)
infrastructure/     Docker Compose (production, development, demo), Caddy, ops scripts
docs/               architecture, threat model, privacy, data and role model, operations
examples/           exercise scenarios (flood, shelter, power outage)
scripts/            lint, test and OpenAPI verification helpers
```

## Testing

| Suite | Command | Covers |
| --- | --- | --- |
| Go unit tests | `cd apps/api && go test ./internal/...` | state machines, crypto, auth, validation, config, CSP |
| Go integration tests | `make test-api` | authorization matrix, over-allocation under concurrency, audit immutability and tamper detection, sync idempotency and conflicts, export privacy, retention, CSRF |
| API contract tests | `make openapi` | every route documented and every documented route implemented |
| Web unit tests | `make test-web` | sync engine (fake IndexedDB), components, utilities, API client, design-token contrast |
| End-to-end | `make test-e2e` | full stack in Chromium: role flows, offline queue across reloads via the service worker, axe accessibility checks, mobile layout |

## Architecture

```
 Browser (PWA)                         Self-hosted server
 +-------------------------+   HTTPS   +-----------+     +-------------+     +------------+
 | Nuxt 3 app (static)     | --------> |  Caddy    | --> |  Go API     | --> | PostgreSQL |
 | Service worker (shell)  |           | (TLS, rp) |     |  (REST)     |     |            |
 | IndexedDB outbox/cache  |           +-----------+     +-------------+     +------------+
 +-------------------------+                 |
                                             +--> web container (nginx, static files)
```

- REST API with an OpenAPI 3.1 description ([apps/api/openapi/openapi.yaml](apps/api/openapi/openapi.yaml), [docs/api.md](docs/api.md)).
- Sessions in HTTP-only `SameSite=Strict` cookies, CSRF tokens, origin checks, Argon2id, login rate limiting and lockout.
- Protected fields sealed with AES-256-GCM, bound to row and column, with key rotation.
- Offline sync via `POST /api/v1/sync/push` (ordered, idempotent operations) and `GET /api/v1/sync/pull`.

Read more in [docs/architecture.md](docs/architecture.md) and [docs/offline-sync.md](docs/offline-sync.md).

## Documentation

| Topic | Document |
| --- | --- |
| Architecture | [docs/architecture.md](docs/architecture.md) |
| Threat model | [docs/threat-model.md](docs/threat-model.md) |
| Privacy model | [docs/privacy-model.md](docs/privacy-model.md) |
| Data model | [docs/data-model.md](docs/data-model.md) |
| Roles and permissions | [docs/role-model.md](docs/role-model.md) |
| Offline synchronization | [docs/offline-sync.md](docs/offline-sync.md) |
| Deployment | [docs/deployment.md](docs/deployment.md) |
| Backup and restore | [docs/backup-and-restore.md](docs/backup-and-restore.md) |
| Running an exercise | [docs/exercise-guide.md](docs/exercise-guide.md) |
| Emergency disclaimer | [docs/emergency-disclaimer.md](docs/emergency-disclaimer.md) |
| API | [docs/api.md](docs/api.md) |
| Accessibility | [docs/accessibility.md](docs/accessibility.md) |
| Roadmap | [docs/roadmap.md](docs/roadmap.md) |

## Project status and roadmap

Version **0.1.0** is a complete MVP for a single organization per instance. Not included by design: automatic matching or dispatch, automated triage, medical features, public maps, SMS/e-mail/push notifications, federation, radio/mesh hardware integration and integrations with emergency services. Possible future work is listed in [docs/roadmap.md](docs/roadmap.md).

## Contributing, security and license

- Contributions are welcome - please read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).
- Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). Do not open public issues for security problems.
- Licensed under the [Apache License 2.0](LICENSE).
