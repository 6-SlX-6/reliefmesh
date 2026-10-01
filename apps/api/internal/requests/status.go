package requests

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// StatusInput requests a lifecycle transition.
type StatusInput struct {
	Status                  string `json:"status"`
	Reason                  string `json:"reason"`
	ResolutionSummary       string `json:"resolution_summary"`
	DuplicateOfReference    string `json:"duplicate_of_reference"`
	CancelActiveAssignments bool   `json:"cancel_active_assignments"`
	// Version is informational (offline clients send the version they saw).
	// Status changes are intent-based: they apply if the transition is still
	// valid from the current status.
	Version *int `json:"version"`
}

// TransitionOptions carry the details of a transition.
type TransitionOptions struct {
	Reason            string
	ResolutionSummary string
	DuplicateOf       *uuid.UUID
	// Cause marks transitions performed as a direct consequence of another
	// explicit human action (e.g. "assignment_created").
	Cause string
}

// ApplyTransition updates the request row for a transition that the caller
// has already authorized, and returns the audit event to record.
func ApplyTransition(ctx context.Context, tx pgx.Tx, p roles.Principal, r *Record, to Status, opt TransitionOptions) (audit.Event, error) {
	from := r.Status
	review, verification := r.ReviewStatus, r.VerificationLevel
	switch to {
	case StatusUnderReview:
		review = "in_review"
	case StatusVerified:
		review = "reviewed"
		if verification == "unverified" || verification == "self_reported" {
			verification = "coordinator_confirmed"
		}
	}
	summary := r.ResolutionSummary
	if opt.ResolutionSummary != "" {
		summary = opt.ResolutionSummary
	}
	_, err := tx.Exec(ctx, `UPDATE aid_requests SET status = $2, review_status = $3, verification_level = $4,
		resolution_summary = $5, duplicate_of_request_id = $6,
		closed_at = CASE WHEN $7 THEN now() ELSE NULL END,
		version = version + 1, updated_at = now() WHERE id = $1`,
		r.ID, string(to), review, verification, summary, opt.DuplicateOf, to.IsClosed())
	if err != nil {
		return audit.Event{}, err
	}
	r.Status, r.ReviewStatus, r.VerificationLevel, r.ResolutionSummary = to, review, verification, summary
	ev := audit.ForActor(p, "request.status_changed", audit.Shared).On("aid_request", r.ID).ForRequest(r.ID).
		Status(string(from), string(to)).WithReason(opt.Reason)
	if opt.Cause != "" {
		ev = ev.With("cause", opt.Cause)
	}
	if opt.DuplicateOf != nil {
		ev = ev.With("duplicate_of_request_id", opt.DuplicateOf.String())
	}
	return ev, nil
}

