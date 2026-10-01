# Security Policy

ReliefMesh processes information about people who may be in vulnerable
situations. We take security reports seriously and appreciate responsible
disclosure.

## Supported versions

| Version | Supported |
| --- | --- |
| 0.1.x | Yes |

## Reporting a vulnerability

**Do not open a public issue.** Instead, use GitHub's private vulnerability
reporting: *Security > Report a vulnerability* on the repository page.

Please include:

- affected version or commit, and deployment type (Compose, binary, ...);
- a description of the issue and its impact (e.g. which role can access what);
- steps to reproduce or a proof of concept;
- whether personal data could be exposed.

**Never include real personal data** in a report. Use the demo scenarios or
synthetic data.

We aim to acknowledge reports within **5 working days**, to provide an initial
assessment within **10 working days**, and to coordinate a fix and disclosure
date with you. We will credit reporters who wish to be named.

## Scope

In scope:

- the Go API (`apps/api`), the web app (`apps/web`) and shared packages;
- the provided deployment configuration (`infrastructure/`);
- authorization bypasses (e.g. a volunteer reading unassigned requests, an
  administrator reading operational personal data), CSRF, session handling,
  injection, cryptographic weaknesses, audit log tampering that is not
  detected, data leaks via exports, logs, caches or the service worker.

Out of scope:

- vulnerabilities in third-party dependencies without a demonstrated impact on
  ReliefMesh (please report them upstream);
- deployments that ignore the documented security requirements (e.g. no HTTPS,
  demo mode with real data, default secrets);
- denial of service through volumetric attacks;
- social engineering of operators.

## Security design summary

- Argon2id password hashing, HTTP-only `SameSite=Strict` session cookies with
  `__Host-` prefix over HTTPS, CSRF tokens bound to the session, Origin checks.
- Login rate limiting per IP and account lockout after repeated failures.
- Server-side role-based authorization; invisible records return `404`.
- Exact locations and contact details encrypted with AES-256-GCM, bound to
  their row and column; key rotation supported.
- Append-only, hash-chained audit log with database triggers and an
  integrity check.
- Strict Content-Security-Policy without third-party origins; no telemetry.

Details: [docs/threat-model.md](docs/threat-model.md).
