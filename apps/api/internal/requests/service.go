package requests

import (
	"context"
	"fmt"
	"slices"
	"strings"
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

// AssignmentCloser lets the request service cancel active assignments when a
// coordinator closes a request, without an import cycle.
type AssignmentCloser interface {
	CancelActiveForRequest(ctx context.Context, tx pgx.Tx, p roles.Principal, requestID uuid.UUID, reason string) ([]audit.Event, error)
}

// Service implements request use cases.
type Service struct {
	DB          *pgxpool.Pool
	Sealer      *locations.Sealer
	Assignments AssignmentCloser
	Now         func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// CreateInput is the payload for creating a request.
type CreateInput struct {
	ClientID                    *uuid.UUID             `json:"client_id"`
	Category                    string                 `json:"category"`
	Urgency                     string                 `json:"urgency"`
	Title                       string                 `json:"title"`
	Description                 string                 `json:"description"`
	EstimatedPeopleAffected     *int                   `json:"estimated_people_affected"`
	RequestedQuantity           *int                   `json:"requested_quantity"`
	RequestedUnit               string                 `json:"requested_unit"`
	RequestedByTime             *time.Time             `json:"requested_by_time"`
	Location                    locations.Input        `json:"location"`
	AccessibilityNotes          string                 `json:"accessibility_notes"`
	Contact                     locations.ContactInput `json:"contact"`
	SensitiveDataFlag           bool                   `json:"sensitive_data_flag"`
	RequiresFormalAuthorization bool                   `json:"requires_formal_authorization"`
	Tags                        []string               `json:"tags"`
	Submit                      *bool                  `json:"submit"`
	EmergencyNoticeAcknowledged bool                   `json:"emergency_notice_acknowledged"`
	ClientCreatedAt             *time.Time             `json:"client_created_at"`
}

type contentFields struct {
	category, urgency, title, description, unit, accessibility string
	people, quantity                                           *int
	byTime                                                     *time.Time
}

func (s *Service) validateContent(ctx context.Context, q database.Querier, c *contentFields, st *settings.Settings,
	errs validation.Errors, sensitive *bool, requireAck bool, ack bool) {
	cat, err := settings.GetCategory(ctx, q, c.category)
	if err != nil || cat == nil || !cat.Enabled {
		errs.Add("category", "Choose an available category.")
	}
	errs.OneOf("urgency", c.urgency, Urgencies)
	if c.urgency == "critical" && requireAck && !ack {
		errs.Add("emergency_notice_acknowledged",
			"Critical urgency requires confirming that you have read the emergency notice.")
	}
	c.title = validation.CleanText(c.title)
	c.description = validation.CleanText(c.description)
	c.accessibility = validation.CleanText(c.accessibility)
	c.unit = validation.CleanText(c.unit)
	if c.category == CategoryMedicinePickup {
		// Logistics only: never accept titles or free text that could carry
		// medication names, diagnoses or prescription details.
		c.title = MedicinePickupTitle
		if c.description != "" && !st.AllowMedicineFreeText {
			errs.Add("description", "Do not enter medical details for medicine pickup requests. Leave this field empty; a coordinator will contact you.")
		}
		*sensitive = true
	} else {
		errs.Text("title", c.title, 3, 120, false)
	}
	errs.Text("description", c.description, 0, 4000, true)
	errs.Text("accessibility_notes", c.accessibility, 0, 1000, true)
	errs.Text("requested_unit", c.unit, 0, 40, false)
	errs.IntRange("estimated_people_affected", c.people, 0, 100000)
	errs.IntRange("requested_quantity", c.quantity, 0, 10000000)
	now := s.now()
	errs.TimeRange("requested_by_time", c.byTime, now.AddDate(0, 0, -30), now.AddDate(1, 0, 0))
}

// Create creates a request. When ClientID is provided the call is
// idempotent: repeating it returns the existing request (created=false).
func (s *Service) Create(ctx context.Context, p roles.Principal, in CreateInput) (*View, bool, error) {
	if !p.Can(roles.CapRequestCreate) {
		return nil, false, apperr.Forbidden("You are not allowed to create requests.")
	}
	if in.ClientID != nil {
		existing, err := LoadByClientID(ctx, s.DB, *in.ClientID)
		if err != nil {
			return nil, false, apperr.Internal(err)
		}
		if existing != nil {
			if existing.CreatedBy == nil || *existing.CreatedBy != p.UserID {
				return nil, false, apperr.Conflict("client_id_conflict", "This client id is already in use.")
			}
			v, err := s.view(ctx, s.DB, p, existing, false)
			return v, false, err
		}
	}
	st, err := settings.Get(ctx, s.DB)
	if err != nil {
		return nil, false, apperr.Internal(err)
	}
	errs := validation.Errors{}
	c := &contentFields{category: in.Category, urgency: in.Urgency, title: in.Title, description: in.Description,
		unit: in.RequestedUnit, accessibility: in.AccessibilityNotes, people: in.EstimatedPeopleAffected,
		quantity: in.RequestedQuantity, byTime: in.RequestedByTime}
	sensitive := in.SensitiveDataFlag
	s.validateContent(ctx, s.DB, c, st, errs, &sensitive, true, in.EmergencyNoticeAcknowledged)
	loc := locations.Normalize(in.Location, st.ApproxLocationDecimals, errs)
	if loc.KeepExact {
		errs.Add("location.exact", "An exact address or coordinates are required.")
	}
	contact := locations.NormalizeContact(in.Contact, errs)
	var tags []string
	if len(in.Tags) > 0 {
		if !p.IsCoordinator() {
			errs.Add("tags", "Only coordinators can set tags.")
		} else {
			tags = errs.Tags("tags", in.Tags)
		}
	}
	if tags == nil {
		tags = []string{}
	}
	if err := errs.Err(); err != nil {
		return nil, false, err
	}

	now := s.now()
	status := StatusSubmitted
	if in.Submit != nil && !*in.Submit {
		status = StatusDraft
	}
	verification := "self_reported"
	if p.IsCoordinator() {
		verification = "unverified"
	}
	expiry := now.Add(time.Duration(st.DefaultRequestExpiryHours) * time.Hour)
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
		ref, err := database.NextReference(ctx, tx, "RM", now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO aid_requests (id, reference, organization_id, created_by_user_id, client_id,
			created_at, updated_at, status, category, urgency, title, description, estimated_people_affected,
			requested_quantity, requested_unit, requested_by_time, location_mode, area_label, approx_lat, approx_lon,
			approx_decimals, exact_location_sealed, accessibility_notes, contact_visibility, contact_method,
			contact_details_sealed, sensitive_data_flag, requires_formal_authorization, verification_level, expiry_at, tags)
			VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30)`,
			id, ref, p.OrgID, p.UserID, in.ClientID, now, string(status), c.category, c.urgency, c.title, c.description,
			c.people, c.quantity, c.unit, c.byTime, string(loc.Mode), loc.AreaLabel, loc.ApproxLat, loc.ApproxLon,
			loc.Decimals, exactSealed, c.accessibility, contact.Visibility, contact.Method, contactSealed, sensitive,
			in.RequiresFormalAuthorization, verification, expiry, tags)
		if err != nil {
			if database.IsUniqueViolation(err, "aid_requests_client_id_key") {
				return apperr.Conflict("client_id_conflict", "This request was already submitted.")
			}
			return err
		}
		ev := audit.ForActor(p, "request.created", audit.Shared).On("aid_request", id).ForRequest(id).
			Status("", string(status)).With("category", c.category).With("urgency", c.urgency)
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
	v, err := s.view(ctx, s.DB, p, rec, false)
	return v, true, err
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

func (s *Service) view(ctx context.Context, q database.Querier, p roles.Principal, r *Record, listMode bool) (*View, error) {
	st, err := settings.Get(ctx, q)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	accMap, err := loadAssignmentAccess(ctx, q, []uuid.UUID{r.ID}, p.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	acc := accMap[r.ID]
	rel := relationFor(p, r, acc)
	if rel == RelationNone {
		return nil, apperr.NotFound("Request not found.")
	}
	return project(r, rel, acc, st, listMode), nil
}

// loadVisible loads a request and the viewer's relation, returning 404 for
// requests the viewer may not see (so existence is not revealed).
func (s *Service) loadVisible(ctx context.Context, q database.Querier, p roles.Principal, id uuid.UUID, forUpdate bool) (*Record, Relation, *assignmentAccess, error) {
	r, err := Load(ctx, q, id, forUpdate)
	if err != nil {
		return nil, RelationNone, nil, apperr.Internal(err)
	}
	if r == nil {
		return nil, RelationNone, nil, apperr.NotFound("Request not found.")
	}
	accMap, err := loadAssignmentAccess(ctx, q, []uuid.UUID{r.ID}, p.UserID)
	if err != nil {
		return nil, RelationNone, nil, apperr.Internal(err)
	}
	acc := accMap[r.ID]
	rel := relationFor(p, r, acc)
	if rel == RelationNone {
		return nil, RelationNone, nil, apperr.NotFound("Request not found.")
	}
	return r, rel, acc, nil
}

// Get returns the role-specific view of a request.
func (s *Service) Get(ctx context.Context, p roles.Principal, id uuid.UUID) (*View, error) {
	r, err := Load(ctx, s.DB, id, false)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if r == nil {
		return nil, apperr.NotFound("Request not found.")
	}
	return s.view(ctx, s.DB, p, r, false)
}

// ListFilter filters request lists.
type ListFilter struct {
	Statuses     []string
	Category     string
	Urgency      string
	Query        string
	UpdatedSince *time.Time
	OpenOnly     bool
	Mine         bool
	Sort         string
	Limit        int
	Offset       int
}

// ListResult is a page of requests.
type ListResult struct {
	Items   []*View `json:"items"`
	HasMore bool    `json:"has_more"`
}

// List returns requests visible to the principal.
func (s *Service) List(ctx context.Context, p roles.Principal, f ListFilter) (*ListResult, error) {
	coordinator := p.Can(roles.CapRequestReadAll)
	if !coordinator && !p.Can(roles.CapRequestCreate) && !p.Can(roles.CapAssignmentWorkOwn) {
		return nil, apperr.Forbidden("Your role does not include operational access to requests.")
	}
	errs := validation.Errors{}
	for _, st := range f.Statuses {
		errs.OneOf("status", st, StatusStrings())
	}
	if f.Urgency != "" {
		errs.OneOf("urgency", f.Urgency, Urgencies)
	}
	if f.Sort == "" {
		f.Sort = "priority"
	}
	errs.OneOf("sort", f.Sort, []string{"priority", "newest", "updated"})
	errs.Text("q", f.Query, 0, 100, false)
	if err := errs.Err(); err != nil {
		return nil, err
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}

	var where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	where = append(where, "r.deleted_at IS NULL")
	if coordinator {
		where = append(where, "r.organization_id = "+arg(p.OrgID))
		if f.Mine {
			where = append(where, "r.created_by_user_id = "+arg(p.UserID))
		}
	} else {
		uid := arg(p.UserID)
		scope := []string{}
		if p.Can(roles.CapRequestCreate) {
			scope = append(scope, "r.created_by_user_id = "+uid)
		}
		if p.Can(roles.CapAssignmentWorkOwn) && !f.Mine {
			scope = append(scope, `EXISTS (SELECT 1 FROM assignments a WHERE a.request_id = r.id
				AND a.volunteer_user_id = `+uid+` AND a.status NOT IN ('declined', 'cancelled'))`)
		}
		if len(scope) == 0 {
			return &ListResult{Items: []*View{}}, nil
		}
		where = append(where, "("+strings.Join(scope, " OR ")+")")
	}
	if len(f.Statuses) > 0 {
		where = append(where, "r.status = ANY("+arg(f.Statuses)+")")
	}
	if f.OpenOnly {
		where = append(where, "r.status NOT IN ('draft', 'resolved', 'cancelled', 'expired', 'duplicate')")
	}
	if f.Category != "" {
		where = append(where, "r.category = "+arg(f.Category))
	}
	if f.Urgency != "" {
		where = append(where, "r.urgency = "+arg(f.Urgency))
	}
	if f.UpdatedSince != nil {
		where = append(where, "r.updated_at >= "+arg(*f.UpdatedSince))
	}
	if f.Query != "" && coordinator {
		pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(f.Query) + "%"
		ph := arg(pattern)
		where = append(where, "(r.reference ILIKE "+ph+" OR r.title ILIKE "+ph+" OR r.area_label ILIKE "+ph+")")
	}
	order := map[string]string{
		"priority": `CASE r.urgency WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
			r.requested_by_time NULLS LAST, r.created_at, r.id`,
		"newest":  "r.created_at DESC, r.id",
		"updated": "r.updated_at DESC, r.id",
	}[f.Sort]
	sql := selectSQL + " WHERE " + strings.Join(where, " AND ") + " ORDER BY " + order +
		" LIMIT " + arg(f.Limit+1) + " OFFSET " + arg(f.Offset)

	rows, err := s.DB.Query(ctx, sql, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	var recs []*Record
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			rows.Close()
			return nil, apperr.Internal(err)
		}
		recs = append(recs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	res := &ListResult{Items: []*View{}}
	if len(recs) > f.Limit {
		res.HasMore = true
		recs = recs[:f.Limit]
	}
	st, err := settings.Get(ctx, s.DB)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	ids := make([]uuid.UUID, len(recs))
	for i, r := range recs {
		ids[i] = r.ID
	}
	accMap, err := loadAssignmentAccess(ctx, s.DB, ids, p.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for _, r := range recs {
		acc := accMap[r.ID]
		rel := relationFor(p, r, acc)
		if rel == RelationNone {
			continue
		}
		res.Items = append(res.Items, project(r, rel, acc, st, true))
	}
	return res, nil
}

// UpdateInput is a partial update. Content fields may be edited by the owner
// until review starts and by coordinators while the request is open;
// coordinator fields only by coordinators.
type UpdateInput struct {
	Version                     *int                      `json:"version"`
	Category                    *string                   `json:"category"`
	Urgency                     *string                   `json:"urgency"`
	Title                       *string                   `json:"title"`
	Description                 *string                   `json:"description"`
	EstimatedPeopleAffected     httpx.Optional[int]       `json:"estimated_people_affected"`
	RequestedQuantity           httpx.Optional[int]       `json:"requested_quantity"`
	RequestedUnit               *string                   `json:"requested_unit"`
	RequestedByTime             httpx.Optional[time.Time] `json:"requested_by_time"`
	Location                    *locations.Input          `json:"location"`
	AccessibilityNotes          *string                   `json:"accessibility_notes"`
	Contact                     *locations.ContactInput   `json:"contact"`
	SensitiveDataFlag           *bool                     `json:"sensitive_data_flag"`
	RequiresFormalAuthorization *bool                     `json:"requires_formal_authorization"`
	EmergencyNoticeAcknowledged bool                      `json:"emergency_notice_acknowledged"`

	ReviewStatus      *string                   `json:"review_status"`
	VerificationLevel *string                   `json:"verification_level"`
	AssignedTeam      *string                   `json:"assigned_team"`
	AssignedUserID    httpx.Optional[uuid.UUID] `json:"assigned_user_id"`
	ExpiryAt          httpx.Optional[time.Time] `json:"expiry_at"`
	Tags              *[]string                 `json:"tags"`
}

func (u *UpdateInput) hasCoordinatorFields() bool {
	return u.ReviewStatus != nil || u.VerificationLevel != nil || u.AssignedTeam != nil ||
		u.AssignedUserID.Set || u.ExpiryAt.Set || u.Tags != nil
}

// Update applies a partial update with optimistic concurrency control.
func (s *Service) Update(ctx context.Context, p roles.Principal, id uuid.UUID, u UpdateInput) (*View, error) {
	var out *Record
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		r, rel, _, err := s.loadVisible(ctx, tx, p, id, true)
		if err != nil {
			return err
		}
		if r.RedactedAt != nil {
			return apperr.Conflict("redacted", "This request was redacted and can no longer be edited.")
		}
		switch rel {
		case RelationCoordinator:
			if r.Status.IsClosed() {
				return apperr.Conflict("request_closed", "Closed requests must be reopened before editing.")
			}
		case RelationOwner:
			if !ownerCanEdit(r) {
				return apperr.ForbiddenCode("edit_locked",
					"This request is being handled. Add an update note instead of editing it.")
			}
			if u.hasCoordinatorFields() {
				return apperr.Forbidden("Only coordinators can change review, verification, assignment, expiry or tags.")
			}
		default:
			return apperr.Forbidden("You cannot edit this request.")
		}
		if u.Version != nil && *u.Version != r.Version {
			return apperr.Conflict("version_conflict", "This request was changed by someone else. Reload and try again.").
				WithDetail("current_version", r.Version)
		}

		st, err := settings.Get(ctx, tx)
		if err != nil {
			return err
		}
		errs := validation.Errors{}
		var changed []string
		mark := func(name string) {
			if !slices.Contains(changed, name) {
				changed = append(changed, name)
			}
		}
		oldUrgency, oldReview, oldVerification := r.Urgency, r.ReviewStatus, r.VerificationLevel

		c := &contentFields{category: r.Category, urgency: r.Urgency, title: r.Title, description: r.Description,
			unit: r.Unit, accessibility: r.AccessibilityNotes, people: r.PeopleAffected, quantity: r.Quantity,
			byTime: r.RequestedByTime}
		setStr := func(name string, dst *string, v *string) {
			if v != nil && *v != *dst {
				*dst = *v
				mark(name)
			}
		}
		setStr("category", &c.category, u.Category)
		setStr("urgency", &c.urgency, u.Urgency)
		setStr("title", &c.title, u.Title)
		setStr("description", &c.description, u.Description)
		setStr("requested_unit", &c.unit, u.RequestedUnit)
		setStr("accessibility_notes", &c.accessibility, u.AccessibilityNotes)
		if u.EstimatedPeopleAffected.Set {
			c.people = u.EstimatedPeopleAffected.Ptr()
			mark("estimated_people_affected")
		}
		if u.RequestedQuantity.Set {
			c.quantity = u.RequestedQuantity.Ptr()
			mark("requested_quantity")
		}
		if u.RequestedByTime.Set {
			c.byTime = u.RequestedByTime.Ptr()
			mark("requested_by_time")
		}
		sensitive := r.Sensitive
		if u.SensitiveDataFlag != nil && *u.SensitiveDataFlag != sensitive {
			sensitive = *u.SensitiveDataFlag
			mark("sensitive_data_flag")
		}
		requireAck := rel == RelationOwner && c.urgency == "critical" && oldUrgency != "critical"
		s.validateContent(ctx, tx, c, st, errs, &sensitive, requireAck, u.EmergencyNoticeAcknowledged)
		if r.Category == CategoryMedicinePickup || c.category == CategoryMedicinePickup {
			// Medicine pickup requests always stay flagged as sensitive.
			if c.category == CategoryMedicinePickup {
				sensitive = true
			}
		}
		if u.RequiresFormalAuthorization != nil && *u.RequiresFormalAuthorization != r.RequiresFormalAuthorization {
			r.RequiresFormalAuthorization = *u.RequiresFormalAuthorization
			mark("requires_formal_authorization")
		}

		if u.Location != nil {
			loc := locations.Normalize(*u.Location, st.ApproxLocationDecimals, errs)
			if loc.KeepExact {
				if r.LocationMode != string(locations.ModeProtectedExact) || len(r.ExactSealed) == 0 {
					errs.Add("location.exact", "There is no stored exact location to keep.")
				}
				r.AreaLabel = loc.AreaLabel
			} else {
				sealed, err := s.Sealer.Seal(loc.ExactBytes, AAD(r.ID, "exact_location"))
				if err != nil {
					return err
				}
				r.LocationMode, r.AreaLabel = string(loc.Mode), loc.AreaLabel
				r.ApproxLat, r.ApproxLon, r.ApproxDecimals, r.ExactSealed = loc.ApproxLat, loc.ApproxLon, loc.Decimals, sealed
			}
			mark("location")
		}
		if u.Contact != nil {
			contact := locations.NormalizeContact(*u.Contact, errs)
			r.ContactMethod, r.ContactVisibility = contact.Method, contact.Visibility
			if !contact.Keep {
				sealed, err := s.Sealer.Seal([]byte(contact.Details), AAD(r.ID, "contact"))
				if err != nil {
					return err
				}
				r.ContactSealed = sealed
			}
			if contact.Visibility == locations.ContactVisibilityNone {
				r.ContactSealed = nil
			}
			mark("contact")
		}

		if rel == RelationCoordinator {
			if u.ReviewStatus != nil {
				errs.OneOf("review_status", *u.ReviewStatus, ReviewStatuses)
				if *u.ReviewStatus != r.ReviewStatus {
					r.ReviewStatus = *u.ReviewStatus
					mark("review_status")
				}
			}
			if u.VerificationLevel != nil {
				errs.OneOf("verification_level", *u.VerificationLevel, VerificationLevels)
				if *u.VerificationLevel != r.VerificationLevel {
					r.VerificationLevel = *u.VerificationLevel
					mark("verification_level")
				}
			}
			if u.AssignedTeam != nil {
				team := validation.CleanText(*u.AssignedTeam)
				errs.Text("assigned_team", team, 0, 120, false)
				if team != r.AssignedTeam {
					r.AssignedTeam = team
					mark("assigned_team")
				}
			}
			if u.AssignedUserID.Set {
				if u.AssignedUserID.Valid {
					var ok bool
					err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND organization_id = $2
						AND is_active)`, u.AssignedUserID.V, r.OrgID).Scan(&ok)
					if err != nil {
						return err
					}
					if !ok {
						errs.Add("assigned_user_id", "Unknown or inactive user.")
					}
				}
				r.AssignedUserID = u.AssignedUserID.Ptr()
				mark("assigned_user_id")
			}
			if u.ExpiryAt.Set {
				r.ExpiryAt = u.ExpiryAt.Ptr()
				now := s.now()
				errs.TimeRange("expiry_at", r.ExpiryAt, now.AddDate(0, 0, -1), now.AddDate(1, 0, 0))
				mark("expiry_at")
			}
			if u.Tags != nil {
				tags := errs.Tags("tags", *u.Tags)
				if tags == nil {
					tags = []string{}
				}
				if !slices.Equal(tags, r.Tags) {
					r.Tags = tags
					mark("tags")
				}
			}
		}
		if err := errs.Err(); err != nil {
			return err
		}
		if len(changed) == 0 {
			out = r
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE aid_requests SET category=$2, urgency=$3, title=$4, description=$5,
			estimated_people_affected=$6, requested_quantity=$7, requested_unit=$8, requested_by_time=$9,
			location_mode=$10, area_label=$11, approx_lat=$12, approx_lon=$13, approx_decimals=$14,
			exact_location_sealed=$15, accessibility_notes=$16, contact_visibility=$17, contact_method=$18,
			contact_details_sealed=$19, sensitive_data_flag=$20, requires_formal_authorization=$21,
			review_status=$22, verification_level=$23, assigned_team=$24, assigned_user_id=$25, expiry_at=$26,
			tags=$27, version=version+1, updated_at=now() WHERE id=$1`,
			r.ID, c.category, c.urgency, c.title, c.description, c.people, c.quantity, c.unit, c.byTime,
			r.LocationMode, r.AreaLabel, r.ApproxLat, r.ApproxLon, r.ApproxDecimals, r.ExactSealed,
			c.accessibility, r.ContactVisibility, r.ContactMethod, r.ContactSealed, sensitive,
			r.RequiresFormalAuthorization, r.ReviewStatus, r.VerificationLevel, r.AssignedTeam, r.AssignedUserID,
			r.ExpiryAt, r.Tags)
		if err != nil {
			return err
		}
		events := []audit.Event{
			audit.ForActor(p, "request.updated", audit.Shared).On("aid_request", r.ID).ForRequest(r.ID).
				With("changed_fields", changed),
		}
		if c.urgency != oldUrgency {
			events = append(events, audit.ForActor(p, "request.urgency_changed", audit.Shared).
				On("aid_request", r.ID).ForRequest(r.ID).With("from", oldUrgency).With("to", c.urgency))
		}
		if r.ReviewStatus != oldReview {
			events = append(events, audit.ForActor(p, "request.review_status_changed", audit.Internal).
				On("aid_request", r.ID).ForRequest(r.ID).With("from", oldReview).With("to", r.ReviewStatus))
		}
		if r.VerificationLevel != oldVerification {
			events = append(events, audit.ForActor(p, "request.verification_changed", audit.Internal).
				On("aid_request", r.ID).ForRequest(r.ID).With("from", oldVerification).With("to", r.VerificationLevel))
		}
		if err := audit.Record(ctx, tx, events...); err != nil {
			return err
		}
		out, err = Load(ctx, tx, r.ID, false)
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	return s.view(ctx, s.DB, p, out, false)
}
