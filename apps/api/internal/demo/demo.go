// Package demo seeds fictional exercise scenarios.
//
// Demo data is only available when RELIEFMESH_ALLOW_DEMO_SEED=true. All
// personas share a publicly documented password, so demo mode must never be
// used for real requests or personal data. Seeding goes through the regular
// services, so every demo record has a complete, valid audit trail.
package demo

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/assignments"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/auth"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/offers"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/requests"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

//go:embed scenarios/*.json
var scenarioFS embed.FS

// Password is the shared password of all demo personas. It is public.
const Password = "reliefmesh-demo-exercise"

// Persona is a demo account.
type Persona struct {
	Key         string
	Username    string
	DisplayName string
	Roles       []roles.Role
	Available   string
}

// Personas are created by every scenario.
var Personas = []Persona{
	{"admin", "demo-admin", "Ada Admin (demo)", []roles.Role{roles.Admin}, ""},
	{"coordinator", "demo-coordinator", "Casey Coordinator (demo)", []roles.Role{roles.Coordinator}, ""},
	{"manager", "demo-manager", "Morgan Manager (demo)", []roles.Role{roles.OrganizationManager}, ""},
	{"volunteer1", "demo-volunteer-1", "Vera Volunteer (demo)", []roles.Role{roles.Volunteer}, "available"},
	{"volunteer2", "demo-volunteer-2", "Viktor Volunteer (demo)", []roles.Role{roles.Volunteer}, "limited"},
	{"volunteer3", "demo-volunteer-3", "Valentina Volunteer (demo)", []roles.Role{roles.Volunteer}, "available"},
	{"requester1", "demo-requester-1", "Robin Requester (demo)", []roles.Role{roles.Requester}, ""},
	{"requester2", "demo-requester-2", "Riley Requester (demo)", []roles.Role{roles.Requester}, ""},
}

type assignSpec struct {
	Offer     string `json:"offer"`
	Volunteer string `json:"volunteer"`
	Team      string `json:"team"`
	Quantity  int    `json:"quantity"`
	Status    string `json:"status"`
}

type requestSpec struct {
	Key              string            `json:"key"`
	Category         string            `json:"category"`
	Urgency          string            `json:"urgency"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	People           *int              `json:"people"`
	Quantity         *int              `json:"quantity"`
	Unit             string            `json:"unit"`
	Area             string            `json:"area"`
	Lat              *float64          `json:"lat"`
	Lon              *float64          `json:"lon"`
	Exact            *exactSpec        `json:"exact"`
	Accessibility    string            `json:"accessibility"`
	Contact          string            `json:"contact"`
	ContactMethod    string            `json:"contact_method"`
	ContactVis       string            `json:"contact_visibility"`
	RequestedByHours *int              `json:"requested_by_hours"`
	Status           string            `json:"status"`
	Resolution       string            `json:"resolution"`
	By               string            `json:"by"`
	Sensitive        bool              `json:"sensitive"`
	RequiresAuth     bool              `json:"requires_authorization"`
	Assign           *assignSpec       `json:"assign"`
	Extra            map[string]string `json:"-"`
}

type exactSpec struct {
	Address string `json:"address"`
}

type offerSpec struct {
	Key          string     `json:"key"`
	Category     string     `json:"category"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Quantity     int        `json:"quantity"`
	Unit         string     `json:"unit"`
	Mode         string     `json:"mode"`
	Area         string     `json:"area"`
	Exact        *exactSpec `json:"exact"`
	Restrictions string     `json:"restrictions"`
	By           string     `json:"by"`
}

// Scenario is a demo exercise scenario.
type Scenario struct {
	Name        string        `json:"name"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Requests    []requestSpec `json:"requests"`
	Offers      []offerSpec   `json:"offers"`
}

// Scenarios lists available scenarios.
func Scenarios() ([]Scenario, error) {
	entries, err := scenarioFS.ReadDir("scenarios")
	if err != nil {
		return nil, err
	}
	var out []Scenario
	for _, e := range entries {
		b, err := scenarioFS.ReadFile("scenarios/" + e.Name())
		if err != nil {
			return nil, err
		}
		var sc Scenario
		if err := json.Unmarshal(b, &sc); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Seeder seeds scenarios.
type Seeder struct {
	DB          *pgxpool.Pool
	Enabled     bool
	Requests    *requests.Service
	Offers      *offers.Service
	Assignments *assignments.Service
}

// Seeded lists scenarios already seeded.
func (s *Seeder) Seeded(ctx context.Context) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT DISTINCT metadata->>'scenario' FROM audit_events WHERE action = 'demo.seeded'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Seeder) ensureOrg(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.DB.QueryRow(ctx, `SELECT id FROM organizations ORDER BY created_at LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !database.IsNoRows(err) {
		return uuid.Nil, err
	}
	id = uuid.New()
	_, err = s.DB.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Demo Exercise Organization')`, id)
	return id, err
}

