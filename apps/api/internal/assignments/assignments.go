// Package assignments links offers, volunteers or teams to requests.
//
// Assignments are always created manually by coordinators. ReliefMesh never
// dispatches, routes or recommends responders, never claims a destination is
// safe, and never requires photographic or biometric evidence of completion.
//
// Lock order (to avoid deadlocks): request -> assignment -> offer.
package assignments

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/offers"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/requests"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Status is an assignment status.
type Status string

const (
	StatusProposed           Status = "proposed"
	StatusAccepted           Status = "accepted"
	StatusDeclined           Status = "declined"
	StatusInProgress         Status = "in_progress"
	StatusDelivered          Status = "delivered"
	StatusPartiallyDelivered Status = "partially_delivered"
	StatusUnableToComplete   Status = "unable_to_complete"
	StatusCancelled          Status = "cancelled"
)

// AllStatuses lists assignment statuses.
var AllStatuses = []string{"proposed", "accepted", "declined", "in_progress", "delivered",
	"partially_delivered", "unable_to_complete", "cancelled"}

// EvidenceTypes lists accepted completion evidence types. None of them
// require photos, biometrics or identity documents.
var EvidenceTypes = []string{"coordinator_confirmation", "requester_confirmation", "volunteer_confirmation",
	"inventory_handover_record", "no_evidence"}

// HandoverStatuses lists handover states.
var HandoverStatuses = []string{"not_started", "handed_over", "received", "not_applicable"}

// IsTerminal reports whether s is final.
func (s Status) IsTerminal() bool {
	return s == StatusDeclined || s == StatusDelivered || s == StatusUnableToComplete || s == StatusCancelled
}

// IsActive reports whether s still holds work (and allocations).
func (s Status) IsActive() bool {
	return s == StatusProposed || s == StatusAccepted || s == StatusInProgress || s == StatusPartiallyDelivered
}

// releasesAllocation reports whether moving to s returns allocated quantity
// to the offer.
func (s Status) releasesAllocation() bool {
	return s == StatusDeclined || s == StatusUnableToComplete || s == StatusCancelled
}

type rule struct{ volunteer, coordinator bool }

var transitions = map[Status]map[Status]rule{
	StatusProposed: {
		StatusAccepted:  {true, true},
		StatusDeclined:  {true, true},
		StatusCancelled: {false, true},
	},
	StatusAccepted: {
		StatusInProgress:       {true, true},
		StatusUnableToComplete: {true, true},
		StatusCancelled:        {false, true},
	},
	StatusInProgress: {
		StatusDelivered:          {true, true},
		StatusPartiallyDelivered: {true, true},
		StatusUnableToComplete:   {true, true},
		StatusCancelled:          {false, true},
	},
	StatusPartiallyDelivered: {
		StatusInProgress:       {true, true},
		StatusDelivered:        {true, true},
		StatusUnableToComplete: {true, true},
		StatusCancelled:        {false, true},
	},
}

// NextStatuses lists allowed target statuses for a volunteer or coordinator.
func NextStatuses(from Status, coordinator bool) []Status {
	var out []Status
	for _, to := range []Status{StatusAccepted, StatusDeclined, StatusInProgress, StatusPartiallyDelivered,
		StatusDelivered, StatusUnableToComplete, StatusCancelled} {
		r, ok := transitions[from][to]
		if ok && ((coordinator && r.coordinator) || (!coordinator && r.volunteer)) {
			out = append(out, to)
		}
	}
	return out
}

// Record is an assignment row.
type Record struct {
	ID                          uuid.UUID
	OrgID                       uuid.UUID
	ClientID                    *uuid.UUID
	RequestID                   uuid.UUID
	OfferID                     *uuid.UUID
	VolunteerUserID             *uuid.UUID
	TeamLabel                   string
	AssignedBy                  uuid.UUID
	AssignedAt                  time.Time
	UpdatedAt                   time.Time
	Status                      Status
	QuantityAssigned            int
	Unit                        string
	AllocationReleased          bool
	Instructions                string
	ContactGrant                bool
	PickupGrant                 bool
	DestinationGrant            bool
	EtaText                     string
	AcceptedAt                  *time.Time
	StartedAt                   *time.Time
	CompletedAt                 *time.Time
	HandoverStatus              string
	HandoverNotes               string
	CompletionEvidenceType      string
	CompletionEvidenceReference string
	CancellationReason          string
	Version                     int
	VolunteerName               *string
	AssignedByName              *string
}

const selectSQL = `SELECT a.id, a.organization_id, a.client_id, a.request_id, a.offer_id, a.volunteer_user_id,
	a.team_label, a.assigned_by_user_id, a.assigned_at, a.updated_at, a.status, a.quantity_assigned, a.unit,
	a.allocation_released, a.instructions, a.protected_contact_access_granted, a.pickup_location_access_granted,
	a.destination_location_access_granted, a.eta_text, a.accepted_at, a.started_at, a.completed_at,
	a.handover_status, a.handover_notes, a.completion_evidence_type, a.completion_evidence_reference,
	a.cancellation_reason, a.version, vu.display_name, bu.display_name
	FROM assignments a
	LEFT JOIN users vu ON vu.id = a.volunteer_user_id
	LEFT JOIN users bu ON bu.id = a.assigned_by_user_id`

