package requests

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// RedactedTitle replaces titles of deleted or retention-redacted requests.
const RedactedTitle = "[redacted]"

// RedactInTx wipes personal and free-text data of a request, its notes and
// its assignments while keeping the statistical skeleton (category, urgency,
// status, quantities, timestamps) and the audit trail intact.
func RedactInTx(ctx context.Context, tx pgx.Tx, r *Record, softDelete bool) error {
	_, err := tx.Exec(ctx, `UPDATE aid_requests SET title = $2, description = '', accessibility_notes = '',
		area_label = '', approx_lat = NULL, approx_lon = NULL, approx_decimals = NULL,
		exact_location_sealed = NULL, contact_details_sealed = NULL, resolution_summary = '',
		assigned_team = '', tags = '{}', redacted_at = COALESCE(redacted_at, now()),
		deleted_at = CASE WHEN $3 THEN now() ELSE deleted_at END,
		version = version + 1, updated_at = now() WHERE id = $1`, r.ID, RedactedTitle, softDelete)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE notes SET body = '', redacted_at = now() WHERE request_id = $1 AND redacted_at IS NULL`, r.ID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE assignments SET instructions = '', handover_notes = '', eta_text = '',
		completion_evidence_reference = '', cancellation_reason = '', updated_at = now() WHERE request_id = $1`, r.ID)
	return err
}

// AdminDelete soft-deletes and redacts a request identified by reference.
// Only administrators may delete; a reason is mandatory and audited.
// Administrators never see the content of the request in this flow.
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
		r, err := LoadByReference(ctx, tx, validation.CleanText(reference))
		if err != nil {
			return err
		}
		if r == nil || r.DeletedAt != nil || r.OrgID != p.OrgID {
			return apperr.NotFound("No request with this reference exists.")
		}
		active, err := CountActiveAssignments(ctx, tx, r.ID)
		if err != nil {
			return err
		}
		if active > 0 {
			return apperr.Conflict("active_assignments", "Close the request and its assignments before deleting it.")
		}
		if err := RedactInTx(ctx, tx, r, true); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "request.deleted", audit.System).
			On("aid_request", r.ID).ForRequest(r.ID).WithReason(reason).With("reference", r.Reference))
	}))
}
