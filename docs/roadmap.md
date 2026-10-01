# Roadmap

ReliefMesh grows carefully. Every feature must keep the product within its
boundaries (see [emergency-disclaimer.md](emergency-disclaimer.md)) and pass a
privacy review.

## v0.1.0 (current)

Single organization per instance; local accounts; five roles; requests,
offers, assignments, notes; lifecycle tracking; audit chain; protected data
encryption; offline PWA with sync; dashboard; non-personal exports;
retention; exercise scenarios; Docker Compose deployment.

## Candidates for future releases

These are documented ideas, **not implemented**:

- **Translations** (German first) with a full i18n framework and right-to-left
  support.
- **Optional self-registration** for requesters with coordinator approval and
  rate limiting.
- **Multi-organization instances** with strict data separation and explicit,
  audited sharing of individual requests between organizations.
- **Outbound synchronization / federation** between instances of cooperating
  organizations - only when an administrator configures it explicitly, with
  minimal data and per-request consent.
- **Notifications** via self-hosted channels (e.g. Web Push with VAPID keys
  held by the operator) - opt-in, no content in notifications.
- **Inventory tracking** for depots (stock levels per offer location).
- **Printable intake forms and handover sheets** for paper fallback.
- **Encrypted local cache** using a device passphrase.
- **Offline sign-in** on devices that were used before (with local key
  derivation).
- **Rate limiting backed by the database** for multi-instance deployments.
- **Mesh and radio transports** (LoRa, Meshtastic-style gateways) for exchanging
  sync batches where no IP network exists - research topic only.

## Out of scope by design

- Automatic matching, prioritization or dispatching of help.
- Automated or algorithmic triage, and any medical diagnosis, assessment or
  advice.
- Routing, navigation or safety assessments of places.
- Public maps or public request boards.
- Social media integration and sharing features.
- Payment processing.
- Facial recognition, biometrics, identity verification with government
  documents.
- Integration with police, ambulance, fire brigade, military or emergency
  dispatch systems.
- Cloud-only features, telemetry and analytics.
