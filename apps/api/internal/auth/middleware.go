package auth

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

// CSRFHeader is the header that must carry the CSRF token.
const CSRFHeader = "X-CSRF-Token"

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// Authenticate attaches the principal for a valid session cookie. It never
// rejects a request by itself; RequireAuth does.
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.CookieName())
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, err := s.AuthenticateSession(r.Context(), c.Value)
		if err != nil {
			if apperr.IsKind(err, apperr.KindUnauthorized) {
				s.ClearCookie(w)
				next.ServeHTTP(w, r)
				return
			}
			httpx.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(roles.WithPrincipal(r.Context(), *p)))
	})
}

// passwordChangeAllowed lists endpoints reachable while a password change is
// pending.
var passwordChangeAllowed = []string{
	"/api/v1/auth/session",
	"/api/v1/auth/logout",
	"/api/v1/auth/change-password",
	"/api/v1/settings",
	"/api/v1/me",
}

// RequireAuth rejects unauthenticated requests and users who must change
// their password first.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := roles.FromContext(r.Context())
		if !ok {
			httpx.Error(w, r, apperr.Unauthorized("Please sign in."))
			return
		}
		if p.MustChangePassword && !slices.Contains(passwordChangeAllowed, r.URL.Path) {
			httpx.Error(w, r, apperr.ForbiddenCode("password_change_required",
				"You must change your temporary password before continuing."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireCapability rejects principals lacking capability c.
func RequireCapability(c roles.Capability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := roles.FromContext(r.Context())
			if !ok {
				httpx.Error(w, r, apperr.Unauthorized("Please sign in."))
				return
			}
			if !p.Can(c) {
				httpx.Error(w, r, apperr.Forbidden("You do not have permission for this action."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF protects state-changing requests:
//   - the Origin (or Referer) header, when present, must match an allowed
//     origin; this also protects the unauthenticated login endpoint against
//     login CSRF;
//   - authenticated requests must carry a valid X-CSRF-Token header bound to
//     the session.
//
// Together with SameSite=Strict, HttpOnly cookies this provides defence in
// depth against cross-site request forgery.
func (s *Service) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if !s.originAllowed(r) {
			httpx.Error(w, r, apperr.ForbiddenCode("origin_rejected", "Cross-origin request rejected."))
			return
		}
		if p, ok := roles.FromContext(r.Context()); ok {
			if !ValidCSRF(s.Secret, p.SessionID, r.Header.Get(CSRFHeader)) {
				httpx.Error(w, r, apperr.ForbiddenCode("csrf_invalid",
					"Security token missing or invalid. Reload the page and try again."))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		ref := r.Header.Get("Referer")
		if ref == "" {
			// Non-browser clients (CLI, tests) send neither header; they are
			// still subject to the CSRF token check for authenticated calls.
			return true
		}
		u, err := url.Parse(ref)
		if err != nil {
			return false
		}
		origin = u.Scheme + "://" + u.Host
	}
	if origin == "null" {
		return false
	}
	origin = strings.TrimRight(origin, "/")
	return slices.Contains(s.AllowedOrigins, origin)
}

// SetCookie writes the session cookie.
func (s *Service) SetCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.CookieName(),
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearCookie removes the session cookie.
func (s *Service) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.CookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}
