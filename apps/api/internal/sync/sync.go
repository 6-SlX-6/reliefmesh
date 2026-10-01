// Package sync implements offline synchronization.
//
// Clients queue mutations locally (IndexedDB) while offline and push them in
// order when the server is reachable. Every operation carries a client
// generated op_id; results are stored so retries are idempotent. Entity
// creation additionally uses client ids so a create can never be applied
// twice, even across op ids.
//
// Conflict policy (never silently discard):
//   - creates and notes: always appended (no conflicts possible);
//   - status changes: intent-based; applied when the transition is still
//     valid from the current server state, otherwise reported as a conflict
//     with the current entity so the user can decide;
//   - field updates: optimistic concurrency via version; a mismatch is
//     reported as a conflict with the current entity;
//   - validation or permission failures are reported as rejected with the
//     reason. Clients keep rejected and conflicting operations until the user
//     explicitly resolves or discards them.
package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/assignments"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/notes"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/offers"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/requests"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
)

// MaxOperations limits a single push.
const MaxOperations = 200

// Result statuses.
const (
	StatusApplied   = "applied"
	StatusDuplicate = "duplicate"
	StatusConflict  = "conflict"
	StatusRejected  = "rejected"
	StatusRetry     = "retry"
)

// OperationTypes lists supported operation types.
var OperationTypes = []string{"request.create", "request.update", "request.status", "request.note",
	"offer.create", "offer.status", "offer.note", "assignment.status", "assignment.update"}

