## Summary

<!-- What does this change and why? Link related issues. -->

## Type of change

- [ ] Bug fix
- [ ] Feature
- [ ] Documentation
- [ ] Refactoring / maintenance
- [ ] Security hardening

## Safety and privacy checklist

- [ ] Does not add dispatching, automatic matching/prioritization, medical assessment, routing or "safe destination" claims
- [ ] No new personal data, **or** the new data is justified, access-controlled on the server, covered by retention and documented in `docs/privacy-model.md`
- [ ] Authorization enforced in the API with tests for allowed and denied roles
- [ ] State changes and access to protected data create audit events (without personal data in metadata)
- [ ] No telemetry, analytics, third-party requests or tracking
- [ ] Offline behavior considered (queued via the sync outbox where appropriate; conflicts surfaced)
- [ ] Accessible UI (labels, keyboard, text + icon instead of color only, target size)

## Tests

- [ ] `make lint`
- [ ] `make test`
- [ ] `make test-e2e` (if UI or sync behavior changed)
- [ ] OpenAPI and shared types updated (if the API changed)

## Screenshots

<!-- Use demo data only. Never include real personal data. -->
