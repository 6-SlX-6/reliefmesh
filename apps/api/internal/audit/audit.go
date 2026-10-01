// Package audit records immutable, hash-chained audit events.
//
// Rules:
//   - Events are only ever inserted. The database rejects UPDATE, DELETE and
//     TRUNCATE on audit_events (see migration 0001).
//   - Every event stores prev_hash and hash = SHA-256(prev_hash || canonical
//     event). Event ids are gapless. Verify() recomputes the chain so storage
//     level tampering (including deleted rows) is detected.
//   - Metadata must never contain personal data or free-text content of
//     requests, offers or notes. Use ids, enums and quantities only.
//   - Record must be called inside a transaction, as the last statement(s)
//     of that transaction, to keep the chain lock short and avoid lock-order
//     inversions with row locks.
package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

// Visibility controls who may see an event in timelines.
type Visibility string

const (
	// Shared events are visible to the request owner, assigned responders and
	// coordinators (e.g. status changes). Actor identities are shown to the
	// owner only as role labels.
	Shared Visibility = "shared"
	// Operational events are visible to coordinators and assigned responders.
	Operational Visibility = "operational"
	// Internal events are visible to coordinators only.
	Internal Visibility = "internal"
	// System events (logins, user management, settings) are visible to
	// administrators only.
	System Visibility = "system"
)

// chainLockID serializes appends to the hash chain.
const chainLockID = 727274002

// Event is an audit event to record.
type Event struct {
	OrgID       *uuid.UUID
	ActorUserID *uuid.UUID
	ActorRoles  []string
	Action      string
	EntityType  string
	EntityID    *uuid.UUID
	RequestID   *uuid.UUID
	FromStatus  string
	ToStatus    string
	Reason      string
	Metadata    map[string]any
	Visibility  Visibility
}

// Stored is an audit event as persisted.
type Stored struct {
	ID          int64          `json:"id"`
	OccurredAt  time.Time      `json:"occurred_at"`
	OrgID       *uuid.UUID     `json:"organization_id,omitempty"`
	ActorUserID *uuid.UUID     `json:"actor_user_id,omitempty"`
	ActorRoles  []string       `json:"actor_roles"`
	Action      string         `json:"action"`
	EntityType  string         `json:"entity_type"`
	EntityID    *uuid.UUID     `json:"entity_id,omitempty"`
	RequestID   *uuid.UUID     `json:"request_id,omitempty"`
	FromStatus  string         `json:"from_status,omitempty"`
	ToStatus    string         `json:"to_status,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	Visibility  Visibility     `json:"visibility"`
	PrevHash    []byte         `json:"-"`
	Hash        []byte         `json:"-"`
}

// ForActor returns an Event pre-filled with actor information.
func ForActor(p roles.Principal, action string, vis Visibility) Event {
	e := Event{Action: action, Visibility: vis, Metadata: map[string]any{}}
	if p.OrgID != uuid.Nil {
		org := p.OrgID
		e.OrgID = &org
	}
	if p.UserID != uuid.Nil {
		uid := p.UserID
		e.ActorUserID = &uid
	}
	e.ActorRoles = p.RoleStrings()
	if p.System {
		e.ActorRoles = append(e.ActorRoles, "system")
	}
	return e
}

// On sets the entity of the event.
func (e Event) On(entityType string, id uuid.UUID) Event {
	e.EntityType = entityType
	e.EntityID = &id
	return e
}

// ForRequest links the event to a request timeline.
func (e Event) ForRequest(id uuid.UUID) Event {
	e.RequestID = &id
	return e
}

// Status sets from/to status.
func (e Event) Status(from, to string) Event {
	e.FromStatus, e.ToStatus = from, to
	return e
}

// WithReason sets the reason.
func (e Event) WithReason(r string) Event {
	e.Reason = r
	return e
}

// With adds a metadata entry.
func (e Event) With(key string, value any) Event {
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	e.Metadata[key] = value
	return e
}

// canonicalMetadata produces a deterministic JSON encoding (object keys are
// sorted by encoding/json; numbers normalized via a decode/encode round trip).
func canonicalMetadata(m map[string]any) ([]byte, error) {
	if m == nil {
		m = map[string]any{}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var norm map[string]any
	if err := json.Unmarshal(raw, &norm); err != nil {
		return nil, err
	}
	if norm == nil {
		norm = map[string]any{}
	}
	return json.Marshal(norm)
}

type canonicalEvent struct {
	ID          int64           `json:"id"`
	OccurredAt  string          `json:"occurred_at"`
	OrgID       string          `json:"organization_id"`
	ActorUserID string          `json:"actor_user_id"`
	ActorRoles  []string        `json:"actor_roles"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id"`
	RequestID   string          `json:"request_id"`
	FromStatus  string          `json:"from_status"`
	ToStatus    string          `json:"to_status"`
	Reason      string          `json:"reason"`
	Metadata    json.RawMessage `json:"metadata"`
	Visibility  string          `json:"visibility"`
}

