# Exercise scenarios

Fictional scenarios for training with ReliefMesh. All names, places and needs
are invented. Each folder contains a facilitator guide with learning
objectives, an inject timeline and an evaluation checklist.

| Scenario | Seed name | Duration | Group size |
| --- | --- | --- | --- |
| [Flood - Riverside district](flood-exercise/README.md) | `flood` | 90-120 min | 6-15 |
| [Temporary shelter - school gymnasium](shelter-exercise/README.md) | `shelter` | 60-90 min | 5-12 |
| [Power outage - northern villages](power-outage-exercise/README.md) | `power-outage` | 120 min | 6-20 |

The machine-readable seed data lives in
[`apps/api/internal/demo/scenarios`](../apps/api/internal/demo/scenarios) and
is loaded with:

```bash
reliefmesh-api seed-demo --scenario <name>        # requires RELIEFMESH_ALLOW_DEMO_SEED=true
```

or under *Administration > Data & retention* on a demo instance. See
[docs/exercise-guide.md](../docs/exercise-guide.md) for general guidance.
