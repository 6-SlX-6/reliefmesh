package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"

	"github.com/google/uuid"
)

// newSessionToken returns a 256-bit random token and its SHA-256 hash. Only
// the hash is stored; a database leak does not reveal usable session tokens.
func newSessionToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// CSRFToken derives the CSRF token for a session:
// base64url(HMAC-SHA256(secret, "csrf:" + session id)).
// The token is returned to the authenticated client in JSON and must be sent
// back in the X-CSRF-Token header on every state-changing request.
func CSRFToken(secret []byte, sessionID uuid.UUID) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("csrf:"))
	m.Write([]byte(sessionID.String()))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// ValidCSRF compares a presented token against the expected one in constant
// time.
func ValidCSRF(secret []byte, sessionID uuid.UUID, presented string) bool {
	if presented == "" {
		return false
	}
	want := CSRFToken(secret, sessionID)
	return subtle.ConstantTimeCompare([]byte(want), []byte(presented)) == 1
}
