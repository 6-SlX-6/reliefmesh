// Package http assembles the ReliefMesh HTTP API: routing, middleware and
// optional static serving of the web app.
package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/assignments"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/auth"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/config"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/dashboard"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/demo"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/export"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/health"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/locations"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/notes"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/offers"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/requests"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/retention"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/settings"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/sync"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/users"
)

// App bundles all services.
type App struct {
	Config      *config.Config
	DB          *pgxpool.Pool
	Auth        *auth.Service
	Settings    *settings.Service
	Users       *users.Service
	Requests    *requests.Service
	Offers      *offers.Service
	Assignments *assignments.Service
	Notes       *notes.Service
	Dashboard   *dashboard.Service
	Export      *export.Service
	Sync        *sync.Service
	Retention   *retention.Service
	Demo        *demo.Seeder
}

// NewApp wires services together.
func NewApp(cfg *config.Config, db *pgxpool.Pool) (*App, error) {
	sealer, err := locations.NewSealer(cfg.DataKeys)
	if err != nil {
		return nil, err
	}
	a := &App{Config: cfg, DB: db}
	a.Auth = &auth.Service{
		DB: db, Secret: cfg.SecretKey, SessionTTL: cfg.SessionTTL, IdleTimeout: cfg.SessionIdleTimeout,
		CookieSecure: cfg.CookieSecure, IPLimiter: auth.NewIPRateLimiter(cfg.LoginRatePerMinute, cfg.LoginBurst),
		TrustedProxies: cfg.TrustedProxies, AllowedOrigins: cfg.AllowedOrigins,
	}
	a.Settings = &settings.Service{DB: db}
	a.Users = &users.Service{DB: db}
	a.Offers = &offers.Service{DB: db, Sealer: sealer}
	a.Assignments = &assignments.Service{DB: db, Sealer: sealer}
	a.Requests = &requests.Service{DB: db, Sealer: sealer, Assignments: a.Assignments}
	a.Notes = &notes.Service{DB: db, Requests: a.Requests, Offers: a.Offers}
	a.Dashboard = &dashboard.Service{DB: db}
	a.Export = &export.Service{DB: db}
	a.Sync = &sync.Service{DB: db, Requests: a.Requests, Offers: a.Offers, Assignments: a.Assignments, Notes: a.Notes}
	a.Retention = &retention.Service{DB: db, Requests: a.Requests, Offers: a.Offers}
	a.Demo = &demo.Seeder{DB: db, Enabled: cfg.AllowDemoSeed, Requests: a.Requests, Offers: a.Offers, Assignments: a.Assignments}
	return a, nil
}

