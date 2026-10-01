// Package offers implements structured resource offers and their manual,
// concurrency-safe allocation to requests.
//
// The system never allocates automatically. Coordinators allocate quantities
// explicitly through assignments; allocation locks the offer row and the
// database CHECK constraint offers_allocation_bounds guarantees that
// remaining quantity never drops below zero, even under concurrent updates.
package offers

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Status is an offer status.
type Status string

const (
	StatusDraft              Status = "draft"
	StatusAvailable          Status = "available"
	StatusPartiallyAllocated Status = "partially_allocated"
	StatusFullyAllocated     Status = "fully_allocated"
	StatusPaused             Status = "paused"
	StatusExpired            Status = "expired"
	StatusCancelled          Status = "cancelled"
)

// AllStatuses lists offer statuses.
var AllStatuses = []string{"draft", "available", "partially_allocated", "fully_allocated", "paused", "expired", "cancelled"}

// DeliveryModes lists pickup/delivery modes.
var DeliveryModes = []string{"pickup", "delivery", "either", "on_site"}

// IsClosed reports whether s is terminal.
func (s Status) IsClosed() bool { return s == StatusExpired || s == StatusCancelled }

// allocatable reports whether new allocations are allowed.
func (s Status) allocatable() bool { return s == StatusAvailable || s == StatusPartiallyAllocated }

// allocationTracking reports whether the status is derived from quantities.
func (s Status) allocationTracking() bool {
	return s == StatusAvailable || s == StatusPartiallyAllocated || s == StatusFullyAllocated
}

// DerivedStatus recomputes the allocation status from quantities. Paused,
// draft and closed offers keep their status.
func DerivedStatus(cur Status, available, assigned int) Status {
	if !cur.allocationTracking() {
		return cur
	}
	switch {
	case assigned <= 0:
		return StatusAvailable
	case assigned >= available:
		return StatusFullyAllocated
	default:
		return StatusPartiallyAllocated
	}
}

// Record is an offer row.
type Record struct {
	ID                 uuid.UUID
	Reference          string
	OrgID              uuid.UUID
	CreatedBy          *uuid.UUID
	ClientID           *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Status             Status
	Category           string
	Title              string
	Description        string
	QuantityAvailable  int
	Unit               string
	AvailabilityStart  *time.Time
	AvailabilityEnd    *time.Time
	DeliveryMode       string
	LocationMode       string
	AreaLabel          string
	ApproxLat          *float64
	ApproxLon          *float64
	ApproxDecimals     *int16
	ExactSealed        []byte
	AccessibilityNotes string
	Restrictions       string
	ContactVisibility  string
	ContactMethod      string
	ContactSealed      []byte
	VerificationLevel  string
	AssignedQuantity   int
	RemainingQuantity  int
	Tags               []string
	Version            int
	RedactedAt         *time.Time
	DeletedAt          *time.Time
	CreatedByName      *string
}

const selectSQL = `SELECT o.id, o.reference, o.organization_id, o.created_by_user_id, o.client_id, o.created_at,
	o.updated_at, o.status, o.category, o.title, o.description, o.quantity_available, o.unit,
	o.availability_start, o.availability_end, o.pickup_or_delivery_mode, o.location_mode, o.area_label,
	o.approx_lat, o.approx_lon, o.approx_decimals, o.exact_location_sealed, o.accessibility_notes,
	o.restrictions, o.contact_visibility, o.contact_method, o.contact_details_sealed, o.verification_level,
	o.assigned_quantity, o.remaining_quantity, o.tags, o.version, o.redacted_at, o.deleted_at, cu.display_name
	FROM offers o LEFT JOIN users cu ON cu.id = o.created_by_user_id`

func scan(row pgx.Row) (*Record, error) {
	var o Record
	var status string
	err := row.Scan(&o.ID, &o.Reference, &o.OrgID, &o.CreatedBy, &o.ClientID, &o.CreatedAt, &o.UpdatedAt,
		&status, &o.Category, &o.Title, &o.Description, &o.QuantityAvailable, &o.Unit, &o.AvailabilityStart,
		&o.AvailabilityEnd, &o.DeliveryMode, &o.LocationMode, &o.AreaLabel, &o.ApproxLat, &o.ApproxLon,
		&o.ApproxDecimals, &o.ExactSealed, &o.AccessibilityNotes, &o.Restrictions, &o.ContactVisibility,
		&o.ContactMethod, &o.ContactSealed, &o.VerificationLevel, &o.AssignedQuantity, &o.RemainingQuantity,
		&o.Tags, &o.Version, &o.RedactedAt, &o.DeletedAt, &o.CreatedByName)
	if err != nil {
		return nil, err
	}
	o.Status = Status(status)
	if o.Tags == nil {
		o.Tags = []string{}
	}
	return &o, nil
}

// Load fetches an offer by id; nil when missing.
func Load(ctx context.Context, q database.Querier, id uuid.UUID, forUpdate bool) (*Record, error) {
	sql := selectSQL + ` WHERE o.id = $1`
	if forUpdate {
		sql += ` FOR UPDATE OF o`
	}
	o, err := scan(q.QueryRow(ctx, sql, id))
	if database.IsNoRows(err) {
		return nil, nil
	}
	return o, err
}

func loadBy(ctx context.Context, q database.Querier, where string, arg any) (*Record, error) {
	o, err := scan(q.QueryRow(ctx, selectSQL+` WHERE `+where, arg))
	if database.IsNoRows(err) {
		return nil, nil
	}
	return o, err
}