func (s *Seeder) ensurePersonas(ctx context.Context, org uuid.UUID) (map[string]roles.Principal, error) {
	hash, err := auth.HashPassword(ctx, Password)
	if err != nil {
		return nil, err
	}
	out := map[string]roles.Principal{}
	for _, ps := range Personas {
		var id uuid.UUID
		var rs []string
		err := s.DB.QueryRow(ctx, `SELECT id, roles FROM users WHERE lower(username) = $1`, ps.Username).Scan(&id, &rs)
		if database.IsNoRows(err) {
			id = uuid.New()
			rs = roles.Strings(ps.Roles)
			availability := "unavailable"
			if ps.Available != "" {
				availability = ps.Available
			}
			err = database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `INSERT INTO users (id, organization_id, username, display_name, roles,
					password_hash, availability) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
					id, org, ps.Username, ps.DisplayName, rs, hash, availability); err != nil {
					return err
				}
				return audit.Record(ctx, tx, audit.Event{OrgID: &org, ActorRoles: []string{"system"}, Action: "user.created",
					EntityType: "user", EntityID: &id, Visibility: audit.System,
					Metadata: map[string]any{"roles": rs, "demo": true}})
			})
		}
		if err != nil {
			return nil, err
		}
		parsed, _ := roles.Parse(rs)
		out[ps.Key] = roles.Principal{UserID: id, OrgID: org, Username: ps.Username, DisplayName: ps.DisplayName, Roles: parsed}
	}
	return out, nil
}

// Seed seeds a scenario. It refuses to run unless demo seeding is enabled
// and refuses to seed the same scenario twice.
func (s *Seeder) Seed(ctx context.Context, actor roles.Principal, name string) error {
	if !s.Enabled {
		return apperr.ForbiddenCode("demo_disabled", "Demo data is disabled on this instance (RELIEFMESH_ALLOW_DEMO_SEED=false).")
	}
	if !actor.System && !actor.Can(roles.CapDemoSeed) {
		return apperr.Forbidden("Only administrators can initialize demo data.")
	}
	all, err := Scenarios()
	if err != nil {
		return apperr.Internal(err)
	}
	idx := slices.IndexFunc(all, func(sc Scenario) bool { return sc.Name == name })
	if idx < 0 {
		return apperr.ValidationField("scenario", "Unknown scenario.")
	}
	sc := all[idx]
	seeded, err := s.Seeded(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	if slices.Contains(seeded, name) {
		return apperr.Conflict("already_seeded", "This scenario was already loaded.")
	}
	org, err := s.ensureOrg(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	personas, err := s.ensurePersonas(ctx, org)
	if err != nil {
		return apperr.Internal(err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE instance_settings SET exercise_mode = true, updated_at = now() WHERE id = 1`); err != nil {
		return apperr.Internal(err)
	}
	coord := personas["coordinator"]

	offerIDs := map[string]uuid.UUID{}
	for _, os := range sc.Offers {
		by, ok := personas[os.By]
		if !ok {
			by = personas["manager"]
		}
		qty := os.Quantity
		in := offers.Input{Category: os.Category, Title: os.Title, Description: os.Description, QuantityAvailable: &qty,
			Unit: os.Unit, DeliveryMode: os.Mode, Restrictions: os.Restrictions,
			Location: locations.Input{Mode: "area_only", AreaLabel: os.Area}}
		if os.Exact != nil {
			in.Location = locations.Input{Mode: "protected_exact", AreaLabel: os.Area,
				Exact: &locations.ExactLocation{Address: os.Exact.Address}}
		}
		v, _, err := s.Offers.Create(ctx, by, in)
		if err != nil {
			return fmt.Errorf("offer %s: %w", os.Key, err)
		}
		offerIDs[os.Key] = v.ID
	}

	now := time.Now()
	for _, rs := range sc.Requests {
		by, ok := personas[rs.By]
		if !ok {
			by = personas["requester1"]
		}
		in := requests.CreateInput{Category: rs.Category, Urgency: rs.Urgency, Title: rs.Title,
			Description: rs.Description, EstimatedPeopleAffected: rs.People, RequestedQuantity: rs.Quantity,
			RequestedUnit: rs.Unit, AccessibilityNotes: rs.Accessibility, SensitiveDataFlag: rs.Sensitive,
			RequiresFormalAuthorization: rs.RequiresAuth, EmergencyNoticeAcknowledged: true}
		if rs.RequestedByHours != nil {
			t := now.Add(time.Duration(*rs.RequestedByHours) * time.Hour)
			in.RequestedByTime = &t
		}
		switch {
		case rs.Exact != nil:
			in.Location = locations.Input{Mode: "protected_exact", AreaLabel: rs.Area,
				Exact: &locations.ExactLocation{Address: rs.Exact.Address}}
		case rs.Lat != nil && rs.Lon != nil:
			in.Location = locations.Input{Mode: "approximate", AreaLabel: rs.Area, Lat: rs.Lat, Lon: rs.Lon}
		case rs.Area != "":
			in.Location = locations.Input{Mode: "area_only", AreaLabel: rs.Area}
		}
		if rs.Contact != "" {
			method := rs.ContactMethod
			if method == "" {
				method = "in_person"
			}
			vis := rs.ContactVis
			if vis == "" {
				vis = locations.ContactVisibilityCoordinatorsOnly
			}
			in.Contact = locations.ContactInput{Method: method, Visibility: vis, Details: rs.Contact}
		}
		v, _, err := s.Requests.Create(ctx, by, in)
		if err != nil {
			return fmt.Errorf("request %s: %w", rs.Key, err)
		}
		if err := s.advance(ctx, coord, personas, v.ID, rs, offerIDs); err != nil {
			return fmt.Errorf("request %s: %w", rs.Key, err)
		}
	}
	return database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		ev := audit.ForActor(actor, "demo.seeded", audit.System).With("scenario", name)
		if actor.UserID == uuid.Nil {
			ev.OrgID = &org
		}
		return audit.Record(ctx, tx, ev)
	})
}

