// Package health exposes liveness and readiness endpoints for container
// orchestration and reverse proxies. They reveal no data.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
)

// Version is set at build time via -ldflags.
var Version = "0.1.0-dev"

// Handler serves health endpoints.
type Handler struct {
	DB *pgxpool.Pool
}

// Live handles GET /healthz: the process is running.
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
}

// Ready handles GET /readyz: the database is reachable and migrated.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var n int
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n == 0 {
		httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready", "version": Version})
}
