// Package export produces non-sensitive operational reports.
//
// Reports never contain names, contact details, exact or approximate
// coordinates, free-text descriptions, notes or instructions. Area labels are
// included only for requests that are not flagged as sensitive. Every export
// is recorded in the audit log.
package export

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

// Service generates reports.
type Service struct {
	DB *pgxpool.Pool
}

// Period bounds a report.
type Period struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func parsePeriod(r *http.Request) (Period, error) {
	now := time.Now().UTC()
	p := Period{From: now.AddDate(0, 0, -30), To: now.Add(time.Minute)}
	parse := func(name string, dst *time.Time) error {
		v := r.URL.Query().Get(name)
		if v == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			if t, err = time.Parse("2006-01-02", v); err != nil {
				return apperr.ValidationField(name, "Use YYYY-MM-DD or an RFC 3339 timestamp.")
			}
		}
		*dst = t
		return nil
	}
	if err := parse("from", &p.From); err != nil {
		return p, err
	}
	if err := parse("to", &p.To); err != nil {
		return p, err
	}
	if p.To.Before(p.From) {
		return p, apperr.ValidationField("to", "Must be after 'from'.")
	}
	return p, nil
}

// SafeCell neutralizes spreadsheet formula injection.
func SafeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func (s *Service) record(ctx context.Context, p roles.Principal, kind string, period Period, rows int) error {
	return database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		return audit.Record(ctx, tx, audit.ForActor(p, "export.generated", audit.Internal).
			With("report", kind).With("from", period.From.Format(time.RFC3339)).
			With("to", period.To.Format(time.RFC3339)).With("rows", rows))
	})
}

// CSVHeader lists the columns of the request report.
var CSVHeader = []string{"reference", "category", "urgency", "status", "verification_level", "created_at",
	"closed_at", "hours_to_close", "estimated_people_affected", "requested_quantity", "requested_unit",
	"area_label", "assignments", "delivered_assignments"}

