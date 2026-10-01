package requests

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

// TimelineActor describes who performed an event, at the level of detail
// appropriate for the viewer.
type TimelineActor struct {
	Label       string     `json:"label"`
	DisplayName string     `json:"display_name,omitempty"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	IsYou       bool       `json:"is_you"`
}

// TimelineEvent is an audit event as shown in a request timeline.
type TimelineEvent struct {
	ID         int64          `json:"id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Action     string         `json:"action"`
	EntityType string         `json:"entity_type"`
	EntityID   *uuid.UUID     `json:"entity_id,omitempty"`
	FromStatus string         `json:"from_status,omitempty"`
	ToStatus   string         `json:"to_status,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	Metadata   map[string]any `json:"metadata"`
	Actor      TimelineActor  `json:"actor"`
}

// metadata keys a requester may see on shared events.
var ownerMetadataKeys = []string{"category", "urgency", "changed_fields", "fields", "from", "to", "created_offline"}

func roleLabel(rs []string) string {
	switch {
	case slices.Contains(rs, "system"):
		return "System"
	case slices.Contains(rs, "coordinator") || slices.Contains(rs, "organization_manager"):
		return "Coordinator"
	case slices.Contains(rs, "volunteer"):
		return "Volunteer"
	case slices.Contains(rs, "requester"):
		return "Requester"
	case slices.Contains(rs, "admin"):
		return "Administrator"
	}
	return "Unknown"
}

// Timeline returns the immutable event history of a request, filtered by
// what the viewer may see.
func (s *Service) Timeline(ctx context.Context, p roles.Principal, id uuid.UUID) ([]TimelineEvent, error) {
	r, rel, _, err := s.loadVisible(ctx, s.DB, p, id, false)
	if err != nil {
		return nil, err
	}
	var vis []audit.Visibility
	switch rel {
	case RelationCoordinator:
		vis = []audit.Visibility{audit.Shared, audit.Operational, audit.Internal}
	case RelationAssignedVolunteer:
		vis = []audit.Visibility{audit.Shared, audit.Operational}
	default:
		vis = []audit.Visibility{audit.Shared}
	}
	events, err := audit.Timeline(ctx, s.DB, audit.TimelineFilter{RequestID: &r.ID, Visibilities: vis})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	names := map[uuid.UUID]string{}
	if rel == RelationCoordinator {
		var ids []uuid.UUID
		for _, e := range events {
			if e.ActorUserID != nil {
				ids = append(ids, *e.ActorUserID)
			}
		}
		if len(ids) > 0 {
			rows, err := s.DB.Query(ctx, `SELECT id, display_name FROM users WHERE id = ANY($1)`, ids)
			if err != nil {
				return nil, apperr.Internal(err)
			}
			for rows.Next() {
				var uid uuid.UUID
				var name string
				if err := rows.Scan(&uid, &name); err == nil {
					names[uid] = name
				}
			}
			rows.Close()
		}
	}
	out := make([]TimelineEvent, 0, len(events))
	for _, e := range events {
		te := TimelineEvent{ID: e.ID, OccurredAt: e.OccurredAt, Action: e.Action, EntityType: e.EntityType,
			EntityID: e.EntityID, FromStatus: e.FromStatus, ToStatus: e.ToStatus, Reason: e.Reason,
			Metadata: e.Metadata, Actor: TimelineActor{Label: roleLabel(e.ActorRoles)}}
		if e.ActorUserID != nil && *e.ActorUserID == p.UserID {
			te.Actor.IsYou = true
			te.Actor.Label = "You"
		}
		switch rel {
		case RelationCoordinator:
			if e.ActorUserID != nil {
				te.Actor.UserID = e.ActorUserID
				te.Actor.DisplayName = names[*e.ActorUserID]
			}
		case RelationOwner:
			filtered := map[string]any{}
			for _, k := range ownerMetadataKeys {
				if v, ok := e.Metadata[k]; ok {
					filtered[k] = v
				}
			}
			te.Metadata = filtered
			te.EntityID = nil
		case RelationAssignedVolunteer:
			// Volunteers do not see who else touched the request.
		}
		out = append(out, te)
	}
	return out, nil
}
