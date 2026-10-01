// Package users implements local account administration and volunteer
// availability.
//
// There is no self-registration and no e-mail based password reset in
// v0.1.0. Administrators create accounts and reset passwords explicitly; the
// generated temporary password is shown once and must be changed at the next
// login.
package users

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/auth"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Availabilities lists volunteer availability values.
var Availabilities = []string{"available", "limited", "unavailable"}

// User is an account as shown to administrators. It contains no operational
// data.
type User struct {
	ID                 uuid.UUID  `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	Roles              []string   `json:"roles"`
	IsActive           bool       `json:"is_active"`
	MustChangePassword bool       `json:"must_change_password"`
	Locked             bool       `json:"locked"`
	Availability       string     `json:"availability"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

// Service implements user administration.
type Service struct {
	DB *pgxpool.Pool
}

const userSelect = `SELECT id, username, display_name, roles, is_active, must_change_password,
	(locked_until IS NOT NULL AND locked_until > now()), availability, last_login_at, created_at FROM users`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Roles, &u.IsActive, &u.MustChangePassword, &u.Locked,
		&u.Availability, &u.LastLoginAt, &u.CreatedAt)
	return &u, err
}

// List returns all users of the administrator's organization.
func (s *Service) List(ctx context.Context, p roles.Principal) ([]*User, error) {
	if !p.Can(roles.CapUserManage) {
		return nil, apperr.Forbidden("Only administrators can manage users.")
	}
	rows, err := s.DB.Query(ctx, userSelect+` WHERE organization_id = $1 ORDER BY lower(display_name), username`, p.OrgID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []*User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateInput creates an account.
type CreateInput struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

// CreateResult includes the one-time temporary password.
type CreateResult struct {
	User              *User  `json:"user"`
	TemporaryPassword string `json:"temporary_password"`
}

// Create creates an account with a temporary password.
func (s *Service) Create(ctx context.Context, p roles.Principal, in CreateInput) (*CreateResult, error) {
	if !p.Can(roles.CapUserManage) {
		return nil, apperr.Forbidden("Only administrators can manage users.")
	}
	errs := validation.Errors{}
	username := errs.Username("username", in.Username)
	display := validation.CleanText(in.DisplayName)
	errs.Text("display_name", display, 1, 80, false)
	rs, err := roles.Parse(in.Roles)
	if err != nil {
		errs.Add("roles", err.Error())
	}
	if err := errs.Err(); err != nil {
		return nil, err
	}
	temp, err := auth.GenerateTemporaryPassword()
	if err != nil {
		return nil, apperr.Internal(err)
	}
	hash, err := auth.HashPassword(ctx, temp)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	id := uuid.New()
	var u *User
	err = database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, organization_id, username, display_name, roles, password_hash,
			must_change_password) VALUES ($1, $2, $3, $4, $5, $6, true)`,
			id, p.OrgID, username, display, roles.Strings(rs), hash)
		if err != nil {
			if database.IsUniqueViolation(err, "users_username_key") {
				return apperr.ValidationField("username", "This username is already taken.")
			}
			return err
		}
		if err := audit.Record(ctx, tx, audit.ForActor(p, "user.created", audit.System).On("user", id).
			With("roles", roles.Strings(rs))); err != nil {
			return err
		}
		u, err = scanUser(tx.QueryRow(ctx, userSelect+` WHERE id = $1`, id))
		return err
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return nil, err
		}
		return nil, apperr.Internal(err)
	}
	return &CreateResult{User: u, TemporaryPassword: temp}, nil
}

// UpdateInput changes an account.
type UpdateInput struct {
	DisplayName *string   `json:"display_name"`
	Roles       *[]string `json:"roles"`
	IsActive    *bool     `json:"is_active"`
	Unlock      bool      `json:"unlock"`
}

// Update changes display name, roles or active state. Administrators cannot
// remove their own admin role or deactivate themselves, and the last active
// administrator cannot be removed.
func (s *Service) Update(ctx context.Context, p roles.Principal, id uuid.UUID, in UpdateInput) (*User, error) {
	if !p.Can(roles.CapUserManage) {
		return nil, apperr.Forbidden("Only administrators can manage users.")
	}
	var out *User
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		// Serialize admin changes so the "last administrator" check is safe.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(727274003)`); err != nil {
			return err
		}
		u, err := scanUser(tx.QueryRow(ctx, userSelect+` WHERE id = $1 AND organization_id = $2`, id, p.OrgID))
		if database.IsNoRows(err) {
			return apperr.NotFound("User not found.")
		}
		if err != nil {
			return err
		}
		errs := validation.Errors{}
		var changed []string
		if in.DisplayName != nil {
			d := validation.CleanText(*in.DisplayName)
			errs.Text("display_name", d, 1, 80, false)
			if d != u.DisplayName {
				u.DisplayName = d
				changed = append(changed, "display_name")
			}
		}
		newRoles := u.Roles
		if in.Roles != nil {
			rs, err := roles.Parse(*in.Roles)
			if err != nil {
				errs.Add("roles", err.Error())
			} else {
				newRoles = roles.Strings(rs)
				if !slices.Equal(newRoles, u.Roles) {
					changed = append(changed, "roles")
				}
			}
		}
		active := u.IsActive
		if in.IsActive != nil && *in.IsActive != u.IsActive {
			active = *in.IsActive
			changed = append(changed, "is_active")
		}
		if in.Unlock && u.Locked {
			changed = append(changed, "unlocked")
		}
		if err := errs.Err(); err != nil {
			return err
		}
		wasAdmin := slices.Contains(u.Roles, string(roles.Admin)) && u.IsActive
		staysAdmin := slices.Contains(newRoles, string(roles.Admin)) && active
		if id == p.UserID && wasAdmin && !staysAdmin {
			return apperr.Conflict("self_lockout", "You cannot remove your own administrator access or deactivate yourself.")
		}
		if wasAdmin && !staysAdmin {
			var admins int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE organization_id = $1 AND is_active
				AND 'admin' = ANY(roles)`, p.OrgID).Scan(&admins); err != nil {
				return err
			}
			if admins <= 1 {
				return apperr.Conflict("last_admin", "At least one active administrator must remain.")
			}
		}
		if len(changed) == 0 {
			out = u
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET display_name = $2, roles = $3, is_active = $4,
			locked_until = CASE WHEN $5 THEN NULL ELSE locked_until END,
			failed_login_count = CASE WHEN $5 THEN 0 ELSE failed_login_count END,
			updated_at = now() WHERE id = $1`, id, u.DisplayName, newRoles, active, in.Unlock); err != nil {
			return err
		}
		// Role or status changes take effect immediately: end existing sessions.
		if slices.Contains(changed, "roles") || slices.Contains(changed, "is_active") {
			if err := auth.RevokeUserSessions(ctx, tx, id); err != nil {
				return err
			}
		}
		ev := audit.ForActor(p, "user.updated", audit.System).On("user", id).With("changed_fields", changed)
		if slices.Contains(changed, "roles") {
			ev = ev.With("roles_from", u.Roles).With("roles_to", newRoles)
		}
		if err := audit.Record(ctx, tx, ev); err != nil {
			return err
		}
		out, err = scanUser(tx.QueryRow(ctx, userSelect+` WHERE id = $1`, id))
		return err
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return nil, err
		}
		return nil, apperr.Internal(err)
	}
	return out, nil
}

