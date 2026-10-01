// Local placeholders for records created while offline. They are shown with
// a "waiting to sync" marker and replaced by the server version after sync.
import type { AidRequest, Me, Offer, OfferCreateInput, RequestCreateInput } from '@reliefmesh/shared-types'
import { MEDICINE_PICKUP, MEDICINE_PICKUP_TITLE } from '@reliefmesh/shared-types'
import { roundCoord } from './location'

const NO_PERMISSIONS = {
  can_edit: false, can_edit_coordinator_fields: false, allowed_statuses: [], can_reopen: false,
  can_reveal_protected: false, can_assign: false, note_visibilities: [],
}

export function requestPlaceholder(input: RequestCreateInput, clientId: string, user: Me, decimals: number): AidRequest {
  const now = new Date().toISOString()
  const approx = input.location.mode === 'approximate' && input.location.lat != null && input.location.lon != null
  return {
    id: `local:${clientId}`,
    client_id: clientId,
    reference: 'Not yet synchronized',
    status: input.submit === false ? 'draft' : 'submitted',
    category: input.category,
    urgency: input.urgency,
    title: input.category === MEDICINE_PICKUP ? MEDICINE_PICKUP_TITLE : input.title,
    description: input.description,
    details_redacted: false,
    estimated_people_affected: input.estimated_people_affected ?? null,
    requested_quantity: input.requested_quantity ?? null,
    requested_unit: input.requested_unit ?? '',
    requested_by_time: input.requested_by_time ?? null,
    location: {
      mode: input.location.mode,
      area_label: input.location.mode === 'none' ? '' : (input.location.area_label ?? ''),
      approx_lat: approx ? roundCoord(input.location.lat!, decimals) : null,
      approx_lon: approx ? roundCoord(input.location.lon!, decimals) : null,
      precision_decimals: approx ? decimals : null,
      has_exact: input.location.mode === 'protected_exact',
    },
    accessibility_notes: input.accessibility_notes ?? '',
    contact_method: input.contact.method,
    contact_visibility: input.contact.visibility,
    has_contact_details: Boolean(input.contact.details),
    sensitive_data_flag: Boolean(input.sensitive_data_flag) || input.category === MEDICINE_PICKUP,
    requires_formal_authorization: Boolean(input.requires_formal_authorization),
    closed_at: null,
    created_at: now,
    updated_at: now,
    version: 0,
    redacted: false,
    viewer_relation: user.capabilities.includes('request.read_all') ? 'coordinator' : 'owner',
    permissions: { ...NO_PERMISSIONS },
  }
}

export function offerPlaceholder(input: OfferCreateInput, clientId: string, user: Me): Offer {
  const now = new Date().toISOString()
  return {
    id: `local:${clientId}`,
    client_id: clientId,
    reference: 'Not yet synchronized',
    status: input.publish === false ? 'draft' : 'available',
    category: input.category,
    title: input.title,
    description: input.description ?? '',
    quantity_available: input.quantity_available,
    unit: input.unit ?? '',
    assigned_quantity: 0,
    remaining_quantity: input.quantity_available,
    availability_start: input.availability_start ?? null,
    availability_end: input.availability_end ?? null,
    pickup_or_delivery_mode: input.pickup_or_delivery_mode,
    location: {
      mode: input.location.mode, area_label: input.location.area_label ?? '', approx_lat: null, approx_lon: null,
      precision_decimals: null, has_exact: input.location.mode === 'protected_exact',
    },
    accessibility_notes: input.accessibility_notes ?? '',
    restrictions: input.restrictions ?? '',
    contact_method: input.contact.method,
    contact_visibility: input.contact.visibility,
    has_contact_details: Boolean(input.contact.details),
    created_at: now,
    updated_at: now,
    version: 0,
    redacted: false,
    viewer_relation: user.capabilities.includes('offer.read_all') ? 'coordinator' : 'owner',
    permissions: { can_edit: false, allowed_statuses: [], can_reveal_protected: false, can_allocate: false, note_visibilities: [] },
  }
}