func (s *Seeder) advance(ctx context.Context, coord roles.Principal, personas map[string]roles.Principal,
	id uuid.UUID, rs requestSpec, offerIDs map[string]uuid.UUID) error {
	step := func(status string) error {
		in := requests.StatusInput{Status: status}
		if status == "resolved" {
			in.ResolutionSummary = rs.Resolution
			if in.ResolutionSummary == "" {
				in.ResolutionSummary = "Completed during exercise."
			}
		}
		_, err := s.Requests.ChangeStatus(ctx, coord, id, in)
		return err
	}
	switch rs.Status {
	case "", "submitted":
		return nil
	case "under_review":
		return step("under_review")
	case "verified":
		return step("verified")
	case "assigned", "resolved":
		if err := step("verified"); err != nil {
			return err
		}
		spec := rs.Assign
		if spec == nil {
			spec = &assignSpec{Team: "Exercise team", Status: "delivered"}
		}
		in := assignments.CreateInput{RequestID: id, TeamLabel: spec.Team, Instructions: "Exercise assignment."}
		if spec.Offer != "" {
			oid := offerIDs[spec.Offer]
			q := spec.Quantity
			in.OfferID, in.QuantityAssigned = &oid, &q
		}
		var volunteer roles.Principal
		if spec.Volunteer != "" {
			volunteer = personas[spec.Volunteer]
			vid := volunteer.UserID
			in.VolunteerUserID = &vid
			in.DestinationLocationAccessGranted = true
		}
		a, _, err := s.Assignments.Create(ctx, coord, in)
		if err != nil {
			return err
		}
		actor := coord
		if spec.Volunteer != "" {
			actor = volunteer
		}
		path := map[string][]string{
			"accepted":  {"accepted"},
			"delivered": {"accepted", "in_progress", "delivered"},
		}[spec.Status]
		for _, st := range path {
			if _, err := s.Assignments.ChangeStatus(ctx, actor, a.ID, assignments.StatusInput{Status: st}); err != nil {
				return err
			}
		}
		if rs.Status == "resolved" {
			return step("resolved")
		}
	}
	return nil
}

// Info describes demo availability.
type Info struct {
	Enabled   bool           `json:"enabled"`
	Scenarios []ScenarioInfo `json:"scenarios"`
	Seeded    []string       `json:"seeded"`
	Personas  []PersonaInfo  `json:"personas"`
	Password  string         `json:"password,omitempty"`
}

// ScenarioInfo summarizes a scenario.
type ScenarioInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// PersonaInfo summarizes a persona.
type PersonaInfo struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// HandleInfo handles GET /api/v1/admin/demo.
func (s *Seeder) HandleInfo(w http.ResponseWriter, r *http.Request) {
	info := Info{Enabled: s.Enabled, Scenarios: []ScenarioInfo{}, Seeded: []string{}, Personas: []PersonaInfo{}}
	all, err := Scenarios()
	if err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	for _, sc := range all {
		info.Scenarios = append(info.Scenarios, ScenarioInfo{Name: sc.Name, Title: sc.Title, Description: sc.Description})
	}
	if info.Seeded, err = s.Seeded(r.Context()); err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	if s.Enabled {
		info.Password = Password
		for _, ps := range Personas {
			info.Personas = append(info.Personas, PersonaInfo{Username: ps.Username, Roles: roles.Strings(ps.Roles)})
		}
	}
	httpx.JSON(w, http.StatusOK, info)
}

// HandleSeed handles POST /api/v1/admin/demo/seed.
func (s *Seeder) HandleSeed(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var in struct {
		Scenario string `json:"scenario"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := s.Seed(r.Context(), p, strings.TrimSpace(in.Scenario)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