// ResetPassword sets a new temporary password, unlocks the account and ends
// all sessions of the user.
func (s *Service) ResetPassword(ctx context.Context, p roles.Principal, id uuid.UUID) (string, error) {
	if !p.Can(roles.CapUserManage) && !p.System {
		return "", apperr.Forbidden("Only administrators can reset passwords.")
	}
	temp, err := auth.GenerateTemporaryPassword()
	if err != nil {
		return "", apperr.Internal(err)
	}
	hash, err := auth.HashPassword(ctx, temp)
	if err != nil {
		return "", apperr.Internal(err)
	}
	err = database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2, must_change_password = true,
			failed_login_count = 0, locked_until = NULL, updated_at = now()
			WHERE id = $1 AND ($3 OR organization_id = $4)`, id, hash, p.System, p.OrgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound("User not found.")
		}
		if err := auth.RevokeUserSessions(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "user.password_reset", audit.System).On("user", id))
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return "", err
		}
		return "", apperr.Internal(err)
	}
	return temp, nil
}

// Volunteer is the coordinator-facing view of a volunteer: display name and
// availability only.
type Volunteer struct {
	ID                uuid.UUID `json:"id"`
	DisplayName       string    `json:"display_name"`
	Availability      string    `json:"availability"`
	AvailabilityNote  string    `json:"availability_note"`
	ActiveAssignments int       `json:"active_assignments"`
}

// ListVolunteers returns active volunteers for assignment planning.
func (s *Service) ListVolunteers(ctx context.Context, p roles.Principal) ([]Volunteer, error) {
	if !p.Can(roles.CapVolunteerList) {
		return nil, apperr.Forbidden("Only coordinators can list volunteers.")
	}
	rows, err := s.DB.Query(ctx, `SELECT u.id, u.display_name, u.availability, u.availability_note,
		(SELECT count(*) FROM assignments a WHERE a.volunteer_user_id = u.id
			AND a.status IN ('proposed', 'accepted', 'in_progress', 'partially_delivered'))
		FROM users u WHERE u.organization_id = $1 AND u.is_active AND 'volunteer' = ANY(u.roles)
		ORDER BY CASE u.availability WHEN 'available' THEN 0 WHEN 'limited' THEN 1 ELSE 2 END, lower(u.display_name)`,
		p.OrgID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []Volunteer{}
	for rows.Next() {
		var v Volunteer
		if err := rows.Scan(&v.ID, &v.DisplayName, &v.Availability, &v.AvailabilityNote, &v.ActiveAssignments); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// AvailabilityInput sets availability.
type AvailabilityInput struct {
	Availability string `json:"availability"`
	Note         string `json:"availability_note"`
}

// SetAvailability updates a volunteer's availability. Volunteers may set
// their own; organization managers may set anyone's.
func (s *Service) SetAvailability(ctx context.Context, p roles.Principal, id uuid.UUID, in AvailabilityInput) error {
	if id != p.UserID && !p.Can(roles.CapVolunteerAvailability) {
		return apperr.Forbidden("Only organization managers can change other people's availability.")
	}
	note := validation.CleanText(in.Note)
	errs := validation.Errors{}
	errs.OneOf("availability", in.Availability, Availabilities)
	errs.Text("availability_note", note, 0, 200, false)
	if err := errs.Err(); err != nil {
		return err
	}
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET availability = $2, availability_note = $3, updated_at = now()
			WHERE id = $1 AND organization_id = $4 AND 'volunteer' = ANY(roles)`, id, in.Availability, note, p.OrgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound("Volunteer not found.")
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "volunteer.availability_changed", audit.Internal).
			On("user", id).With("availability", in.Availability))
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return err
		}
		return apperr.Internal(err)
	}
	return nil
}

