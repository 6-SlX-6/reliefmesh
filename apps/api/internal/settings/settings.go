// Package settings manages instance-wide configuration that administrators
// can change at runtime: notices (including the emergency disclaimer),
// privacy precision, retention and categories.
package settings

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Settings is the instance configuration.
type Settings struct {
	InstanceName                 string    `json:"instance_name"`
	EmergencyNotice              string    `json:"emergency_notice"`
	CriticalUrgencyNotice        string    `json:"critical_urgency_notice"`
	MedicinePrivacyNotice        string    `json:"medicine_privacy_notice"`
	ExerciseMode                 bool      `json:"exercise_mode"`
	ExerciseLabel                string    `json:"exercise_label"`
	ApproxLocationDecimals       int       `json:"approx_location_decimals"`
	RetentionDaysClosed          int       `json:"retention_days_closed"`
	DefaultRequestExpiryHours    int       `json:"default_request_expiry_hours"`
	AllowMedicineFreeText        bool      `json:"allow_medicine_free_text"`
	VolunteerAccessRequiresGrant bool      `json:"volunteer_access_requires_grant"`
	UpdatedAt                    time.Time `json:"updated_at"`
}

// PublicNotice is shown before login.
type PublicNotice struct {
	InstanceName    string `json:"instance_name"`
	EmergencyNotice string `json:"emergency_notice"`
	ExerciseMode    bool   `json:"exercise_mode"`
	ExerciseLabel   string `json:"exercise_label"`
}

