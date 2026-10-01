# Contributing to ReliefMesh

Thank you for helping! ReliefMesh supports people in difficult situations, so we
value safety, privacy and clarity over feature count.

## Ground rules

1. **Respect the product boundaries.** ReliefMesh is not an emergency service.
   Contributions must not add automatic dispatching or matching, automated triage,
   medical advice or assessment, routing or "safe destination" claims, public
   maps, social sharing or integrations with emergency dispatch systems. See
   [docs/emergency-disclaimer.md](docs/emergency-disclaimer.md) and
   [docs/roadmap.md](docs/roadmap.md).
2. **Privacy first.** Collect only what a feature needs. New personal or
   sensitive fields need a justification, access control on the server, a
   retention rule and documentation in [docs/privacy-model.md](docs/privacy-model.md).
   Never add telemetry, analytics, third-party fonts, CDNs or tracking.
3. **Authorization lives on the server.** Every new endpoint needs explicit
   permission checks and integration tests for allowed *and* denied roles.
   Records a caller may not see must return `404`.
4. **Audit everything that matters.** State changes, access to protected data
   and administrative actions must record an audit event without personal data
   in its metadata.
5. **Offline is a first-class mode.** If a user-facing mutation should work in
   the field, route it through the sync outbox and handle conflicts explicitly.
6. **Accessible and calm UI.** Large targets, text plus icons (never color
   alone), labels for every control, keyboard support, clear confirmations.
7. **No real data.** Never paste real personal data from operations or
   exercises into issues, pull requests, fixtures or screenshots.

## Getting started

```bash
pnpm install
make dev-db
make api-run      # terminal 1
make web-dev      # terminal 2
```

See the [README](README.md#development) for details.

## Before you open a pull request

```bash
make lint         # gofmt, go vet, eslint, TypeScript
make test         # Go unit + integration tests, web unit tests
make test-e2e     # Playwright (needs PostgreSQL)
make openapi      # route <-> OpenAPI contract
```

- Update `apps/api/openapi/openapi.yaml` for every API change (the contract test
  fails otherwise) and the shared types in `packages/shared-types`.
- Database changes go into a **new** migration file in `apps/api/migrations`;
  never edit a released migration.
- Update the documentation in `docs/` when behavior changes.
- Keep pull requests focused; describe the user-facing effect and the privacy
  and safety impact (the PR template asks for it).

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):
`feat(api): ...`, `fix(web): ...`, `docs: ...`, `test: ...`, `chore: ...`.

## Translations

User-facing labels for domain values live in `apps/web/utils/labels.ts`. A full
i18n setup is on the roadmap; please open an issue before starting a
translation so we can coordinate the approach.

## Reporting security issues

Please do **not** open public issues for vulnerabilities. Follow
[SECURITY.md](SECURITY.md).

## License

By contributing you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