// AAD returns the additional authenticated data for a protected offer field.
func AAD(id uuid.UUID, field string) string { return locations.AAD("offer", id.String(), field) }

// Service implements offer use cases.
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

// Relation of a viewer to an offer.
type Relation string

const (
	RelationNone              Relation = ""
	RelationCoordinator       Relation = "coordinator"
	RelationOwner             Relation = "owner"
	RelationAssignedVolunteer Relation = "assigned_volunteer"
)

type pickupAccess struct {
	visible bool
	active  bool
	grant   bool
}

func loadPickupAccess(ctx context.Context, q database.Querier, offerIDs []uuid.UUID, userID uuid.UUID) (map[uuid.UUID]*pickupAccess, error) {
	out := map[uuid.UUID]*pickupAccess{}
	if len(offerIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT offer_id, status, pickup_location_access_granted FROM assignments
		WHERE offer_id = ANY($1) AND volunteer_user_id = $2`, offerIDs, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var oid uuid.UUID
		var status string
		var grant bool
		if err := rows.Scan(&oid, &status, &grant); err != nil {
			return nil, err
		}
		a := out[oid]
		if a == nil {
			a = &pickupAccess{}
			out[oid] = a
		}
		if status == "declined" || status == "cancelled" {
			continue
		}
		a.visible = true
		if status == "accepted" || status == "in_progress" || status == "partially_delivered" {
			a.active = true
			a.grant = a.grant || grant
		}
	}
	return out, rows.Err()
}

func relationFor(p roles.Principal, o *Record, acc *pickupAccess) Relation {
	if o.DeletedAt != nil {
		return RelationNone
	}
	if p.Can(roles.CapOfferReadAll) && p.OrgID == o.OrgID {
		return RelationCoordinator
	}
	if o.CreatedBy != nil && *o.CreatedBy == p.UserID && p.Can(roles.CapOfferCreate) {
		return RelationOwner
	}
	if acc != nil && acc.visible && p.Can(roles.CapAssignmentWorkOwn) {
		return RelationAssignedVolunteer
	}
	return RelationNone
}

// UserRef is a minimal user reference.
type UserRef struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
}

// Permissions computed for the viewer.
type Permissions struct {
	CanEdit            bool     `json:"can_edit"`
	AllowedStatuses    []string `json:"allowed_statuses"`
	CanRevealProtected bool     `json:"can_reveal_protected"`
	CanAllocate        bool     `json:"can_allocate"`
	NoteVisibilities   []string `json:"note_visibilities"`
}

// View is the role-specific projection of an offer.
type View struct {
	ID                 uuid.UUID      `json:"id"`
	ClientID           *uuid.UUID     `json:"client_id,omitempty"`
	Reference          string         `json:"reference"`
	Status             string         `json:"status"`
	Category           string         `json:"category"`
	Title              string         `json:"title"`
	Description        string         `json:"description"`
	QuantityAvailable  int            `json:"quantity_available"`
	Unit               string         `json:"unit"`
	AssignedQuantity   int            `json:"assigned_quantity"`
	RemainingQuantity  int            `json:"remaining_quantity"`
	AvailabilityStart  *time.Time     `json:"availability_start"`
	AvailabilityEnd    *time.Time     `json:"availability_end"`
	DeliveryMode       string         `json:"pickup_or_delivery_mode"`
	Location           locations.View `json:"location"`
	AccessibilityNotes string         `json:"accessibility_notes"`
	Restrictions       string         `json:"restrictions"`
	ContactMethod      string         `json:"contact_method"`
	ContactVisibility  string         `json:"contact_visibility"`
	HasContactDetails  bool           `json:"has_contact_details"`
	VerificationLevel  string         `json:"verification_level,omitempty"`
	Tags               []string       `json:"tags,omitempty"`
	CreatedBy          *UserRef       `json:"created_by,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	Version            int            `json:"version"`
	Redacted           bool           `json:"redacted"`
	ViewerRelation     Relation       `json:"viewer_relation"`
	Permissions        Permissions    `json:"permissions"`
}

func ownerAllowedStatuses(o *Record) []Status {
	switch o.Status {
	case StatusDraft:
		return []Status{StatusAvailable, StatusCancelled}
	case StatusAvailable, StatusPartiallyAllocated, StatusFullyAllocated:
		if o.AssignedQuantity == 0 {
			return []Status{StatusPaused, StatusCancelled}
		}
		return []Status{StatusPaused}
	case StatusPaused:
		if o.AssignedQuantity == 0 {
			return []Status{StatusAvailable, StatusCancelled}
		}
		return []Status{StatusAvailable}
	}
	return nil
}

func coordinatorAllowedStatuses(o *Record) []Status {
	switch o.Status {
	case StatusDraft:
		return []Status{StatusAvailable, StatusCancelled}
	case StatusAvailable, StatusPartiallyAllocated, StatusFullyAllocated:
		return []Status{StatusPaused, StatusExpired, StatusCancelled}
	case StatusPaused:
		return []Status{StatusAvailable, StatusExpired, StatusCancelled}
	}
	return nil
}

func allowedFor(rel Relation, o *Record) []Status {
	switch rel {
	case RelationCoordinator:
		return coordinatorAllowedStatuses(o)
	case RelationOwner:
		return ownerAllowedStatuses(o)
	}
	return nil
}

func project(o *Record, rel Relation, acc *pickupAccess, st *settings.Settings) *View {
	v := &View{
		ID: o.ID, Reference: o.Reference, Status: string(o.Status), Category: o.Category, Title: o.Title,
		Description: o.Description, QuantityAvailable: o.QuantityAvailable, Unit: o.Unit,
		AssignedQuantity: o.AssignedQuantity, RemainingQuantity: o.RemainingQuantity,
		AvailabilityStart: o.AvailabilityStart, AvailabilityEnd: o.AvailabilityEnd, DeliveryMode: o.DeliveryMode,
		AccessibilityNotes: o.AccessibilityNotes, Restrictions: o.Restrictions, ContactMethod: o.ContactMethod,
		ContactVisibility: o.ContactVisibility, HasContactDetails: len(o.ContactSealed) > 0,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, Version: o.Version, Redacted: o.RedactedAt != nil,
		ViewerRelation: rel,
		Location: locations.View{Mode: locations.Mode(o.LocationMode), AreaLabel: o.AreaLabel,
			ApproxLat: o.ApproxLat, ApproxLon: o.ApproxLon, PrecisionDecimals: o.ApproxDecimals,
			HasExact: len(o.ExactSealed) > 0},
	}
	perm := Permissions{AllowedStatuses: []string{}, NoteVisibilities: []string{}}
	if o.RedactedAt == nil {
		for _, s := range allowedFor(rel, o) {
			perm.AllowedStatuses = append(perm.AllowedStatuses, string(s))
		}
	}
	switch rel {
	case RelationCoordinator:
		v.ClientID = o.ClientID
		v.VerificationLevel = o.VerificationLevel
		v.Tags = o.Tags
		if o.CreatedBy != nil {
			name := ""
			if o.CreatedByName != nil {
				name = *o.CreatedByName
			}
			v.CreatedBy = &UserRef{ID: *o.CreatedBy, DisplayName: name}
		}
		perm.CanEdit = !o.Status.IsClosed() && o.RedactedAt == nil
		perm.CanAllocate = o.Status.allocatable() && o.RemainingQuantity > 0
		perm.CanRevealProtected = len(o.ContactSealed) > 0 || len(o.ExactSealed) > 0
		perm.NoteVisibilities = []string{"internal", "shared"}
	case RelationOwner:
		v.ClientID = o.ClientID
		v.VerificationLevel = o.VerificationLevel
		perm.CanEdit = !o.Status.IsClosed() && o.AssignedQuantity == 0 && o.RedactedAt == nil
		perm.CanRevealProtected = len(o.ContactSealed) > 0 || len(o.ExactSealed) > 0
		perm.NoteVisibilities = []string{"shared"}
	case RelationAssignedVolunteer:
		// Pickup volunteers see logistics details only.
		v.ContactMethod, v.HasContactDetails = "", false
		perm.CanRevealProtected = acc != nil && acc.active && len(o.ExactSealed) > 0 &&
			(acc.grant || !st.VolunteerAccessRequiresGrant)
	}
	v.Permissions = perm
	return v
}

// Input is the payload for creating an offer.
type Input struct {
	ClientID           *uuid.UUID             `json:"client_id"`
	Category           string                 `json:"category"`
	Title              string                 `json:"title"`
	Description        string                 `json:"description"`
	QuantityAvailable  *int                   `json:"quantity_available"`
	Unit               string                 `json:"unit"`
	AvailabilityStart  *time.Time             `json:"availability_start"`
	AvailabilityEnd    *time.Time             `json:"availability_end"`
	DeliveryMode       string                 `json:"pickup_or_delivery_mode"`
	Location           locations.Input        `json:"location"`
	AccessibilityNotes string                 `json:"accessibility_notes"`
	Restrictions       string                 `json:"restrictions"`
	Contact            locations.ContactInput `json:"contact"`
	Tags               []string               `json:"tags"`
	Publish            *bool                  `json:"publish"`
	ClientCreatedAt    *time.Time             `json:"client_created_at"`
}

func (s *Service) validateCommon(ctx context.Context, q database.Querier, category string, title, description,
	unit, accessibility, restrictions *string, mode *string, start, end *time.Time, errs validation.Errors) {
	cat, err := settings.GetCategory(ctx, q, category)
	if err != nil || cat == nil || !cat.Enabled {
		errs.Add("category", "Choose an available category.")
	}
	*title = validation.CleanText(*title)
	*description = validation.CleanText(*description)
	*unit = validation.CleanText(*unit)
	*accessibility = validation.CleanText(*accessibility)
	*restrictions = validation.CleanText(*restrictions)
	errs.Text("title", *title, 3, 120, false)
	errs.Text("description", *description, 0, 4000, true)
	errs.Text("unit", *unit, 0, 40, false)
	errs.Text("accessibility_notes", *accessibility, 0, 1000, true)
	errs.Text("restrictions", *restrictions, 0, 1000, true)
	if *mode == "" {
		*mode = "pickup"
	}
	errs.OneOf("pickup_or_delivery_mode", *mode, DeliveryModes)
	now := s.now()
	errs.TimeRange("availability_start", start, now.AddDate(-1, 0, 0), now.AddDate(2, 0, 0))
	errs.TimeRange("availability_end", end, now.AddDate(-1, 0, 0), now.AddDate(2, 0, 0))
	if start != nil && end != nil && end.Before(*start) {
		errs.Add("availability_end", "The end must be after the start.")
	}
}

// Create creates an offer (idempotent with ClientID).
func (s *Service) Create(ctx context.Context, p roles.Principal, in Input) (*View, bool, error) {
	if !p.Can(roles.CapOfferCreate) {
		return nil, false, apperr.Forbidden("You are not allowed to create offers.")
	}
	if in.ClientID != nil {
		existing, err := loadBy(ctx, s.DB, "o.client_id = $1", *in.ClientID)
		if err != nil {
			return nil, false, apperr.Internal(err)
		}
		if existing != nil {
			if existing.CreatedBy == nil || *existing.CreatedBy != p.UserID {
				return nil, false, apperr.Conflict("client_id_conflict", "This client id is already in use.")
			}
			v, err := s.view(ctx, p, existing)
			return v, false, err
		}
	}
	st, err := settings.Get(ctx, s.DB)
	if err != nil {
		return nil, false, apperr.Internal(err)
	}
	errs := validation.Errors{}
	s.validateCommon(ctx, s.DB, in.Category, &in.Title, &in.Description, &in.Unit, &in.AccessibilityNotes,
		&in.Restrictions, &in.DeliveryMode, in.AvailabilityStart, in.AvailabilityEnd, errs)
	if in.QuantityAvailable == nil {
		errs.Add("quantity_available", "Enter the available quantity.")
	} else {
		errs.IntRange("quantity_available", in.QuantityAvailable, 0, 10000000)
	}
	loc := locations.Normalize(in.Location, st.ApproxLocationDecimals, errs)
	if loc.KeepExact {
		errs.Add("location.exact", "An exact address or coordinates are required.")
	}
	contact := locations.NormalizeContact(in.Contact, errs)
	tags := []string{}
	if len(in.Tags) > 0 {
		if !p.IsCoordinator() {
			errs.Add("tags", "Only coordinators can set tags.")
		} else if t := errs.Tags("tags", in.Tags); t != nil {
			tags = t
		}
	}
	if err := errs.Err(); err != nil {
		return nil, false, err
	}
	status := StatusAvailable
	if in.Publish != nil && !*in.Publish {
		status = StatusDraft
	}
	verification := "self_reported"
	if p.IsCoordinator() {
		verification = "unverified"
	}
	now := s.now()
	id := uuid.New()
	exactSealed, err := s.Sealer.Seal(loc.ExactBytes, AAD(id, "exact_location"))
	if err != nil {
		return nil, false, apperr.Internal(err)
	}
	contactSealed, err := s.Sealer.Seal([]byte(contact.Details), AAD(id, "contact"))
	if err != nil {
		return nil, false, apperr.Internal(err)
	}
	var rec *Record
	err = database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		ref, err := database.NextReference(ctx, tx, "OF", now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO offers (id, reference, organization_id, created_by_user_id, client_id,
			created_at, updated_at, status, category, title, description, quantity_available, unit,
			availability_start, availability_end, pickup_or_delivery_mode, location_mode, area_label, approx_lat,
			approx_lon, approx_decimals, exact_location_sealed, accessibility_notes, restrictions,
			contact_visibility, contact_method, contact_details_sealed, verification_level, tags)
			VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28)`,
			id, ref, p.OrgID, p.UserID, in.ClientID, now, string(status), in.Category, in.Title, in.Description,
			*in.QuantityAvailable, in.Unit, in.AvailabilityStart, in.AvailabilityEnd, in.DeliveryMode, string(loc.Mode),
			loc.AreaLabel, loc.ApproxLat, loc.ApproxLon, loc.Decimals, exactSealed, in.AccessibilityNotes,
			in.Restrictions, contact.Visibility, contact.Method, contactSealed, verification, tags)
		if err != nil {
			if database.IsUniqueViolation(err, "offers_client_id_key") {
				return apperr.Conflict("client_id_conflict", "This offer was already submitted.")
			}
			return err
		}
		ev := audit.ForActor(p, "offer.created", audit.Shared).On("offer", id).Status("", string(status)).
			With("category", in.Category).With("quantity_available", *in.QuantityAvailable)
		if in.ClientCreatedAt != nil {
			ev = ev.With("created_offline", true)
		}
		if err := audit.Record(ctx, tx, ev); err != nil {
			return err
		}
		rec, err = Load(ctx, tx, id, false)
		return err
	})
	if err != nil {
		return nil, false, wrap(err)
	}
	v, err := s.view(ctx, p, rec)
	return v, true, err
}

func (s *Service) view(ctx context.Context, p roles.Principal, o *Record) (*View, error) {
	st, err := settings.Get(ctx, s.DB)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	accMap, err := loadPickupAccess(ctx, s.DB, []uuid.UUID{o.ID}, p.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	rel := relationFor(p, o, accMap[o.ID])
	if rel == RelationNone {
		return nil, apperr.NotFound("Offer not found.")
	}
	return project(o, rel, accMap[o.ID], st), nil
}

func (s *Service) loadVisible(ctx context.Context, q database.Querier, p roles.Principal, id uuid.UUID, forUpdate bool) (*Record, Relation, *pickupAccess, error) {
	o, err := Load(ctx, q, id, forUpdate)
	if err != nil {
		return nil, RelationNone, nil, apperr.Internal(err)
	}
	if o == nil {
		return nil, RelationNone, nil, apperr.NotFound("Offer not found.")
	}
	accMap, err := loadPickupAccess(ctx, q, []uuid.UUID{o.ID}, p.UserID)
	if err != nil {
		return nil, RelationNone, nil, apperr.Internal(err)
	}
	rel := relationFor(p, o, accMap[o.ID])
	if rel == RelationNone {
		return nil, RelationNone, nil, apperr.NotFound("Offer not found.")
	}
	return o, rel, accMap[o.ID], nil
}

// Get returns an offer view.
func (s *Service) Get(ctx context.Context, p roles.Principal, id uuid.UUID) (*View, error) {
	o, _, _, err := s.loadVisible(ctx, s.DB, p, id, false)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, p, o)
}

// ListFilter filters offers.
type ListFilter struct {
	Statuses       []string
	Category       string
	AllocatableNow bool
	UpdatedSince   *time.Time
	Limit, Offset  int
}

// ListResult is a page of offers.
type ListResult struct {
	Items   []*View `json:"items"`
	HasMore bool    `json:"has_more"`
}

// List returns offers visible to the principal.
func (s *Service) List(ctx context.Context, p roles.Principal, f ListFilter) (*ListResult, error) {
	coordinator := p.Can(roles.CapOfferReadAll)
	if !coordinator && !p.Can(roles.CapOfferCreate) {
		return nil, apperr.Forbidden("Your role does not include access to offers.")
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
	args := []any{p.OrgID, p.UserID, coordinator, f.Statuses, f.Category, f.AllocatableNow, f.UpdatedSince, f.Limit + 1, f.Offset}
	rows, err := s.DB.Query(ctx, selectSQL+` WHERE o.deleted_at IS NULL AND o.organization_id = $1
		AND ($3 OR o.created_by_user_id = $2 OR EXISTS (SELECT 1 FROM assignments a WHERE a.offer_id = o.id
			AND a.volunteer_user_id = $2 AND a.status NOT IN ('declined', 'cancelled')))
		AND (COALESCE(cardinality($4::text[]), 0) = 0 OR o.status = ANY($4))
		AND ($5 = '' OR o.category = $5)
		AND (NOT $6 OR (o.status IN ('available', 'partially_allocated') AND o.remaining_quantity > 0))
		AND ($7::timestamptz IS NULL OR o.updated_at >= $7)
		ORDER BY o.category, o.created_at DESC, o.id LIMIT $8 OFFSET $9`, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	var recs []*Record
	for rows.Next() {
		o, err := scan(rows)
		if err != nil {
			rows.Close()
			return nil, apperr.Internal(err)
		}
		recs = append(recs, o)
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
	ids := make([]uuid.UUID, len(recs))
	for i, o := range recs {
		ids[i] = o.ID
	}
	accMap, err := loadPickupAccess(ctx, s.DB, ids, p.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for _, o := range recs {
		if rel := relationFor(p, o, accMap[o.ID]); rel != RelationNone {
			res.Items = append(res.Items, project(o, rel, accMap[o.ID], st))
		}
	}
	return res, nil
}

// UpdateInput is a partial offer update.
type UpdateInput struct {
	Version            *int                      `json:"version"`
	Category           *string                   `json:"category"`
	Title              *string                   `json:"title"`
	Description        *string                   `json:"description"`
	QuantityAvailable  *int                      `json:"quantity_available"`
	Unit               *string                   `json:"unit"`
	AvailabilityStart  httpx.Optional[time.Time] `json:"availability_start"`
	AvailabilityEnd    httpx.Optional[time.Time] `json:"availability_end"`
	DeliveryMode       *string                   `json:"pickup_or_delivery_mode"`
	Location           *locations.Input          `json:"location"`
	AccessibilityNotes *string                   `json:"accessibility_notes"`
	Restrictions       *string                   `json:"restrictions"`
	Contact            *locations.ContactInput   `json:"contact"`
	VerificationLevel  *string                   `json:"verification_level"`
	Tags               *[]string                 `json:"tags"`
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, p roles.Principal, id uuid.UUID, u UpdateInput) (*View, error) {
	var out *Record
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		o, rel, _, err := s.loadVisible(ctx, tx, p, id, true)
		if err != nil {
			return err
		}
		if o.Status.IsClosed() || o.RedactedAt != nil {
			return apperr.Conflict("offer_closed", "Closed offers cannot be edited.")
		}
		switch rel {
		case RelationCoordinator:
		case RelationOwner:
			if o.AssignedQuantity > 0 {
				return apperr.ForbiddenCode("edit_locked", "Part of this offer is allocated. Ask a coordinator to change it.")
			}
			if u.VerificationLevel != nil || u.Tags != nil {
				return apperr.Forbidden("Only coordinators can change verification or tags.")
			}
		default:
			return apperr.Forbidden("You cannot edit this offer.")
		}
		if u.Version != nil && *u.Version != o.Version {
			return apperr.Conflict("version_conflict", "This offer was changed by someone else. Reload and try again.").
				WithDetail("current_version", o.Version)
		}
		st, err := settings.Get(ctx, tx)
		if err != nil {
			return err
		}
		errs := validation.Errors{}
		var changed []string
		mark := func(n string) {
			if !slices.Contains(changed, n) {
				changed = append(changed, n)
			}
		}
		setStr := func(name string, dst *string, v *string) {
			if v != nil && *v != *dst {
				*dst = *v
				mark(name)
			}
		}
		setStr("category", &o.Category, u.Category)
		setStr("title", &o.Title, u.Title)
		setStr("description", &o.Description, u.Description)
		setStr("unit", &o.Unit, u.Unit)
		setStr("pickup_or_delivery_mode", &o.DeliveryMode, u.DeliveryMode)
		setStr("accessibility_notes", &o.AccessibilityNotes, u.AccessibilityNotes)
		setStr("restrictions", &o.Restrictions, u.Restrictions)
		if u.AvailabilityStart.Set {
			o.AvailabilityStart = u.AvailabilityStart.Ptr()
			mark("availability_start")
		}
		if u.AvailabilityEnd.Set {
			o.AvailabilityEnd = u.AvailabilityEnd.Ptr()
			mark("availability_end")
		}
		s.validateCommon(ctx, tx, o.Category, &o.Title, &o.Description, &o.Unit, &o.AccessibilityNotes,
			&o.Restrictions, &o.DeliveryMode, o.AvailabilityStart, o.AvailabilityEnd, errs)
		oldQty := o.QuantityAvailable
		if u.QuantityAvailable != nil && *u.QuantityAvailable != o.QuantityAvailable {
			errs.IntRange("quantity_available", u.QuantityAvailable, 0, 10000000)
			if *u.QuantityAvailable < o.AssignedQuantity {
				errs.Add("quantity_available", "Cannot be lower than the quantity already allocated.")
			}
			o.QuantityAvailable = *u.QuantityAvailable
			mark("quantity_available")
		}
		if u.Location != nil {
			loc := locations.Normalize(*u.Location, st.ApproxLocationDecimals, errs)
			if loc.KeepExact {
				if len(o.ExactSealed) == 0 {
					errs.Add("location.exact", "There is no stored exact location to keep.")
				}
				o.AreaLabel = loc.AreaLabel
			} else {
				sealed, err := s.Sealer.Seal(loc.ExactBytes, AAD(o.ID, "exact_location"))
				if err != nil {
					return err
				}
				o.LocationMode, o.AreaLabel = string(loc.Mode), loc.AreaLabel
				o.ApproxLat, o.ApproxLon, o.ApproxDecimals, o.ExactSealed = loc.ApproxLat, loc.ApproxLon, loc.Decimals, sealed
			}
			mark("location")
		}
		if u.Contact != nil {
			c := locations.NormalizeContact(*u.Contact, errs)
			o.ContactMethod, o.ContactVisibility = c.Method, c.Visibility
			if !c.Keep {
				sealed, err := s.Sealer.Seal([]byte(c.Details), AAD(o.ID, "contact"))
				if err != nil {
					return err
				}
				o.ContactSealed = sealed
			}
			if c.Visibility == locations.ContactVisibilityNone {
				o.ContactSealed = nil
			}
			mark("contact")
		}
		if u.VerificationLevel != nil && *u.VerificationLevel != o.VerificationLevel {
			errs.OneOf("verification_level", *u.VerificationLevel,
				[]string{"unverified", "self_reported", "coordinator_confirmed", "field_confirmed"})
			o.VerificationLevel = *u.VerificationLevel
			mark("verification_level")
		}
		if u.Tags != nil {
			if t := errs.Tags("tags", *u.Tags); t != nil && !slices.Equal(t, o.Tags) {
				o.Tags = t
				mark("tags")
			}
		}
		if err := errs.Err(); err != nil {
			return err
		}
		if len(changed) == 0 {
			out = o
			return nil
		}
		o.Status = DerivedStatus(o.Status, o.QuantityAvailable, o.AssignedQuantity)
		_, err = tx.Exec(ctx, `UPDATE offers SET category=$2, title=$3, description=$4, quantity_available=$5,
			unit=$6, availability_start=$7, availability_end=$8, pickup_or_delivery_mode=$9, location_mode=$10,
			area_label=$11, approx_lat=$12, approx_lon=$13, approx_decimals=$14, exact_location_sealed=$15,
			accessibility_notes=$16, restrictions=$17, contact_visibility=$18, contact_method=$19,
			contact_details_sealed=$20, verification_level=$21, tags=$22, status=$23,
			version=version+1, updated_at=now() WHERE id=$1`,
			o.ID, o.Category, o.Title, o.Description, o.QuantityAvailable, o.Unit, o.AvailabilityStart,
			o.AvailabilityEnd, o.DeliveryMode, o.LocationMode, o.AreaLabel, o.ApproxLat, o.ApproxLon,
			o.ApproxDecimals, o.ExactSealed, o.AccessibilityNotes, o.Restrictions, o.ContactVisibility,
			o.ContactMethod, o.ContactSealed, o.VerificationLevel, o.Tags, string(o.Status))
		if err != nil {
			if database.IsCheckViolation(err, "offers_allocation_bounds") {
				return apperr.Conflict("over_allocation", "Quantity cannot be lower than the allocated quantity.")
			}
			return err
		}
		ev := audit.ForActor(p, "offer.updated", audit.Shared).On("offer", o.ID).With("changed_fields", changed)
		if oldQty != o.QuantityAvailable {
			ev = ev.With("quantity_from", oldQty).With("quantity_to", o.QuantityAvailable)
		}
		if err := audit.Record(ctx, tx, ev); err != nil {
			return err
		}
		out, err = Load(ctx, tx, o.ID, false)
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	return s.view(ctx, p, out)
}

// StatusInput changes an offer status.
type StatusInput struct {
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Version *int   `json:"version"`
}

// ChangeStatus changes an offer status manually. Allocation statuses
// (partially/fully allocated) are derived and cannot be set directly.
func (s *Service) ChangeStatus(ctx context.Context, p roles.Principal, id uuid.UUID, in StatusInput) (*View, error) {
	var out *Record
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		o, rel, _, err := s.loadVisible(ctx, tx, p, id, true)
		if err != nil {
			return err
		}
		to := Status(in.Status)
		if !slices.Contains(AllStatuses, in.Status) {
			return apperr.ValidationField("status", "Unknown status.")
		}
		if to == o.Status {
			out = o
			return nil
		}
		allowed := allowedFor(rel, o)
		if !slices.Contains(allowed, to) {
			if rel == RelationOwner && slices.Contains(coordinatorAllowedStatuses(o), to) {
				return apperr.Forbidden("Only coordinators can set this status.")
			}
			return apperr.Conflict("invalid_transition", "This status change is not allowed from the current status.").
				WithDetail("current_status", string(o.Status)).WithDetail("allowed_statuses", allowed)
		}
		reason := validation.CleanText(in.Reason)
		errs := validation.Errors{}
		errs.Text("reason", reason, 0, 1000, true)
		if to == StatusCancelled {
			errs.Text("reason", reason, 3, 1000, true)
		}
		if err := errs.Err(); err != nil {
			return err
		}
		if to.IsClosed() {
			var active int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM assignments WHERE offer_id = $1
				AND status IN ('proposed', 'accepted', 'in_progress', 'partially_delivered')`, o.ID).Scan(&active); err != nil {
				return err
			}
			if active > 0 {
				return apperr.Conflict("active_assignments", "Cancel or complete the assignments using this offer first.").
					WithDetail("active_assignments", active)
			}
		}
		next := to
		if to == StatusAvailable {
			next = DerivedStatus(StatusAvailable, o.QuantityAvailable, o.AssignedQuantity)
		}
		if _, err := tx.Exec(ctx, `UPDATE offers SET status = $2, version = version + 1, updated_at = now() WHERE id = $1`,
			o.ID, string(next)); err != nil {
			return err
		}
		ev := audit.ForActor(p, "offer.status_changed", audit.Shared).On("offer", o.ID).
			Status(string(o.Status), string(next)).WithReason(reason)
		if err := audit.Record(ctx, tx, ev); err != nil {
			return err
		}
		out, err = Load(ctx, tx, o.ID, false)
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	return s.view(ctx, p, out)
}

