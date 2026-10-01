// Human-readable labels for domain values. Kept in one place so they can be
// translated later (see docs/roadmap.md) and so status wording is consistent
// across the whole interface.
import type {
  AssignmentStatus,
  Availability,
  ContactMethod,
  ContactVisibility,
  DeliveryMode,
  EvidenceType,
  HandoverStatus,
  LocationMode,
  NoteVisibility,
  OfferStatus,
  RequestStatus,
  ReviewStatus,
  Role,
  Urgency,
  VerificationLevel,
} from '@reliefmesh/shared-types'

export const requestStatusLabel: Record<RequestStatus, string> = {
  draft: 'Draft (not sent)',
  submitted: 'Submitted',
  under_review: 'Under review',
  verified: 'Verified',
  assigned: 'Assigned',
  in_progress: 'In progress',
  partially_resolved: 'Partially resolved',
  resolved: 'Resolved',
  cancelled: 'Cancelled',
  expired: 'Expired',
  duplicate: 'Duplicate',
}

export const requestStatusHelp: Record<RequestStatus, string> = {
  draft: 'Saved but not yet sent to coordinators.',
  submitted: 'Received. Waiting for a coordinator to review it.',
  under_review: 'A coordinator is reviewing this request.',
  verified: 'Checked by a coordinator. Waiting for someone to be assigned.',
  assigned: 'Help or resources have been assigned.',
  in_progress: 'Someone is working on this request now.',
  partially_resolved: 'Part of the need has been met.',
  resolved: 'A coordinator confirmed that the need was met.',
  cancelled: 'This request was cancelled.',
  expired: 'This request expired without being completed.',
  duplicate: 'This request is handled under another reference.',
}

export const offerStatusLabel: Record<OfferStatus, string> = {
  draft: 'Draft',
  available: 'Available',
  partially_allocated: 'Partly allocated',
  fully_allocated: 'Fully allocated',
  paused: 'Paused',
  expired: 'Expired',
  cancelled: 'Cancelled',
}

export const assignmentStatusLabel: Record<AssignmentStatus, string> = {
  proposed: 'Proposed - awaiting answer',
  accepted: 'Accepted',
  declined: 'Declined',
  in_progress: 'In progress',
  delivered: 'Delivered',
  partially_delivered: 'Partly delivered',
  unable_to_complete: 'Unable to complete',
  cancelled: 'Cancelled',
}

/** Verb-style labels for action buttons. */
export const requestStatusAction: Partial<Record<RequestStatus, string>> = {
  submitted: 'Submit request',
  under_review: 'Start review',
  verified: 'Mark as verified',
  assigned: 'Mark as assigned',
  in_progress: 'Mark in progress',
  partially_resolved: 'Mark partially resolved',
  resolved: 'Confirm resolved',
  cancelled: 'Cancel request',
  expired: 'Mark expired',
  duplicate: 'Mark as duplicate',
}

export const assignmentStatusAction: Partial<Record<AssignmentStatus, string>> = {
  accepted: 'Accept',
  declined: 'Decline',
  in_progress: 'Start',
  delivered: 'Mark delivered',
  partially_delivered: 'Partly delivered',
  unable_to_complete: 'Unable to complete',
  cancelled: 'Cancel assignment',
}

export const offerStatusAction: Partial<Record<OfferStatus, string>> = {
  available: 'Make available',
  paused: 'Pause',
  expired: 'Mark expired',
  cancelled: 'Cancel offer',
}

export const urgencyLabel: Record<Urgency, string> = {
  critical: 'Critical',
  high: 'High',
  normal: 'Normal',
  low: 'Low',
}

export const urgencyHelp: Record<Urgency, string> = {
  critical: 'Needed within hours and serious harm is likely without it. Not for emergencies - call emergency services.',
  high: 'Needed today.',
  normal: 'Needed within a day or two.',
  low: 'Helpful, but can wait.',
}

export const categoryFallbackLabel: Record<string, string> = {
  drinking_water: 'Drinking water',
  food: 'Food',
  shelter: 'Shelter',
  blankets: 'Blankets',
  hygiene: 'Hygiene products',
  baby_supplies: 'Baby supplies',
  power_charging: 'Power / charging',
  transport: 'Transport',
  volunteer_support: 'Volunteer support',
  translation: 'Translation',
  accessibility_support: 'Accessibility support',
  information: 'Information',
  pet_animal_support: 'Pet / animal support',
  medicine_pickup: 'Medicine pickup',
  other: 'Other',
}

export const roleLabel: Record<Role, string> = {
  requester: 'Requester',
  volunteer: 'Volunteer',
  coordinator: 'Coordinator',
  organization_manager: 'Organization manager',
  admin: 'Administrator',
}

