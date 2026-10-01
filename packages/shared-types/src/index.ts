/**
 * Types and domain constants mirroring the ReliefMesh API (apps/api/openapi/openapi.yaml).
 *
 * The server is the single source of truth for authorization: every entity view
 * carries a `permissions` object computed by the API. The constants here exist for
 * display, form validation and offline rendering only.
 */

export type Role = 'requester' | 'volunteer' | 'coordinator' | 'organization_manager' | 'admin'
export const ROLES: readonly Role[] = ['requester', 'volunteer', 'coordinator', 'organization_manager', 'admin']

export type Urgency = 'critical' | 'high' | 'normal' | 'low'
export const URGENCIES: readonly Urgency[] = ['critical', 'high', 'normal', 'low']

export type RequestStatus =
  | 'draft'
  | 'submitted'
  | 'under_review'
  | 'verified'
  | 'assigned'
  | 'in_progress'
  | 'partially_resolved'
  | 'resolved'
  | 'cancelled'
  | 'expired'
  | 'duplicate'

export const REQUEST_STATUSES: readonly RequestStatus[] = [
  'draft', 'submitted', 'under_review', 'verified', 'assigned', 'in_progress',
  'partially_resolved', 'resolved', 'cancelled', 'expired', 'duplicate',
]

export const CLOSED_REQUEST_STATUSES: readonly RequestStatus[] = ['resolved', 'cancelled', 'expired', 'duplicate']

export function isClosedRequestStatus(s: RequestStatus): boolean {
  return CLOSED_REQUEST_STATUSES.includes(s)
}

export type OfferStatus =
  | 'draft'
  | 'available'
  | 'partially_allocated'
  | 'fully_allocated'
  | 'paused'
  | 'expired'
  | 'cancelled'

export const OFFER_STATUSES: readonly OfferStatus[] = [
  'draft', 'available', 'partially_allocated', 'fully_allocated', 'paused', 'expired', 'cancelled',
]

export type AssignmentStatus =
  | 'proposed'
  | 'accepted'
  | 'declined'
  | 'in_progress'
  | 'delivered'
  | 'partially_delivered'
  | 'unable_to_complete'
  | 'cancelled'

export const ASSIGNMENT_STATUSES: readonly AssignmentStatus[] = [
  'proposed', 'accepted', 'declined', 'in_progress', 'delivered', 'partially_delivered', 'unable_to_complete', 'cancelled',
]

export type CategoryCode =
  | 'drinking_water'
  | 'food'
  | 'shelter'
  | 'blankets'
  | 'hygiene'
  | 'baby_supplies'
  | 'power_charging'
  | 'transport'
  | 'volunteer_support'
  | 'translation'
  | 'accessibility_support'
  | 'information'
  | 'pet_animal_support'
  | 'medicine_pickup'
  | 'other'

export const MEDICINE_PICKUP: CategoryCode = 'medicine_pickup'
export const MEDICINE_PICKUP_TITLE = 'Medicine pickup (logistics only)'

export type LocationMode = 'none' | 'area_only' | 'approximate' | 'protected_exact'
export const LOCATION_MODES: readonly LocationMode[] = ['none', 'area_only', 'approximate', 'protected_exact']

export type ContactVisibility = 'none' | 'coordinators_only' | 'assigned_responders'
export type ContactMethod = 'none' | 'phone' | 'messenger' | 'email' | 'in_person' | 'via_shelter_desk' | 'other'
export const CONTACT_METHODS: readonly ContactMethod[] = ['none', 'phone', 'messenger', 'email', 'in_person', 'via_shelter_desk', 'other']

export type ReviewStatus = 'pending' | 'in_review' | 'reviewed' | 'needs_information'
export type VerificationLevel = 'unverified' | 'self_reported' | 'coordinator_confirmed' | 'field_confirmed'
export const VERIFICATION_LEVELS: readonly VerificationLevel[] = ['unverified', 'self_reported', 'coordinator_confirmed', 'field_confirmed']

export type DeliveryMode = 'pickup' | 'delivery' | 'either' | 'on_site'
export const DELIVERY_MODES: readonly DeliveryMode[] = ['pickup', 'delivery', 'either', 'on_site']

export type EvidenceType =
  | 'coordinator_confirmation'
  | 'requester_confirmation'
  | 'volunteer_confirmation'
  | 'inventory_handover_record'
  | 'no_evidence'