func scan(row pgx.Row) (*Record, error) {
	var a Record
	var status string
	err := row.Scan(&a.ID, &a.OrgID, &a.ClientID, &a.RequestID, &a.OfferID, &a.VolunteerUserID, &a.TeamLabel,
		&a.AssignedBy, &a.AssignedAt, &a.UpdatedAt, &status, &a.QuantityAssigned, &a.Unit, &a.AllocationReleased,
		&a.Instructions, &a.ContactGrant, &a.PickupGrant, &a.DestinationGrant, &a.EtaText, &a.AcceptedAt,
		&a.StartedAt, &a.CompletedAt, &a.HandoverStatus, &a.HandoverNotes, &a.CompletionEvidenceType,
		&a.CompletionEvidenceReference, &a.CancellationReason, &a.Version, &a.VolunteerName, &a.AssignedByName)
	if err != nil {
		return nil, err
	}
	a.Status = Status(status)
	return &a, nil
}

// Load fetches an assignment; nil when missing.
func Load(ctx context.Context, q database.Querier, id uuid.UUID, forUpdate bool) (*Record, error) {
	sql := selectSQL + ` WHERE a.id = $1`
	if forUpdate {
		sql += ` FOR UPDATE OF a`
	}
	a, err := scan(q.QueryRow(ctx, sql, id))
	if database.IsNoRows(err) {
		return nil, nil
	}
	return a, err
}

// Service implements assignment use cases.
type Service struct {
	DB     *pgxpool.Pool
	Sealer *locations.Sealer
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(err)
}

// UserRef is a minimal user reference.
type UserRef struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
}

// RequestSummary is the part of a request a responder needs for the task.
type RequestSummary struct {
	ID                          uuid.UUID      `json:"id"`
	Reference                   string         `json:"reference"`
	Status                      string         `json:"status"`
	Category                    string         `json:"category"`
	Urgency                     string         `json:"urgency"`
	Title                       string         `json:"title"`
	RequestedQuantity           *int           `json:"requested_quantity"`
	RequestedUnit               string         `json:"requested_unit"`
	RequestedByTime             *time.Time     `json:"requested_by_time"`
	Location                    locations.View `json:"location"`
	AccessibilityNotes          string         `json:"accessibility_notes"`
	RequiresFormalAuthorization bool           `json:"requires_formal_authorization"`
	SensitiveDataFlag           bool           `json:"sensitive_data_flag"`
	HasContactDetails           bool           `json:"has_contact_details"`
	ContactMethod               string         `json:"contact_method"`
}

// OfferSummary is the part of an offer a responder needs for pickup.
type OfferSummary struct {
	ID           uuid.UUID      `json:"id"`
	Reference    string         `json:"reference"`
	Title        string         `json:"title"`
	Category     string         `json:"category"`
	Unit         string         `json:"unit"`
	DeliveryMode string         `json:"pickup_or_delivery_mode"`
	Location     locations.View `json:"location"`
	Restrictions string         `json:"restrictions"`
}

// Permissions computed for the viewer.
type Permissions struct {
	AllowedStatuses    []string `json:"allowed_statuses"`
	CanEditAssignment  bool     `json:"can_edit_assignment"`
	CanUpdateHandover  bool     `json:"can_update_handover"`
	CanRevealProtected bool     `json:"can_reveal_protected"`
}

// View is an assignment as returned by the API.
type View struct {
	ID                               uuid.UUID       `json:"id"`
	ClientID                         *uuid.UUID      `json:"client_id,omitempty"`
	RequestID                        uuid.UUID       `json:"request_id"`
	OfferID                          *uuid.UUID      `json:"offer_id"`
	Volunteer                        *UserRef        `json:"volunteer"`
	TeamLabel                        string          `json:"team_label"`
	AssignedBy                       *UserRef        `json:"assigned_by,omitempty"`
	AssignedAt                       time.Time       `json:"assigned_at"`
	UpdatedAt                        time.Time       `json:"updated_at"`
	Status                           string          `json:"status"`
	QuantityAssigned                 int             `json:"quantity_assigned"`
	Unit                             string          `json:"unit"`
	Instructions                     string          `json:"instructions"`
	ProtectedContactAccessGranted    bool            `json:"protected_contact_access_granted"`
	PickupLocationAccessGranted      bool            `json:"pickup_location_access_granted"`
	DestinationLocationAccessGranted bool            `json:"destination_location_access_granted"`
	EtaText                          string          `json:"eta_text"`
	AcceptedAt                       *time.Time      `json:"accepted_at"`
	StartedAt                        *time.Time      `json:"started_at"`
	CompletedAt                      *time.Time      `json:"completed_at"`
	HandoverStatus                   string          `json:"handover_status"`
	HandoverNotes                    string          `json:"handover_notes"`
	CompletionEvidenceType           string          `json:"completion_evidence_type"`
	CompletionEvidenceReference      string          `json:"completion_evidence_reference"`
	CancellationReason               string          `json:"cancellation_reason"`
	Version                          int             `json:"version"`
	Request                          *RequestSummary `json:"request"`
	Offer                            *OfferSummary   `json:"offer"`
	ViewerRelation                   string          `json:"viewer_relation"`
	Permissions                      Permissions     `json:"permissions"`
}