// Allocate reserves qty of a locked offer for an assignment. The caller must
// hold the row lock (Load with forUpdate) inside tx.
func Allocate(ctx context.Context, tx pgx.Tx, p roles.Principal, o *Record, qty int, assignmentID uuid.UUID) (audit.Event, error) {
	if !o.Status.allocatable() {
		return audit.Event{}, apperr.Conflict("offer_not_available", "This offer is not available for allocation.").
			WithDetail("offer_status", string(o.Status))
	}
	if qty <= 0 {
		return audit.Event{}, apperr.ValidationField("quantity_assigned", "Allocate at least 1 unit from an offer.")
	}
	if qty > o.RemainingQuantity {
		return audit.Event{}, apperr.Conflict("over_allocation",
			"Not enough remaining quantity on this offer.").WithDetail("remaining_quantity", o.RemainingQuantity)
	}
	newAssigned := o.AssignedQuantity + qty
	next := DerivedStatus(o.Status, o.QuantityAvailable, newAssigned)
	_, err := tx.Exec(ctx, `UPDATE offers SET assigned_quantity = $2, status = $3, version = version + 1,
		updated_at = now() WHERE id = $1`, o.ID, newAssigned, string(next))
	if err != nil {
		if database.IsCheckViolation(err, "offers_allocation_bounds") {
			return audit.Event{}, apperr.Conflict("over_allocation", "Not enough remaining quantity on this offer.")
		}
		return audit.Event{}, err
	}
	ev := audit.ForActor(p, "offer.allocated", audit.Internal).On("offer", o.ID).
		Status(string(o.Status), string(next)).With("quantity", qty).With("assignment_id", assignmentID.String()).
		With("remaining_quantity", o.QuantityAvailable-newAssigned)
	o.AssignedQuantity, o.RemainingQuantity, o.Status = newAssigned, o.QuantityAvailable-newAssigned, next
	return ev, nil
}

