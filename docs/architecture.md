# Architecture

ReliefMesh is deliberately small: one Go API, one PostgreSQL database and a
static Progressive Web App. There are no microservices, message brokers or
external cloud services.

```
 Device (browser / installed PWA)                       Self-hosted server
 +-------------------------------+          +----------------------------------------------+
 | Nuxt 3 SPA (Vue 3, Pinia)     |  HTTPS   |  Caddy  -- /api, /healthz -->  Go API :8080  |
 | Service worker: app shell     | -------> |  (TLS)  -- everything else -> nginx (static) |
 | IndexedDB (Dexie):            |          |                                   |          |
 |   outbox of offline changes   |          |                              PostgreSQL 16   |
 |   per-user cache of records   |          +----------------------------------------------+
 +-------------------------------+
```

## Components

| Component | Technology | Responsibility |
| --- | --- | --- |
| API (`apps/api`) | Go 1.26, chi, pgx, `log/slog` | Authentication, authorization, domain rules, audit chain, encryption of protected fields, offline sync, exports |
| Database | PostgreSQL 16 | Single source of truth; constraints enforce critical invariants (allocation bounds, status values, append-only audit log) |
| Web app (`apps/web`) | Nuxt 3 (SPA mode), Tailwind, Pinia, VueUse, Dexie, Workbox | Role-aware UI, offline queue and cache, service worker |
| Shared packages (`packages/*`) | TypeScript | API types, typed fetch client, design tokens |
| Reverse proxy | Caddy 2 | TLS (automatic certificates), routing, HSTS, privacy-preserving access logs |

## API structure

```
cmd/reliefmesh-api      entry point and CLI (serve, migrate, bootstrap-admin, reset-password, seed-demo, apply-retention)
internal/http           router, middleware (request id, access log, recover, security headers, body limits), static serving
internal/httpx          JSON helpers, strict decoding, error mapping, Optional[T] for PATCH
internal/apperr         transport-independent error kinds
internal/config         environment configuration (+ *_FILE secrets)
internal/database       pgx pool, transactions, embedded migrations, reference counters
internal/roles          roles, capabilities, principal
internal/auth           Argon2id, sessions, CSRF, rate limiting, lockout
internal/audit          hash-chained append-only audit log
internal/locations      location privacy modes, rounding, AES-256-GCM sealer, contact data
internal/validation     input validation helpers
internal/settings       instance settings, notices, categories
internal/requests       requests: lifecycle, projections, reveal, timeline, deletion
internal/offers         offers: allocation (row lock + constraint), reveal
internal/assignments    assignments: lifecycle, grants, reveal
internal/notes          append-only notes with visibility levels
internal/users          account administration, volunteer availability
internal/dashboard      aggregates and review queues
internal/export         non-personal CSV/JSON reports
internal/sync           offline push/pull
internal/retention      retention and record deletion
internal/demo           fictional exercise scenarios
internal/health         liveness/readiness
migrations/             SQL schema (embedded)
openapi/openapi.yaml    API description (contract-tested against the router)
tests/                  integration and contract tests
```

Domain packages expose a `Service` with use-case methods that take the
authenticated `roles.Principal`. Every method performs its own authorization
check; HTTP handlers only decode input and encode output.

## Key design decisions

1. **Server-side projections.** The API returns a role-specific view of each
   record (`viewer_relation`) plus a `permissions` object listing the actions
   the caller may take. The UI never derives permissions on its own.
2. **Invisible means 404.** Records outside a caller's scope return `404`, not
   `403`, so their existence is not revealed.
3. **Protected data is opt-in per access.** Contact details and exact
   locations are encrypted in the database and never included in regular
   responses. They are returned only by `.../protected` endpoints, which
   record an audit event visible to the data subject.
4. **Invariants live in the database too.** Allocation bounds, status values
   and the append-only audit log are enforced by PostgreSQL constraints and
   triggers, not only by application code.
5. **One write path for offline and online.** Offline-capable mutations go
   through the sync outbox and `POST /sync/push`, which reuses the same
   services as the REST endpoints. See [offline-sync.md](offline-sync.md).
6. **Audit events are written last** in each transaction, under a short
   advisory lock that serializes the hash chain. Lock order for allocations is
   always request -> assignment -> offer.
7. **No automation of decisions.** Urgency, verification, assignment and
   resolution are human decisions. The only automatic status changes are
   direct, recorded consequences of an explicit human action (creating an
   assignment marks a verified request as assigned; starting an assignment
   marks the request in progress).

## Request lifecycle

```
draft -> submitted -> under_review -> verified -> assigned -> in_progress -> partially_resolved -> resolved
                 \__________________________/        \_____________________________________/
                    cancelled | duplicate | expired     cancelled | expired      (coordinators)
closed (resolved, cancelled, expired, duplicate) --reopen with reason--> under_review
```

See [data-model.md](data-model.md) for the complete transition tables.

## Deployment topology

The production Compose file runs four containers on two networks: an internal
`backend` network (PostgreSQL, API) and a `frontend` network (Caddy, API,
web). Only Caddy publishes ports. All containers run as non-root with
`no-new-privileges`; API and web containers have read-only file systems. See
[deployment.md](deployment.md).
