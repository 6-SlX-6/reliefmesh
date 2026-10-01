package assignments

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/offers"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/requests"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
)

// Protected holds decrypted details needed to carry out an assignment.
type Protected struct {
	DestinationContact  *requests.ProtectedContact `json:"destination_contact,omitempty"`
	DestinationLocation *locations.ExactLocation   `json:"destination_location,omitempty"`
	PickupLocation      *locations.ExactLocation   `json:"pickup_location,omitempty"`
	Fields              []string                   `json:"fields"`
}

// RevealProtected returns the protected details granted for an assignment
// and records audit events on the request (and offer) timelines.
func (s *Service) RevealProtected(ctx context.Context, p roles.Principal, id uuid.UUID) (*Protected, error) {
	var out *Protected
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		a, rel, err := s.loadVisible(ctx, tx, p, id, false)
		if err != nil {
			return err
		}
		st, err := settings.Get(ctx, tx)
		if err != nil {
			return err
		}
		v, err := s.project(ctx, tx, p, a, st)
		if err != nil {
			return err
		}
		req, err := requests.Load(ctx, tx, a.RequestID, false)
		if err != nil {
			return err
		}
		fields := s.revealableFields(rel, a, v, req, st)
		if len(fields) == 0 {
			return apperr.Forbidden("No protected details have been shared with you for this assignment.")
		}
		res := &Protected{Fields: fields}
		var events []audit.Event
		var reqFields []string
		if slices.Contains(fields, "destination_contact") {
			pt, err := s.Sealer.Open(req.ContactSealed, requests.AAD(req.ID, "contact"))
			if err != nil {
				return err
			}
			res.DestinationContact = &requests.ProtectedContact{Method: req.ContactMethod, Details: string(pt)}
			reqFields = append(reqFields, "contact")
		}
		if slices.Contains(fields, "destination_location") {
			pt, err := s.Sealer.Open(req.ExactSealed, requests.AAD(req.ID, "exact_location"))
			if err != nil {
				return err
			}
			var ex locations.ExactLocation
			if err := json.Unmarshal(pt, &ex); err != nil {
				return err
			}
			res.DestinationLocation = &ex
			reqFields = append(reqFields, "exact_location")
		}
		if len(reqFields) > 0 {
			events = append(events, audit.ForActor(p, "request.protected_viewed", audit.Shared).
				On("aid_request", req.ID).ForRequest(req.ID).With("fields", reqFields).
				With("viewer_relation", rel).With("assignment_id", a.ID.String()))
		}
		if slices.Contains(fields, "pickup_location") && a.OfferID != nil {
			o, err := offers.Load(ctx, tx, *a.OfferID, false)
			if err != nil {
				return err
			}
			pt, err := s.Sealer.Open(o.ExactSealed, offers.AAD(o.ID, "exact_location"))
			if err != nil {
				return err
			}
			var ex locations.ExactLocation
			if err := json.Unmarshal(pt, &ex); err != nil {
				return err
			}
			res.PickupLocation = &ex
			events = append(events, audit.ForActor(p, "offer.protected_viewed", audit.Shared).On("offer", o.ID).
				With("fields", []string{"exact_location"}).With("viewer_relation", rel).With("assignment_id", a.ID.String()))
		}
		out = res
		return audit.Record(ctx, tx, events...)
	})
	if err != nil {
		return nil, wrap(err)
	}
	return out, nil
}