// Release returns qty of an allocation to the offer.
func Release(ctx context.Context, tx pgx.Tx, p roles.Principal, offerID uuid.UUID, qty int, assignmentID uuid.UUID) (audit.Event, error) {
	o, err := Load(ctx, tx, offerID, true)
	if err != nil {
		return audit.Event{}, err
	}
	if o == nil {
		return audit.Event{}, apperr.NotFound("Offer not found.")
	}
	newAssigned := o.AssignedQuantity - qty
	if newAssigned < 0 {
		newAssigned = 0
	}
	next := DerivedStatus(o.Status, o.QuantityAvailable, newAssigned)
	if _, err := tx.Exec(ctx, `UPDATE offers SET assigned_quantity = $2, status = $3, version = version + 1,
		updated_at = now() WHERE id = $1`, o.ID, newAssigned, string(next)); err != nil {
		return audit.Event{}, err
	}
	return audit.ForActor(p, "offer.allocation_released", audit.Internal).On("offer", o.ID).
		Status(string(o.Status), string(next)).With("quantity", qty).With("assignment_id", assignmentID.String()).
		With("remaining_quantity", o.QuantityAvailable-newAssigned), nil
}

// Protected is the result of an audited reveal.
type Protected struct {
	Contact *struct {
		Method  string `json:"method"`
		Details string `json:"details"`
	} `json:"contact,omitempty"`
	ExactLocation *locations.ExactLocation `json:"exact_location,omitempty"`
	Fields        []string                 `json:"fields"`
}