func relation(p roles.Principal, a *Record) string {
	if p.Can(roles.CapAssignmentManage) && p.OrgID == a.OrgID {
		return "coordinator"
	}
	if a.VolunteerUserID != nil && *a.VolunteerUserID == p.UserID && p.Can(roles.CapAssignmentWorkOwn) {
		return "volunteer"
	}
	return ""
}

func (s *Service) project(ctx context.Context, q database.Querier, p roles.Principal, a *Record, st *settings.Settings) (*View, error) {
	rel := relation(p, a)
	v := &View{
		ID: a.ID, RequestID: a.RequestID, OfferID: a.OfferID, TeamLabel: a.TeamLabel, AssignedAt: a.AssignedAt,
		UpdatedAt: a.UpdatedAt, Status: string(a.Status), QuantityAssigned: a.QuantityAssigned, Unit: a.Unit,
		Instructions: a.Instructions, ProtectedContactAccessGranted: a.ContactGrant,
		PickupLocationAccessGranted: a.PickupGrant, DestinationLocationAccessGranted: a.DestinationGrant,
		EtaText: a.EtaText, AcceptedAt: a.AcceptedAt, StartedAt: a.StartedAt, CompletedAt: a.CompletedAt,
		HandoverStatus: a.HandoverStatus, HandoverNotes: a.HandoverNotes,
		CompletionEvidenceType: a.CompletionEvidenceType, CompletionEvidenceReference: a.CompletionEvidenceReference,
		CancellationReason: a.CancellationReason, Version: a.Version, ViewerRelation: rel,
	}
	if a.VolunteerUserID != nil {
		name := ""
		if a.VolunteerName != nil {
			name = *a.VolunteerName
		}
		v.Volunteer = &UserRef{ID: *a.VolunteerUserID, DisplayName: name}
	}
	if rel == "coordinator" {
		v.ClientID = a.ClientID
		name := ""
		if a.AssignedByName != nil {
			name = *a.AssignedByName
		}
		v.AssignedBy = &UserRef{ID: a.AssignedBy, DisplayName: name}
	}
	r, err := requests.Load(ctx, q, a.RequestID, false)
	if err != nil {
		return nil, err
	}
	if r != nil {
		v.Request = &RequestSummary{ID: r.ID, Reference: r.Reference, Status: string(r.Status), Category: r.Category,
			Urgency: r.Urgency, Title: r.Title, RequestedQuantity: r.Quantity, RequestedUnit: r.Unit,
			RequestedByTime: r.RequestedByTime, AccessibilityNotes: r.AccessibilityNotes,
			RequiresFormalAuthorization: r.RequiresFormalAuthorization, SensitiveDataFlag: r.Sensitive,
			HasContactDetails: len(r.ContactSealed) > 0, ContactMethod: r.ContactMethod,
			Location: locations.View{Mode: locations.Mode(r.LocationMode), AreaLabel: r.AreaLabel,
				ApproxLat: r.ApproxLat, ApproxLon: r.ApproxLon, PrecisionDecimals: r.ApproxDecimals,
				HasExact: len(r.ExactSealed) > 0}}
	}
	if a.OfferID != nil {
		o, err := offers.Load(ctx, q, *a.OfferID, false)
		if err != nil {
			return nil, err
		}
		if o != nil {
			v.Offer = &OfferSummary{ID: o.ID, Reference: o.Reference, Title: o.Title, Category: o.Category,
				Unit: o.Unit, DeliveryMode: o.DeliveryMode, Restrictions: o.Restrictions,
				Location: locations.View{Mode: locations.Mode(o.LocationMode), AreaLabel: o.AreaLabel,
					ApproxLat: o.ApproxLat, ApproxLon: o.ApproxLon, PrecisionDecimals: o.ApproxDecimals,
					HasExact: len(o.ExactSealed) > 0}}
		}
	}
	perm := Permissions{AllowedStatuses: []string{}}
	for _, st := range NextStatuses(a.Status, rel == "coordinator") {
		perm.AllowedStatuses = append(perm.AllowedStatuses, string(st))
	}
	perm.CanEditAssignment = rel == "coordinator" && !a.Status.IsTerminal()
	perm.CanUpdateHandover = a.Status == StatusAccepted || a.Status == StatusInProgress ||
		a.Status == StatusPartiallyDelivered || a.Status == StatusDelivered
	perm.CanRevealProtected = len(s.revealableFields(rel, a, v, r, st)) > 0
	v.Permissions = perm
	return v, nil
}

