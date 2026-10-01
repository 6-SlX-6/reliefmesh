# Temporary shelter exercise - school gymnasium

> Fictional scenario. EXERCISE - not a real emergency.

After an evacuation, the West school gymnasium opens as a temporary shelter.
The shelter team coordinates places, hygiene kits, pet care and accessibility
needs.

## Learning objectives

1. Handle sensitive requests respectfully: minimal data, sensitive flag, no
   health details.
2. Track shelter capacity as offers with quantities ("places").
3. Use notes with the right visibility (internal vs. shared with requester).
4. Close the loop with requesters through shared notes and clear statuses.

## Seeded situation (`seed-demo --scenario shelter`)

| Request | Urgency | Status at start |
| --- | --- | --- |
| Shelter places for 8 people tonight | high | verified |
| Hygiene kits (30) | normal | submitted |
| Pet food and crates | low | submitted |
| Accessible cot and step-free sleeping area (sensitive) | normal | under review |
| Printed information sheet on return times | low | verified |

Offers: 20 field cots (on site), 25 hygiene kits (pickup, market square),
10 bags of pet food (delivery).

## Inject timeline

| Time | Inject | Expected handling |
| --- | --- | --- |
| T+0 | Briefing; shelter desk role explained | Contact method "via shelter desk" understood |
| T+5 | Allocate 8 of 20 cots to the shelter request | Offer shows 12 remaining; request assigned |
| T+15 | Requester adds a shared note with a diagnosis | Coordinator explains the privacy rule; no medical details in notes |
| T+20 | Hygiene request needs 30, offer has 25 | Partial allocation; request partially resolved; follow-up request or offer |
| T+30 | A family arrives with a dog | Pet request verified and assigned to a volunteer with delivery |
| T+40 | Volunteer cannot complete the pet food delivery | "Unable to complete" with reason; allocation released; reassignment |
| T+50 | Shelter closes the information sheet request as resolved | Resolution summary recorded |
| T+60 | Debrief using the request histories | Who saw what and when is traceable |

## Evaluation checklist

- [ ] Sensitive request details never appeared in lists or exports.
- [ ] Medical details were removed or avoided.
- [ ] Cot capacity matched allocations at all times.
- [ ] "Unable to complete" released quantities and led to reassignment.
- [ ] Requesters were informed through shared notes or status changes.
