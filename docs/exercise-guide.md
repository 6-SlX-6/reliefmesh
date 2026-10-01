# Running an exercise with ReliefMesh

ReliefMesh is well suited for tabletop and field exercises of community aid
groups, shelter teams and civil-protection training. This guide describes how
to prepare, run and evaluate an exercise.

## Before the exercise

1. **Use a separate instance** for exercises. Enable *exercise mode* so every
   screen shows the banner (default text: "EXERCISE - not a real emergency").
2. **Decide on data**: either load a prepared scenario (demo mode) or enter
   invented data during the exercise. Never use real personal data.
3. **Prepare accounts**: one per participant, with the role they play. In
   demo mode the following personas exist (password
   `reliefmesh-demo-exercise`):

   | Username | Role |
   | --- | --- |
   | `demo-coordinator` | Coordinator |
   | `demo-manager` | Organization manager |
   | `demo-volunteer-1` (available), `demo-volunteer-2` (limited), `demo-volunteer-3` (available) | Volunteers |
   | `demo-requester-1`, `demo-requester-2` | Requesters |
   | `demo-admin` | Administrator |

4. **Set the emergency notice** to the real local emergency number. Brief all
   participants: in a real emergency during the exercise, stop and call the
   emergency services.
5. **Test devices**: open the app once on every device while online so the
   service worker caches it; try the offline mode (airplane mode) once.

### Loading a scenario

Demo mode (`RELIEFMESH_ALLOW_DEMO_SEED=true`) on an exercise instance:

```bash
docker compose -f infrastructure/compose/docker-compose.demo.yml up -d --build   # loads "flood"
# further scenarios: Administration > Data & retention, or
docker compose -f infrastructure/compose/docker-compose.demo.yml exec api reliefmesh-api seed-demo --scenario shelter
```

| Scenario | Folder | Focus |
| --- | --- | --- |
| `flood` | [examples/flood-exercise](../examples/flood-exercise/README.md) | Review queue, allocation from depots, protected addresses, accessible transport |
| `shelter` | [examples/shelter-exercise](../examples/shelter-exercise/README.md) | Shelter capacity, sensitive requests, pets, hygiene |
| `power-outage` | [examples/power-outage-exercise](../examples/power-outage-exercise/README.md) | Offline operation, water and food logistics, check-in visits |

Each scenario folder contains a timeline of injects for the exercise control
team, learning objectives and an evaluation checklist.

## During the exercise

Suggested roles in the exercise control team:

- **Exercise director**: starts/stops phases, watches safety.
- **Inject team**: plays requesters (creates requests, adds updates, calls the
  "shelter desk"), plays offer donors.
- **Observers**: use the evaluation checklist, note timestamps.

Recommended phases (90-120 minutes):

1. *Intake (20 min)*: requesters submit requests; coordinators review and
   verify. Observe review times and use of urgency.
2. *Allocation (30 min)*: coordinators assign offers and volunteers;
   volunteers accept, start and deliver.
3. *Disruption (20 min)*: switch selected devices to airplane mode or
   disconnect the access point. Participants continue working offline.
   Restore the connection and observe synchronization and conflict handling.
4. *Closure (20 min)*: coordinators confirm resolutions, handle duplicates and
   cancellations, review the dashboard.

## After the exercise

- **Debrief** using the dashboard, request timelines and the audit log.
- **Reports**: organization managers export the non-personal CSV and summary.
- **Reset**: drop the exercise instance (`docker compose -f
  docker-compose.demo.yml down -v`) or apply retention.

## Evaluation checklist (generic)

- Were all critical requests reviewed within the agreed time?
- Was urgency assigned by people and changed with a recorded reason?
- Were protected details opened only when needed (check the audit log)?
- Did volunteers receive only the information they needed?
- Did every delivered assignment lead to a confirmed resolution - or to a
  follow-up?
- Were offline changes synchronized without loss? How were conflicts handled?
- Were duplicates detected and linked to the canonical request?
- Did anyone treat ReliefMesh as an emergency channel? (Brief again if so.)
