// Package requests implements structured aid/resource requests: creation,
// manual review and prioritization, lifecycle transitions, protected data
// access and per-role projections.
package requests

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
)

// Urgency levels. Urgency is always selected and reviewed by humans; the
// system never derives it automatically.
var Urgencies = []string{"critical", "high", "normal", "low"}

// ReviewStatuses lists review states.
var ReviewStatuses = []string{"pending", "in_review", "reviewed", "needs_information"}

// VerificationLevels lists verification levels.
var VerificationLevels = []string{"unverified", "self_reported", "coordinator_confirmed", "field_confirmed"}

// CategoryMedicinePickup is handled as logistics-only with extra privacy
// rules.
const CategoryMedicinePickup = "medicine_pickup"

// MedicinePickupTitle replaces any user-supplied title for medicine pickup
// requests so no medication names or diagnoses end up in titles.
const MedicinePickupTitle = "Medicine pickup (logistics only)"

// Record is a request row.
type Record struct {
	ID                          uuid.UUID
	Reference                   string
	OrgID                       uuid.UUID
	CreatedBy                   *uuid.UUID
	ClientID                    *uuid.UUID
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	Status                      Status
	Category                    string
	Urgency                     string
	Title                       string
	Description                 string
	PeopleAffected              *int
	Quantity                    *int
	Unit                        string
	RequestedByTime             *time.Time
	LocationMode                string
	AreaLabel                   string
	ApproxLat                   *float64
	ApproxLon                   *float64
	ApproxDecimals              *int16
	ExactSealed                 []byte
	AccessibilityNotes          string
	ContactVisibility           string
	ContactMethod               string
	ContactSealed               []byte
	Sensitive                   bool
	RequiresFormalAuthorization bool
	ReviewStatus                string
	VerificationLevel           string
	AssignedTeam                string
	AssignedUserID              *uuid.UUID
	ResolutionSummary           string
	ClosedAt                    *time.Time
	ExpiryAt                    *time.Time
	Tags                        []string
	DuplicateOf                 *uuid.UUID
	Version                     int
	RedactedAt                  *time.Time
	DeletedAt                   *time.Time

	// Joined, read-only.
	DuplicateOfReference *string
	AssignedUserName     *string
	CreatedByName        *string
}

const selectSQL = `SELECT r.id, r.reference, r.organization_id, r.created_by_user_id, r.client_id, r.created_at,
	r.updated_at, r.status, r.category, r.urgency, r.title, r.description, r.estimated_people_affected,
	r.requested_quantity, r.requested_unit, r.requested_by_time, r.location_mode, r.area_label, r.approx_lat,
	r.approx_lon, r.approx_decimals, r.exact_location_sealed, r.accessibility_notes, r.contact_visibility,
	r.contact_method, r.contact_details_sealed, r.sensitive_data_flag, r.requires_formal_authorization,
	r.review_status, r.verification_level, r.assigned_team, r.assigned_user_id, r.resolution_summary,
	r.closed_at, r.expiry_at, r.tags, r.duplicate_of_request_id, r.version, r.redacted_at, r.deleted_at,
	d.reference, au.display_name, cu.display_name
	FROM aid_requests r
	LEFT JOIN aid_requests d ON d.id = r.duplicate_of_request_id
	LEFT JOIN users au ON au.id = r.assigned_user_id
	LEFT JOIN users cu ON cu.id = r.created_by_user_id`