func uuidStr(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func computeHash(prev []byte, s *Stored) ([]byte, error) {
	meta, err := canonicalMetadata(s.Metadata)
	if err != nil {
		return nil, err
	}
	actorRoles := s.ActorRoles
	if actorRoles == nil {
		actorRoles = []string{}
	}
	ce := canonicalEvent{
		ID:          s.ID,
		OccurredAt:  s.OccurredAt.UTC().Format(time.RFC3339Nano),
		OrgID:       uuidStr(s.OrgID),
		ActorUserID: uuidStr(s.ActorUserID),
		ActorRoles:  actorRoles,
		Action:      s.Action,
		EntityType:  s.EntityType,
		EntityID:    uuidStr(s.EntityID),
		RequestID:   uuidStr(s.RequestID),
		FromStatus:  s.FromStatus,
		ToStatus:    s.ToStatus,
		Reason:      s.Reason,
		Metadata:    meta,
		Visibility:  string(s.Visibility),
	}
	b, err := json.Marshal(ce)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write(prev)
	h.Write(b)
	return h.Sum(nil), nil
}

// genesis is the prev_hash of the very first event.
var genesis = make([]byte, sha256.Size)

// Record appends events to the audit chain inside tx.
func Record(ctx context.Context, tx pgx.Tx, events ...Event) error {
	if len(events) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, chainLockID); err != nil {
		return fmt.Errorf("audit lock: %w", err)
	}
	var lastID int64
	var prev []byte
	err := tx.QueryRow(ctx, `SELECT id, hash FROM audit_events ORDER BY id DESC LIMIT 1`).Scan(&lastID, &prev)
	if database.IsNoRows(err) {
		lastID, prev = 0, genesis
	} else if err != nil {
		return fmt.Errorf("audit chain head: %w", err)
	}
	// Truncate to microseconds: PostgreSQL timestamptz precision.
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, e := range events {
		if e.Visibility == "" {
			return fmt.Errorf("audit event %q has no visibility", e.Action)
		}
		lastID++
		s := &Stored{
			ID: lastID, OccurredAt: now, OrgID: e.OrgID, ActorUserID: e.ActorUserID,
			ActorRoles: e.ActorRoles, Action: e.Action, EntityType: e.EntityType,
			EntityID: e.EntityID, RequestID: e.RequestID, FromStatus: e.FromStatus,
			ToStatus: e.ToStatus, Reason: e.Reason, Metadata: e.Metadata, Visibility: e.Visibility,
		}
		if s.ActorRoles == nil {
			s.ActorRoles = []string{}
		}
		meta, err := canonicalMetadata(e.Metadata)
		if err != nil {
			return fmt.Errorf("audit metadata: %w", err)
		}
		hash, err := computeHash(prev, s)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events
			(id, occurred_at, organization_id, actor_user_id, actor_roles, action, entity_type,
			 entity_id, request_id, from_status, to_status, reason, metadata, visibility, prev_hash, hash)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
			s.ID, s.OccurredAt, s.OrgID, s.ActorUserID, s.ActorRoles, s.Action, s.EntityType,
			s.EntityID, s.RequestID, s.FromStatus, s.ToStatus, s.Reason, meta, string(s.Visibility), prev, hash)
		if err != nil {
			return fmt.Errorf("insert audit event: %w", err)
		}
		prev = hash
	}
	return nil
}

const selectColumns = `id, occurred_at, organization_id, actor_user_id, actor_roles, action, entity_type,
	entity_id, request_id, from_status, to_status, reason, metadata, visibility, prev_hash, hash`

