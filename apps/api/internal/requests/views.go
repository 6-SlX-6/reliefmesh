package requests

import (
	"time"

	"github.com/google/uuid"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
)

// Relation describes how the viewer relates to a request.
type Relation string

const (
	RelationNone              Relation = ""
	RelationCoordinator       Relation = "coordinator"
	RelationOwner             Relation = "owner"
	RelationAssignedVolunteer Relation = "assigned_volunteer"
)

// UserRef is a minimal user reference.
type UserRef struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
}

// RefView references another request.
type RefView struct {
	ID        uuid.UUID `json:"id"`
	Reference string    `json:"reference"`
}

// Permissions are computed on the server so the client never needs to
// duplicate authorization rules. The server re-checks every action anyway.
type Permissions struct {
	CanEdit                  bool     `json:"can_edit"`
	CanEditCoordinatorFields bool     `json:"can_edit_coordinator_fields"`
	AllowedStatuses          []string `json:"allowed_statuses"`
	CanReopen                bool     `json:"can_reopen"`
	CanRevealProtected       bool     `json:"can_reveal_protected"`
	CanAssign                bool     `json:"can_assign"`
	NoteVisibilities         []string `json:"note_visibilities"`
}

// View is the role-specific projection of a request returned by the API.
// Protected values (contact details, exact location) are never part of it;
// they are only returned by the audited reveal endpoint.
type View struct {
	ID                          uuid.UUID      `json:"id"`
	ClientID                    *uuid.UUID     `json:"client_id,omitempty"`
	Reference                   string         `json:"reference"`
	Status                      string         `json:"status"`
	Category                    string         `json:"category"`
	Urgency                     string         `json:"urgency"`
	Title                       string         `json:"title"`
	Description                 string         `json:"description"`
	DetailsRedacted             bool           `json:"details_redacted"`
	EstimatedPeopleAffected     *int           `json:"estimated_people_affected"`
	RequestedQuantity           *int           `json:"requested_quantity"`
	RequestedUnit               string         `json:"requested_unit"`
	RequestedByTime             *time.Time     `json:"requested_by_time"`
	Location                    locations.View `json:"location"`
	AccessibilityNotes          string         `json:"accessibility_notes"`
	ContactMethod               string         `json:"contact_method"`
	ContactVisibility           string         `json:"contact_visibility"`
	HasContactDetails           bool           `json:"has_contact_details"`
	SensitiveDataFlag           bool           `json:"sensitive_data_flag"`
	RequiresFormalAuthorization bool           `json:"requires_formal_authorization"`
	ReviewStatus                string         `json:"review_status,omitempty"`
	VerificationLevel           string         `json:"verification_level,omitempty"`
	AssignedTeam                *string        `json:"assigned_team,omitempty"`
	AssignedUser                *UserRef       `json:"assigned_user,omitempty"`
	RespondersAssigned          *bool          `json:"responders_assigned,omitempty"`
	ResolutionSummary           string         `json:"resolution_summary,omitempty"`
	ClosedAt                    *time.Time     `json:"closed_at"`
	ExpiryAt                    *time.Time     `json:"expiry_at,omitempty"`
	Tags                        []string       `json:"tags,omitempty"`
	DuplicateOf                 *RefView       `json:"duplicate_of,omitempty"`
	CreatedBy                   *UserRef       `json:"created_by,omitempty"`
	CreatedAt                   time.Time      `json:"created_at"`
	UpdatedAt                   time.Time      `json:"updated_at"`
	Version                     int            `json:"version"`
	Redacted                    bool           `json:"redacted"`
	ViewerRelation              Relation       `json:"viewer_relation"`
	Permissions                 Permissions    `json:"permissions"`
}

// relationFor determines the viewer relation, or RelationNone when the
// request must be invisible to the viewer.
func relationFor(p roles.Principal, r *Record, acc *assignmentAccess) Relation {
	if r.DeletedAt != nil {
		return RelationNone
	}
	if p.Can(roles.CapRequestReadAll) && p.OrgID == r.OrgID {
		return RelationCoordinator
	}
	if r.CreatedBy != nil && *r.CreatedBy == p.UserID && p.Can(roles.CapRequestCreate) {
		return RelationOwner
	}
	if acc != nil && acc.visible && p.Can(roles.CapAssignmentWorkOwn) {
		return RelationAssignedVolunteer
	}
	return RelationNone
}

func actorKind(rel Relation) ActorKind {
	switch rel {
	case RelationCoordinator:
		return ActorCoordinator
	case RelationAssignedVolunteer:
		return ActorAssignedVolunteer
	default:
		return ActorOwner
	}
}

// ownerCanEdit: requesters may edit content until a coordinator starts the
// review; afterwards they add update notes instead.
func ownerCanEdit(r *Record) bool {
	return (r.Status == StatusDraft || r.Status == StatusSubmitted) && r.ReviewStatus == "pending"
}