func scan(row pgx.Row) (*Record, error) {
	var r Record
	var status string
	err := row.Scan(&r.ID, &r.Reference, &r.OrgID, &r.CreatedBy, &r.ClientID, &r.CreatedAt, &r.UpdatedAt,
		&status, &r.Category, &r.Urgency, &r.Title, &r.Description, &r.PeopleAffected, &r.Quantity, &r.Unit,
		&r.RequestedByTime, &r.LocationMode, &r.AreaLabel, &r.ApproxLat, &r.ApproxLon, &r.ApproxDecimals,
		&r.ExactSealed, &r.AccessibilityNotes, &r.ContactVisibility, &r.ContactMethod, &r.ContactSealed,
		&r.Sensitive, &r.RequiresFormalAuthorization, &r.ReviewStatus, &r.VerificationLevel, &r.AssignedTeam,
		&r.AssignedUserID, &r.ResolutionSummary, &r.ClosedAt, &r.ExpiryAt, &r.Tags, &r.DuplicateOf, &r.Version,
		&r.RedactedAt, &r.DeletedAt, &r.DuplicateOfReference, &r.AssignedUserName, &r.CreatedByName)
	if err != nil {
		return nil, err
	}
	r.Status = Status(status)
	if r.Tags == nil {
		r.Tags = []string{}
	}
	return &r, nil
}

// Load fetches a request by id (including soft-deleted rows). Returns nil
// when not found.
func Load(ctx context.Context, q database.Querier, id uuid.UUID, forUpdate bool) (*Record, error) {
	sql := selectSQL + ` WHERE r.id = $1`
	if forUpdate {
		sql += ` FOR UPDATE OF r`
	}
	r, err := scan(q.QueryRow(ctx, sql, id))
	if database.IsNoRows(err) {
		return nil, nil
	}
	return r, err
}

// LoadByReference fetches a request by reference.
func LoadByReference(ctx context.Context, q database.Querier, ref string) (*Record, error) {
	r, err := scan(q.QueryRow(ctx, selectSQL+` WHERE r.reference = $1`, ref))
	if database.IsNoRows(err) {
		return nil, nil
	}
	return r, err
}

// LoadByClientID fetches a request by its offline client id.
func LoadByClientID(ctx context.Context, q database.Querier, clientID uuid.UUID) (*Record, error) {
	r, err := scan(q.QueryRow(ctx, selectSQL+` WHERE r.client_id = $1`, clientID))
	if database.IsNoRows(err) {
		return nil, nil
	}
	return r, err
}

// AAD returns the additional authenticated data for a protected field of
// request id.
func AAD(id uuid.UUID, field string) string {
	return locations.AAD("aid_request", id.String(), field)
}

// assignmentAccess summarizes a volunteer's assignments on a request.
type assignmentAccess struct {
	visible          bool // any assignment that is not declined/cancelled
	active           bool // accepted, in progress or partially delivered
	contactGrant     bool
	destinationGrant bool
}

func loadAssignmentAccess(ctx context.Context, q database.Querier, requestIDs []uuid.UUID, userID uuid.UUID) (map[uuid.UUID]*assignmentAccess, error) {
	out := map[uuid.UUID]*assignmentAccess{}
	if len(requestIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT request_id, status, protected_contact_access_granted,
		destination_location_access_granted FROM assignments
		WHERE request_id = ANY($1) AND volunteer_user_id = $2`, requestIDs, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rid uuid.UUID
		var status string
		var contact, dest bool
		if err := rows.Scan(&rid, &status, &contact, &dest); err != nil {
			return nil, err
		}
		a := out[rid]
		if a == nil {
			a = &assignmentAccess{}
			out[rid] = a
		}
		if status == "declined" || status == "cancelled" {
			continue
		}
		a.visible = true
		if status == "accepted" || status == "in_progress" || status == "partially_delivered" {
			a.active = true
			a.contactGrant = a.contactGrant || contact
			a.destinationGrant = a.destinationGrant || dest
		}
	}
	return out, rows.Err()
}

// CountActiveAssignments counts assignments that still hold work or
// allocations for a request.
func CountActiveAssignments(ctx context.Context, q database.Querier, requestID uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM assignments WHERE request_id = $1
		AND status IN ('proposed', 'accepted', 'in_progress', 'partially_delivered')`, requestID).Scan(&n)
	return n, err
}