export const EVIDENCE_TYPES: readonly EvidenceType[] = [
  'coordinator_confirmation', 'requester_confirmation', 'volunteer_confirmation', 'inventory_handover_record', 'no_evidence',
]

export type HandoverStatus = 'not_started' | 'handed_over' | 'received' | 'not_applicable'
export type NoteVisibility = 'internal' | 'responders' | 'shared'
export type Availability = 'available' | 'limited' | 'unavailable'
export type ViewerRelation = 'coordinator' | 'owner' | 'assigned_volunteer'

/** Mirrors the maximum precision allowed for approximate coordinates. */
export const MAX_APPROX_DECIMALS = 3

export interface UserRef {
  id: string
  display_name: string
}

export interface Me {
  id: string
  username: string
  display_name: string
  roles: Role[]
  capabilities: string[]
  must_change_password: boolean
  availability: Availability
  availability_note: string
  organization: { id: string; name: string }
}

export interface SessionResponse {
  user: Me
  csrf_token: string
}

export interface Category {
  code: CategoryCode
  label: string
  description: string
  is_sensitive: boolean
  enabled: boolean
  sort_order: number
}

export interface Settings {
  instance_name: string
  emergency_notice: string
  critical_urgency_notice: string
  medicine_privacy_notice: string
  exercise_mode: boolean
  exercise_label: string
  approx_location_decimals: number
  retention_days_closed: number
  default_request_expiry_hours: number
  allow_medicine_free_text: boolean
  volunteer_access_requires_grant: boolean
  updated_at: string
}

export interface ClientSettings {
  settings: Settings
  categories: Category[]
}

export interface PublicNotice {
  instance_name: string
  emergency_notice: string
  exercise_mode: boolean
  exercise_label: string
}

export interface LocationView {
  mode: LocationMode
  area_label: string
  approx_lat: number | null
  approx_lon: number | null
  precision_decimals: number | null
  has_exact: boolean
}

export interface ExactLocation {
  lat?: number
  lon?: number
  address?: string
  directions?: string
}

export interface LocationInput {
  mode: LocationMode
  area_label?: string
  lat?: number | null
  lon?: number | null
  exact?: ExactLocation
  keep_exact?: boolean
}

export interface ContactInput {
  method: ContactMethod
  visibility: ContactVisibility
  details?: string
  keep_details?: boolean
}

export interface RequestPermissions {
  can_edit: boolean
  can_edit_coordinator_fields: boolean
  allowed_statuses: RequestStatus[]
  can_reopen: boolean
  can_reveal_protected: boolean
  can_assign: boolean
  note_visibilities: NoteVisibility[]
}

export interface AidRequest {
  id: string
  client_id?: string
  reference: string
  status: RequestStatus
  category: CategoryCode
  urgency: Urgency
  title: string
  description: string
  details_redacted: boolean
  estimated_people_affected: number | null
  requested_quantity: number | null
  requested_unit: string
  requested_by_time: string | null
  location: LocationView
  accessibility_notes: string
  contact_method: ContactMethod
  contact_visibility: ContactVisibility
  has_contact_details: boolean
  sensitive_data_flag: boolean
  requires_formal_authorization: boolean
  review_status?: ReviewStatus
  verification_level?: VerificationLevel
  assigned_team?: string
  assigned_user?: UserRef
  responders_assigned?: boolean
  resolution_summary?: string
  closed_at: string | null
  expiry_at?: string | null
  tags?: string[]
  duplicate_of?: { id: string; reference: string }
  created_by?: UserRef
  created_at: string
  updated_at: string
  version: number
  redacted: boolean
  viewer_relation: ViewerRelation
  permissions: RequestPermissions
}

export interface RequestCreateInput {
  client_id?: string
  category: CategoryCode
  urgency: Urgency
  title: string
  description: string
  estimated_people_affected?: number | null
  requested_quantity?: number | null
  requested_unit?: string
  requested_by_time?: string | null
  location: LocationInput
  accessibility_notes?: string
  contact: ContactInput
  sensitive_data_flag?: boolean
  requires_formal_authorization?: boolean
  tags?: string[]
  submit?: boolean
  emergency_notice_acknowledged?: boolean
  client_created_at?: string
}

export type RequestUpdateInput = Partial<Omit<RequestCreateInput, 'client_id' | 'submit' | 'client_created_at'>> & {
  version?: number
  review_status?: ReviewStatus
  verification_level?: VerificationLevel
  assigned_team?: string
  assigned_user_id?: string | null
  expiry_at?: string | null
}