// AdminRoutes mounts /api/v1/admin/users.
func (s *Service) AdminRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		p, _ := roles.FromContext(req.Context())
		list, err := s.List(req.Context(), p)
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": list})
	})
	r.Post("/", func(w http.ResponseWriter, req *http.Request) {
		p, _ := roles.FromContext(req.Context())
		var in CreateInput
		if err := httpx.Decode(req, &in); err != nil {
			httpx.Error(w, req, err)
			return
		}
		res, err := s.Create(req.Context(), p, in)
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, res)
	})
	r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
		p, _ := roles.FromContext(req.Context())
		id, err := httpx.UUIDParam(req, "id")
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		var in UpdateInput
		if err := httpx.Decode(req, &in); err != nil {
			httpx.Error(w, req, err)
			return
		}
		u, err := s.Update(req.Context(), p, id, in)
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		httpx.JSON(w, http.StatusOK, u)
	})
	r.Post("/{id}/reset-password", func(w http.ResponseWriter, req *http.Request) {
		p, _ := roles.FromContext(req.Context())
		id, err := httpx.UUIDParam(req, "id")
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		temp, err := s.ResetPassword(req.Context(), p, id)
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"temporary_password": temp})
	})
}

// VolunteerRoutes mounts /api/v1/volunteers.
func (s *Service) VolunteerRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		p, _ := roles.FromContext(req.Context())
		list, err := s.ListVolunteers(req.Context(), p)
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": list})
	})
	r.Patch("/{id}/availability", func(w http.ResponseWriter, req *http.Request) {
		p, _ := roles.FromContext(req.Context())
		id, err := httpx.UUIDParam(req, "id")
		if err != nil {
			httpx.Error(w, req, err)
			return
		}
		var in AvailabilityInput
		if err := httpx.Decode(req, &in); err != nil {
			httpx.Error(w, req, err)
			return
		}
		if err := s.SetAvailability(req.Context(), p, id, in); err != nil {
			httpx.Error(w, req, err)
			return
		}
		httpx.NoContent(w)
	})
}

// HandleSetOwnAvailability handles PATCH /api/v1/me/availability.
func (s *Service) HandleSetOwnAvailability(w http.ResponseWriter, req *http.Request) {
	p, _ := roles.FromContext(req.Context())
	if !p.Has(roles.Volunteer) {
		httpx.Error(w, req, apperr.Forbidden("Only volunteers have an availability status."))
		return
	}
	var in AvailabilityInput
	if err := httpx.Decode(req, &in); err != nil {
		httpx.Error(w, req, err)
		return
	}
	if err := s.SetAvailability(req.Context(), p, p.UserID, in); err != nil {
		httpx.Error(w, req, err)
		return
	}
	httpx.NoContent(w)
}