export const roleHelp: Record<Role, string> = {
  requester: 'Creates requests and sees only their own requests.',
  volunteer: 'Works on assignments given to them; can offer resources.',
  coordinator: 'Reviews, prioritizes and assigns requests and offers.',
  organization_manager: 'Coordinator rights plus volunteer availability and reports.',
  admin: 'Manages users and settings. No access to operational personal data.',
}

export const locationModeLabel: Record<LocationMode, string> = {
  none: 'No location',
  area_only: 'Area only',
  approximate: 'Approximate position',
  protected_exact: 'Exact address (protected)',
}

export const locationModeHelp: Record<LocationMode, string> = {
  none: 'No location is stored.',
  area_only: 'A district, village or shelter name. No coordinates.',
  approximate: 'A rounded position (about 1 km). Visible to people handling the request.',
  protected_exact: 'Encrypted. Only coordinators and responders you are assigned to can open it, and every access is logged.',
}

export const contactMethodLabel: Record<ContactMethod, string> = {
  none: 'No contact',
  phone: 'Phone',
  messenger: 'Messenger app',
  email: 'E-mail',
  in_person: 'In person',
  via_shelter_desk: 'Via shelter desk',
  other: 'Other',
}

export const contactVisibilityLabel: Record<ContactVisibility, string> = {
  none: 'Do not contact me',
  coordinators_only: 'Coordinators only',
  assigned_responders: 'Coordinators and assigned responders',
}

export const verificationLabel: Record<VerificationLevel, string> = {
  unverified: 'Not verified',
  self_reported: 'Self-reported',
  coordinator_confirmed: 'Confirmed by coordinator',
  field_confirmed: 'Confirmed in the field',
}

export const reviewStatusLabel: Record<ReviewStatus, string> = {
  pending: 'Not reviewed yet',
  in_review: 'In review',
  reviewed: 'Reviewed',
  needs_information: 'Needs more information',
}

export const deliveryModeLabel: Record<DeliveryMode, string> = {
  pickup: 'Pickup',
  delivery: 'Delivery',
  either: 'Pickup or delivery',
  on_site: 'On site',
}

export const evidenceLabel: Record<EvidenceType, string> = {
  coordinator_confirmation: 'Confirmed by coordinator',
  requester_confirmation: 'Confirmed by requester',
  volunteer_confirmation: 'Confirmed by volunteer',
  inventory_handover_record: 'Inventory handover record',
  no_evidence: 'No evidence recorded',
}

export const handoverLabel: Record<HandoverStatus, string> = {
  not_started: 'Not started',
  handed_over: 'Handed over',
  received: 'Received',
  not_applicable: 'Not applicable',
}

export const noteVisibilityLabel: Record<NoteVisibility, string> = {
  internal: 'Internal (coordinators only)',
  responders: 'Responders (coordinators and assigned volunteers)',
  shared: 'Shared (also visible to the requester / offer owner)',
}

export const availabilityLabel: Record<Availability, string> = {
  available: 'Available',
  limited: 'Limited',
  unavailable: 'Unavailable',
}

const actionLabels: Record<string, string> = {
  'request.created': 'Request created',
  'request.updated': 'Details updated',
  'request.status_changed': 'Status changed',
  'request.reopened': 'Request reopened',
  'request.urgency_changed': 'Urgency changed',
  'request.review_status_changed': 'Review status changed',
  'request.verification_changed': 'Verification changed',
  'request.protected_viewed': 'Protected details viewed',
  'request.deleted': 'Request deleted',
  'assignment.created': 'Assignment created',
  'assignment.status_changed': 'Assignment status changed',
  'assignment.updated': 'Assignment updated',
  'offer.created': 'Offer created',
  'offer.updated': 'Offer updated',
  'offer.status_changed': 'Offer status changed',
  'offer.allocated': 'Quantity allocated',
  'offer.allocation_released': 'Allocation released',
  'offer.protected_viewed': 'Protected details viewed',
  'offer.deleted': 'Offer deleted',
  'note.added': 'Note added',
  'auth.login_succeeded': 'Signed in',
  'auth.login_failed': 'Failed sign-in',
  'auth.login_blocked': 'Sign-in blocked',
  'auth.account_locked': 'Account locked',
  'auth.logout': 'Signed out',
  'auth.password_changed': 'Password changed',
  'user.created': 'User created',
  'user.updated': 'User updated',
  'user.password_reset': 'Password reset',
  'settings.updated': 'Settings updated',
  'category.updated': 'Category updated',
  'retention.applied': 'Retention applied',
  'export.generated': 'Report exported',
  'demo.seeded': 'Demo data loaded',
  'volunteer.availability_changed': 'Availability changed',
}

export function actionLabel(action: string): string {
  return actionLabels[action] ?? action
}

export function statusLabelAny(status: string | undefined): string {
  if (!status) return ''
  return (
    (requestStatusLabel as Record<string, string>)[status] ??
    (assignmentStatusLabel as Record<string, string>)[status] ??
    (offerStatusLabel as Record<string, string>)[status] ??
    status
  )
}