type ctxKeyRequestID struct{}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9\-]{8,64}$`)

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID{}, id)))
	})
}

func init() {
	httpx.RequestIDFunc = func(r *http.Request) string {
		id, _ := r.Context().Value(ctxKeyRequestID{}).(string)
		return id
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// accessLog writes one structured line per request. It never logs bodies,
// query strings or headers, which could contain personal data.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		route := chi.RouteContext(r.Context())
		pattern := r.URL.Path
		if route != nil && route.RoutePattern() != "" {
			pattern = route.RoutePattern()
		}
		attrs := []any{"method", r.Method, "route", pattern, "status", rec.status, "bytes", rec.bytes,
			"duration_ms", time.Since(start).Milliseconds(), "request_id", httpx.RequestIDFunc(r)}
		if p, ok := roles.FromContext(r.Context()); ok {
			attrs = append(attrs, "user_id", p.UserID.String())
		}
		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		}
		slog.Log(r.Context(), level, "http request", attrs...)
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.Error("panic", "value", v, "stack", string(debug.Stack()), "request_id", httpx.RequestIDFunc(r))
				httpx.Error(w, r, apperr.Internal(nil))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// apiSecurityHeaders applies strict headers to API responses: nothing is
// cacheable and nothing can be framed or executed.
func apiSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Pragma", "no-cache")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		next.ServeHTTP(w, r)
	})
}

func limitBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// Router builds the HTTP handler.
func (a *App) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(requestID, recoverer, accessLog)

	hh := &health.Handler{DB: a.DB}
	r.Get("/healthz", hh.Live)
	r.Get("/readyz", hh.Ready)

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(apiSecurityHeaders)
		api.Use(a.Auth.Authenticate)
		api.Use(a.Auth.CSRF)
		api.NotFound(func(w http.ResponseWriter, r *http.Request) {
			httpx.Error(w, r, apperr.NotFound("Unknown API endpoint."))
		})
		api.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			httpx.Error(w, r, &apperr.Error{Kind: apperr.KindBadRequest, Code: "method_not_allowed", Message: "Method not allowed."})
		})

		// Public endpoints.
		api.With(limitBody(a.Config.MaxBodyBytes)).Post("/auth/login", a.Auth.HandleLogin)
		api.Get("/public/notice", a.Settings.HandleGetPublic)

		// Authenticated endpoints.
		api.Group(func(g chi.Router) {
			g.Use(auth.RequireAuth)
			g.With(limitBody(a.Config.SyncMaxBodyBytes)).Post("/sync/push", a.Sync.HandlePush)
			g.Group(func(g chi.Router) {
				g.Use(limitBody(a.Config.MaxBodyBytes))
				g.Get("/auth/session", a.Auth.HandleSession)
				g.Post("/auth/logout", a.Auth.HandleLogout)
				g.Post("/auth/change-password", a.Auth.HandleChangePassword)
				g.Get("/me", a.Auth.HandleMe)
				g.Patch("/me/availability", a.Users.HandleSetOwnAvailability)
				g.Get("/settings", a.Settings.HandleGetClient)
				g.Get("/sync/pull", a.Sync.HandlePull)
				g.Get("/dashboard", a.Dashboard.Handle)

				g.Route("/requests", func(rr chi.Router) {
					a.Requests.Routes(rr)
					rr.Route("/{id}/notes", a.Notes.Routes("request"))
				})
				g.Route("/offers", func(rr chi.Router) {
					a.Offers.Routes(rr)
					rr.Route("/{id}/notes", a.Notes.Routes("offer"))
				})
				g.Route("/assignments", a.Assignments.Routes)
				g.Route("/volunteers", a.Users.VolunteerRoutes)

				g.Route("/exports", func(er chi.Router) {
					er.Use(auth.RequireCapability(roles.CapExportReports))
					er.Get("/requests.csv", a.Export.HandleRequestsCSV)
					er.Get("/summary.json", a.Export.HandleSummary)
				})

				g.Route("/admin", func(ar chi.Router) {
					ah := &audit.Handler{DB: a.DB}
					ar.With(auth.RequireCapability(roles.CapSettingsManage)).Put("/settings", a.Settings.HandleUpdate)
					ar.With(auth.RequireCapability(roles.CapCategoriesManage)).Get("/categories", a.Settings.HandleListCategoriesAdmin)
					ar.With(auth.RequireCapability(roles.CapCategoriesManage)).Patch("/categories/{code}", a.Settings.HandleUpdateCategory)
					ar.With(auth.RequireCapability(roles.CapUserManage)).Route("/users", a.Users.AdminRoutes)
					ar.With(auth.RequireCapability(roles.CapAuditViewSystem)).Get("/audit", ah.List)
					ar.With(auth.RequireCapability(roles.CapAuditViewSystem)).Get("/audit/verify", ah.Verify)
					ar.With(auth.RequireCapability(roles.CapRetentionManage)).Get("/retention", a.Retention.HandlePreview)
					ar.With(auth.RequireCapability(roles.CapRetentionManage)).Post("/retention/apply", a.Retention.HandleApply)
					ar.With(auth.RequireCapability(roles.CapRecordDelete)).Post("/records/delete", a.Retention.HandleDelete)
					ar.With(auth.RequireCapability(roles.CapDemoSeed)).Get("/demo", a.Demo.HandleInfo)
					ar.With(auth.RequireCapability(roles.CapDemoSeed)).Post("/demo/seed", a.Demo.HandleSeed)
				})
			})
		})
	})

	if a.Config.WebDir != "" {
		static, err := NewStaticHandler(a.Config.WebDir)
		if err != nil {
			slog.Error("static web directory unusable; web app will not be served", "error", err)
		} else {
			r.NotFound(static.ServeHTTP)
		}
	}
	return r
}
