package requests

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
)

// ProtectedContact is decrypted contact information.
type ProtectedContact struct {
	Method  string `json:"method"`
	Details string `json:"details"`
}

// Protected is the result of an audited reveal. Clients must not persist it
// (the web app keeps it in memory only).
type Protected struct {
	Contact       *ProtectedContact        `json:"contact,omitempty"`
	ExactLocation *locations.ExactLocation `json:"exact_location,omitempty"`
	Fields        []string                 `json:"fields"`
}

// RevealProtected decrypts the protected fields the viewer is entitled to and
// records an audit event naming which fields were revealed. Requesters are
// shown this event in their timeline, so access to their data is
// transparent.
func (s *Service) RevealProtected(ctx context.Context, p roles.Principal, id uuid.UUID) (*Protected, error) {
	var out *Protected
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		r, rel, acc, err := s.loadVisible(ctx, tx, p, id, false)
		if err != nil {
			return err
		}
		st, err := settings.Get(ctx, tx)
		if err != nil {
			return err
		}
		if !canRevealProtected(rel, r, acc, st) {
			return apperr.Forbidden("You are not authorized to view protected details of this request.")
		}
		res := &Protected{Fields: []string{}}
		allowContact, allowExact := true, true
		if rel == RelationAssignedVolunteer {
			allowContact = r.ContactVisibility == locations.ContactVisibilityAssignedResponders &&
				(acc.contactGrant || !st.VolunteerAccessRequiresGrant)
			allowExact = acc.destinationGrant || !st.VolunteerAccessRequiresGrant
		}
		if rel == RelationCoordinator && r.ContactVisibility == locations.ContactVisibilityNone {
			allowContact = false
		}
		if allowContact && len(r.ContactSealed) > 0 {
			pt, err := s.Sealer.Open(r.ContactSealed, AAD(r.ID, "contact"))
			if err != nil {
				return err
			}
			res.Contact = &ProtectedContact{Method: r.ContactMethod, Details: string(pt)}
			res.Fields = append(res.Fields, "contact")
		}
		if allowExact && len(r.ExactSealed) > 0 {
			pt, err := s.Sealer.Open(r.ExactSealed, AAD(r.ID, "exact_location"))
			if err != nil {
				return err
			}
			var ex locations.ExactLocation
			if err := json.Unmarshal(pt, &ex); err != nil {
				return err
			}
			res.ExactLocation = &ex
			res.Fields = append(res.Fields, "exact_location")
		}
		if len(res.Fields) == 0 {
			return apperr.NotFound("No protected details are available.")
		}
		out = res
		return audit.Record(ctx, tx, audit.ForActor(p, "request.protected_viewed", audit.Shared).
			On("aid_request", r.ID).ForRequest(r.ID).With("fields", res.Fields).With("viewer_relation", string(rel)))
	})
	if err != nil {
		return nil, wrap(err)
	}
	return out, nil
}
