// Package auth implements local username/password authentication with
// Argon2id hashes, server-side sessions in HTTP-only cookies, CSRF
// protection, login rate limiting and account lockout.
//
// Password reset deliberately does not use e-mail in v0.1.0. Administrators
// reset passwords explicitly (see internal/users), and the CLI offers a
// break-glass reset for operators with shell access.
package auth

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Lockout policy.
const (
	MaxFailedAttempts = 5
	LockDuration      = 15 * time.Minute
	touchInterval     = time.Minute
)

// ErrLoginFailed is deliberately generic so responses do not reveal whether
// an account exists, is locked or is deactivated.
var ErrLoginFailed = &apperr.Error{
	Kind:    apperr.KindUnauthorized,
	Code:    "login_failed",
	Message: "Login failed. Check your username and password. After repeated failures an account is locked for 15 minutes; an administrator can reset it.",
}

// Service handles authentication and sessions.
type Service struct {
	DB             *pgxpool.Pool
	Secret         []byte
	SessionTTL     time.Duration
	IdleTimeout    time.Duration
	CookieSecure   bool
	IPLimiter      *IPRateLimiter
	TrustedProxies []*net.IPNet
	AllowedOrigins []string
	Now            func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// CookieName returns the session cookie name. With Secure cookies the
// __Host- prefix pins the cookie to this exact host and path.
func (s *Service) CookieName() string {
	if s.CookieSecure {
		return "__Host-rm_session"
	}
	return "rm_session"
}

// LoginResult is returned on successful login.
type LoginResult struct {
	Token     string
	SessionID uuid.UUID
	ExpiresAt time.Time
	Principal roles.Principal
}

type userRow struct {
	ID                 uuid.UUID
	OrgID              uuid.UUID
	Username           string
	DisplayName        string
	Roles              []string
	PasswordHash       string
	IsActive           bool
	MustChangePassword bool
	FailedLoginCount   int
	LockedUntil        *time.Time
}

func (u *userRow) principal() roles.Principal {
	rs, _ := roles.Parse(u.Roles)
	return roles.Principal{UserID: u.ID, OrgID: u.OrgID, Username: u.Username, DisplayName: u.DisplayName,
		Roles: rs, MustChangePassword: u.MustChangePassword}
}

// Login verifies credentials and creates a session.
func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	username = validation.CleanText(username)
	if username == "" || password == "" || len(username) > 64 || len(password) > MaxPasswordLength {
		return nil, ErrLoginFailed
	}
	var u userRow
	err := s.DB.QueryRow(ctx, `SELECT id, organization_id, username, display_name, roles, password_hash,
		is_active, must_change_password, failed_login_count, locked_until
		FROM users WHERE lower(username) = lower($1)`, username).Scan(
		&u.ID, &u.OrgID, &u.Username, &u.DisplayName, &u.Roles, &u.PasswordHash,
		&u.IsActive, &u.MustChangePassword, &u.FailedLoginCount, &u.LockedUntil)
	if database.IsNoRows(err) {
		_, _ = VerifyPassword(ctx, password, dummyHash)
		s.recordSystem(ctx, audit.Event{Action: "auth.login_failed", Visibility: audit.System,
			Metadata: map[string]any{"reason": "invalid_credentials"}})
		return nil, ErrLoginFailed
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}

	now := s.now()
	ok, verr := VerifyPassword(ctx, password, u.PasswordHash)
	if verr != nil {
		slog.ErrorContext(ctx, "password verification error", "user_id", u.ID, "error", verr)
		return nil, ErrLoginFailed
	}
	base := audit.Event{OrgID: &u.OrgID, ActorUserID: &u.ID, ActorRoles: u.Roles, Visibility: audit.System,
		EntityType: "user", EntityID: &u.ID, Metadata: map[string]any{}}

	if u.LockedUntil != nil && u.LockedUntil.After(now) {
		e := base
		e.Action = "auth.login_blocked"
		e.Metadata = map[string]any{"reason": "locked"}
		s.recordSystem(ctx, e)
		return nil, ErrLoginFailed
	}
	if !u.IsActive {
		e := base
		e.Action = "auth.login_blocked"
		e.Metadata = map[string]any{"reason": "inactive"}
		s.recordSystem(ctx, e)
		return nil, ErrLoginFailed
	}
	if !ok {
		err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
			var count int
			if err := tx.QueryRow(ctx, `UPDATE users SET failed_login_count = failed_login_count + 1
				WHERE id = $1 RETURNING failed_login_count`, u.ID).Scan(&count); err != nil {
				return err
			}
			e := base
			e.Action = "auth.login_failed"
			e.Metadata = map[string]any{"reason": "invalid_credentials", "consecutive_failures": count}
			events := []audit.Event{e}
			if count >= MaxFailedAttempts {
				if _, err := tx.Exec(ctx, `UPDATE users SET locked_until = $2, failed_login_count = 0 WHERE id = $1`,
					u.ID, now.Add(LockDuration)); err != nil {
					return err
				}
				lock := base
				lock.Action = "auth.account_locked"
				lock.Metadata = map[string]any{"lock_minutes": int(LockDuration.Minutes())}
				events = append(events, lock)
			}
			return audit.Record(ctx, tx, events...)
		})
		if err != nil {
			return nil, apperr.Internal(err)
		}
		return nil, ErrLoginFailed
	}

	token, hash, err := newSessionToken()
	if err != nil {
		return nil, apperr.Internal(err)
	}
	sessionID := uuid.New()
	expires := now.Add(s.SessionTTL)
	err = database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = $2
			WHERE id = $1`, u.ID, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO sessions (id, token_hash, user_id, created_at, last_seen_at, expires_at)
			VALUES ($1, $2, $3, $4, $4, $5)`, sessionID, hash, u.ID, now, expires); err != nil {
			return err
		}
		e := base
		e.Action = "auth.login_succeeded"
		return audit.Record(ctx, tx, e)
	})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	p := u.principal()
	p.SessionID = sessionID
	return &LoginResult{Token: token, SessionID: sessionID, ExpiresAt: expires, Principal: p}, nil
}

