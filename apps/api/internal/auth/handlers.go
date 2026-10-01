package auth

import (
	"net/http"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/httpx"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
)

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SessionResponse is returned by login and session endpoints.
type SessionResponse struct {
	User      *Me    `json:"user"`
	CSRFToken string `json:"csrf_token"`
}

// HandleLogin handles POST /api/v1/auth/login.
func (s *Service) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if s.IPLimiter != nil && !s.IPLimiter.Allow(ClientIP(r, s.TrustedProxies)) {
		w.Header().Set("Retry-After", "60")
		httpx.Error(w, r, apperr.TooManyRequests("Too many login attempts. Wait a minute and try again."))
		return
	}
	var in loginInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := s.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	me, err := s.LoadMe(r.Context(), res.Principal)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	s.SetCookie(w, res.Token, res.ExpiresAt)
	httpx.JSON(w, http.StatusOK, SessionResponse{User: me, CSRFToken: CSRFToken(s.Secret, res.SessionID)})
}

// HandleSession handles GET /api/v1/auth/session.
func (s *Service) HandleSession(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	me, err := s.LoadMe(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, SessionResponse{User: me, CSRFToken: CSRFToken(s.Secret, p.SessionID)})
}

// HandleMe handles GET /api/v1/me.
func (s *Service) HandleMe(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	me, err := s.LoadMe(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, me)
}

// HandleLogout handles POST /api/v1/auth/logout.
func (s *Service) HandleLogout(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	if err := s.Logout(r.Context(), p); err != nil {
		httpx.Error(w, r, err)
		return
	}
	s.ClearCookie(w)
	httpx.NoContent(w)
}

type changePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// HandleChangePassword handles POST /api/v1/auth/change-password.
func (s *Service) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	p, _ := roles.FromContext(r.Context())
	var in changePasswordInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := s.ChangePassword(r.Context(), p, in.CurrentPassword, in.NewPassword); err != nil {
		httpx.Error(w, r, err)
		return
	}
	p.MustChangePassword = false
	me, err := s.LoadMe(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, SessionResponse{User: me, CSRFToken: CSRFToken(s.Secret, p.SessionID)})
}