// revealableFields lists protected fields the viewer may reveal through this
// assignment.
func (s *Service) revealableFields(rel string, a *Record, v *View, r *requests.Record, st *settings.Settings) []string {
	if rel == "" || r == nil {
		return nil
	}
	if rel == "volunteer" && !(a.Status == StatusAccepted || a.Status == StatusInProgress || a.Status == StatusPartiallyDelivered) {
		return nil
	}
	needGrant := rel == "volunteer" && st.VolunteerAccessRequiresGrant
	var out []string
	if len(r.ContactSealed) > 0 && r.ContactVisibility == locations.ContactVisibilityAssignedResponders &&
		(!needGrant || a.ContactGrant) {
		out = append(out, "destination_contact")
	}
	if len(r.ExactSealed) > 0 && (!needGrant || a.DestinationGrant) {
		out = append(out, "destination_location")
	}
	if v.Offer != nil && v.Offer.Location.HasExact && (!needGrant || a.PickupGrant) {
		out = append(out, "pickup_location")
	}
	return out
}

func (s *Service) viewOf(ctx context.Context, p roles.Principal, a *Record) (*View, error) {
	st, err := settings.Get(ctx, s.DB)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	v, err := s.project(ctx, s.DB, p, a, st)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return v, nil
}

// CreateInput creates an assignment.
type CreateInput struct {
	ClientID                         *uuid.UUID `json:"client_id"`
	RequestID                        uuid.UUID  `json:"request_id"`
	OfferID                          *uuid.UUID `json:"offer_id"`
	VolunteerUserID                  *uuid.UUID `json:"volunteer_user_id"`
	TeamLabel                        string     `json:"team_label"`
	QuantityAssigned                 *int       `json:"quantity_assigned"`
	Unit                             string     `json:"unit"`
	Instructions                     string     `json:"instructions"`
	ProtectedContactAccessGranted    bool       `json:"protected_contact_access_granted"`
	PickupLocationAccessGranted      bool       `json:"pickup_location_access_granted"`
	DestinationLocationAccessGranted bool       `json:"destination_location_access_granted"`
	EtaText                          string     `json:"eta_text"`
}

