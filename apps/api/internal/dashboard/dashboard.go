// Package dashboard provides the operational overview.
//
// Everything here is a manual-review aid: counts and queues. Nothing is
// ranked by demographic or health data, and nothing is assigned or
// prioritized automatically. Administrators without a coordination role
// receive aggregate counts only (no titles or references).
package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

// QueueItem is a request in a dashboard queue.
type QueueItem struct {
	ID              uuid.UUID  `json:"id"`
	Reference       string     `json:"reference"`
	Title           string     `json:"title"`
	Category        string     `json:"category"`
	Urgency         string     `json:"urgency"`
	Status          string     `json:"status"`
	AreaLabel       string     `json:"area_label"`
	RequestedByTime *time.Time `json:"requested_by_time"`
	CreatedAt       time.Time  `json:"created_at"`
	Sensitive       bool       `json:"sensitive_data_flag"`
}

// Overview is the dashboard payload.
type Overview struct {
	GeneratedAt time.Time `json:"generated_at"`
	Requests    struct {
		Open                 int            `json:"open"`
		ByStatus             map[string]int `json:"by_status"`
		OpenByUrgency        map[string]int `json:"open_by_urgency"`
		OpenByCategory       map[string]int `json:"open_by_category"`
		NeedsReview          int            `json:"needs_review"`
		Overdue              int            `json:"overdue"`
		PastExpiry           int            `json:"past_expiry"`
		AwaitingConfirmation int            `json:"awaiting_confirmation"`
		ResolvedLast24h      int            `json:"resolved_last_24h"`
	} `json:"requests"`
	Offers struct {
		Active              int            `json:"active"`
		RemainingByCategory map[string]int `json:"remaining_by_category"`
		FullyAllocated      int            `json:"fully_allocated"`
	} `json:"offers"`
	Assignments struct {
		ByStatus map[string]int `json:"by_status"`
		Active   int            `json:"active"`
	} `json:"assignments"`
	Volunteers struct {
		Available   int `json:"available"`
		Limited     int `json:"limited"`
		Unavailable int `json:"unavailable"`
	} `json:"volunteers"`
	Queues *Queues `json:"queues,omitempty"`
}

// Queues are lists for coordinators.
type Queues struct {
	NeedsReview          []QueueItem `json:"needs_review"`
	UrgentOpen           []QueueItem `json:"urgent_open"`
	Overdue              []QueueItem `json:"overdue"`
	AwaitingConfirmation []QueueItem `json:"awaiting_confirmation"`
}

// Service computes the dashboard.
type Service struct {
	DB *pgxpool.Pool
}

const openStatuses = `('submitted', 'under_review', 'verified', 'assigned', 'in_progress', 'partially_resolved')`

const awaitingConfirmation = `r.status IN ('assigned', 'in_progress', 'partially_resolved')
	AND EXISTS (SELECT 1 FROM assignments a WHERE a.request_id = r.id AND a.status IN ('delivered', 'partially_delivered'))
	AND NOT EXISTS (SELECT 1 FROM assignments a WHERE a.request_id = r.id AND a.status IN ('proposed', 'accepted', 'in_progress'))`