export interface RequestStatusInput {
  status: RequestStatus
  reason?: string
  resolution_summary?: string
  duplicate_of_reference?: string
  cancel_active_assignments?: boolean
  version?: number
}

export interface ProtectedContact {
  method: ContactMethod
  details: string
}

export interface RequestProtected {
  contact?: ProtectedContact
  exact_location?: ExactLocation
  fields: string[]
}

export interface OfferPermissions {
  can_edit: boolean
  allowed_statuses: OfferStatus[]
  can_reveal_protected: boolean
  can_allocate: boolean
  note_visibilities: NoteVisibility[]
}

export interface Offer {
  id: string
  client_id?: string
  reference: string
  status: OfferStatus
  category: CategoryCode
  title: string
  description: string
  quantity_available: number
  unit: string
  assigned_quantity: number
  remaining_quantity: number
  availability_start: string | null
  availability_end: string | null
  pickup_or_delivery_mode: DeliveryMode
  location: LocationView
  accessibility_notes: string
  restrictions: string
  contact_method: ContactMethod
  contact_visibility: ContactVisibility
  has_contact_details: boolean
  verification_level?: VerificationLevel
  tags?: string[]
  created_by?: UserRef
  created_at: string
  updated_at: string
  version: number
  redacted: boolean
  viewer_relation: ViewerRelation
  permissions: OfferPermissions
}

export interface OfferCreateInput {
  client_id?: string
  category: CategoryCode
  title: string
  description?: string
  quantity_available: number
  unit?: string
  availability_start?: string | null
  availability_end?: string | null
  pickup_or_delivery_mode: DeliveryMode
  location: LocationInput
  accessibility_notes?: string
  restrictions?: string
  contact: ContactInput
  tags?: string[]
  publish?: boolean
  client_created_at?: string
}

export interface AssignmentPermissions {
  allowed_statuses: AssignmentStatus[]
  can_edit_assignment: boolean
  can_update_handover: boolean
  can_reveal_protected: boolean
}

export interface AssignmentRequestSummary {
  id: string
  reference: string
  status: RequestStatus
  category: CategoryCode
  urgency: Urgency
  title: string
  requested_quantity: number | null
  requested_unit: string
  requested_by_time: string | null
  location: LocationView
  accessibility_notes: string
  requires_formal_authorization: boolean
  sensitive_data_flag: boolean
  has_contact_details: boolean
  contact_method: ContactMethod
}

export interface AssignmentOfferSummary {
  id: string
  reference: string
  title: string
  category: CategoryCode
  unit: string
  pickup_or_delivery_mode: DeliveryMode
  location: LocationView
  restrictions: string
}

export interface Assignment {
  id: string
  client_id?: string
  request_id: string
  offer_id: string | null
  volunteer: UserRef | null
  team_label: string
  assigned_by?: UserRef
  assigned_at: string
  updated_at: string
  status: AssignmentStatus
  quantity_assigned: number
  unit: string
  instructions: string
  protected_contact_access_granted: boolean
  pickup_location_access_granted: boolean
  destination_location_access_granted: boolean
  eta_text: string
  accepted_at: string | null
  started_at: string | null
  completed_at: string | null
  handover_status: HandoverStatus
  handover_notes: string
  completion_evidence_type: EvidenceType
  completion_evidence_reference: string
  cancellation_reason: string
  version: number
  request: AssignmentRequestSummary | null
  offer: AssignmentOfferSummary | null
  viewer_relation: 'coordinator' | 'volunteer'
  permissions: AssignmentPermissions
}

export interface AssignmentCreateInput {
  client_id?: string
  request_id: string
  offer_id?: string | null
  volunteer_user_id?: string | null
  team_label?: string
  quantity_assigned?: number
  unit?: string
  instructions?: string
  protected_contact_access_granted?: boolean
  pickup_location_access_granted?: boolean
  destination_location_access_granted?: boolean
  eta_text?: string
}

export interface AssignmentStatusInput {
  status: AssignmentStatus
  reason?: string
  completion_evidence_type?: EvidenceType
  completion_evidence_reference?: string
  handover_status?: HandoverStatus
  handover_notes?: string
  eta_text?: string
  version?: number
}

export interface AssignmentUpdateInput {
  version?: number
  instructions?: string
  protected_contact_access_granted?: boolean
  pickup_location_access_granted?: boolean
  destination_location_access_granted?: boolean
  eta_text?: string
  handover_status?: HandoverStatus
  handover_notes?: string
}

