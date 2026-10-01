<div align="center">

<img src="apps/web/public/favicon.svg" alt="ReliefMesh logo: four connected nodes forming a mesh" width="88" height="88">

# ReliefMesh

**Open-source, offline-first coordination software for disaster-response exercises, volunteer groups and community aid.**

Track who needs what, who is helping and whether a need was actually met, even when the internet connection drops.
Self-hosted, privacy-preserving and free under the Apache License 2.0.

[![CI](https://github.com/6-SlX-6/reliefmesh/actions/workflows/ci.yml/badge.svg)](https://github.com/6-SlX-6/reliefmesh/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](apps/api)
[![Nuxt 3](https://img.shields.io/badge/Nuxt-3-00DC82?logo=nuxt&logoColor=white)](apps/web)
[![PostgreSQL 16](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](docs/data-model.md)
[![Offline-first PWA](https://img.shields.io/badge/PWA-offline--first-5A0FC8)](docs/offline-sync.md)

[Quick start](#quick-start-demo-in-5-minutes) · [Features](#features) · [Screenshots](#screenshots) · [FAQ](#frequently-asked-questions) · [Documentation](#documentation)

</div>

> [!IMPORTANT]
> **Emergency notice:** If there is immediate danger, contact your local emergency services. In the EU, call 112 where available. **ReliefMesh is not an emergency dispatch service.**
>
> ReliefMesh is intended for authorized preparedness exercises, local aid coordination and non-emergency community support. It never notifies authorities, never dispatches or recommends emergency responders and never performs medical triage.

<p align="center">
  <img src="docs/images/dashboard.png" alt="ReliefMesh coordinator dashboard in exercise mode with counters for requests that need review, critical and high urgency requests, overdue requests, deliveries awaiting confirmation, open requests, active assignments and available volunteers" width="880">
</p>

---

## What is ReliefMesh?

**ReliefMesh is a self-hosted web application for coordinating aid requests, volunteer offers and assignments during local disaster-response exercises and community aid operations.** It runs as an installable progressive web app (PWA) that keeps working offline and synchronizes automatically when the server is reachable again.

It is built for community organizations, volunteer groups, temporary shelters, mutual-aid networks and civil-protection training teams. It answers one question:

**Who needs what, where, how urgently, who is handling it, and has the request actually been resolved?**

| | |
| --- | --- |
| **Type** | Self-hosted web application (progressive web app) for aid and volunteer coordination |
| **Main use** | Disaster-preparedness exercises, local aid coordination, non-emergency community support |
| **Works offline** | Yes: changes are queued on the device (IndexedDB) and synchronized later |
| **Hosting** | Your own server or laptop; Docker Compose with Caddy (automatic HTTPS) |
| **Tech stack** | Go API, PostgreSQL 16, Nuxt 3 / Vue 3 / TypeScript front end |
| **Data protection** | Role-based access, AES-256-GCM encryption of addresses and contact details, append-only audit log |
| **Telemetry** | None: no analytics, no tracking, no third-party requests from the app |
| **License** | [Apache License 2.0](LICENSE), free to use, modify and self-host |
| **Status** | Version 0.1.0, complete MVP for one organization per instance |

## Contents

- [Why ReliefMesh?](#why-reliefmesh)
- [Who uses ReliefMesh?](#who-uses-reliefmesh)
- [How it works](#how-it-works)
- [Features](#features)
- [Screenshots](#screenshots)
- [What ReliefMesh is not](#what-reliefmesh-is-not)
- [Roles and permissions](#roles-and-permissions)
- [Quick start (demo in 5 minutes)](#quick-start-demo-in-5-minutes)
- [Production deployment](#production-deployment)
- [Development](#development)
- [Testing](#testing)
- [Architecture](#architecture)
- [Documentation](#documentation)
- [Frequently asked questions](#frequently-asked-questions)
- [Project status and roadmap](#project-status-and-roadmap)
- [Contributing, security and license](#contributing-security-and-license)

## Why ReliefMesh?

During floods, storms, heat waves or power outages, local helpers often coordinate with spreadsheets, paper lists and group chats. That works until it does not: requests get lost in chat history, two volunteers drive to the same address, personal data ends up in large group chats, and nobody knows whether a need was really met. When the mobile network is overloaded, many online tools stop working entirely.

ReliefMesh replaces that patchwork with a structured, auditable workflow that is designed for stress, poor connectivity and data protection:

| Challenge | Spreadsheets and group chats | ReliefMesh |
| --- | --- | --- |
| Overview of open needs | Scattered across messages and tabs | One list with status, urgency, category and "needed by" time |
| Double allocation of supplies | Easy to happen | Prevented by the database, even with simultaneous assignments |
| Personal data | Visible to everyone in the group | Need-to-know per role; addresses and contacts encrypted and revealed only with an audit entry |
| Unreliable connection | Edits fail or get lost | Offline queue on the device, automatic sync, conflicts shown and never discarded silently |
| "Is this actually done?" | Unclear | Delivery and resolution are separate steps; a coordinator confirms the need was met |
| Accountability | No reliable history | Append-only, hash-chained audit log with integrity check |
| Hosting and privacy | Third-party cloud services | Self-hosted, no telemetry, no vendor account needed |

## Who uses ReliefMesh?

- **Civil-protection and disaster-preparedness training teams** running tabletop or field exercises (flood, shelter, power outage scenarios are included).
- **Community organizations and mutual-aid networks** coordinating non-emergency support such as drinking water, food, blankets, transport or translation.
- **Temporary shelters** keeping track of supply requests, volunteer tasks and handovers.
- **Volunteer groups** that need clear task lists on their phones, also without a stable connection.
- **Developers and researchers** looking for a real-world example of an offline-first PWA with idempotent sync, role-based data projections and a tamper-evident audit log in Go and Vue.

## How it works

1. **Request.** A requester (or a coordinator on their behalf) records a structured request: category, quantity, urgency, needed-by time, approximate area and accessibility needs. Exact address and contact details are optional and stored encrypted.
2. **Review.** A coordinator reviews and verifies the request and sets the urgency. ReliefMesh never prioritizes automatically; urgency is always a human decision.
3. **Match manually.** The coordinator assigns a volunteer and, if needed, allocates quantities from registered offers (for example "40 blankets at depot B"). Over-allocation is impossible.
4. **Deliver.** The volunteer accepts the task on their phone, sees only the details the task needs and records the handover, offline if necessary.
5. **Confirm.** A coordinator confirms that the need was actually met and closes the request. Every step is recorded in the audit log and visible in the request timeline.

## Features

| Area | What you get |
| --- | --- |
| **Structured requests and offers** | 15 categories (drinking water, food, shelter, blankets, transport, translation, medicine pickup and more), quantities, needed-by time, accessibility needs, human-readable references such as `RM-2026-0001042` and `OF-2026-0000012`. |
| **Manual coordination** | Coordinators review, verify, prioritize and assign. Allocation from offers is manual and **concurrency-safe**: over-allocation is impossible, even under simultaneous assignments (row locks plus a database constraint). |
| **Lifecycle you can trust** | Server-enforced state machines for requests (11 statuses), offers and assignments. Delivery never resolves a request automatically; a coordinator confirms that the need was actually met. |
| **Immutable audit trail** | Every status change, assignment, access to protected data and admin action is recorded in an append-only, **hash-chained** audit log with integrity verification. |
| **Privacy by design** | Location modes from "none" to "protected exact"; exact addresses and contact details are **encrypted at rest (AES-256-GCM)** and only revealed through an audited endpoint, visible to the data subject in their request history. Volunteers see only what their task needs. No tracking, no analytics, no third-party requests. |
| **Offline-first PWA** | Create requests, offers, notes and status changes without a connection. Changes queue in IndexedDB, survive reloads and synchronize idempotently when the server is reachable again. Conflicts are shown, **never discarded silently**. |
| **Built for stress** | Large touch targets, high contrast (WCAG AA verified in tests), clear status wording with icons and text, keyboard navigation, screen-reader labels, mobile bottom navigation. |
| **Safety guardrails** | Always-visible, per-deployment configurable emergency notice; explicit acknowledgement before critical requests; medicine pickup is logistics-only (no free-text medical data); no routing and no "safe destination" claims; exercise-mode banner. |
| **Exercise support** | Exercise mode, three ready-made fictional scenarios with facilitator guides, demo accounts for every role and one-click scenario loading for administrators. |
| **Self-hosted** | Go API + PostgreSQL + static PWA. Docker Compose with Caddy (automatic TLS). No vendor cloud, no API keys, no telemetry. |

## Screenshots

All screenshots show the fictional flood exercise that ships with the demo.

| Request list for coordinators | Request detail with next steps |
| --- | --- |
| <img src="docs/images/requests.png" alt="ReliefMesh request list with filters for status, urgency and category, showing a high urgency transport request under review and a verified drinking water request for a community hall shelter" width="440"> | <img src="docs/images/request-detail.png" alt="ReliefMesh request detail page with urgency and status badges, reference number, next-step buttons for the coordinator and an internal notes panel" width="440"> |

| Volunteer tasks on a phone | Offline: changes wait on the device |
| --- | --- |
| <img src="docs/images/mobile-tasks.png" alt="ReliefMesh on a phone showing a volunteer's task list with an accepted task for blankets and the mobile bottom navigation" width="300"> | <img src="docs/images/mobile-offline.png" alt="ReliefMesh on a phone without connection, showing the offline indicator and a new drinking water request waiting to sync on the pending changes screen" width="300"> |

## What ReliefMesh is not

- not an emergency call system, and not a replacement for emergency services, authorities or a professional emergency operations center;
- not a dispatch system for police, fire brigade, ambulance, rescue services or military units;
- not a medical system: no diagnosis, no triage, no medical advice;
- not a public-safety decision system and not a tool for coordinating dangerous activities;
- not a social network: no public posting, no public maps, no sharing features.

These boundaries are enforced in the product (see [docs/emergency-disclaimer.md](docs/emergency-disclaimer.md)).

## Roles and permissions

| Role | Can do | Cannot do |
| --- | --- | --- |
| **Requester** | Create requests, follow their status, add updates | See other requests, offers, volunteer identities or internal notes |
| **Volunteer** | Work on tasks assigned to them, record handover, offer resources | Browse requests; open contact details or exact locations unless granted for an active assignment |
| **Coordinator** | Review, verify, prioritize, assign, close requests; manage offers; internal notes; dashboard | Manage users or instance settings |
| **Organization manager** | Everything a coordinator can, plus volunteer availability and non-personal reports | Change security settings |
| **Administrator** | Users and roles, password resets, notices, categories, retention, audit log, demo data | Read operational personal data (administration is separate from coordination) |

All permissions are enforced by the API; the interface only mirrors them. See [docs/role-model.md](docs/role-model.md).

## Quick start (demo in 5 minutes)

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

> [!WARNING]
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

Running on a **local network without internet** (training room, shelter Wi-Fi)? See [Local networks without internet](docs/deployment.md#local-networks-without-internet). Without Docker, the API can also run as a single binary that serves the web app ([details](docs/deployment.md#single-binary-without-docker)).

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

### Technical highlights for developers

| Pattern | Where to look |
| --- | --- |
| Offline outbox with idempotent push, dependent operations and explicit conflicts | [`apps/web/utils/sync-engine.ts`](apps/web/utils/sync-engine.ts), [`apps/api/internal/sync`](apps/api/internal/sync), [docs/offline-sync.md](docs/offline-sync.md) |
| Concurrency-safe allocation with `SELECT ... FOR UPDATE` and a `CHECK` constraint | [`apps/api/internal/assignments`](apps/api/internal/assignments), [`apps/api/migrations/0001_init.sql`](apps/api/migrations/0001_init.sql) |
| Append-only, hash-chained audit log with database triggers | [`apps/api/internal/audit`](apps/api/internal/audit) |
| Field-level AES-256-GCM encryption with key ring and row binding | [`apps/api/internal/locations`](apps/api/internal/locations) |
| Role-specific data projections (invisible records return 404) | [`apps/api/internal/requests/views.go`](apps/api/internal/requests/views.go) |
| Service worker that caches only the app shell, CSP hashes generated at build time | [`apps/web/service-worker/sw.ts`](apps/web/service-worker/sw.ts), [`apps/web/scripts/postbuild.mjs`](apps/web/scripts/postbuild.mjs) |

Read more in [docs/architecture.md](docs/architecture.md) and [docs/threat-model.md](docs/threat-model.md).

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
| Exercise scenarios | [examples/README.md](examples/README.md) |
| Emergency disclaimer | [docs/emergency-disclaimer.md](docs/emergency-disclaimer.md) |
| API | [docs/api.md](docs/api.md) |
| Accessibility | [docs/accessibility.md](docs/accessibility.md) |
| Roadmap | [docs/roadmap.md](docs/roadmap.md) |

## Frequently asked questions

### What is ReliefMesh used for?

ReliefMesh is used to coordinate aid requests, volunteer offers and assignments during disaster-preparedness exercises and local, non-emergency community aid, for example distributing drinking water, blankets or transport after a flood or during a power outage. Coordinators see all open needs in one place, assign volunteers manually and confirm when a need has been met.

### Is ReliefMesh an emergency call or dispatch system?

No. ReliefMesh does not replace emergency services, does not notify authorities and never dispatches police, fire brigade, ambulance or rescue services. Every screen shows a configurable emergency notice. In immediate danger, people should always call their local emergency number (112 in the EU) first.

### Does ReliefMesh work without an internet connection?

Yes. ReliefMesh is an offline-first progressive web app. Once you have signed in on a device (and not signed out), the app also opens without a connection. New requests, offers, notes and task updates are stored in an outbox on the device and sent automatically, in their original order, when the server is reachable again. Conflicts are shown to the user and never discarded silently.

### Can ReliefMesh run on a local network without internet access?

Yes. Because ReliefMesh is self-hosted, the server can run on a laptop or small server inside a training room or shelter network. For devices other than `localhost`, offline mode needs HTTPS; the repository includes scripts for local certificates. See [Local networks without internet](docs/deployment.md#local-networks-without-internet).

### How does ReliefMesh protect personal data?

Each role sees only what it needs. Exact addresses and contact details are encrypted at rest with AES-256-GCM and only revealed through an audited action that the affected person can see in their request history. Volunteers see a contact or an exact location only when a coordinator grants it for an active assignment. There is no public map, no tracking and no telemetry, and retention rules delete or redact old data. Details: [docs/privacy-model.md](docs/privacy-model.md).

### Does ReliefMesh prioritize or match requests automatically?

No. Urgency, verification and matching are always human decisions by a coordinator. ReliefMesh supports those decisions with filters, counters and clear status information, but it never ranks people or performs triage.

### Does ReliefMesh show requests on a map?

No. ReliefMesh deliberately has no public maps. Locations are stored as an area label or an approximate, rounded position; exact addresses are encrypted and only shown to authorized people when needed for a task.

### Is ReliefMesh free and open source?

Yes. ReliefMesh is licensed under the Apache License 2.0. You can use, modify and self-host it at no cost. It needs no vendor account, no API keys and no cloud service.

### What do I need to run ReliefMesh?

For the demo, Docker with Compose v2. For production, a Linux server with Docker Compose and a domain name for automatic HTTPS (or local certificates for an isolated network). The stack consists of PostgreSQL, the Go API, a static web app and the Caddy reverse proxy.

### Can several organizations share one ReliefMesh instance?

Version 0.1.0 is designed for one organization per instance. Multi-organization instances with strict data separation are on the [roadmap](docs/roadmap.md).

### Which languages does ReliefMesh support?

The user interface is currently in English and uses plain language for people under stress. Translations, starting with German, are planned (see [roadmap](docs/roadmap.md)).

### How can I practice with ReliefMesh?

Start the demo and load one of the included fictional scenarios ([flood](examples/flood-exercise/README.md), [temporary shelter](examples/shelter-exercise/README.md), [power outage](examples/power-outage-exercise/README.md)). Each scenario has learning objectives, an inject timeline and an evaluation checklist for facilitators. The [exercise guide](docs/exercise-guide.md) explains how to run a session.

## Project status and roadmap

Version **0.1.0** is a complete MVP for a single organization per instance. Not included by design: automatic matching or dispatch, automated triage, medical features, public maps, SMS/e-mail/push notifications, federation, radio/mesh hardware integration and integrations with emergency services. Possible future work is listed in [docs/roadmap.md](docs/roadmap.md).

## Contributing, security and license

- Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).
- Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). Do not open public issues for security problems.
- If you use ReliefMesh in training material or research, you can cite it using [CITATION.cff](CITATION.cff).
- Licensed under the [Apache License 2.0](LICENSE).