func scanStored(rows pgx.Rows) (*Stored, error) {
	var s Stored
	var meta []byte
	var vis string
	if err := rows.Scan(&s.ID, &s.OccurredAt, &s.OrgID, &s.ActorUserID, &s.ActorRoles, &s.Action,
		&s.EntityType, &s.EntityID, &s.RequestID, &s.FromStatus, &s.ToStatus, &s.Reason, &meta,
		&vis, &s.PrevHash, &s.Hash); err != nil {
		return nil, err
	}
	s.Visibility = Visibility(vis)
	if err := json.Unmarshal(meta, &s.Metadata); err != nil {
		return nil, err
	}
	if s.Metadata == nil {
		s.Metadata = map[string]any{}
	}
	return &s, nil
}

// TimelineFilter restricts which events of a request timeline are returned.
type TimelineFilter struct {
	RequestID    *uuid.UUID
	EntityType   string
	EntityID     *uuid.UUID
	Visibilities []Visibility
}

// Timeline lists events for a request (including assignment events linked to
// it) or for a single entity, restricted to the given visibilities.
func Timeline(ctx context.Context, q database.Querier, f TimelineFilter) ([]*Stored, error) {
	vis := make([]string, len(f.Visibilities))
	for i, v := range f.Visibilities {
		vis[i] = string(v)
	}
	var rows pgx.Rows
	var err error
	if f.RequestID != nil {
		rows, err = q.Query(ctx, `SELECT `+selectColumns+` FROM audit_events
			WHERE (request_id = $1 OR (entity_type = 'aid_request' AND entity_id = $1))
			  AND visibility = ANY($2) ORDER BY id LIMIT 1000`, *f.RequestID, vis)
	} else {
		rows, err = q.Query(ctx, `SELECT `+selectColumns+` FROM audit_events
			WHERE entity_type = $1 AND entity_id = $2 AND visibility = ANY($3) ORDER BY id LIMIT 1000`,
			f.EntityType, f.EntityID, vis)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Stored
	for rows.Next() {
		s, err := scanStored(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListFilter filters the system audit log.
type ListFilter struct {
	Action     string
	EntityType string
	BeforeID   int64
	Limit      int
}

// List returns audit events (newest first) for administrators.
func List(ctx context.Context, q database.Querier, f ListFilter) ([]*Stored, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	before := f.BeforeID
	if before <= 0 {
		before = 1<<62 - 1
	}
	rows, err := q.Query(ctx, `SELECT `+selectColumns+` FROM audit_events
		WHERE id < $1 AND ($2 = '' OR action = $2) AND ($3 = '' OR entity_type = $3)
		ORDER BY id DESC LIMIT $4`, before, f.Action, f.EntityType, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Stored
	for rows.Next() {
		s, err := scanStored(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// VerifyResult reports the outcome of a chain verification.
type VerifyResult struct {
	Valid         bool   `json:"valid"`
	EventsChecked int64  `json:"events_checked"`
	FirstBrokenID int64  `json:"first_broken_id,omitempty"`
	Problem       string `json:"problem,omitempty"`
}

// Verify recomputes the full hash chain.
func Verify(ctx context.Context, q database.Querier) (VerifyResult, error) {
	rows, err := q.Query(ctx, `SELECT `+selectColumns+` FROM audit_events ORDER BY id`)
	if err != nil {
		return VerifyResult{}, err
	}
	defer rows.Close()
	res := VerifyResult{Valid: true}
	prev := genesis
	var expectedID int64 = 1
	for rows.Next() {
		s, err := scanStored(rows)
		if err != nil {
			return VerifyResult{}, err
		}
		res.EventsChecked++
		if !res.Valid {
			continue
		}
		switch {
		case s.ID != expectedID:
			res.Valid, res.FirstBrokenID, res.Problem = false, s.ID, fmt.Sprintf("gap in event ids: expected %d", expectedID)
		case !bytes.Equal(s.PrevHash, prev):
			res.Valid, res.FirstBrokenID, res.Problem = false, s.ID, "prev_hash does not match previous event"
		default:
			h, err := computeHash(prev, s)
			if err != nil {
				return VerifyResult{}, err
			}
			if !bytes.Equal(h, s.Hash) {
				res.Valid, res.FirstBrokenID, res.Problem = false, s.ID, "event content does not match its hash"
			}
		}
		prev = s.Hash
		expectedID = s.ID + 1
	}
	return res, rows.Err()
}