// Create creates an assignment, allocating offer quantity atomically.
func (s *Service) Create(ctx context.Context, p roles.Principal, in CreateInput) (*View, bool, error) {
	if !p.Can(roles.CapAssignmentManage) {
		return nil, false, apperr.Forbidden("Only coordinators can create assignments.")
	}
	if in.ClientID != nil {
		var existing uuid.UUID
		err := s.DB.QueryRow(ctx, `SELECT id FROM assignments WHERE client_id = $1`, *in.ClientID).Scan(&existing)
		if err == nil {
			a, err := Load(ctx, s.DB, existing, false)
			if err != nil {
				return nil, false, apperr.Internal(err)
			}
			if a.AssignedBy != p.UserID {
				return nil, false, apperr.Conflict("client_id_conflict", "This client id is already in use.")
			}
			v, err := s.viewOf(ctx, p, a)
			return v, false, err
		} else if !database.IsNoRows(err) {
			return nil, false, apperr.Internal(err)
		}
	}
	errs := validation.Errors{}
	in.TeamLabel = validation.CleanText(in.TeamLabel)
	in.Unit = validation.CleanText(in.Unit)
	in.Instructions = validation.CleanText(in.Instructions)
	in.EtaText = validation.CleanText(in.EtaText)
	errs.Text("team_label", in.TeamLabel, 0, 120, false)
	errs.Text("unit", in.Unit, 0, 40, false)
	errs.Text("instructions", in.Instructions, 0, 2000, true)
	errs.Text("eta_text", in.EtaText, 0, 120, false)
	if in.OfferID == nil && in.VolunteerUserID == nil && in.TeamLabel == "" {
		errs.Add("volunteer_user_id", "Choose an offer, a volunteer or a team.")
	}
	if in.VolunteerUserID == nil && (in.ProtectedContactAccessGranted || in.PickupLocationAccessGranted || in.DestinationLocationAccessGranted) {
		errs.Add("protected_contact_access_granted", "Access can only be granted to an individual volunteer.")
	}
	if in.OfferID == nil && in.PickupLocationAccessGranted {
		errs.Add("pickup_location_access_granted", "Pickup location access requires an offer.")
	}
	qty := 0
	if in.QuantityAssigned != nil {
		qty = *in.QuantityAssigned
		errs.IntRange("quantity_assigned", in.QuantityAssigned, 0, 10000000)
	}
	if in.OfferID != nil && qty <= 0 {
		errs.Add("quantity_assigned", "Enter the quantity to allocate from the offer.")
	}
	if err := errs.Err(); err != nil {
		return nil, false, err
	}

	var out *Record
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		req, err := requests.Load(ctx, tx, in.RequestID, true)
		if err != nil {
			return err
		}
		if req == nil || req.DeletedAt != nil || req.OrgID != p.OrgID {
			return apperr.NotFound("Request not found.")
		}
		switch req.Status {
		case requests.StatusVerified, requests.StatusAssigned, requests.StatusInProgress, requests.StatusPartiallyResolved:
		default:
			return apperr.Conflict("request_not_assignable",
				"Only verified, open requests can be assigned. Review and verify the request first.").
				WithDetail("request_status", string(req.Status))
		}
		if in.VolunteerUserID != nil {
			var userRoles []string
			var active bool
			err := tx.QueryRow(ctx, `SELECT roles, is_active FROM users WHERE id = $1 AND organization_id = $2`,
				*in.VolunteerUserID, p.OrgID).Scan(&userRoles, &active)
			if database.IsNoRows(err) || (err == nil && (!active || !slices.Contains(userRoles, string(roles.Volunteer)))) {
				return apperr.ValidationField("volunteer_user_id", "Choose an active volunteer.")
			}
			if err != nil {
				return err
			}
		}
		id := uuid.New()
		var events []audit.Event
		unit := in.Unit
		if in.OfferID != nil {
			o, err := offers.Load(ctx, tx, *in.OfferID, true)
			if err != nil {
				return err
			}
			if o == nil || o.DeletedAt != nil || o.OrgID != p.OrgID {
				return apperr.ValidationField("offer_id", "Offer not found.")
			}
			ev, err := offers.Allocate(ctx, tx, p, o, qty, id)
			if err != nil {
				return err
			}
			ev = ev.ForRequest(req.ID)
			events = append(events, ev)
			if unit == "" {
				unit = o.Unit
			}
		}
		if unit == "" {
			unit = req.Unit
		}
		_, err = tx.Exec(ctx, `INSERT INTO assignments (id, organization_id, client_id, request_id, offer_id,
			volunteer_user_id, team_label, assigned_by_user_id, status, quantity_assigned, unit, instructions,
			protected_contact_access_granted, pickup_location_access_granted, destination_location_access_granted,
			eta_text) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'proposed',$9,$10,$11,$12,$13,$14,$15)`,
			id, p.OrgID, in.ClientID, req.ID, in.OfferID, in.VolunteerUserID, in.TeamLabel, p.UserID, qty, unit,
			in.Instructions, in.ProtectedContactAccessGranted, in.PickupLocationAccessGranted,
			in.DestinationLocationAccessGranted, in.EtaText)
		if err != nil {
			return err
		}
		created := audit.ForActor(p, "assignment.created", audit.Operational).On("assignment", id).ForRequest(req.ID).
			Status("", string(StatusProposed)).With("quantity", qty).
			With("protected_contact_access_granted", in.ProtectedContactAccessGranted).
			With("pickup_location_access_granted", in.PickupLocationAccessGranted).
			With("destination_location_access_granted", in.DestinationLocationAccessGranted)
		if in.OfferID != nil {
			created = created.With("offer_id", in.OfferID.String())
		}
		if in.VolunteerUserID != nil {
			created = created.With("volunteer_user_id", in.VolunteerUserID.String())
		}
		events = append([]audit.Event{created}, events...)
		if req.Status == requests.StatusVerified {
			ev, err := requests.ApplyTransition(ctx, tx, p, req, requests.StatusAssigned,
				requests.TransitionOptions{Cause: "assignment_created"})
			if err != nil {
				return err
			}
			events = append(events, ev)
		}
		if err := audit.Record(ctx, tx, events...); err != nil {
			return err
		}
		out, err = Load(ctx, tx, id, false)
		return err
	})
	if err != nil {
		return nil, false, wrap(err)
	}
	v, err := s.viewOf(ctx, p, out)
	return v, true, err
}

// loadVisible returns an assignment visible to the principal (404 otherwise).
func (s *Service) loadVisible(ctx context.Context, q database.Querier, p roles.Principal, id uuid.UUID, forUpdate bool) (*Record, string, error) {
	a, err := Load(ctx, q, id, forUpdate)
	if err != nil {
		return nil, "", apperr.Internal(err)
	}
	if a == nil {
		return nil, "", apperr.NotFound("Assignment not found.")
	}
	rel := relation(p, a)
	if rel == "" {
		return nil, "", apperr.NotFound("Assignment not found.")
	}
	return a, rel, nil
}

// Get returns an assignment.
func (s *Service) Get(ctx context.Context, p roles.Principal, id uuid.UUID) (*View, error) {
	a, _, err := s.loadVisible(ctx, s.DB, p, id, false)
	if err != nil {
		return nil, err
	}
	return s.viewOf(ctx, p, a)
}

// ListFilter filters assignments.
type ListFilter struct {
	RequestID    *uuid.UUID
	OfferID      *uuid.UUID
	VolunteerID  *uuid.UUID
	Statuses     []string
	ActiveOnly   bool
	Mine         bool
	UpdatedSince *time.Time
	Limit        int
	Offset       int
}

// ListResult is a page of assignments.
type ListResult struct {
	Items   []*View `json:"items"`
	HasMore bool    `json:"has_more"`
}