func countMap(ctx context.Context, db *pgxpool.Pool, sql string, args ...any) (map[string]int, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

func (s *Service) queue(ctx context.Context, org uuid.UUID, where, order string) ([]QueueItem, error) {
	rows, err := s.DB.Query(ctx, `SELECT r.id, r.reference, r.title, r.category, r.urgency, r.status, r.area_label,
		r.requested_by_time, r.created_at, r.sensitive_data_flag FROM aid_requests r
		WHERE r.organization_id = $1 AND r.deleted_at IS NULL AND `+where+` ORDER BY `+order+` LIMIT 10`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QueueItem{}
	for rows.Next() {
		var q QueueItem
		if err := rows.Scan(&q.ID, &q.Reference, &q.Title, &q.Category, &q.Urgency, &q.Status, &q.AreaLabel,
			&q.RequestedByTime, &q.CreatedAt, &q.Sensitive); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

const urgencyOrder = `CASE r.urgency WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END`

// Get computes the overview.
func (s *Service) Get(ctx context.Context, p roles.Principal) (*Overview, error) {
	if !p.Can(roles.CapDashboardView) {
		return nil, apperr.Forbidden("You do not have access to the dashboard.")
	}
	o := &Overview{GeneratedAt: time.Now().UTC()}
	org := p.OrgID
	var err error
	base := ` FROM aid_requests r WHERE r.organization_id = $1 AND r.deleted_at IS NULL`
	if o.Requests.ByStatus, err = countMap(ctx, s.DB, `SELECT r.status, count(*)`+base+` GROUP BY r.status`, org); err != nil {
		return nil, apperr.Internal(err)
	}
	if o.Requests.OpenByUrgency, err = countMap(ctx, s.DB, `SELECT r.urgency, count(*)`+base+
		` AND r.status IN `+openStatuses+` GROUP BY r.urgency`, org); err != nil {
		return nil, apperr.Internal(err)
	}
	if o.Requests.OpenByCategory, err = countMap(ctx, s.DB, `SELECT r.category, count(*)`+base+
		` AND r.status IN `+openStatuses+` GROUP BY r.category`, org); err != nil {
		return nil, apperr.Internal(err)
	}
	err = s.DB.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE r.status IN `+openStatuses+`),
		count(*) FILTER (WHERE r.status IN ('submitted', 'under_review')),
		count(*) FILTER (WHERE r.status IN `+openStatuses+` AND r.requested_by_time < now()),
		count(*) FILTER (WHERE r.status IN `+openStatuses+` AND r.expiry_at < now()),
		count(*) FILTER (WHERE `+awaitingConfirmation+`),
		count(*) FILTER (WHERE r.status = 'resolved' AND r.closed_at > now() - interval '24 hours')`+base, org).Scan(
		&o.Requests.Open, &o.Requests.NeedsReview, &o.Requests.Overdue, &o.Requests.PastExpiry,
		&o.Requests.AwaitingConfirmation, &o.Requests.ResolvedLast24h)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if o.Offers.RemainingByCategory, err = countMap(ctx, s.DB, `SELECT category, sum(remaining_quantity)::int FROM offers
		WHERE organization_id = $1 AND deleted_at IS NULL AND status IN ('available', 'partially_allocated')
		GROUP BY category`, org); err != nil {
		return nil, apperr.Internal(err)
	}
	err = s.DB.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status IN ('available', 'partially_allocated')),
		count(*) FILTER (WHERE status = 'fully_allocated') FROM offers WHERE organization_id = $1 AND deleted_at IS NULL`,
		org).Scan(&o.Offers.Active, &o.Offers.FullyAllocated)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if o.Assignments.ByStatus, err = countMap(ctx, s.DB, `SELECT status, count(*) FROM assignments
		WHERE organization_id = $1 GROUP BY status`, org); err != nil {
		return nil, apperr.Internal(err)
	}
	for _, st := range []string{"proposed", "accepted", "in_progress", "partially_delivered"} {
		o.Assignments.Active += o.Assignments.ByStatus[st]
	}
	err = s.DB.QueryRow(ctx, `SELECT count(*) FILTER (WHERE availability = 'available'),
		count(*) FILTER (WHERE availability = 'limited'), count(*) FILTER (WHERE availability = 'unavailable')
		FROM users WHERE organization_id = $1 AND is_active AND 'volunteer' = ANY(roles)`, org).Scan(
		&o.Volunteers.Available, &o.Volunteers.Limited, &o.Volunteers.Unavailable)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	if p.Can(roles.CapDashboardQueues) {
		q := &Queues{}
		if q.NeedsReview, err = s.queue(ctx, org, `r.status IN ('submitted', 'under_review')`,
			urgencyOrder+`, r.created_at`); err != nil {
			return nil, apperr.Internal(err)
		}
		if q.UrgentOpen, err = s.queue(ctx, org, `r.status IN `+openStatuses+` AND r.urgency IN ('critical', 'high')`,
			urgencyOrder+`, r.requested_by_time NULLS LAST, r.created_at`); err != nil {
			return nil, apperr.Internal(err)
		}
		if q.Overdue, err = s.queue(ctx, org, `r.status IN `+openStatuses+` AND r.requested_by_time < now()`,
			`r.requested_by_time`); err != nil {
			return nil, apperr.Internal(err)
		}
		if q.AwaitingConfirmation, err = s.queue(ctx, org, awaitingConfirmation, `r.updated_at`); err != nil {
			return nil, apperr.Internal(err)
		}
		// Sensitive titles stay generic in queues.
		for _, list := range [][]QueueItem{q.NeedsReview, q.UrgentOpen, q.Overdue, q.AwaitingConfirmation} {
			for i := range list {
				if list[i].Sensitive {
					list[i].Title = "Sensitive request"
				}
			}
		}
		o.Queues = q
	}
	return o, nil
}

// Handle handles GET /api/v1/dashboard.
func (s *Service) Handle(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	o, err := s.Get(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, o)
}
