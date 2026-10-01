# Power outage exercise - northern villages

> Fictional scenario. EXERCISE - not a real emergency.

A multi-day power outage affects three villages. The Northfield community
centre has a generator and serves as a warming and charging point. Mobile
networks are unreliable.

## Learning objectives

1. Operate ReliefMesh primarily offline; understand the connection indicator
   and "Pending changes".
2. Plan water and food logistics with limited transport.
3. Organize non-medical check-in visits without collecting health data.
4. Handle synchronization conflicts calmly.

## Preparation

- Install the app on all devices while online (open it once).
- Plan a "connectivity schedule": devices are online only every 30 minutes
  (simulating a satellite or courier uplink at the community centre).

## Seeded situation (`seed-demo --scenario power-outage`)

| Request | Urgency | Status at start |
| --- | --- | --- |
| Warming space for elderly residents | high | verified |
| Charging for device batteries (sensitive, logistics only) | normal | submitted |
| Water for households with electric pumps (200 l) | high | verified |
| Door-to-door check-in visits (non-medical) | normal | submitted |
| Non-perishable food packages (15) | normal | submitted |

Offers: generator charging point (30 sockets), water tank trailer (1000 l),
food bank packages (40, protected pickup address).

## Inject timeline

| Time | Inject | Expected handling |
| --- | --- | --- |
| T+0 | All devices go offline | Indicator shows "Offline"; work continues |
| T+10 | Volunteers in villages create requests from door-to-door visits | Requests queue on devices ("Waiting to sync") |
| T+30 | Uplink window (5 min online) | Changes synchronize; coordinators review new requests |
| T+35 | Offline again; coordinator cancels a request that a volunteer just started offline | On next sync: conflict shown; resolved explicitly |
| T+60 | Uplink window | Assignments for water deliveries created online |
| T+70 | Volunteers deliver water offline, record handover | Deliveries synchronize at the next window |
| T+100 | Final uplink; coordinators resolve requests | Dashboard reflects the situation |
| T+110 | Debrief | Review "Pending changes" handling and timelines |

## Evaluation checklist

- [ ] No change made offline was lost.
- [ ] Every conflict was resolved deliberately (retry or discard).
- [ ] Check-in visit notes contained no health data.
- [ ] Water allocations never exceeded the trailer capacity.
- [ ] Participants could explain what "Server unreachable" means.
