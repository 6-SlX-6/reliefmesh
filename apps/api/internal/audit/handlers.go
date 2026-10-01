package audit

import (
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
)

// Handler serves the administrator audit log.
type Handler struct {
	DB *pgxpool.Pool
}

// List handles GET /api/v1/admin/audit. Events contain ids, actions,
// statuses and non-personal metadata only.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := httpx.IntQuery(r, "limit", 100, 1, 500)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var before int64
	if v := q.Get("before_id"); v != "" {
		if before, err = strconv.ParseInt(v, 10, 64); err != nil || before < 0 {
			httpx.Error(w, r, apperr.ValidationField("before_id", "Must be a positive integer."))
			return
		}
	}
	events, err := List(r.Context(), h.DB, ListFilter{Action: q.Get("action"), EntityType: q.Get("entity_type"),
		BeforeID: before, Limit: limit})
	if err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	if events == nil {
		events = []*Stored{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": events})
}

// Verify handles GET /api/v1/admin/audit/verify.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	res, err := Verify(r.Context(), h.DB)
	if err != nil {
		httpx.Error(w, r, apperr.Internal(err))
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}
