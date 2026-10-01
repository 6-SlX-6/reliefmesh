// Package roles defines ReliefMesh roles, capabilities and the authenticated
// principal. Authorization decisions are always made on the server using
// these definitions; the frontend only mirrors them for display.
package roles

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
)

// Role is a named set of capabilities. A user may hold several roles.
type Role string

const (
	Requester           Role = "requester"
	Volunteer           Role = "volunteer"
	Coordinator         Role = "coordinator"
	OrganizationManager Role = "organization_manager"
	Admin               Role = "admin"
)

// All lists every role in ascending privilege order for display purposes.
var All = []Role{Requester, Volunteer, Coordinator, OrganizationManager, Admin}

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return slices.Contains(All, r) }

// Parse converts and validates raw role names, removing duplicates.
func Parse(raw []string) ([]Role, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("at least one role is required")
	}
	var out []Role
	for _, s := range raw {
		r := Role(s)
		if !r.Valid() {
			return nil, fmt.Errorf("unknown role %q", s)
		}
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b Role) int { return slices.Index(All, a) - slices.Index(All, b) })
	return out, nil
}

// Strings converts roles to their string form.
func Strings(rs []Role) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return out
}

// Capability is a single permission checked by the backend.
type Capability string

const (
	CapRequestCreate         Capability = "request.create"
	CapRequestReadAll        Capability = "request.read_all"
	CapRequestManage         Capability = "request.manage"
	CapOfferCreate           Capability = "offer.create"
	CapOfferReadAll          Capability = "offer.read_all"
	CapOfferManage           Capability = "offer.manage"
	CapAssignmentManage      Capability = "assignment.manage"
	CapAssignmentWorkOwn     Capability = "assignment.work_own"
	CapDashboardView         Capability = "dashboard.view"
	CapDashboardQueues       Capability = "dashboard.queues"
	CapVolunteerList         Capability = "volunteer.list"
	CapVolunteerAvailability Capability = "volunteer.availability.manage"
	CapExportReports         Capability = "export.reports"
	CapUserManage            Capability = "user.manage"
	CapSettingsManage        Capability = "settings.manage"
	CapCategoriesManage      Capability = "categories.manage"
	CapAuditViewSystem       Capability = "audit.view_system"
	CapDemoSeed              Capability = "demo.seed"
	CapRetentionManage       Capability = "retention.manage"
	CapRecordDelete          Capability = "record.delete"
)

// coordinatorCaps are shared by coordinators and organization managers.
var coordinatorCaps = []Capability{
	CapRequestCreate, CapRequestReadAll, CapRequestManage,
	CapOfferCreate, CapOfferReadAll, CapOfferManage,
	CapAssignmentManage, CapDashboardView, CapDashboardQueues, CapVolunteerList,
}

var roleCaps = map[Role][]Capability{
	Requester:   {CapRequestCreate},
	Volunteer:   {CapRequestCreate, CapOfferCreate, CapAssignmentWorkOwn},
	Coordinator: coordinatorCaps,
	OrganizationManager: append(slices.Clone(coordinatorCaps),
		CapVolunteerAvailability, CapExportReports),
	// Administrators manage the instance. They intentionally do NOT receive
	// operational read access to requests, offers or personal data; an
	// administrator who also coordinates must hold the coordinator role too.
	// The dashboard is limited to aggregate counts for admins (no queues).
	Admin: {CapDashboardView, CapUserManage, CapSettingsManage, CapCategoriesManage,
		CapAuditViewSystem, CapDemoSeed, CapRetentionManage, CapRecordDelete},
}

// CapabilitiesFor returns the union of capabilities granted by roles.
func CapabilitiesFor(rs []Role) []Capability {
	var out []Capability
	for _, r := range rs {
		for _, c := range roleCaps[r] {
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	slices.Sort(out)
	return out
}

// Principal is the authenticated actor of a request.
type Principal struct {
	UserID             uuid.UUID
	OrgID              uuid.UUID
	Username           string
	DisplayName        string
	Roles              []Role
	SessionID          uuid.UUID
	MustChangePassword bool
	// System marks actions taken by CLI maintenance commands.
	System bool
}

// Has reports whether the principal holds role r.
func (p Principal) Has(r Role) bool { return slices.Contains(p.Roles, r) }

// Can reports whether any of the principal's roles grants capability c.
func (p Principal) Can(c Capability) bool {
	for _, r := range p.Roles {
		if slices.Contains(roleCaps[r], c) {
			return true
		}
	}
	return false
}

// IsCoordinator reports operational coordination rights (coordinator or
// organization manager).
func (p Principal) IsCoordinator() bool { return p.Can(CapRequestManage) }

// RoleStrings returns the principal's roles as strings.
func (p Principal) RoleStrings() []string { return Strings(p.Roles) }

type ctxKey struct{}

// WithPrincipal stores p in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the principal stored in ctx, if any.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
