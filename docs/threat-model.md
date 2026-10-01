# Threat model

This document describes what ReliefMesh protects, against whom, and how. It
follows a STRIDE-style structure and lists residual risks honestly.

## Assets

| Asset | Why it matters |
| --- | --- |
| Personal data of requesters | People in need may be vulnerable; location and contact data could enable harm (stalking, theft, exploitation of displaced persons). |
| Exact locations and contact details | Highest sensitivity; encrypted at rest. |
| Operational integrity | Wrong statuses or forged assignments could misdirect volunteers or hide unmet needs. |
| Audit trail | Accountability for access to personal data and for decisions. |
| Volunteer identities | Volunteers should not be exposed to requesters or the public. |
| Availability | The tool must keep working during network disruption. |

## Actors

- **Legitimate users** in five roles (see [role-model.md](role-model.md)).
- **Curious insider**: a user trying to see more than their role allows (e.g. a volunteer browsing requests).
- **Malicious insider with admin rights**: wants to read personal data or cover tracks.
- **External attacker** on the internet or local network.
- **Device thief / finder**: obtains an unlocked or locked phone used for ReliefMesh.
- **Server operator / hosting provider** with database or backup access.

## Trust boundaries

1. Browser <-> reverse proxy (TLS).
2. Reverse proxy <-> API (internal network).
3. API <-> PostgreSQL (internal network, credentials).
4. Device storage (IndexedDB, service worker cache).

## Threats and mitigations

### Spoofing

| Threat | Mitigation |
| --- | --- |
| Password guessing | Argon2id hashes; per-IP rate limit (token bucket); account lockout for 15 minutes after 5 failures; generic error messages; timing equalization for unknown users. |
| Session theft | 256-bit random session tokens; only SHA-256 hashes stored; HTTP-only, `Secure`, `SameSite=Strict` cookie with `__Host-` prefix; absolute (24 h) and idle (8 h) expiry; sessions revoked on password change, password reset, role change and deactivation. |
| Cross-site request forgery | Session-bound CSRF token (HMAC) required in `X-CSRF-Token` for every state change; `Origin`/`Referer` check (also for login, against login CSRF); SameSite=Strict. |
| Default credentials | No default accounts; the first administrator is created explicitly (`bootstrap-admin`) with a strong or one-time password. Demo personas exist only when demo mode is explicitly enabled. |

### Tampering

| Threat | Mitigation |
| --- | --- |
| Invalid state transitions | Server-side state machines; database `CHECK` constraints. |
| Over-allocation of offers | Row lock (`SELECT ... FOR UPDATE`) plus database constraint `assigned_quantity <= quantity_available`; tested with 25 concurrent allocations. |
| Lost updates | Optimistic concurrency (`version`) for field updates; explicit conflicts. |
| Audit log manipulation | Append-only via triggers (UPDATE/DELETE/TRUNCATE rejected); SHA-256 hash chain with gapless ids; integrity check in the admin UI and API. A database superuser can still disable triggers - the hash chain then detects modifications (tested). Truncating the newest events is detectable only by comparing with an external copy, so keep backups. |
| Ciphertext swapping | AES-GCM additional data binds each ciphertext to its table, row id and column. |
| Malicious input | Strict JSON decoding (unknown fields rejected), length limits, control-character filtering, parameterized SQL only, CSV formula injection neutralized in exports. |

### Repudiation

Every status change, assignment, grant change, access to protected data,
export, login, user and settings change is recorded with actor id, roles and
time. Requesters see in their own timeline when their protected data was
opened (with role labels).

### Information disclosure

| Threat | Mitigation |
| --- | --- |
| Volunteer browses requests | Volunteers only see requests and offers linked to their own non-declined assignments, in a reduced projection without requester identity. |
| Volunteer reads contact/exact location | Only while the assignment is active, only with an explicit grant by a coordinator (policy configurable), only if the requester allowed responders, and every access is audited. |
| Administrator reads personal data | Administrators have no operational capabilities: request and offer endpoints return 403/404; the dashboard shows aggregates only; the audit log contains no personal data; deletion works by reference without displaying content. |
| Database or backup leak | Exact locations and contact details are AES-256-GCM encrypted with keys outside the database; session tokens stored as hashes; passwords as Argon2id. Free text (titles, descriptions, notes) is not encrypted - see residual risks. |
| Leaks via logs | Access logs contain method, route pattern, status, duration, request id and user id only; no bodies, query strings or headers. Caddy log filters remove query strings, headers and client IPs. |
| Leaks via exports | Exports contain no names, contact data, coordinates, free text or notes; area labels of sensitive requests are omitted; exports are restricted to organization managers and audited. |
| Leaks via device storage | Revealed protected data is kept in memory only. Sensitive descriptions are not included in list responses (thus not cached). The cache is scoped per user and cleared on logout. The service worker caches only the app shell, never API responses. |
| Leaks via third parties | No CDNs, web fonts, analytics or telemetry; strict CSP with `connect-src 'self'`; `Referrer-Policy: no-referrer`. |
| Enumeration | Invisible records return 404; login errors are generic. |

### Denial of service

Body size limits (1 MiB, 4 MiB for sync), server timeouts, bounded Argon2id
concurrency, login rate limiting, paginated lists (max 500). Volumetric attacks
must be handled at the network level. The offline-first client keeps working
when the server is unreachable.

### Elevation of privilege

Capabilities are checked in every service method; route-level checks are
defense in depth. Integration tests cover allowed and denied roles for critical
endpoints. Administrators cannot remove their own admin role or deactivate the
last administrator.

## Residual risks and operator responsibilities

- **Free text may contain personal data.** Titles, descriptions, notes and
  reasons are stored in plain text in the database (to keep search and review
  practical). The UI warns against entering health or contact data, medicine
  pickup requests reject free text, and retention redacts free text after the
  configured period. Protect database access and backups accordingly.
- **Encryption keys live with the operator.** Anyone with both the database and
  `RELIEFMESH_DATA_ENCRYPTION_KEYS` can decrypt protected fields. Store keys
  separately from backups; restrict server access.
- **Lost or shared devices.** Cached records remain readable on an unlocked
  device until logout or session expiry. Train users to sign out on shared
  devices and to use device screen locks.
- **Rate limiting is per instance.** It is held in memory; run one API
  instance or add rate limiting at the proxy.
- **Exercise data in production.** Use exercise mode and separate instances;
  never enable demo mode on an instance with real data.
- **Social engineering.** Password resets are manual by design; administrators
  must verify identities out of band before resetting passwords.

## Security testing

- Integration tests: authorization matrix, CSRF/origin, lockout, temporary
  passwords, audit immutability and tamper detection, encryption at rest,
  export privacy, concurrent allocation.
- CI: `go vet`, staticcheck, govulncheck, CodeQL (Go and TypeScript),
  dependency review.