// RevealProtected decrypts protected offer fields for authorized viewers and
// records an audit event.
func (s *Service) RevealProtected(ctx context.Context, p roles.Principal, id uuid.UUID) (*Protected, error) {
	var out *Protected
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		o, rel, acc, err := s.loadVisible(ctx, tx, p, id, false)
		if err != nil {
			return err
		}
		st, err := settings.Get(ctx, tx)
		if err != nil {
			return err
		}
		if !project(o, rel, acc, st).Permissions.CanRevealProtected {
			return apperr.Forbidden("You are not authorized to view protected details of this offer.")
		}
		res := &Protected{Fields: []string{}}
		if rel != RelationAssignedVolunteer && len(o.ContactSealed) > 0 {
			pt, err := s.Sealer.Open(o.ContactSealed, AAD(o.ID, "contact"))
			if err != nil {
				return err
			}
			res.Contact = &struct {
				Method  string `json:"method"`
				Details string `json:"details"`
			}{Method: o.ContactMethod, Details: string(pt)}
			res.Fields = append(res.Fields, "contact")
		}
		if len(o.ExactSealed) > 0 {
			pt, err := s.Sealer.Open(o.ExactSealed, AAD(o.ID, "exact_location"))
			if err != nil {
				return err
			}
			var ex locations.ExactLocation
			if err := json.Unmarshal(pt, &ex); err != nil {
				return err
			}
			res.ExactLocation = &ex
			res.Fields = append(res.Fields, "exact_location")
		}
		out = res
		return audit.Record(ctx, tx, audit.ForActor(p, "offer.protected_viewed", audit.Shared).On("offer", o.ID).
			With("fields", res.Fields).With("viewer_relation", string(rel)))
	})
	if err != nil {
		return nil, wrap(err)
	}
	return out, nil
}

