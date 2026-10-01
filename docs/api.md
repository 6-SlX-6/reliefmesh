# API

The REST API is described in OpenAPI 3.1:
[`apps/api/openapi/openapi.yaml`](../apps/api/openapi/openapi.yaml). A contract
test ensures that every route is documented and every documented operation
exists (`make openapi`).

## Basics

- Base path: `/api/v1`; health endpoints `/healthz` and `/readyz`.
- JSON only; unknown fields are rejected; body limit 1 MiB (4 MiB for sync).
- Errors use a common envelope:

```json
{ "error": { "code": "validation_failed", "message": "Some fields are invalid.", "fields": { "title": "This field is required." }, "request_id": "4f2c..." } }
```

| HTTP | Typical codes |
| --- | --- |
| 400 | `invalid_json`, `unknown_field`, `body_too_large` |
| 401 | `unauthorized`, `login_failed` |
| 403 | `forbidden`, `csrf_invalid`, `origin_rejected`, `password_change_required`, `edit_locked` |
| 404 | `not_found` (also for records you may not see) |
| 409 | `invalid_transition`, `over_allocation`, `version_conflict`, `active_assignments`, `request_not_assignable`, `no_assignment` |
| 422 | `validation_failed` (with `fields`) |
| 429 | `rate_limited` |

## Authentication

```bash
# Sign in (stores the session cookie) and read the CSRF token.
curl -c jar.txt -H 'Content-Type: application/json' \
  -d '{"username":"demo-coordinator","password":"reliefmesh-demo-exercise"}' \
  https://relief.example.org/api/v1/auth/login
# -> { "user": {...}, "csrf_token": "..." }

# State-changing calls send the token.
curl -b jar.txt -H "X-CSRF-Token: $TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"verified"}' https://relief.example.org/api/v1/requests/<id>/status
```

Browsers additionally send `Origin`, which must match the configured public
origin.

## Endpoint overview

| Area | Endpoints |
| --- | --- |
| Auth | `POST /auth/login`, `POST /auth/logout`, `GET /auth/session`, `POST /auth/change-password`, `GET /me`, `PATCH /me/availability` |
| Public | `GET /public/notice` |
| Settings | `GET /settings` |
| Requests | `GET/POST /requests`, `GET/PATCH /requests/{id}`, `POST /requests/{id}/status`, `POST /requests/{id}/reopen`, `POST /requests/{id}/protected`, `GET /requests/{id}/events`, `GET/POST /requests/{id}/notes` |
| Offers | `GET/POST /offers`, `GET/PATCH /offers/{id}`, `POST /offers/{id}/status`, `POST /offers/{id}/protected`, `GET /offers/{id}/events`, `GET/POST /offers/{id}/notes` |
| Assignments | `GET/POST /assignments`, `GET/PATCH /assignments/{id}`, `POST /assignments/{id}/status`, `POST /assignments/{id}/protected` |
| Volunteers | `GET /volunteers`, `PATCH /volunteers/{id}/availability` |
| Dashboard | `GET /dashboard` |
| Exports | `GET /exports/requests.csv`, `GET /exports/summary.json` |
| Sync | `POST /sync/push`, `GET /sync/pull` |
| Admin | `PUT /admin/settings`, `GET /admin/categories`, `PATCH /admin/categories/{code}`, `GET/POST /admin/users`, `PATCH /admin/users/{id}`, `POST /admin/users/{id}/reset-password`, `GET /admin/audit`, `GET /admin/audit/verify`, `GET /admin/retention`, `POST /admin/retention/apply`, `POST /admin/records/delete`, `GET /admin/demo`, `POST /admin/demo/seed` |

## Role-specific responses

Request, offer and assignment responses are projections for the caller:

- `viewer_relation` tells the client how the caller relates to the record
  (`coordinator`, `owner`, `assigned_volunteer` / `volunteer`);
- fields the caller may not see are omitted (e.g. `created_by` for
  volunteers);
- `permissions` lists allowed actions (`allowed_statuses`, `can_edit`,
  `can_reveal_protected`, `note_visibilities`, ...).

Protected values are only returned by the `.../protected` endpoints.

## Idempotency

`POST /requests`, `POST /offers`, `POST /assignments` and note creation accept
a `client_id` (UUID). Repeating a call with the same `client_id` returns the
existing record with status `200`. Sync operations are idempotent by `op_id`.

## TypeScript client

`packages/api-client` provides a typed client used by the web app:

```ts
import { createApiClient } from '@reliefmesh/api-client'
const api = createApiClient({ getCsrfToken: () => token })
const { items } = await api.requests.list({ open: true, sort: 'priority' })
```
