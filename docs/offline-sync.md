# Offline synchronization

ReliefMesh keeps working when the network does not: in shelters without
uplink, in basements, in rural areas. This document describes how.

## What works offline

| Works offline | Needs a connection |
| --- | --- |
| Starting the app (after the first visit) | Signing in for the first time on a device |
| Viewing requests, offers and assignments saved on the device | Dashboard numbers, notes and history |
| Creating requests and offers | Creating assignments (quantities must be checked live) |
| Adding notes | Revealing protected details |
| Request and offer status changes, request edits | Administration, exports, volunteer list |
| Assignment status changes and handover updates | |

## Building blocks

1. **Service worker** (Workbox, `apps/web/service-worker/sw.ts`) precaches the
   application shell. It never caches API responses.
2. **IndexedDB** (Dexie, `apps/web/utils/offline-db.ts`):
   - `outbox`: queued operations of the signed-in user;
   - `requests`, `offers`, `assignments`: records the user may see, scoped
     by user id, including local placeholders for records created offline;
   - `meta`: settings, session profile, sync cursors.
3. **Sync engine** (`apps/web/utils/sync-engine.ts`) pushes the outbox in order
   and pulls changes.
4. **API**: `POST /api/v1/sync/push` and `GET /api/v1/sync/pull`
   (`apps/api/internal/sync`).

## Connection state

"Online" means the ReliefMesh **server** is reachable - not just that the
device has a network. The app checks `/healthz` every 20 seconds, reacts to
browser `online`/`offline` events and marks the server unreachable as soon as
a request fails. The header shows *Online*, *Offline* or *Server unreachable*,
plus the number of changes waiting and needing attention. Status changes are
announced to screen readers.

## Operations

Each queued operation has:

```json
{
  "op_id": "uuid (idempotency key)",
  "type": "request.create | request.update | request.status | request.note | offer.create | offer.status | offer.note | assignment.status | assignment.update",
  "entity_id": "server id (when known)",
  "entity_client_id": "client id of a record created offline (when the server id is not known yet)",
  "payload": { "...": "same body as the corresponding REST endpoint" },
  "client_created_at": "ISO timestamp"
}
```

Creates carry a `client_id`. The server stores the result of every operation
by `op_id`, so retrying after a timeout never applies an operation twice, and a
`client_id` can only ever create one record.

## Push algorithm

1. Take pending operations of the current user, oldest first (batches of 50).
2. `POST /sync/push`. The server applies them **in order** using the same
   services and authorization as the REST API.
3. Per result:
   - `applied` / `duplicate`: remove from the outbox, store the returned
     entity, remove the local placeholder;
   - `conflict`: keep, mark as conflict, store the current server version;
   - `rejected` (validation or permission): keep, mark as rejected with the
     reason - **unless** it depends on a record still waiting in the outbox
     (e.g. a note on an offline-created request whose creation failed), in
     which case it stays pending;
   - `retry` (server error): keep pending, try later.
4. Network error: stop, keep everything. Session expired: stop, ask the user to
   sign in again; the outbox survives and is pushed afterwards.

Flushes happen when the connection returns, every 30 seconds while changes are
pending, when the app becomes visible and when the user taps *Sync now*.

## Conflict policy

| Operation | Policy |
| --- | --- |
| Creates, notes | Always appended; no conflicts possible. |
| Status changes | Intent-based: applied if the transition is still allowed from the current server status; otherwise a conflict with the current version (e.g. someone else already cancelled the request). Setting the status a record already has is treated as success. |
| Field updates | Optimistic concurrency: the `version` the user edited must match, otherwise conflict. |

**Nothing is discarded silently.** Conflicting and rejected operations stay in
*Pending changes* until the user chooses *Try again* or *Discard* (with
confirmation; dependent operations are listed and discarded together). Users
can copy an operation's details before discarding it.

## Pull

`GET /sync/pull?since=<server_time>` returns records visible to the user that
changed since the cursor (with a 10-second overlap), plus settings and
categories. Without `since` (first sync, or every 6 hours) the server returns
everything visible and the client replaces its cache, so records the user may
no longer see (e.g. a cancelled assignment) disappear from the device.

## Multiple accounts on one device

Operations are stored with the user id. Only the signed-in user's operations
are pushed. Changes of another account are shown in *Pending changes* and are
sent when that account signs in again, or can be discarded explicitly.

## Privacy rules for offline data

- Protected values revealed through the audited endpoints are never stored.
- Sensitive descriptions are not part of list responses and are cached only
  for records the user opened.
- Signing out clears the cache (the outbox only after confirmation).

## Testing

- Unit tests with fake IndexedDB: `apps/web/test/sync-engine.test.ts`.
- API tests: `TestSyncPushIdempotencyAndConflicts`, `TestSyncPullScopesData`.
- End-to-end: `apps/web/e2e/offline.spec.ts` goes offline in Chromium, reloads
  the app from the service worker, creates a request and a task update offline
  and verifies automatic synchronization after reconnecting.