// List returns assignments visible to the principal. Volunteers only ever
// see their own assignments.
func (s *Service) List(ctx context.Context, p roles.Principal, f ListFilter) (*ListResult, error) {
	coordinator := p.Can(roles.CapAssignmentManage)
	if !coordinator && !p.Can(roles.CapAssignmentWorkOwn) {
		return nil, apperr.Forbidden("Your role does not include assignments.")
	}
	errs := validation.Errors{}
	for _, st := range f.Statuses {
		errs.OneOf("status", st, AllStatuses)
	}
	if err := errs.Err(); err != nil {
		return nil, err
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	mineOnly := !coordinator || f.Mine
	rows, err := s.DB.Query(ctx, selectSQL+` WHERE a.organization_id = $1
		AND (NOT $2 OR a.volunteer_user_id = $3)
		AND ($4::uuid IS NULL OR a.request_id = $4)
		AND ($5::uuid IS NULL OR a.offer_id = $5)
		AND ($6::uuid IS NULL OR a.volunteer_user_id = $6)
		AND (COALESCE(cardinality($7::text[]), 0) = 0 OR a.status = ANY($7))
		AND (NOT $8 OR a.status IN ('proposed', 'accepted', 'in_progress', 'partially_delivered'))
		AND ($9::timestamptz IS NULL OR a.updated_at >= $9)
		AND NOT EXISTS (SELECT 1 FROM aid_requests r WHERE r.id = a.request_id AND r.deleted_at IS NOT NULL)
		ORDER BY CASE WHEN a.status IN ('proposed', 'accepted', 'in_progress', 'partially_delivered') THEN 0 ELSE 1 END,
			a.assigned_at DESC, a.id LIMIT $10 OFFSET $11`,
		p.OrgID, mineOnly, p.UserID, f.RequestID, f.OfferID, f.VolunteerID, f.Statuses, f.ActiveOnly,
		f.UpdatedSince, f.Limit+1, f.Offset)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	var recs []*Record
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			rows.Close()
			return nil, apperr.Internal(err)
		}
		recs = append(recs, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	res := &ListResult{Items: []*View{}}
	if len(recs) > f.Limit {
		res.HasMore, recs = true, recs[:f.Limit]
	}
	st, err := settings.Get(ctx, s.DB)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for _, a := range recs {
		if relation(p, a) == "" {
			continue
		}
		v, err := s.project(ctx, s.DB, p, a, st)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		res.Items = append(res.Items, v)
	}
	return res, nil
}

// UpdateInput is a partial assignment update. Coordinators may change
// instructions, access grants and ETA; the assigned volunteer may change ETA
// and handover information.
type UpdateInput struct {
	Version                          *int    `json:"version"`
	Instructions                     *string `json:"instructions"`
	ProtectedContactAccessGranted    *bool   `json:"protected_contact_access_granted"`
	PickupLocationAccessGranted      *bool   `json:"pickup_location_access_granted"`
	DestinationLocationAccessGranted *bool   `json:"destination_location_access_granted"`
	EtaText                          *string `json:"eta_text"`
	HandoverStatus                   *string `json:"handover_status"`
	HandoverNotes                    *string `json:"handover_notes"`
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, p roles.Principal, id uuid.UUID, u UpdateInput) (*View, error) {
	var out *Record
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		a, rel, err := s.loadVisible(ctx, tx, p, id, true)
		if err != nil {
			return err
		}
		if u.Version != nil && *u.Version != a.Version {
			return apperr.Conflict("version_conflict", "This assignment was changed by someone else. Reload and try again.").
				WithDetail("current_version", a.Version)
		}
		coordFields := u.Instructions != nil || u.ProtectedContactAccessGranted != nil ||
			u.PickupLocationAccessGranted != nil || u.DestinationLocationAccessGranted != nil
		handoverFields := u.HandoverStatus != nil || u.HandoverNotes != nil
		if coordFields && rel != "coordinator" {
			return apperr.Forbidden("Only coordinators can change instructions or access grants.")
		}
		if coordFields && a.Status.IsTerminal() {
			return apperr.Conflict("assignment_closed", "This assignment is closed.")
		}
		if handoverFields && !(a.Status == StatusAccepted || a.Status == StatusInProgress ||
			a.Status == StatusPartiallyDelivered || a.Status == StatusDelivered) {
			return apperr.Conflict("handover_not_allowed", "Handover details can be recorded once the assignment is accepted.")
		}
		if u.EtaText != nil && a.Status.IsTerminal() {
			return apperr.Conflict("assignment_closed", "This assignment is closed.")
		}
		errs := validation.Errors{}
		var changed []string
		if u.Instructions != nil {
			v := validation.CleanText(*u.Instructions)
			errs.Text("instructions", v, 0, 2000, true)
			if v != a.Instructions {
				a.Instructions = v
				changed = append(changed, "instructions")
			}
		}
		grant := func(name string, dst *bool, v *bool) {
			if v != nil && *v != *dst {
				if a.VolunteerUserID == nil && *v {
					errs.Add(name, "Access can only be granted to an individual volunteer.")
				}
				*dst = *v
				changed = append(changed, name)
			}
		}
		grant("protected_contact_access_granted", &a.ContactGrant, u.ProtectedContactAccessGranted)
		grant("pickup_location_access_granted", &a.PickupGrant, u.PickupLocationAccessGranted)
		grant("destination_location_access_granted", &a.DestinationGrant, u.DestinationLocationAccessGranted)
		if a.PickupGrant && a.OfferID == nil {
			errs.Add("pickup_location_access_granted", "Pickup location access requires an offer.")
		}
		if u.EtaText != nil {
			v := validation.CleanText(*u.EtaText)
			errs.Text("eta_text", v, 0, 120, false)
			if v != a.EtaText {
				a.EtaText = v
				changed = append(changed, "eta_text")
			}
		}
		if u.HandoverStatus != nil {
			errs.OneOf("handover_status", *u.HandoverStatus, HandoverStatuses)
			if *u.HandoverStatus != a.HandoverStatus {
				a.HandoverStatus = *u.HandoverStatus
				changed = append(changed, "handover_status")
			}
		}
		if u.HandoverNotes != nil {
			v := validation.CleanText(*u.HandoverNotes)
			errs.Text("handover_notes", v, 0, 2000, true)
			if v != a.HandoverNotes {
				a.HandoverNotes = v
				changed = append(changed, "handover_notes")
			}
		}
		if err := errs.Err(); err != nil {
			return err
		}
		if len(changed) == 0 {
			out = a
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE assignments SET instructions=$2, protected_contact_access_granted=$3,
			pickup_location_access_granted=$4, destination_location_access_granted=$5, eta_text=$6,
			handover_status=$7, handover_notes=$8, version=version+1, updated_at=now() WHERE id=$1`,
			a.ID, a.Instructions, a.ContactGrant, a.PickupGrant, a.DestinationGrant, a.EtaText, a.HandoverStatus,
			a.HandoverNotes)
		if err != nil {
			return err
		}
		ev := audit.ForActor(p, "assignment.updated", audit.Operational).On("assignment", a.ID).
			ForRequest(a.RequestID).With("changed_fields", changed)
		if slices.ContainsFunc(changed, func(c string) bool { return len(c) > 7 && c[len(c)-7:] == "granted" }) {
			ev = ev.With("protected_contact_access_granted", a.ContactGrant).
				With("pickup_location_access_granted", a.PickupGrant).
				With("destination_location_access_granted", a.DestinationGrant)
		}
		if err := audit.Record(ctx, tx, ev); err != nil {
			return err
		}
		out, err = Load(ctx, tx, a.ID, false)
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	return s.viewOf(ctx, p, out)
}

// StatusInput changes an assignment status.
type StatusInput struct {
	Status                      string  `json:"status"`
	Reason                      string  `json:"reason"`
	CompletionEvidenceType      string  `json:"completion_evidence_type"`
	CompletionEvidenceReference string  `json:"completion_evidence_reference"`
	HandoverStatus              *string `json:"handover_status"`
	HandoverNotes               *string `json:"handover_notes"`
	EtaText                     *string `json:"eta_text"`
	Version                     *int    `json:"version"`
}

// ChangeStatus moves an assignment through its lifecycle.
func (s *Service) ChangeStatus(ctx context.Context, p roles.Principal, id uuid.UUID, in StatusInput) (*View, error) {
	// Resolve the request id first so locks are taken in the global order
	// request -> assignment -> offer.
	pre, _, err := s.loadVisible(ctx, s.DB, p, id, false)
	if err != nil {
		return nil, err
	}
	var out *Record
	err = database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		req, err := requests.Load(ctx, tx, pre.RequestID, true)
		if err != nil {
			return err
		}
		a, rel, err := s.loadVisible(ctx, tx, p, id, true)
		if err != nil {
			return err
		}
		to := Status(in.Status)
		if !slices.Contains(AllStatuses, in.Status) {
			return apperr.ValidationField("status", "Unknown status.")
		}
		if to == a.Status {
			out = a
			return nil
		}
		coordinator := rel == "coordinator"
		r, ok := transitions[a.Status][to]
		if !ok {
			return apperr.Conflict("invalid_transition", "This status change is not allowed from the current status.").
				WithDetail("current_status", string(a.Status)).
				WithDetail("allowed_statuses", NextStatuses(a.Status, coordinator))
		}
		if (coordinator && !r.coordinator) || (!coordinator && !r.volunteer) {
			return apperr.Forbidden("Only coordinators can set this status.")
		}
		reason := validation.CleanText(in.Reason)
		evRef := validation.CleanText(in.CompletionEvidenceReference)
		errs := validation.Errors{}
		errs.Text("reason", reason, 0, 1000, true)
		errs.Text("completion_evidence_reference", evRef, 0, 200, false)
		if to.releasesAllocation() {
			errs.Text("reason", reason, 3, 1000, true)
		}
		evType := a.CompletionEvidenceType
		if to == StatusDelivered || to == StatusPartiallyDelivered {
			evType = in.CompletionEvidenceType
			if evType == "" {
				evType = "volunteer_confirmation"
				if coordinator {
					evType = "coordinator_confirmation"
				}
			}
			errs.OneOf("completion_evidence_type", evType, EvidenceTypes)
		}
		handoverStatus, handoverNotes, eta := a.HandoverStatus, a.HandoverNotes, a.EtaText
		if in.HandoverStatus != nil {
			errs.OneOf("handover_status", *in.HandoverStatus, HandoverStatuses)
			handoverStatus = *in.HandoverStatus
		}
		if in.HandoverNotes != nil {
			handoverNotes = validation.CleanText(*in.HandoverNotes)
			errs.Text("handover_notes", handoverNotes, 0, 2000, true)
		}
		if in.EtaText != nil {
			eta = validation.CleanText(*in.EtaText)
			errs.Text("eta_text", eta, 0, 120, false)
		}
		if err := errs.Err(); err != nil {
			return err
		}

		now := s.now()
		accepted, started, completed := a.AcceptedAt, a.StartedAt, a.CompletedAt
		switch to {
		case StatusAccepted:
			accepted = &now
		case StatusInProgress:
			if started == nil {
				started = &now
			}
		case StatusDelivered, StatusDeclined, StatusUnableToComplete, StatusCancelled:
			completed = &now
		}
		cancellation := a.CancellationReason
		if to.releasesAllocation() {
			cancellation = reason
		}
		events := []audit.Event{
			audit.ForActor(p, "assignment.status_changed", audit.Operational).On("assignment", a.ID).
				ForRequest(a.RequestID).Status(string(a.Status), string(to)).WithReason(reason).
				With("completion_evidence_type", evType),
		}
		released := a.AllocationReleased
		if to.releasesAllocation() && a.OfferID != nil && !a.AllocationReleased && a.QuantityAssigned > 0 {
			ev, err := offers.Release(ctx, tx, p, *a.OfferID, a.QuantityAssigned, a.ID)
			if err != nil {
				return err
			}
			events = append(events, ev.ForRequest(a.RequestID))
			released = true
		}
		_, err = tx.Exec(ctx, `UPDATE assignments SET status=$2, accepted_at=$3, started_at=$4, completed_at=$5,
			completion_evidence_type=$6, completion_evidence_reference=$7, cancellation_reason=$8,
			handover_status=$9, handover_notes=$10, eta_text=$11, allocation_released=$12,
			version=version+1, updated_at=now() WHERE id=$1`,
			a.ID, string(to), accepted, started, completed, evType, evRef, cancellation, handoverStatus,
			handoverNotes, eta, released)
		if err != nil {
			return err
		}
		// Starting work on an assigned request moves the request to in
		// progress. This is the direct consequence of the explicit human
		// action and is recorded as such; no other request status is ever
		// changed automatically (e.g. delivery never resolves a request).
		if to == StatusInProgress && req != nil &&
			(req.Status == requests.StatusAssigned || req.Status == requests.StatusPartiallyResolved) {
			ev, err := requests.ApplyTransition(ctx, tx, p, req, requests.StatusInProgress,
				requests.TransitionOptions{Cause: "assignment_started"})
			if err != nil {
				return err
			}
			events = append(events, ev)
		}
		if err := audit.Record(ctx, tx, events...); err != nil {
			return err
		}
		out, err = Load(ctx, tx, a.ID, false)
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	return s.viewOf(ctx, p, out)
}

// CancelActiveForRequest cancels all active assignments of a request (inside
// the caller's transaction, which already holds the request lock) and
// releases their allocations. Implements requests.AssignmentCloser.
func (s *Service) CancelActiveForRequest(ctx context.Context, tx pgx.Tx, p roles.Principal, requestID uuid.UUID, reason string) ([]audit.Event, error) {
	rows, err := tx.Query(ctx, `SELECT id FROM assignments WHERE request_id = $1
		AND status IN ('proposed', 'accepted', 'in_progress', 'partially_delivered') ORDER BY id FOR UPDATE`, requestID)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	var events []audit.Event
	for _, id := range ids {
		a, err := Load(ctx, tx, id, false)
		if err != nil {
			return nil, err
		}
		released := a.AllocationReleased
		if a.OfferID != nil && !a.AllocationReleased && a.QuantityAssigned > 0 {
			ev, err := offers.Release(ctx, tx, p, *a.OfferID, a.QuantityAssigned, a.ID)
			if err != nil {
				return nil, err
			}
			events = append(events, ev.ForRequest(requestID))
			released = true
		}
		if _, err := tx.Exec(ctx, `UPDATE assignments SET status = 'cancelled', cancellation_reason = $2,
			completed_at = now(), allocation_released = $3, version = version + 1, updated_at = now() WHERE id = $1`,
			a.ID, reason, released); err != nil {
			return nil, err
		}
		events = append(events, audit.ForActor(p, "assignment.status_changed", audit.Operational).
			On("assignment", a.ID).ForRequest(requestID).Status(string(a.Status), string(StatusCancelled)).
			WithReason(reason).With("cause", "request_closed"))
	}
	return events, nil
}