func canRevealProtected(rel Relation, r *Record, acc *assignmentAccess, st *settings.Settings) bool {
	hasContact := len(r.ContactSealed) > 0
	hasExact := len(r.ExactSealed) > 0
	switch rel {
	case RelationCoordinator, RelationOwner:
		return hasContact || hasExact
	case RelationAssignedVolunteer:
		if acc == nil || !acc.active {
			return false
		}
		contactOK := hasContact && r.ContactVisibility == locations.ContactVisibilityAssignedResponders &&
			(acc.contactGrant || !st.VolunteerAccessRequiresGrant)
		exactOK := hasExact && (acc.destinationGrant || !st.VolunteerAccessRequiresGrant)
		return contactOK || exactOK
	}
	return false
}

func permissionsFor(rel Relation, r *Record, acc *assignmentAccess, st *settings.Settings) Permissions {
	p := Permissions{AllowedStatuses: []string{}, NoteVisibilities: []string{}}
	if r.RedactedAt != nil {
		return p
	}
	kind := actorKind(rel)
	for _, s := range NextStatuses(r.Status, kind) {
		p.AllowedStatuses = append(p.AllowedStatuses, string(s))
	}
	p.CanReopen = CanReopen(r.Status, kind)
	p.CanRevealProtected = canRevealProtected(rel, r, acc, st)
	switch rel {
	case RelationCoordinator:
		p.CanEdit = !r.Status.IsClosed()
		p.CanEditCoordinatorFields = !r.Status.IsClosed()
		p.CanAssign = r.Status == StatusVerified || r.Status == StatusAssigned ||
			r.Status == StatusInProgress || r.Status == StatusPartiallyResolved
		p.NoteVisibilities = []string{"internal", "responders", "shared"}
	case RelationOwner:
		p.CanEdit = ownerCanEdit(r)
		p.NoteVisibilities = []string{"shared"}
	case RelationAssignedVolunteer:
		if acc != nil && acc.visible {
			p.NoteVisibilities = []string{"responders"}
		}
	}
	return p
}

func project(r *Record, rel Relation, acc *assignmentAccess, st *settings.Settings, listMode bool) *View {
	v := &View{
		ID:                          r.ID,
		Reference:                   r.Reference,
		Status:                      string(r.Status),
		Category:                    r.Category,
		Urgency:                     r.Urgency,
		Title:                       r.Title,
		Description:                 r.Description,
		EstimatedPeopleAffected:     r.PeopleAffected,
		RequestedQuantity:           r.Quantity,
		RequestedUnit:               r.Unit,
		RequestedByTime:             r.RequestedByTime,
		AccessibilityNotes:          r.AccessibilityNotes,
		ContactMethod:               r.ContactMethod,
		ContactVisibility:           r.ContactVisibility,
		HasContactDetails:           len(r.ContactSealed) > 0,
		SensitiveDataFlag:           r.Sensitive,
		RequiresFormalAuthorization: r.RequiresFormalAuthorization,
		ClosedAt:                    r.ClosedAt,
		CreatedAt:                   r.CreatedAt,
		UpdatedAt:                   r.UpdatedAt,
		Version:                     r.Version,
		Redacted:                    r.RedactedAt != nil,
		ViewerRelation:              rel,
		Location: locations.View{
			Mode:              locations.Mode(r.LocationMode),
			AreaLabel:         r.AreaLabel,
			ApproxLat:         r.ApproxLat,
			ApproxLon:         r.ApproxLon,
			PrecisionDecimals: r.ApproxDecimals,
			HasExact:          len(r.ExactSealed) > 0,
		},
	}
	if r.DuplicateOf != nil && r.DuplicateOfReference != nil {
		v.DuplicateOf = &RefView{ID: *r.DuplicateOf, Reference: *r.DuplicateOfReference}
	}
	// Sensitive free text is never included in list responses (and therefore
	// never lands in list caches on devices); it is only shown on the detail
	// view to authorized viewers.
	if listMode && r.Sensitive && rel != RelationOwner {
		v.Description, v.AccessibilityNotes, v.DetailsRedacted = "", "", true
	}

	switch rel {
	case RelationCoordinator:
		v.ClientID = r.ClientID
		v.ReviewStatus = r.ReviewStatus
		v.VerificationLevel = r.VerificationLevel
		team := r.AssignedTeam
		v.AssignedTeam = &team
		if r.AssignedUserID != nil {
			name := ""
			if r.AssignedUserName != nil {
				name = *r.AssignedUserName
			}
			v.AssignedUser = &UserRef{ID: *r.AssignedUserID, DisplayName: name}
		}
		v.ResolutionSummary = r.ResolutionSummary
		v.ExpiryAt = r.ExpiryAt
		v.Tags = r.Tags
		if r.CreatedBy != nil {
			name := ""
			if r.CreatedByName != nil {
				name = *r.CreatedByName
			}
			v.CreatedBy = &UserRef{ID: *r.CreatedBy, DisplayName: name}
		}
	case RelationOwner:
		v.ClientID = r.ClientID
		v.ReviewStatus = r.ReviewStatus
		v.VerificationLevel = r.VerificationLevel
		v.ResolutionSummary = r.ResolutionSummary
		assigned := r.Status == StatusAssigned || r.Status == StatusInProgress || r.Status == StatusPartiallyResolved
		v.RespondersAssigned = &assigned
	case RelationAssignedVolunteer:
		// Volunteers see what they need to perform the task, nothing more:
		// no requester identity, review internals, tags or resolution notes.
	}
	v.Permissions = permissionsFor(rel, r, acc, st)
	return v
}