// RequestsCSV writes the request report.
func (s *Service) RequestsCSV(ctx context.Context, p roles.Principal, period Period, w *csv.Writer) (int, error) {
	if !p.Can(roles.CapExportReports) {
		return 0, apperr.Forbidden("Only organization managers can export reports.")
	}
	rows, err := s.DB.Query(ctx, `SELECT r.reference, r.category, r.urgency, r.status, r.verification_level,
		r.created_at, r.closed_at, r.estimated_people_affected, r.requested_quantity, r.requested_unit,
		CASE WHEN r.sensitive_data_flag OR r.location_mode = 'none' THEN '' ELSE r.area_label END,
		(SELECT count(*) FROM assignments a WHERE a.request_id = r.id),
		(SELECT count(*) FROM assignments a WHERE a.request_id = r.id AND a.status IN ('delivered', 'partially_delivered'))
		FROM aid_requests r WHERE r.organization_id = $1 AND r.deleted_at IS NULL AND r.status <> 'draft'
		AND r.created_at >= $2 AND r.created_at < $3 ORDER BY r.created_at`, p.OrgID, period.From, period.To)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	defer rows.Close()
	if err := w.Write(CSVHeader); err != nil {
		return 0, err
	}
	n := 0
	for rows.Next() {
		var ref, cat, urg, status, verification, unit, area string
		var created time.Time
		var closed *time.Time
		var people, qty *int
		var assignments, delivered int
		if err := rows.Scan(&ref, &cat, &urg, &status, &verification, &created, &closed, &people, &qty, &unit,
			&area, &assignments, &delivered); err != nil {
			return n, apperr.Internal(err)
		}
		closedStr, hours := "", ""
		if closed != nil {
			closedStr = closed.UTC().Format(time.RFC3339)
			hours = strconv.FormatFloat(closed.Sub(created).Hours(), 'f', 1, 64)
		}
		intStr := func(v *int) string {
			if v == nil {
				return ""
			}
			return strconv.Itoa(*v)
		}
		rec := []string{ref, cat, urg, status, verification, created.UTC().Format(time.RFC3339), closedStr, hours,
			intStr(people), intStr(qty), SafeCell(unit), SafeCell(area), strconv.Itoa(assignments), strconv.Itoa(delivered)}
		if err := w.Write(rec); err != nil {
			return n, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return n, apperr.Internal(err)
	}
	w.Flush()
	return n, w.Error()
}

// Summary is an aggregate report.
type Summary struct {
	Period                  Period                    `json:"period"`
	GeneratedAt             time.Time                 `json:"generated_at"`
	RequestsTotal           int                       `json:"requests_total"`
	RequestsByStatus        map[string]int            `json:"requests_by_status"`
	RequestsByCategory      map[string]int            `json:"requests_by_category"`
	RequestsByUrgency       map[string]int            `json:"requests_by_urgency"`
	MedianHoursToResolution *float64                  `json:"median_hours_to_resolution"`
	EstimatedPeopleReached  int                       `json:"estimated_people_reached"`
	OffersByCategory        map[string]OfferAggregate `json:"offers_by_category"`
	AssignmentsByStatus     map[string]int            `json:"assignments_by_status"`
	Notice                  string                    `json:"notice"`
}

// OfferAggregate sums offer quantities.
type OfferAggregate struct {
	Offers            int `json:"offers"`
	QuantityAvailable int `json:"quantity_available"`
	QuantityAllocated int `json:"quantity_allocated"`
}

func countBy(ctx context.Context, db *pgxpool.Pool, col string, p roles.Principal, period Period) (map[string]int, error) {
	rows, err := db.Query(ctx, `SELECT `+col+`, count(*) FROM aid_requests WHERE organization_id = $1
		AND deleted_at IS NULL AND status <> 'draft' AND created_at >= $2 AND created_at < $3 GROUP BY 1`,
		p.OrgID, period.From, period.To)
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

// BuildSummary computes the aggregate report.
func (s *Service) BuildSummary(ctx context.Context, p roles.Principal, period Period) (*Summary, error) {
	if !p.Can(roles.CapExportReports) {
		return nil, apperr.Forbidden("Only organization managers can export reports.")
	}
	sum := &Summary{Period: period, GeneratedAt: time.Now().UTC(),
		Notice: "Aggregated, non-personal report generated by ReliefMesh. Not an official record of any authority."}
	var err error
	for col, dst := range map[string]*map[string]int{"status": &sum.RequestsByStatus, "category": &sum.RequestsByCategory,
		"urgency": &sum.RequestsByUrgency} {
		if *dst, err = countBy(ctx, s.DB, col, p, period); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	for _, n := range sum.RequestsByStatus {
		sum.RequestsTotal += n
	}
	err = s.DB.QueryRow(ctx, `SELECT
		percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM closed_at - created_at) / 3600)
			FILTER (WHERE status = 'resolved'),
		COALESCE(sum(estimated_people_affected) FILTER (WHERE status IN ('resolved', 'partially_resolved')), 0)
		FROM aid_requests WHERE organization_id = $1 AND deleted_at IS NULL AND created_at >= $2 AND created_at < $3`,
		p.OrgID, period.From, period.To).Scan(&sum.MedianHoursToResolution, &sum.EstimatedPeopleReached)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if sum.MedianHoursToResolution != nil {
		v := float64(int(*sum.MedianHoursToResolution*10)) / 10
		sum.MedianHoursToResolution = &v
	}
	sum.OffersByCategory = map[string]OfferAggregate{}
	rows, err := s.DB.Query(ctx, `SELECT category, count(*), COALESCE(sum(quantity_available), 0),
		COALESCE(sum(assigned_quantity), 0) FROM offers WHERE organization_id = $1 AND deleted_at IS NULL
		AND status <> 'draft' AND created_at >= $2 AND created_at < $3 GROUP BY category`, p.OrgID, period.From, period.To)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for rows.Next() {
		var cat string
		var a OfferAggregate
		if err := rows.Scan(&cat, &a.Offers, &a.QuantityAvailable, &a.QuantityAllocated); err != nil {
			rows.Close()
			return nil, apperr.Internal(err)
		}
		sum.OffersByCategory[cat] = a
	}
	rows.Close()
	sum.AssignmentsByStatus = map[string]int{}
	rows, err = s.DB.Query(ctx, `SELECT status, count(*) FROM assignments WHERE organization_id = $1
		AND assigned_at >= $2 AND assigned_at < $3 GROUP BY status`, p.OrgID, period.From, period.To)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			rows.Close()
			return nil, apperr.Internal(err)
		}
		sum.AssignmentsByStatus[st] = n
	}
	rows.Close()
	return sum, nil
}

// HandleRequestsCSV handles GET /api/v1/exports/requests.csv.
func (s *Service) HandleRequestsCSV(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	if !p.Can(roles.CapExportReports) {
		httpx.Error(w, r, apperr.Forbidden("Only organization managers can export reports."))
		return
	}
	period, err := parsePeriod(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var sb strings.Builder
	cw := csv.NewWriter(&sb)
	n, err := s.RequestsCSV(r.Context(), p, period, cw)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := s.record(r.Context(), p, "requests_csv", period, n); err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	name := fmt.Sprintf("reliefmesh-requests-%s-%s.csv", period.From.Format("20060102"), period.To.Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

// HandleSummary handles GET /api/v1/exports/summary.json.
func (s *Service) HandleSummary(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	period, err := parsePeriod(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	sum, err := s.BuildSummary(r.Context(), p, period)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := s.record(r.Context(), p, "summary_json", period, sum.RequestsTotal); err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	httpx.JSON(w, http.StatusOK, sum)
}

// SortedKeys is a helper for deterministic output in tests.
func SortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