export interface AssignmentProtected {
  destination_contact?: ProtectedContact
  destination_location?: ExactLocation
  pickup_location?: ExactLocation
  fields: string[]
}

export interface Note {
  id: string
  client_id?: string
  visibility: NoteVisibility
  is_sensitive: boolean
  body: string
  created_at: string
  redacted: boolean
  author: { label: string; display_name?: string; user_id?: string; is_you: boolean }
}

export interface NoteInput {
  client_id?: string
  body: string
  visibility?: NoteVisibility
  is_sensitive?: boolean
}

export interface TimelineEvent {
  id: number
  occurred_at: string
  action: string
  entity_type: string
  entity_id?: string
  from_status?: string
  to_status?: string
  reason?: string
  metadata: Record<string, unknown>
  actor: { label: string; display_name?: string; user_id?: string; is_you: boolean }
}

export interface AuditEvent {
  id: number
  occurred_at: string
  organization_id?: string
  actor_user_id?: string
  actor_roles: string[]
  action: string
  entity_type: string
  entity_id?: string
  request_id?: string
  from_status?: string
  to_status?: string
  reason?: string
  metadata: Record<string, unknown>
  visibility: 'shared' | 'operational' | 'internal' | 'system'
}

export interface Volunteer {
  id: string
  display_name: string
  availability: Availability
  availability_note: string
  active_assignments: number
}

export interface AdminUser {
  id: string
  username: string
  display_name: string
  roles: Role[]
  is_active: boolean
  must_change_password: boolean
  locked: boolean
  availability: Availability
  last_login_at: string | null
  created_at: string
}

export interface QueueItem {
  id: string
  reference: string
  title: string
  category: CategoryCode
  urgency: Urgency
  status: RequestStatus
  area_label: string
  requested_by_time: string | null
  created_at: string
  sensitive_data_flag: boolean
}

export interface Dashboard {
  generated_at: string
  requests: {
    open: number
    by_status: Record<string, number>
    open_by_urgency: Record<string, number>
    open_by_category: Record<string, number>
    needs_review: number
    overdue: number
    past_expiry: number
    awaiting_confirmation: number
    resolved_last_24h: number
  }
  offers: { active: number; remaining_by_category: Record<string, number>; fully_allocated: number }
  assignments: { by_status: Record<string, number>; active: number }
  volunteers: { available: number; limited: number; unavailable: number }
  queues?: {
    needs_review: QueueItem[]
    urgent_open: QueueItem[]
    overdue: QueueItem[]
    awaiting_confirmation: QueueItem[]
  }
}

export interface ListResult<T> {
  items: T[]
  has_more?: boolean
}

export type SyncOperationType =
  | 'request.create'
  | 'request.update'
  | 'request.status'
  | 'request.note'
  | 'offer.create'
  | 'offer.status'
  | 'offer.note'
  | 'assignment.status'
  | 'assignment.update'

export interface SyncOperation {
  op_id: string
  type: SyncOperationType
  entity_id?: string
  entity_client_id?: string
  payload: unknown
  client_created_at?: string
}

export type SyncResultStatus = 'applied' | 'duplicate' | 'conflict' | 'rejected' | 'retry'

export interface ApiErrorDetail {
  code: string
  message: string
  fields?: Record<string, string>
  details?: Record<string, unknown>
  request_id?: string
}

export interface SyncOpResult {
  op_id: string
  status: SyncResultStatus
  entity_type?: 'aid_request' | 'offer' | 'assignment' | 'note'
  entity_id?: string
  reference?: string
  error?: ApiErrorDetail
  entity?: unknown
}

export interface SyncPushResult {
  results: SyncOpResult[]
  server_time: string
}

export interface SyncPullResult {
  server_time: string
  full: boolean
  has_more: boolean
  settings: ClientSettings
  requests: AidRequest[]
  offers: Offer[]
  assignments: Assignment[]
}

export interface RetentionPreview {
  retention_days: number
  cutoff: string
  requests: number
  offers: number
}

export interface DemoInfo {
  enabled: boolean
  scenarios: { name: string; title: string; description: string }[]
  seeded: string[]
  personas: { username: string; roles: string[] }[]
  password?: string
}

export interface AuditVerifyResult {
  valid: boolean
  events_checked: number
  first_broken_id?: number
  problem?: string
}