// Timeline returns audit events for an offer visible to the viewer.
func (s *Service) Timeline(ctx context.Context, p roles.Principal, id uuid.UUID) ([]*audit.Stored, error) {
	o, rel, _, err := s.loadVisible(ctx, s.DB, p, id, false)
	if err != nil {
		return nil, err
	}
	vis := []audit.Visibility{audit.Shared}
	if rel == RelationCoordinator {
		vis = append(vis, audit.Operational, audit.Internal)
	} else if rel == RelationAssignedVolunteer {
		return []*audit.Stored{}, nil
	}
	events, err := audit.Timeline(ctx, s.DB, audit.TimelineFilter{EntityType: "offer", EntityID: &o.ID, Visibilities: vis})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if rel != RelationCoordinator {
		for _, e := range events {
			e.ActorUserID = nil
			e.Metadata = map[string]any{}
		}
	}
	if events == nil {
		events = []*audit.Stored{}
	}
	return events, nil
}

// RedactInTx wipes personal data from an offer.
func RedactInTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, softDelete bool) error {
	_, err := tx.Exec(ctx, `UPDATE offers SET title = '[redacted]', description = '', accessibility_notes = '',
		restrictions = '', area_label = '', approx_lat = NULL, approx_lon = NULL, approx_decimals = NULL,
		exact_location_sealed = NULL, contact_details_sealed = NULL, tags = '{}',
		redacted_at = COALESCE(redacted_at, now()), deleted_at = CASE WHEN $2 THEN now() ELSE deleted_at END,
		version = version + 1, updated_at = now() WHERE id = $1`, id, softDelete)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE notes SET body = '', redacted_at = now() WHERE offer_id = $1 AND redacted_at IS NULL`, id)
	return err
}

// AdminDelete soft-deletes and redacts an offer by reference.
func (s *Service) AdminDelete(ctx context.Context, p roles.Principal, reference, reason string) error {
	if !p.Can(roles.CapRecordDelete) {
		return apperr.Forbidden("Only administrators can delete records.")
	}
	reason = validation.CleanText(reason)
	errs := validation.Errors{}
	errs.Text("reason", reason, 5, 1000, true)
	if err := errs.Err(); err != nil {
		return err
	}
	return wrap(database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		o, err := loadBy(ctx, tx, "o.reference = $1", validation.CleanText(reference))
		if err != nil {
			return err
		}
		if o == nil || o.DeletedAt != nil || o.OrgID != p.OrgID {
			return apperr.NotFound("No offer with this reference exists.")
		}
		var active int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM assignments WHERE offer_id = $1
			AND status IN ('proposed', 'accepted', 'in_progress', 'partially_delivered')`, o.ID).Scan(&active); err != nil {
			return err
		}
		if active > 0 {
			return apperr.Conflict("active_assignments", "Complete or cancel assignments using this offer before deleting it.")
		}
		if err := RedactInTx(ctx, tx, o.ID, true); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "offer.deleted", audit.System).On("offer", o.ID).
			WithReason(reason).With("reference", o.Reference))
	}))
}