// Operation is a queued client mutation.
type Operation struct {
	OpID            uuid.UUID       `json:"op_id"`
	Type            string          `json:"type"`
	EntityID        *uuid.UUID      `json:"entity_id,omitempty"`
	EntityClientID  *uuid.UUID      `json:"entity_client_id,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	ClientCreatedAt *time.Time      `json:"client_created_at,omitempty"`
}

// PushInput is a batch of operations.
type PushInput struct {
	Operations []Operation `json:"operations"`
}

// OpResult reports the outcome of one operation.
type OpResult struct {
	OpID       uuid.UUID          `json:"op_id"`
	Status     string             `json:"status"`
	EntityType string             `json:"entity_type,omitempty"`
	EntityID   *uuid.UUID         `json:"entity_id,omitempty"`
	Reference  string             `json:"reference,omitempty"`
	Error      *httpx.ErrorDetail `json:"error,omitempty"`
	Entity     any                `json:"entity,omitempty"`
}

// PushResult is returned by push.
type PushResult struct {
	Results    []OpResult `json:"results"`
	ServerTime time.Time  `json:"server_time"`
}

// Service implements sync.
type Service struct {
	DB          *pgxpool.Pool
	Requests    *requests.Service
	Offers      *offers.Service
	Assignments *assignments.Service
	Notes       *notes.Service
}

func decodeStrict(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.BadRequest("invalid_payload", "Operation payload is invalid: "+strings.TrimPrefix(err.Error(), "json: "))
	}
	return nil
}

func (s *Service) resolveEntity(ctx context.Context, table string, op Operation) (uuid.UUID, error) {
	if op.EntityID != nil {
		return *op.EntityID, nil
	}
	if op.EntityClientID == nil {
		return uuid.Nil, apperr.ValidationField("entity_id", "entity_id or entity_client_id is required.")
	}
	var id uuid.UUID
	err := s.DB.QueryRow(ctx, `SELECT id FROM `+table+` WHERE client_id = $1`, *op.EntityClientID).Scan(&id)
	if database.IsNoRows(err) {
		return uuid.Nil, apperr.NotFound("The referenced record was not found. It may not have been synchronized yet.")
	}
	return id, err
}

// apply executes one operation and returns entity info.
func (s *Service) apply(ctx context.Context, p roles.Principal, op Operation) (string, *uuid.UUID, string, any, error) {
	switch op.Type {
	case "request.create":
		var in requests.CreateInput
		if err := decodeStrict(op.Payload, &in); err != nil {
			return "", nil, "", nil, err
		}
		if in.ClientID == nil {
			return "", nil, "", nil, apperr.ValidationField("client_id", "client_id is required for offline creation.")
		}
		if in.ClientCreatedAt == nil {
			in.ClientCreatedAt = op.ClientCreatedAt
		}
		v, _, err := s.Requests.Create(ctx, p, in)
		if err != nil {
			return "", nil, "", nil, err
		}
		return "aid_request", &v.ID, v.Reference, v, nil
	case "request.update", "request.status", "request.note":
		id, err := s.resolveEntity(ctx, "aid_requests", op)
		if err != nil {
			return "", nil, "", nil, err
		}
		switch op.Type {
		case "request.update":
			var in requests.UpdateInput
			if err := decodeStrict(op.Payload, &in); err != nil {
				return "", nil, "", nil, err
			}
			v, err := s.Requests.Update(ctx, p, id, in)
			if err != nil {
				return "", &id, "", nil, err
			}
			return "aid_request", &v.ID, v.Reference, v, nil
		case "request.status":
			var in requests.StatusInput
			if err := decodeStrict(op.Payload, &in); err != nil {
				return "", nil, "", nil, err
			}
			v, err := s.Requests.ChangeStatus(ctx, p, id, in)
			if err != nil {
				return "", &id, "", nil, err
			}
			return "aid_request", &v.ID, v.Reference, v, nil
		default:
			var in notes.Input
			if err := decodeStrict(op.Payload, &in); err != nil {
				return "", nil, "", nil, err
			}
			n, _, err := s.Notes.Create(ctx, p, "request", id, in)
			if err != nil {
				return "", &id, "", nil, err
			}
			return "note", &n.ID, "", n, nil
		}
	case "offer.create":
		var in offers.Input
		if err := decodeStrict(op.Payload, &in); err != nil {
			return "", nil, "", nil, err
		}
		if in.ClientID == nil {
			return "", nil, "", nil, apperr.ValidationField("client_id", "client_id is required for offline creation.")
		}
		if in.ClientCreatedAt == nil {
			in.ClientCreatedAt = op.ClientCreatedAt
		}
		v, _, err := s.Offers.Create(ctx, p, in)
		if err != nil {
			return "", nil, "", nil, err
		}
		return "offer", &v.ID, v.Reference, v, nil
	case "offer.status", "offer.note":
		id, err := s.resolveEntity(ctx, "offers", op)
		if err != nil {
			return "", nil, "", nil, err
		}
		if op.Type == "offer.status" {
			var in offers.StatusInput
			if err := decodeStrict(op.Payload, &in); err != nil {
				return "", nil, "", nil, err
			}
			v, err := s.Offers.ChangeStatus(ctx, p, id, in)
			if err != nil {
				return "", &id, "", nil, err
			}
			return "offer", &v.ID, v.Reference, v, nil
		}
		var in notes.Input
		if err := decodeStrict(op.Payload, &in); err != nil {
			return "", nil, "", nil, err
		}
		n, _, err := s.Notes.Create(ctx, p, "offer", id, in)
		if err != nil {
			return "", &id, "", nil, err
		}
		return "note", &n.ID, "", n, nil
	case "assignment.status", "assignment.update":
		id, err := s.resolveEntity(ctx, "assignments", op)
		if err != nil {
			return "", nil, "", nil, err
		}
		if op.Type == "assignment.status" {
			var in assignments.StatusInput
			if err := decodeStrict(op.Payload, &in); err != nil {
				return "", nil, "", nil, err
			}
			v, err := s.Assignments.ChangeStatus(ctx, p, id, in)
			if err != nil {
				return "", &id, "", nil, err
			}
			return "assignment", &v.ID, "", v, nil
		}
		var in assignments.UpdateInput
		if err := decodeStrict(op.Payload, &in); err != nil {
			return "", nil, "", nil, err
		}
		v, err := s.Assignments.Update(ctx, p, id, in)
		if err != nil {
			return "", &id, "", nil, err
		}
		return "assignment", &v.ID, "", v, nil
	}
	return "", nil, "", nil, apperr.BadRequest("unknown_operation", fmt.Sprintf("Unknown operation type %q.", op.Type))
}

func (s *Service) currentEntity(ctx context.Context, p roles.Principal, entityType string, id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	var v any
	var err error
	switch entityType {
	case "aid_request":
		v, err = s.Requests.Get(ctx, p, *id)
	case "offer":
		v, err = s.Offers.Get(ctx, p, *id)
	case "assignment":
		v, err = s.Assignments.Get(ctx, p, *id)
	}
	if err != nil {
		return nil
	}
	return v
}

func entityTypeFor(opType string) string {
	switch {
	case strings.HasPrefix(opType, "request."):
		return "aid_request"
	case strings.HasPrefix(opType, "offer."):
		return "offer"
	case strings.HasPrefix(opType, "assignment."):
		return "assignment"
	}
	return ""
}

type stored struct {
	Status     string             `json:"status"`
	EntityType string             `json:"entity_type,omitempty"`
	EntityID   *uuid.UUID         `json:"entity_id,omitempty"`
	Reference  string             `json:"reference,omitempty"`
	Error      *httpx.ErrorDetail `json:"error,omitempty"`
}

// Push applies operations in order.
func (s *Service) Push(ctx context.Context, p roles.Principal, in PushInput) (*PushResult, error) {
	if len(in.Operations) > MaxOperations {
		return nil, apperr.BadRequest("too_many_operations", fmt.Sprintf("Push at most %d operations at once.", MaxOperations))
	}
	res := &PushResult{Results: make([]OpResult, 0, len(in.Operations))}
	seen := map[uuid.UUID]bool{}
	for _, op := range in.Operations {
		r := OpResult{OpID: op.OpID}
		if op.OpID == uuid.Nil || seen[op.OpID] {
			r.Status = StatusRejected
			r.Error = &httpx.ErrorDetail{Code: "invalid_op_id", Message: "Each operation needs a unique op_id."}
			res.Results = append(res.Results, r)
			continue
		}
		seen[op.OpID] = true

		var owner uuid.UUID
		var raw []byte
		err := s.DB.QueryRow(ctx, `SELECT user_id, result FROM sync_operations WHERE op_id = $1`, op.OpID).Scan(&owner, &raw)
		if err == nil {
			if owner != p.UserID {
				r.Status = StatusRejected
				r.Error = &httpx.ErrorDetail{Code: "invalid_op_id", Message: "This operation id belongs to another account."}
			} else {
				var st stored
				_ = json.Unmarshal(raw, &st)
				r.Status = StatusDuplicate
				r.EntityType, r.EntityID, r.Reference, r.Error = st.EntityType, st.EntityID, st.Reference, st.Error
				if st.Status != StatusApplied {
					r.Status = st.Status
				}
				r.Entity = s.currentEntity(ctx, p, r.EntityType, r.EntityID)
			}
			res.Results = append(res.Results, r)
			continue
		} else if !database.IsNoRows(err) {
			return nil, apperr.Internal(err)
		}

		entityType, entityID, ref, entity, err := s.apply(ctx, p, op)
		if err != nil {
			ae, ok := apperr.As(err)
			if !ok || ae.Kind == apperr.KindInternal || ae.Kind == apperr.KindUnavailable {
				slog.ErrorContext(ctx, "sync operation failed", "op_type", op.Type, "error", err)
				r.Status = StatusRetry
				r.Error = &httpx.ErrorDetail{Code: "retry_later", Message: "The server could not process this change. It will be retried."}
				res.Results = append(res.Results, r)
				continue
			}
			r.Error = &httpx.ErrorDetail{Code: ae.Code, Message: ae.Message, Fields: ae.Fields, Details: ae.Details}
			r.Status = StatusRejected
			if ae.Kind == apperr.KindConflict {
				r.Status = StatusConflict
			}
			r.EntityType = entityTypeFor(op.Type)
			r.EntityID = entityID
			r.Entity = s.currentEntity(ctx, p, r.EntityType, entityID)
		} else {
			r.Status, r.EntityType, r.EntityID, r.Reference, r.Entity = StatusApplied, entityType, entityID, ref, entity
		}
		st := stored{Status: r.Status, EntityType: r.EntityType, EntityID: r.EntityID, Reference: r.Reference, Error: r.Error}
		b, _ := json.Marshal(st)
		if _, err := s.DB.Exec(ctx, `INSERT INTO sync_operations (op_id, user_id, op_type, status, result)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (op_id) DO NOTHING`, op.OpID, p.UserID, op.Type, r.Status, b); err != nil {
			return nil, apperr.Internal(err)
		}
		res.Results = append(res.Results, r)
	}
	if err := s.DB.QueryRow(ctx, `SELECT now()`).Scan(&res.ServerTime); err != nil {
		return nil, apperr.Internal(err)
	}
	return res, nil
}

// PullResult carries everything a client caches for offline use.
type PullResult struct {
	ServerTime  time.Time                `json:"server_time"`
	Full        bool                     `json:"full"`
	Settings    *settings.ClientSettings `json:"settings"`
	Requests    []*requests.View         `json:"requests"`
	Offers      []*offers.View           `json:"offers"`
	Assignments []*assignments.View      `json:"assignments"`
	HasMore     bool                     `json:"has_more"`
}

// overlap compensates for transactions that commit after the previous pull
// started; clients upsert idempotently, so duplicates are harmless.
const overlap = 10 * time.Second

// Pull returns entities visible to the principal changed since the cursor
// (or everything visible when since is nil).
func (s *Service) Pull(ctx context.Context, p roles.Principal, since *time.Time) (*PullResult, error) {
	res := &PullResult{Full: since == nil, Requests: []*requests.View{}, Offers: []*offers.View{},
		Assignments: []*assignments.View{}}
	if err := s.DB.QueryRow(ctx, `SELECT now()`).Scan(&res.ServerTime); err != nil {
		return nil, apperr.Internal(err)
	}
	var updatedSince *time.Time
	if since != nil {
		t := since.Add(-overlap)
		updatedSince = &t
	}
	st, err := settings.Get(ctx, s.DB)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	cats, err := settings.ListCategories(ctx, s.DB, false)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	res.Settings = &settings.ClientSettings{Settings: st, Categories: cats}

	ignoreForbidden := func(err error) error {
		if apperr.IsKind(err, apperr.KindForbidden) {
			return nil
		}
		return err
	}
	if rl, err := s.Requests.List(ctx, p, requests.ListFilter{UpdatedSince: updatedSince, Sort: "updated", Limit: 500}); err == nil {
		res.Requests, res.HasMore = rl.Items, res.HasMore || rl.HasMore
	} else if err := ignoreForbidden(err); err != nil {
		return nil, err
	}
	if ol, err := s.Offers.List(ctx, p, offers.ListFilter{UpdatedSince: updatedSince, Limit: 500}); err == nil {
		res.Offers, res.HasMore = ol.Items, res.HasMore || ol.HasMore
	} else if err := ignoreForbidden(err); err != nil {
		return nil, err
	}
	if al, err := s.Assignments.List(ctx, p, assignments.ListFilter{UpdatedSince: updatedSince, Limit: 500}); err == nil {
		res.Assignments, res.HasMore = al.Items, res.HasMore || al.HasMore
	} else if err := ignoreForbidden(err); err != nil {
		return nil, err
	}
	return res, nil
}

// HandlePush handles POST /api/v1/sync/push.
func (s *Service) HandlePush(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var in PushInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := s.Push(r.Context(), p, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// HandlePull handles GET /api/v1/sync/pull.
func (s *Service) HandlePull(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var since *time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			httpx.Error(w, r, apperr.ValidationField("since", "Must be an RFC 3339 timestamp."))
			return
		}
		since = &t
	}
	res, err := s.Pull(r.Context(), p, since)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// CleanupOld removes stored operation results older than maxAge.
func (s *Service) CleanupOld(ctx context.Context, maxAge time.Duration) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM sync_operations WHERE received_at < $1`, time.Now().Add(-maxAge))
	return err
}
