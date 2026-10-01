# Flood exercise - Riverside district

> Fictional scenario. EXERCISE - not a real emergency.

A river flood has displaced residents of the Riverside district. The community
hall operates as a temporary shelter, the sports hall hosts families
overnight. A municipal depot and local businesses offer supplies.

## Learning objectives

1. Review and verify incoming requests consistently; use urgency deliberately.
2. Allocate from offers without over-allocation; react to the warning.
3. Share protected details (address, contact) only when needed and only with
   the assigned volunteer.
4. Distinguish "delivered" from "resolved" and confirm outcomes.
5. Keep working through a network outage.

## Seeded situation (`seed-demo --scenario flood`)

| Ref. | Request | Urgency | Status at start |
| --- | --- | --- | --- |
| 1 | Drinking water for community hall shelter (60 people) | high | verified |
| 2 | Blankets for overnight stay (sports hall) | normal | assigned, accepted by Vera Volunteer |
| 3 | Phone charging station at shelter | normal | submitted |
| 4 | Transport from Mill Street to shelter (wheelchair, protected address) | high | under review |
| 5 | Diapers and baby food (sensitive) | normal | submitted |
| 6 | Medicine pickup (logistics only, authorization required) | normal | submitted |
| 7 | Interpreter for information session | low | verified |
| 8 | Warm meals for volunteers | normal | resolved |

Offers: bottled water from the depot (400 l, protected pickup address),
blankets from a clothing store (50), power banks (20), accessible van (1 trip).

## Roles

| Participant | Account |
| --- | --- |
| Coordinator(s) | `demo-coordinator`, `demo-manager` |
| Volunteers | `demo-volunteer-1..3` |
| Inject team (requesters) | `demo-requester-1`, `demo-requester-2` |
| Exercise control (optional) | `demo-admin` |

Password for all demo accounts: `reliefmesh-demo-exercise`.

## Inject timeline

| Time | Inject (by inject team) | Expected handling |
| --- | --- | --- |
| T+0 | Briefing: emergency notice, roles, exercise banner | Everybody can explain why ReliefMesh is not an emergency channel |
| T+5 | Coordinators review requests 3-6 | Requests verified, merged or rejected with reasons; urgency checked |
| T+15 | Requester 1 submits "Drinking water for 12 more people at sports hall" (normal) | New request reviewed; allocation from depot offer |
| T+20 | Requester 2 submits the transport request **again** | Detected as duplicate of the Mill Street request (reference) |
| T+25 | Coordinator assigns the accessible van to the transport request with destination grant | Volunteer accepts, reveals the address (audited), starts |
| T+35 | Inject: depot reports only 100 l left - coordinator reduces the offer quantity | Quantity cannot go below what is allocated; remaining allocations adjusted |
| T+40 | **Network outage**: switch the access point off for 15 minutes | Requesters keep submitting, volunteers mark tasks started/delivered offline |
| T+55 | Network restored | Offline changes synchronize; conflicts handled in "Pending changes" |
| T+60 | Requester 1 marks critical: "water entering the basement, people trapped" | Participants recognize a real-life-type emergency: **stop and call emergency services** (in the exercise: report to the director). ReliefMesh shows the critical notice |
| T+70 | Coordinators confirm deliveries and resolve or partially resolve requests | Each delivered assignment leads to a resolution decision |
| T+85 | Organization manager exports the summary report | Report contains no personal data |

## Evaluation checklist

- [ ] All submitted requests reviewed within 15 minutes.
- [ ] The duplicate transport request linked to the original.
- [ ] No allocation exceeded offer quantities; the warning was understood.
- [ ] Protected address opened only by the coordinator and the assigned
      volunteer (check the request history and audit log).
- [ ] Contact details not shared with volunteers unless the requester allowed it.
- [ ] Offline changes synchronized without loss; conflicts resolved explicitly.
- [ ] The "trapped people" inject was escalated to emergency services, not
      handled in ReliefMesh.
- [ ] Delivered requests were resolved by coordinators with a summary.
