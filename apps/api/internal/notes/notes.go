// Package notes implements append-only notes on requests and offers.
//
// Visibility:
//
//	internal   – coordinators only
//	responders – coordinators and assigned volunteers (handover notes)
//	shared     – additionally the requester / offer owner
//
// Notes are never edited. Retention and deletion redact the body.
package notes

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
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/offers"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/requests"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Visibilities lists valid note visibilities.
var Visibilities = []string{"internal", "responders", "shared"}

// Note is a note as returned by the API.
type Note struct {
	ID          uuid.UUID  `json:"id"`
	ClientID    *uuid.UUID `json:"client_id,omitempty"`
	Visibility  string     `json:"visibility"`
	IsSensitive bool       `json:"is_sensitive"`
	Body        string     `json:"body"`
	CreatedAt   time.Time  `json:"created_at"`
	Redacted    bool       `json:"redacted"`
	Author      Author     `json:"author"`
}

// Author describes the note author at the detail level allowed for the
// viewer.
type Author struct {
	Label       string     `json:"label"`
	DisplayName string     `json:"display_name,omitempty"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	IsYou       bool       `json:"is_you"`
}

// Input creates a note.
type Input struct {
	ClientID    *uuid.UUID `json:"client_id"`
	Body        string     `json:"body"`
	Visibility  string     `json:"visibility"`
	IsSensitive bool       `json:"is_sensitive"`
}

// Service implements notes.
type Service struct {
	DB       *pgxpool.Pool
	Requests *requests.Service
	Offers   *offers.Service
}

type parent struct {
	kind      string // "request" or "offer"
	id        uuid.UUID
	orgID     uuid.UUID
	relation  string
	allowed   []string
	sensitive bool // notes on this parent are always sensitive
}

func (s *Service) resolve(ctx context.Context, p roles.Principal, kind string, id uuid.UUID) (*parent, error) {
	switch kind {
	case "request":
		v, err := s.Requests.Get(ctx, p, id)
		if err != nil {
			return nil, err
		}
		r, err := requests.Load(ctx, s.DB, id, false)
		if err != nil || r == nil {
			return nil, apperr.NotFound("Request not found.")
		}
		return &parent{kind: kind, id: id, orgID: r.OrgID, relation: string(v.ViewerRelation),
			allowed: v.Permissions.NoteVisibilities, sensitive: r.Category == requests.CategoryMedicinePickup || r.Sensitive}, nil
	case "offer":
		v, err := s.Offers.Get(ctx, p, id)
		if err != nil {
			return nil, err
		}
		o, err := offers.Load(ctx, s.DB, id, false)
		if err != nil || o == nil {
			return nil, apperr.NotFound("Offer not found.")
		}
		return &parent{kind: kind, id: id, orgID: o.OrgID, relation: string(v.ViewerRelation),
			allowed: v.Permissions.NoteVisibilities}, nil
	}
	return nil, apperr.NotFound("Not found.")
}

func visibleFor(relation string) []string {
	switch relation {
	case "coordinator":
		return Visibilities
	case "assigned_volunteer":
		return []string{"responders", "shared"}
	default:
		return []string{"shared"}
	}
}

func roleLabel(rs []string) string {
	switch {
	case slices.Contains(rs, "coordinator") || slices.Contains(rs, "organization_manager"):
		return "Coordinator"
	case slices.Contains(rs, "volunteer"):
		return "Volunteer"
	case slices.Contains(rs, "requester"):
		return "Requester"
	}
	return "User"
}

// Create adds a note.
func (s *Service) Create(ctx context.Context, p roles.Principal, kind string, parentID uuid.UUID, in Input) (*Note, bool, error) {
	par, err := s.resolve(ctx, p, kind, parentID)
	if err != nil {
		return nil, false, err
	}
	if in.ClientID != nil {
		var existingID uuid.UUID
		var author *uuid.UUID
		err := s.DB.QueryRow(ctx, `SELECT id, author_user_id FROM notes WHERE client_id = $1`, *in.ClientID).Scan(&existingID, &author)
		if err == nil {
			if author == nil || *author != p.UserID {
				return nil, false, apperr.Conflict("client_id_conflict", "This client id is already in use.")
			}
			n, err := s.get(ctx, p, par, existingID)
			return n, false, err
		} else if !database.IsNoRows(err) {
			return nil, false, apperr.Internal(err)
		}
	}
	if in.Visibility == "" && len(par.allowed) > 0 {
		in.Visibility = par.allowed[0]
		if slices.Contains(par.allowed, "shared") && par.relation != "coordinator" {
			in.Visibility = "shared"
		}
	}
	body := validation.CleanText(in.Body)
	errs := validation.Errors{}
	errs.Text("body", body, 1, 4000, true)
	errs.OneOf("visibility", in.Visibility, Visibilities)
	if err := errs.Err(); err != nil {
		return nil, false, err
	}
	if !slices.Contains(par.allowed, in.Visibility) {
		return nil, false, apperr.Forbidden("You cannot add a note with this visibility.")
	}
	sensitive := in.IsSensitive || par.sensitive
	id := uuid.New()
	var reqID, offerID *uuid.UUID
	if kind == "request" {
		reqID = &par.id
	} else {
		offerID = &par.id
	}
	err = database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO notes (id, organization_id, client_id, request_id, offer_id, author_user_id,
			visibility, is_sensitive, body) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			id, par.orgID, in.ClientID, reqID, offerID, p.UserID, in.Visibility, sensitive, body)
		if err != nil {
			if database.IsUniqueViolation(err, "notes_client_id_key") {
				return apperr.Conflict("client_id_conflict", "This note was already submitted.")
			}
			return err
		}
		vis := audit.Internal
		switch in.Visibility {
		case "shared":
			vis = audit.Shared
		case "responders":
			vis = audit.Operational
		}
		entity := "aid_request"
		if kind == "offer" {
			entity = "offer"
		}
		ev := audit.ForActor(p, "note.added", vis).On(entity, par.id).With("note_id", id.String()).
			With("visibility", in.Visibility)
		if reqID != nil {
			ev = ev.ForRequest(*reqID)
		}
		return audit.Record(ctx, tx, ev)
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return nil, false, err
		}
		return nil, false, apperr.Internal(err)
	}
	n, err := s.get(ctx, p, par, id)
	return n, true, err
}

const noteSelect = `SELECT n.id, n.client_id, n.visibility, n.is_sensitive, n.body, n.created_at, n.redacted_at,
	n.author_user_id, u.display_name, u.roles FROM notes n LEFT JOIN users u ON u.id = n.author_user_id`

func (s *Service) scanNotes(rows pgx.Rows, p roles.Principal, relation string) ([]Note, error) {
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		var redacted *time.Time
		var author *uuid.UUID
		var name *string
		var authorRoles []string
		if err := rows.Scan(&n.ID, &n.ClientID, &n.Visibility, &n.IsSensitive, &n.Body, &n.CreatedAt, &redacted,
			&author, &name, &authorRoles); err != nil {
			return nil, err
		}
		n.Redacted = redacted != nil
		n.Author.Label = roleLabel(authorRoles)
		if author != nil && *author == p.UserID {
			n.Author.IsYou, n.Author.Label = true, "You"
		} else {
			n.ClientID = nil
		}
		if relation == "coordinator" && author != nil {
			n.Author.UserID = author
			if name != nil {
				n.Author.DisplayName = *name
			}
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Service) get(ctx context.Context, p roles.Principal, par *parent, id uuid.UUID) (*Note, error) {
	rows, err := s.DB.Query(ctx, noteSelect+` WHERE n.id = $1`, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	list, err := s.scanNotes(rows, p, par.relation)
	if err != nil || len(list) == 0 {
		return nil, apperr.Internal(err)
	}
	return &list[0], nil
}

// List returns notes visible to the viewer.
func (s *Service) List(ctx context.Context, p roles.Principal, kind string, parentID uuid.UUID) ([]Note, error) {
	par, err := s.resolve(ctx, p, kind, parentID)
	if err != nil {
		return nil, err
	}
	col := "n.request_id"
	if kind == "offer" {
		col = "n.offer_id"
	}
	rows, err := s.DB.Query(ctx, noteSelect+` WHERE `+col+` = $1 AND n.visibility = ANY($2) ORDER BY n.created_at, n.id`,
		parentID, visibleFor(par.relation))
	if err != nil {
		return nil, apperr.Internal(err)
	}
	notes, err := s.scanNotes(rows, p, par.relation)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return notes, nil
}

// Routes returns handlers for /requests/{id}/notes and /offers/{id}/notes.
func (s *Service) Routes(kind string) func(r chi.Router) {
	return func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			id, err := httpx.UUIDParam(req, "id")
			if err != nil {
				httpx.Error(w, req, err)
				return
			}
			p, _ := roles.FromContext(req.Context())
			list, err := s.List(req.Context(), p, kind, id)
			if err != nil {
				httpx.Error(w, req, err)
				return
			}
			httpx.JSON(w, http.StatusOK, map[string]any{"items": list})
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			id, err := httpx.UUIDParam(req, "id")
			if err != nil {
				httpx.Error(w, req, err)
				return
			}
			var in Input
			if err := httpx.Decode(req, &in); err != nil {
				httpx.Error(w, req, err)
				return
			}
			p, _ := roles.FromContext(req.Context())
			n, created, err := s.Create(req.Context(), p, kind, id, in)
			if err != nil {
				httpx.Error(w, req, err)
				return
			}
			status := http.StatusCreated
			if !created {
				status = http.StatusOK
			}
			httpx.JSON(w, status, n)
		})
	}
}