func (s *Service) recordSystem(ctx context.Context, e audit.Event) {
	err := database.InTx(ctx, s.DB, func(tx pgx.Tx) error { return audit.Record(ctx, tx, e) })
	if err != nil {
		slog.ErrorContext(ctx, "failed to record audit event", "action", e.Action, "error", err)
	}
}

// AuthenticateSession resolves a session token to a principal.
func (s *Service) AuthenticateSession(ctx context.Context, token string) (*roles.Principal, error) {
	if token == "" || len(token) > 128 {
		return nil, apperr.Unauthorized("Not signed in.")
	}
	var u userRow
	var sessionID uuid.UUID
	var lastSeen, expires time.Time
	err := s.DB.QueryRow(ctx, `SELECT s.id, s.last_seen_at, s.expires_at,
		u.id, u.organization_id, u.username, u.display_name, u.roles, u.is_active, u.must_change_password
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = $1`, hashToken(token)).Scan(
		&sessionID, &lastSeen, &expires, &u.ID, &u.OrgID, &u.Username, &u.DisplayName, &u.Roles,
		&u.IsActive, &u.MustChangePassword)
	if database.IsNoRows(err) {
		return nil, apperr.Unauthorized("Your session has ended. Please sign in again.")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.now()
	if !u.IsActive || now.After(expires) || now.Sub(lastSeen) > s.IdleTimeout {
		_, _ = s.DB.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
		return nil, apperr.Unauthorized("Your session has ended. Please sign in again.")
	}
	if now.Sub(lastSeen) > touchInterval {
		_, _ = s.DB.Exec(ctx, `UPDATE sessions SET last_seen_at = $2 WHERE id = $1`, sessionID, now)
	}
	p := u.principal()
	p.SessionID = sessionID
	return &p, nil
}

// Logout deletes the session.
func (s *Service) Logout(ctx context.Context, p roles.Principal) error {
	return database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, p.SessionID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "auth.logout", audit.System).On("user", p.UserID))
	})
}

// ChangePassword changes the caller's password and ends all their other
// sessions.
func (s *Service) ChangePassword(ctx context.Context, p roles.Principal, current, next string) error {
	var hash string
	if err := s.DB.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, p.UserID).Scan(&hash); err != nil {
		return apperr.Internal(err)
	}
	ok, err := VerifyPassword(ctx, current, hash)
	if err != nil || !ok {
		return apperr.ValidationField("current_password", "The current password is not correct.")
	}
	if msg := ValidatePassword(next, p.Username); msg != "" {
		return apperr.ValidationField("new_password", msg)
	}
	if next == current {
		return apperr.ValidationField("new_password", "Choose a password different from the current one.")
	}
	newHash, err := HashPassword(ctx, next)
	if err != nil {
		return apperr.Internal(err)
	}
	return database.InTx(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2, must_change_password = false, updated_at = now()
			WHERE id = $1`, p.UserID, newHash); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id <> $2`, p.UserID, p.SessionID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.ForActor(p, "auth.password_changed", audit.System).On("user", p.UserID))
	})
}

// RevokeUserSessions deletes all sessions of a user.
func RevokeUserSessions(ctx context.Context, q database.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// CleanupExpiredSessions removes expired and idle sessions.
func (s *Service) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	now := s.now()
	tag, err := s.DB.Exec(ctx, `DELETE FROM sessions WHERE expires_at < $1 OR last_seen_at < $2`,
		now, now.Add(-s.IdleTimeout))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RunSessionJanitor periodically removes expired sessions until ctx ends.
func (s *Service) RunSessionJanitor(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.CleanupExpiredSessions(ctx); err != nil {
				slog.Error("session cleanup failed", "error", err)
			} else if n > 0 {
				slog.Info("expired sessions removed", "count", n)
			}
		}
	}
}

// Me describes the authenticated user for the client.
type Me struct {
	ID                 uuid.UUID `json:"id"`
	Username           string    `json:"username"`
	DisplayName        string    `json:"display_name"`
	Roles              []string  `json:"roles"`
	Capabilities       []string  `json:"capabilities"`
	MustChangePassword bool      `json:"must_change_password"`
	Availability       string    `json:"availability"`
	AvailabilityNote   string    `json:"availability_note"`
	Organization       OrgRef    `json:"organization"`
}

// OrgRef is a minimal organization reference.
type OrgRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// LoadMe loads the profile of the principal.
func (s *Service) LoadMe(ctx context.Context, p roles.Principal) (*Me, error) {
	m := &Me{ID: p.UserID}
	err := s.DB.QueryRow(ctx, `SELECT u.username, u.display_name, u.roles, u.must_change_password,
		u.availability, u.availability_note, o.id, o.name
		FROM users u JOIN organizations o ON o.id = u.organization_id WHERE u.id = $1`, p.UserID).Scan(
		&m.Username, &m.DisplayName, &m.Roles, &m.MustChangePassword, &m.Availability, &m.AvailabilityNote,
		&m.Organization.ID, &m.Organization.Name)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	rs, _ := roles.Parse(m.Roles)
	for _, c := range roles.CapabilitiesFor(rs) {
		m.Capabilities = append(m.Capabilities, string(c))
	}
	if m.Capabilities == nil {
		m.Capabilities = []string{}
	}
	return m, nil
}
