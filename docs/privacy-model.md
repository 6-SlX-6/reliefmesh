# Privacy model

ReliefMesh is built on data minimization. This document explains what is
stored, who can see it, how long it is kept and which choices the operator
makes. It is not legal advice; operators remain responsible for compliance
with applicable law (for example the GDPR) and should document their own
processing (purpose, legal basis, retention, contact).

## Principles

1. **Collect only what is needed to organize help.** No identity documents, no
   dates of birth, no health data, no photos, no biometrics.
2. **Least privilege by default.** Every role sees the minimum required.
3. **Protected data is encrypted and access is transparent.** Exact locations
   and contact details are encrypted at rest; each access is audited and shown
   to the person concerned.
4. **Data stays local.** The operator owns the data. No telemetry, analytics,
   tracking, advertising, third-party fonts or external API calls. No data
   leaves the instance unless an administrator exports a non-personal report.
5. **Limited retention.** Closed records are redacted after a configurable
   period (default 90 days).

## Data inventory

| Data | Purpose | Stored as | Visible to |
| --- | --- | --- | --- |
| Account: username, display name, roles, password hash | Sign-in, attribution | Plain (hash: Argon2id) | Admins (no hashes); coordinators see display names of volunteers |
| Volunteer availability and short note | Assignment planning | Plain | Coordinators, organization managers, the volunteer |
| Request content: category, urgency, title, description, quantity, people affected, needed-by, accessibility needs | Organize help | Plain | Requester (own), coordinators; assigned volunteers (task-relevant fields only) |
| Area label and approximate coordinates | Rough planning | Plain; coordinates rounded (default 2 decimals, about 1.1 km) | As above |
| Exact location (address, directions, coordinates) | Delivery when needed | **AES-256-GCM** | Requester; coordinators; assigned volunteer only with an explicit destination grant during an active assignment; every reveal audited |
| Contact method and details | Reaching the requester | Method plain; details **AES-256-GCM** | Requester; coordinators (unless "do not contact"); assigned volunteer only if the requester allowed responders **and** a coordinator granted access; audited |
| Notes | Coordination | Plain, append-only | Per note: internal (coordinators), responders (plus assigned volunteers), shared (plus requester/offer owner) |
| Offers: content, quantities, location, contact | Resource planning | As for requests | Offer owner, coordinators; pickup volunteers see logistics and (with grant) the exact pickup location |
| Assignments: instructions, grants, ETA, handover, evidence type | Execution | Plain | Coordinators, the assigned volunteer |
| Audit events | Accountability | Append-only, hash-chained; ids, enums, quantities and short reasons only | Filtered timelines for involved people; full log for admins |
| Sessions | Sign-in | Token hash, timestamps (no IP, no user agent) | Nobody |

### Deliberately not collected

IP addresses in the database or API logs, user agents, precise device
locations (the "use my location" button rounds coordinates on the device
before anything is stored or sent), photos, identity documents, diagnoses,
medication names, prescriptions.

## Location privacy

| Mode | Stored | Shown |
| --- | --- | --- |
| `none` | nothing | nothing |
| `area_only` | free-text area (district, village, shelter) | area |
| `approximate` | coordinates rounded to the instance precision (0-3 decimals; default 2 = about 1.1 km) | rounded coordinates and area |
| `protected_exact` | encrypted address/directions/coordinates, plus a derived rounded approximation | area and rounded coordinates; exact data only via audited reveal |

The precision is configurable by administrators. Anything finer than three
decimals (about 110 m) must use `protected_exact`. There are no public maps.

## Sensitive requests and medicine pickup

- Any request can be flagged **sensitive**: its description and accessibility
  notes are omitted from list responses (and therefore from device caches) and
  shown only on the detail view to authorized viewers; dashboard queues show a
  generic title.
- **Medicine pickup** is handled as logistics only. The server replaces the
  title with "Medicine pickup (logistics only)", rejects free-text descriptions
  (unless an administrator explicitly allows them), marks the request and its
  notes as sensitive and records only whether formal authorization is needed.
  No medical validation is performed.

## Device storage

- The service worker caches the app shell only.
- IndexedDB holds (a) the outbox of unsynchronized changes and (b) a cache of
  records the signed-in user may see, scoped by user id.
- Revealed protected data is never written to storage; it stays in memory and
  is cleared when leaving the page.
- Signing out clears the cache. Unsynchronized changes are kept only after an
  explicit confirmation so nothing is lost silently; they are only sent for
  the account that created them.

## Retention and deletion

- **Retention** (default 90 days after closing): titles, descriptions,
  accessibility notes, area labels, coordinates, protected fields, notes,
  instructions and handover notes are wiped; category, urgency, status,
  quantities and timestamps remain for statistics. Administrators preview and
  apply retention in the UI; operators can schedule `reliefmesh-api
  apply-retention`.
- **Deletion on request**: administrators delete a request or offer by
  reference with a mandatory reason. Content is wiped and the record hidden;
  the audit trail keeps the fact of deletion. Administrators never see the
  content in this flow.
- **Accounts** are deactivated rather than deleted so the audit trail stays
  attributable. Display names can be changed to pseudonyms.

### Audit log and erasure

The audit log is immutable by design (accountability). It therefore stores no
free-text content of requests, offers or notes - only identifiers, actions,
statuses, quantities and short reasons. UI hints ask users not to put personal
data into reasons. Operators should document this trade-off in their records
of processing.

## Exports

Organization managers can export a request list (CSV) and a summary (JSON).
Exports exclude names, contact details, coordinates, free text and notes;
areas of sensitive requests are omitted. Each export is audited.

## Operator checklist

- [ ] Configure HTTPS (Caddy does this automatically).
- [ ] Set the emergency notice for your country.
- [ ] Choose location precision and retention period.
- [ ] Keep `volunteer_access_requires_grant` enabled.
- [ ] Store `RELIEFMESH_DATA_ENCRYPTION_KEYS` separately from backups.
- [ ] Encrypt backups and restrict who can access the server.
- [ ] Document your processing and inform requesters (purpose, contact,
      retention) - for example on a printed sheet at the shelter desk.
- [ ] Never use demo mode with real data.
