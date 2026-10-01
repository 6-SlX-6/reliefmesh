// Package retention applies the instance data retention policy and handles
// administrative record deletion.
//
// Retention never runs silently: an administrator previews and applies it
// explicitly (or an operator runs the CLI command, e.g. from cron). It
// redacts personal and free-text data of requests closed longer than the
// configured number of days, and of offers cancelled/expired that long ago,
// keeping only the statistical skeleton. The audit trail is preserved; it
// never contains personal data by design.
package retention

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/offers"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/requests"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Service applies retention.
type Service struct {
	DB       *pgxpool.Pool
	Requests *requests.Service
	Offers   *offers.Service
}

// Preview describes what retention would redact.
type Preview struct {
	RetentionDays int       `json:"retention_days"`
	Cutoff        time.Time `json:"cutoff"`
	Requests      int       `json:"requests"`
	Offers        int       `json:"offers"`
}

func (s *Service) candidates(ctx context.Context, q database.Querier, org uuid.UUID) (*Preview, []uuid.UUID, []uuid.UUID, error) {
	st, err := settings.Get(ctx, q)
	if err != nil {
		return nil, nil, nil, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -st.RetentionDaysClosed)
	pv := &Preview{RetentionDays: st.RetentionDaysClosed, Cutoff: cutoff}
	collect := func(sql string) ([]uuid.UUID, error) {
		rows, err := q.Query(ctx, sql, org, cutoff)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var ids []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}
	reqIDs, err := collect(`SELECT id FROM aid_requests WHERE organization_id = $1 AND redacted_at IS NULL
		AND status IN ('resolved', 'cancelled', 'expired', 'duplicate') AND closed_at < $2`)
	if err != nil {
		return nil, nil, nil, err
	}
	offerIDs, err := collect(`SELECT id FROM offers WHERE organization_id = $1 AND redacted_at IS NULL
		AND status IN ('cancelled', 'expired') AND updated_at < $2`)
	if err != nil {
		return nil, nil, nil, err
	}
	pv.Requests, pv.Offers = len(reqIDs), len(offerIDs)
	return pv, reqIDs, offerIDs, nil
}

// PreviewRetention returns candidate counts.
func (s *Service) PreviewRetention(ctx context.Context, p roles.Principal) (*Preview, error) {
	if !p.Can(roles.CapRetentionManage) && !p.System {
		return nil, apperr.Forbidden("Only administrators can manage retention.")
	}
	pv, _, _, err := s.candidates(ctx, s.DB, p.OrgID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return pv, nil
}

// Apply redacts all candidates.
func (s *Service) Apply(ctx context.Context, p roles.Principal) (*Preview, error) {
	if !p.Can(roles.CapRetentionManage) && !p.System {
		return nil, apperr.Forbidden("Only administrators can manage retention.")
	}
	var pv *Preview
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		var reqIDs, offerIDs []uuid.UUID
		var err error
		pv, reqIDs, offerIDs, err = s.candidates(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		for _, id := range reqIDs {
			r, err := requests.Load(ctx, tx, id, true)
			if err != nil {
				return err
			}
			if err := requests.RedactInTx(ctx, tx, r, false); err != nil {
				return err
			}
		}
		for _, id := range offerIDs {
			if err := offers.RedactInTx(ctx, tx, id, false); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "retention.applied", audit.System).
			With("requests_redacted", len(reqIDs)).With("offers_redacted", len(offerIDs)).
			With("retention_days", pv.RetentionDays))
	})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return pv, nil
}

// HandlePreview handles GET /api/v1/admin/retention.
func (s *Service) HandlePreview(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	pv, err := s.PreviewRetention(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, pv)
}

// HandleApply handles POST /api/v1/admin/retention/apply.
func (s *Service) HandleApply(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var in struct {
		Confirm bool `json:"confirm"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if !in.Confirm {
		httpx.Error(w, r, apperr.ValidationField("confirm", "Confirm that you want to redact data permanently."))
		return
	}
	pv, err := s.Apply(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, pv)
}

type deleteInput struct {
	EntityType string `json:"entity_type"`
	Reference  string `json:"reference"`
	Reason     string `json:"reason"`
}

// HandleDelete handles POST /api/v1/admin/records/delete. The administrator
// supplies a reference (e.g. from a data subject's deletion request) and a
// reason; the record content is never displayed in this flow.
func (s *Service) HandleDelete(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var in deleteInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	errs := validation.Errors{}
	errs.OneOf("entity_type", in.EntityType, []string{"request", "offer"})
	if err := errs.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}
	var err error
	if in.EntityType == "request" {
		err = s.Requests.AdminDelete(r.Context(), p, in.Reference, in.Reason)
	} else {
		err = s.Offers.AdminDelete(r.Context(), p, in.Reference, in.Reason)
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