// ChangeStatus performs a lifecycle transition after enforcing role rules.
func (s *Service) ChangeStatus(ctx context.Context, p roles.Principal, id uuid.UUID, in StatusInput) (*View, error) {
	var out *Record
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		r, rel, acc, err := s.loadVisible(ctx, tx, p, id, true)
		if err != nil {
			return err
		}
		if r.RedactedAt != nil {
			return apperr.Conflict("redacted", "This request was redacted.")
		}
		to := Status(in.Status)
		if !slices.Contains(AllStatuses, to) {
			return apperr.ValidationField("status", "Unknown status.")
		}
		if to == r.Status {
			out = r // idempotent no-op
			return nil
		}
		kind := actorKind(rel)
		if rel == RelationAssignedVolunteer && (acc == nil || !acc.active) {
			return apperr.Forbidden("Accept your assignment before starting work.")
		}
		if !CanTransition(r.Status, to, kind) {
			if kind != ActorCoordinator && CanTransition(r.Status, to, ActorCoordinator) {
				return apperr.Forbidden("Only coordinators can set this status.")
			}
			allowed := NextStatuses(r.Status, kind)
			return apperr.Conflict("invalid_transition", "This status change is not allowed from the current status.").
				WithDetail("current_status", string(r.Status)).WithDetail("allowed_statuses", allowed)
		}

		reason := validation.CleanText(in.Reason)
		summary := validation.CleanText(in.ResolutionSummary)
		errs := validation.Errors{}
		errs.Text("reason", reason, 0, 1000, true)
		errs.Text("resolution_summary", summary, 0, 2000, true)
		opt := TransitionOptions{Reason: reason, ResolutionSummary: summary}
		switch to {
		case StatusCancelled:
			errs.Text("reason", reason, 3, 1000, true)
		case StatusResolved:
			errs.Text("resolution_summary", summary, 3, 2000, true)
		case StatusDuplicate:
			ref := validation.CleanText(in.DuplicateOfReference)
			if ref == "" {
				errs.Add("duplicate_of_reference", "Enter the reference of the original request.")
				break
			}
			canonical, err := LoadByReference(ctx, tx, ref)
			if err != nil {
				return err
			}
			switch {
			case canonical == nil || canonical.DeletedAt != nil || canonical.OrgID != r.OrgID:
				errs.Add("duplicate_of_reference", "No request with this reference exists.")
			case canonical.ID == r.ID:
				errs.Add("duplicate_of_reference", "A request cannot duplicate itself.")
			case canonical.Status == StatusDuplicate:
				errs.Add("duplicate_of_reference", "That request is itself a duplicate. Reference its original instead.")
			default:
				cid := canonical.ID
				opt.DuplicateOf = &cid
			}
		}
		if err := errs.Err(); err != nil {
			return err
		}

		active, err := CountActiveAssignments(ctx, tx, r.ID)
		if err != nil {
			return err
		}
		if to == StatusAssigned && active == 0 && r.AssignedTeam == "" && r.AssignedUserID == nil {
			return apperr.Conflict("no_assignment", "Create an assignment (or set a responsible team) before marking the request as assigned.")
		}
		var events []audit.Event
		if to.IsClosed() && active > 0 {
			if !in.CancelActiveAssignments {
				return apperr.Conflict("active_assignments",
					"This request still has active assignments. Confirm that they should be cancelled when closing it.").
					WithDetail("active_assignments", active)
			}
			if s.Assignments == nil {
				return apperr.Internal(errNoAssignmentCloser)
			}
			evs, err := s.Assignments.CancelActiveForRequest(ctx, tx, p, r.ID, "Request closed as "+string(to))
			if err != nil {
				return err
			}
			events = append(events, evs...)
		}
		ev, err := ApplyTransition(ctx, tx, p, r, to, opt)
		if err != nil {
			return err
		}
		events = append(events, ev)
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

var errNoAssignmentCloser = errors.New("assignment closer not configured")

// Reopen moves a closed request back to under_review. Only coordinators may
// reopen, and a reason is mandatory.
func (s *Service) Reopen(ctx context.Context, p roles.Principal, id uuid.UUID, reason string) (*View, error) {
	reason = validation.CleanText(reason)
	errs := validation.Errors{}
	errs.Text("reason", reason, 3, 1000, true)
	var out *Record
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		r, rel, _, err := s.loadVisible(ctx, tx, p, id, true)
		if err != nil {
			return err
		}
		if !CanReopen(r.Status, actorKind(rel)) {
			if rel != RelationCoordinator {
				return apperr.Forbidden("Only coordinators can reopen requests.")
			}
			return apperr.Conflict("not_closed", "Only closed requests can be reopened.")
		}
		if r.RedactedAt != nil {
			return apperr.Conflict("redacted", "Redacted requests cannot be reopened.")
		}
		if err := errs.Err(); err != nil {
			return err
		}
		from := r.Status
		_, err = tx.Exec(ctx, `UPDATE aid_requests SET status = $2, review_status = 'in_review', closed_at = NULL,
			duplicate_of_request_id = NULL, version = version + 1, updated_at = now() WHERE id = $1`,
			r.ID, string(ReopenTarget))
		if err != nil {
			return err
		}
		ev := audit.ForActor(p, "request.reopened", audit.Shared).On("aid_request", r.ID).ForRequest(r.ID).
			Status(string(from), string(ReopenTarget)).WithReason(reason)
		if err := audit.Record(ctx, tx, ev); err != nil {
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