// Category is a request/offer category.
type Category struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Description string `json:"description"`
	IsSensitive bool   `json:"is_sensitive"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

// Service reads and updates settings.
type Service struct {
	DB *pgxpool.Pool
}

// Get loads the current settings.
func Get(ctx context.Context, q database.Querier) (*Settings, error) {
	var s Settings
	err := q.QueryRow(ctx, `SELECT instance_name, emergency_notice, critical_urgency_notice, medicine_privacy_notice,
		exercise_mode, exercise_label, approx_location_decimals, retention_days_closed, default_request_expiry_hours,
		allow_medicine_free_text, volunteer_access_requires_grant, updated_at
		FROM instance_settings WHERE id = 1`).Scan(
		&s.InstanceName, &s.EmergencyNotice, &s.CriticalUrgencyNotice, &s.MedicinePrivacyNotice,
		&s.ExerciseMode, &s.ExerciseLabel, &s.ApproxLocationDecimals, &s.RetentionDaysClosed,
		&s.DefaultRequestExpiryHours, &s.AllowMedicineFreeText, &s.VolunteerAccessRequiresGrant, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Get loads settings using the service pool.
func (s *Service) Get(ctx context.Context) (*Settings, error) { return Get(ctx, s.DB) }

// Update is a partial settings update.
type Update struct {
	InstanceName                 *string `json:"instance_name"`
	EmergencyNotice              *string `json:"emergency_notice"`
	CriticalUrgencyNotice        *string `json:"critical_urgency_notice"`
	MedicinePrivacyNotice        *string `json:"medicine_privacy_notice"`
	ExerciseMode                 *bool   `json:"exercise_mode"`
	ExerciseLabel                *string `json:"exercise_label"`
	ApproxLocationDecimals       *int    `json:"approx_location_decimals"`
	RetentionDaysClosed          *int    `json:"retention_days_closed"`
	DefaultRequestExpiryHours    *int    `json:"default_request_expiry_hours"`
	AllowMedicineFreeText        *bool   `json:"allow_medicine_free_text"`
	VolunteerAccessRequiresGrant *bool   `json:"volunteer_access_requires_grant"`
}

// UpdateSettings applies a partial update. Only administrators may call it.
func (s *Service) UpdateSettings(ctx context.Context, p roles.Principal, u Update) (*Settings, error) {
	if !p.Can(roles.CapSettingsManage) {
		return nil, apperr.Forbidden("Only administrators can change instance settings.")
	}
	errs := validation.Errors{}
	text := func(field string, v *string, min, max int) {
		if v != nil {
			*v = validation.CleanText(*v)
			errs.Text(field, *v, min, max, true)
		}
	}
	text("instance_name", u.InstanceName, 1, 80)
	text("emergency_notice", u.EmergencyNotice, 10, 1000)
	text("critical_urgency_notice", u.CriticalUrgencyNotice, 10, 1500)
	text("medicine_privacy_notice", u.MedicinePrivacyNotice, 10, 1500)
	text("exercise_label", u.ExerciseLabel, 1, 120)
	errs.IntRange("approx_location_decimals", u.ApproxLocationDecimals, 0, 3)
	errs.IntRange("retention_days_closed", u.RetentionDaysClosed, 1, 3650)
	errs.IntRange("default_request_expiry_hours", u.DefaultRequestExpiryHours, 1, 8760)
	if err := errs.Err(); err != nil {
		return nil, err
	}

	var changed []string
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		cur, err := Get(ctx, tx)
		if err != nil {
			return err
		}
		next := *cur
		setS := func(name string, dst *string, v *string) {
			if v != nil && *v != *dst {
				*dst = *v
				changed = append(changed, name)
			}
		}
		setB := func(name string, dst *bool, v *bool) {
			if v != nil && *v != *dst {
				*dst = *v
				changed = append(changed, name)
			}
		}
		setI := func(name string, dst *int, v *int) {
			if v != nil && *v != *dst {
				*dst = *v
				changed = append(changed, name)
			}
		}
		setS("instance_name", &next.InstanceName, u.InstanceName)
		setS("emergency_notice", &next.EmergencyNotice, u.EmergencyNotice)
		setS("critical_urgency_notice", &next.CriticalUrgencyNotice, u.CriticalUrgencyNotice)
		setS("medicine_privacy_notice", &next.MedicinePrivacyNotice, u.MedicinePrivacyNotice)
		setB("exercise_mode", &next.ExerciseMode, u.ExerciseMode)
		setS("exercise_label", &next.ExerciseLabel, u.ExerciseLabel)
		setI("approx_location_decimals", &next.ApproxLocationDecimals, u.ApproxLocationDecimals)
		setI("retention_days_closed", &next.RetentionDaysClosed, u.RetentionDaysClosed)
		setI("default_request_expiry_hours", &next.DefaultRequestExpiryHours, u.DefaultRequestExpiryHours)
		setB("allow_medicine_free_text", &next.AllowMedicineFreeText, u.AllowMedicineFreeText)
		setB("volunteer_access_requires_grant", &next.VolunteerAccessRequiresGrant, u.VolunteerAccessRequiresGrant)
		if len(changed) == 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE instance_settings SET instance_name=$1, emergency_notice=$2,
			critical_urgency_notice=$3, medicine_privacy_notice=$4, exercise_mode=$5, exercise_label=$6,
			approx_location_decimals=$7, retention_days_closed=$8, default_request_expiry_hours=$9,
			allow_medicine_free_text=$10, volunteer_access_requires_grant=$11, updated_at=now(), updated_by_user_id=$12
			WHERE id = 1`,
			next.InstanceName, next.EmergencyNotice, next.CriticalUrgencyNotice, next.MedicinePrivacyNotice,
			next.ExerciseMode, next.ExerciseLabel, next.ApproxLocationDecimals, next.RetentionDaysClosed,
			next.DefaultRequestExpiryHours, next.AllowMedicineFreeText, next.VolunteerAccessRequiresGrant, p.UserID)
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "settings.updated", audit.System).With("changed_fields", changed))
	})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return s.Get(ctx)
}

