# Data model

The schema is defined in [`apps/api/migrations/0001_init.sql`](../apps/api/migrations/0001_init.sql).
All operational tables carry `organization_id` (one organization per instance
in v0.1.0) so future multi-organization support needs no data migration.

## Entities

```
organizations 1--n users
organizations 1--n aid_requests 1--n assignments n--1 offers
                    aid_requests 1--n notes      offers 1--n notes
users (volunteer) 1--n assignments
audit_events (append-only, references entities by id)
sessions, instance_settings (1 row), categories, reference_counters, sync_operations
```

## aid_requests

| Field | Notes |
| --- | --- |
| `id`, `reference` | UUID and human-readable `RM-YYYY-NNNNNNN` (per-year counter) |
| `organization_id`, `created_by_user_id`, `client_id` | `client_id` makes offline creation idempotent |
| `status` | see lifecycle below |
| `category`, `urgency` | urgency: `critical`, `high`, `normal`, `low` - always chosen by people |
| `title`, `description` | for `medicine_pickup` the title is fixed and the description empty |
| `estimated_people_affected`, `requested_quantity`, `requested_unit`, `requested_by_time` | |
| `location_mode`, `area_label`, `approx_lat`, `approx_lon`, `approx_decimals` | see [privacy-model.md](privacy-model.md) |
| `exact_location_sealed` | AES-256-GCM ciphertext (protected) |
| `accessibility_notes` | |
| `contact_visibility`, `contact_method`, `contact_details_sealed` | details encrypted |
| `sensitive_data_flag`, `requires_formal_authorization` | |
| `review_status`, `verification_level` | coordinator review |
| `assigned_team`, `assigned_user_id` | optional responsible team / lead |
| `resolution_summary`, `closed_at`, `expiry_at`, `tags` | `expiry_at` is a review deadline; nothing expires automatically |
| `duplicate_of_request_id` | required when status is `duplicate` (constraint) |
| `version` | optimistic concurrency |
| `redacted_at`, `deleted_at` | retention / deletion |

The event history is served from `audit_events` (`GET /requests/{id}/events`).

### Request lifecycle

| From | Coordinator may set | Owner may set | Assigned volunteer may set |
| --- | --- | --- | --- |
| draft | submitted, cancelled | submitted | - |
| submitted | under_review, verified, cancelled, duplicate, expired | - | - |
| under_review | verified, cancelled, duplicate, expired | - | - |
| verified | assigned, under_review, cancelled, duplicate, expired | - | - |
| assigned | in_progress, partially_resolved, resolved, verified, cancelled, expired | - | in_progress (active assignment) |
| in_progress | partially_resolved, resolved, assigned, cancelled, expired | - | - |
| partially_resolved | in_progress, assigned, resolved, cancelled, expired | - | in_progress |
| resolved, cancelled, expired, duplicate | reopen -> under_review (reason required) | - | - |

Additional rules: `cancelled` needs a reason, `resolved` a resolution summary,
`duplicate` the canonical reference (which must not itself be a duplicate),
`assigned` an active assignment or a responsible team. Closing a request with
active assignments requires explicit confirmation and cancels them (releasing
allocations). Every transition writes an audit event.

## offers

Same structure as requests for content, location and contact, plus
`quantity_available`, `unit`, `availability_start/end`,
`pickup_or_delivery_mode` (`pickup`, `delivery`, `either`, `on_site`),
`restrictions`, `assigned_quantity` and the generated column
`remaining_quantity = quantity_available - assigned_quantity`.

Constraint `offers_allocation_bounds`: `0 <= assigned_quantity <= quantity_available`.

| Status | Set by |
| --- | --- |
| draft | creator |
| available, partially_allocated, fully_allocated | derived from quantities while the offer is active |
| paused | owner or coordinator |
| expired | coordinator |
| cancelled | coordinator; owner only without allocations |

## assignments

| Field | Notes |
| --- | --- |
| `request_id`, `offer_id?`, `volunteer_user_id?`, `team_label` | at least one of offer, volunteer or team |
| `assigned_by_user_id`, `assigned_at` | |
| `status` | lifecycle below |
| `quantity_assigned`, `unit`, `allocation_released` | allocation is reserved on creation and released on declined / unable_to_complete / cancelled |
| `instructions` | free-text operational notes; no routing |
| `protected_contact_access_granted`, `pickup_location_access_granted`, `destination_location_access_granted` | explicit grants to the volunteer |
| `eta_text`, `accepted_at`, `started_at`, `completed_at` | |
| `handover_status`, `handover_notes` | |
| `completion_evidence_type`, `completion_evidence_reference` | `coordinator_confirmation`, `requester_confirmation`, `volunteer_confirmation`, `inventory_handover_record`, `no_evidence` - never photos or IDs |
| `cancellation_reason`, `version` | |

| From | Volunteer (own) | Coordinator |
| --- | --- | --- |
| proposed | accepted, declined | accepted, declined, cancelled |
| accepted | in_progress, unable_to_complete | + cancelled |
| in_progress | delivered, partially_delivered, unable_to_complete | + cancelled |
| partially_delivered | in_progress, delivered, unable_to_complete | + cancelled |
| declined, delivered, unable_to_complete, cancelled | terminal | terminal |

## notes

Append-only. `visibility`: `internal`, `responders`, `shared`; `is_sensitive`;
exactly one parent (request or offer).

## audit_events

`id` (gapless), `occurred_at`, `organization_id`, `actor_user_id`,
`actor_roles`, `action`, `entity_type`, `entity_id`, `request_id` (timeline
grouping), `from_status`, `to_status`, `reason`, `metadata` (JSON, no personal
data), `visibility` (`shared`, `operational`, `internal`, `system`),
`prev_hash`, `hash` = SHA-256(prev_hash || canonical JSON of the event).
UPDATE, DELETE and TRUNCATE are rejected by triggers.

## Supporting tables

- `sessions`: token hash, user, timestamps.
- `instance_settings`: notices, exercise mode, location precision, retention,
  default review deadline, medicine free-text switch, volunteer grant policy.
- `categories`: 15 fixed codes; labels, descriptions, order and enabled flag
  are editable by administrators.
- `reference_counters`: per prefix and year.
- `sync_operations`: results of pushed offline operations by `op_id`
  (idempotency); cleaned up after 30 days.