// ListCategories returns all categories ordered for display.
func ListCategories(ctx context.Context, q database.Querier, includeDisabled bool) ([]Category, error) {
	rows, err := q.Query(ctx, `SELECT code, label, description, is_sensitive, enabled, sort_order
		FROM categories WHERE enabled OR $1 ORDER BY sort_order, code`, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Code, &c.Label, &c.Description, &c.IsSensitive, &c.Enabled, &c.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCategory loads a category by code.
func GetCategory(ctx context.Context, q database.Querier, code string) (*Category, error) {
	var c Category
	err := q.QueryRow(ctx, `SELECT code, label, description, is_sensitive, enabled, sort_order
		FROM categories WHERE code = $1`, code).Scan(&c.Code, &c.Label, &c.Description, &c.IsSensitive, &c.Enabled, &c.SortOrder)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CategoryUpdate is a partial category update. Category codes are fixed in
// v0.1.0; administrators can relabel, reorder and enable/disable them.
type CategoryUpdate struct {
	Label       *string `json:"label"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sort_order"`
}

// UpdateCategory applies a category update.
func (s *Service) UpdateCategory(ctx context.Context, p roles.Principal, code string, u CategoryUpdate) (*Category, error) {
	if !p.Can(roles.CapCategoriesManage) {
		return nil, apperr.Forbidden("Only administrators can manage categories.")
	}
	errs := validation.Errors{}
	if u.Label != nil {
		*u.Label = validation.CleanText(*u.Label)
		errs.Text("label", *u.Label, 1, 60, false)
	}
	if u.Description != nil {
		*u.Description = validation.CleanText(*u.Description)
		errs.Text("description", *u.Description, 0, 300, true)
	}
	errs.IntRange("sort_order", u.SortOrder, 0, 10000)
	if err := errs.Err(); err != nil {
		return nil, err
	}
	var out *Category
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		c, err := GetCategory(ctx, tx, code)
		if err != nil {
			return err
		}
		if c == nil {
			return apperr.NotFound("Category not found.")
		}
		if u.Label != nil {
			c.Label = *u.Label
		}
		if u.Description != nil {
			c.Description = *u.Description
		}
		if u.Enabled != nil {
			c.Enabled = *u.Enabled
		}
		if u.SortOrder != nil {
			c.SortOrder = *u.SortOrder
		}
		if _, err := tx.Exec(ctx, `UPDATE categories SET label=$2, description=$3, enabled=$4, sort_order=$5 WHERE code=$1`,
			c.Code, c.Label, c.Description, c.Enabled, c.SortOrder); err != nil {
			return err
		}
		out = c
		return audit.Record(ctx, tx, audit.ForActor(p, "category.updated", audit.System).
			With("code", c.Code).With("enabled", c.Enabled))
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return nil, err
		}
		return nil, apperr.Internal(err)
	}
	return out, nil
}

// ClientSettings bundles what every authenticated client needs, including
// offline: notices, precision and categories.
type ClientSettings struct {
	Settings   *Settings  `json:"settings"`
	Categories []Category `json:"categories"`
}

// HandleGetPublic handles GET /api/v1/public/notice.
func (s *Service) HandleGetPublic(w http.ResponseWriter, r *http.Request) {
	st, err := s.Get(r.Context())
	if err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	httpx.JSON(w, http.StatusOK, PublicNotice{InstanceName: st.InstanceName, EmergencyNotice: st.EmergencyNotice,
		ExerciseMode: st.ExerciseMode, ExerciseLabel: st.ExerciseLabel})
}

// HandleGetClient handles GET /api/v1/settings.
func (s *Service) HandleGetClient(w http.ResponseWriter, r *http.Request) {
	st, err := s.Get(r.Context())
	if err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	cats, err := ListCategories(r.Context(), s.DB, false)
	if err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	httpx.JSON(w, http.StatusOK, ClientSettings{Settings: st, Categories: cats})
}

// HandleUpdate handles PUT /api/v1/admin/settings.
func (s *Service) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var u Update
	if err := httpx.Decode(r, &u); err != nil {
		httpx.Error(w, r, err)
		return
	}
	st, err := s.UpdateSettings(r.Context(), p, u)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

// HandleListCategoriesAdmin handles GET /api/v1/admin/categories.
func (s *Service) HandleListCategoriesAdmin(w http.ResponseWriter, r *http.Request) {
	cats, err := ListCategories(r.Context(), s.DB, true)
	if err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": cats})
}

// HandleUpdateCategory handles PATCH /api/v1/admin/categories/{code}.
func (s *Service) HandleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var u CategoryUpdate
	if err := httpx.Decode(r, &u); err != nil {
		httpx.Error(w, r, err)
		return
	}
	c, err := s.UpdateCategory(r.Context(), p, chi.URLParam(r, "code"), u)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}
