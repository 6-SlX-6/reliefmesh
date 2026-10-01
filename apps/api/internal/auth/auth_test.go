package auth

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashAndVerifyPassword(t *testing.T) {
	ctx := context.Background()
	h, err := HashPassword(ctx, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected hash format: %s", h)
	}
	ok, err := VerifyPassword(ctx, "correct horse battery staple", h)
	if err != nil || !ok {
		t.Fatalf("verify failed: %v", err)
	}
	ok, _ = VerifyPassword(ctx, "wrong password here", h)
	if ok {
		t.Fatal("wrong password accepted")
	}
	h2, _ := HashPassword(ctx, "correct horse battery staple")
	if h == h2 {
		t.Fatal("hashes must be salted")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$AA$AA", "$argon2id$v=19$m=99999999,t=1,p=1$AA$AA",
		"$argon2id$v=19$m=19456,t=2,p=0$AAAA$AAAA"} {
		if ok, err := VerifyPassword(context.Background(), "x", h); ok || err == nil {
			t.Errorf("malformed hash %q accepted", h)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := map[string]bool{
		"short":                       false,
		"password1234":                false,
		"aaaaaaaaaaaaaa":              false,
		"alice-secret-phrase":         false, // contains username
		"three random words here":     true,
		strings.Repeat("x", 129) + "": false,
	}
	for pw, want := range cases {
		got := ValidatePassword(pw, "alice") == ""
		if got != want {
			t.Errorf("ValidatePassword(%q) ok=%v want %v", pw, got, want)
		}
	}
}

func TestTemporaryPassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		p, err := GenerateTemporaryPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != 19 || strings.Count(p, "-") != 3 {
			t.Fatalf("unexpected format %q", p)
		}
		if ValidatePassword(p, "someone") != "" {
			t.Fatalf("temporary password violates policy: %q", p)
		}
		if seen[p] {
			t.Fatal("duplicate temporary password")
		}
		seen[p] = true
	}
}

func TestCSRFToken(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	sid := uuid.New()
	tok := CSRFToken(secret, sid)
	if !ValidCSRF(secret, sid, tok) {
		t.Fatal("valid token rejected")
	}
	if ValidCSRF(secret, uuid.New(), tok) {
		t.Fatal("token must be bound to the session")
	}
	if ValidCSRF([]byte(strings.Repeat("t", 32)), sid, tok) {
		t.Fatal("token must depend on the secret")
	}
	if ValidCSRF(secret, sid, "") {
		t.Fatal("empty token accepted")
	}
}

func TestSessionTokenHash(t *testing.T) {
	tok, hash, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) < 40 || len(hash) != 32 {
		t.Fatal("unexpected token sizes")
	}
	if string(hashToken(tok)) != string(hash) {
		t.Fatal("hash mismatch")
	}
}

func TestIPRateLimiter(t *testing.T) {
	l := NewIPRateLimiter(60, 3)
	now := time.Unix(1_700_000_000, 0)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("burst exceeded but allowed")
	}
	if !l.Allow("5.6.7.8") {
		t.Fatal("other IP must have its own bucket")
	}
	now = now.Add(2 * time.Second)
	if !l.Allow("1.2.3.4") {
		t.Fatal("tokens should refill over time")
	}
}

func TestClientIP(t *testing.T) {
	_, proxyNet, _ := net.ParseCIDR("10.0.0.0/8")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	if got := ClientIP(r, nil); got != "203.0.113.9" {
		t.Fatalf("untrusted peer: got %s", got)
	}
	if got := ClientIP(r, nil); got == "1.1.1.1" {
		t.Fatal("XFF must be ignored without trusted proxies")
	}
	r.RemoteAddr = "10.1.2.3:443"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.7, 10.0.0.5")
	if got := ClientIP(r, []*net.IPNet{proxyNet}); got != "198.51.100.7" {
		t.Fatalf("trusted proxy: got %s", got)
	}
}

func TestOriginAllowed(t *testing.T) {
	s := &Service{AllowedOrigins: []string{"https://relief.example.org"}}
	cases := []struct {
		origin, referer string
		want            bool
	}{
		{"https://relief.example.org", "", true},
		{"https://evil.example", "", false},
		{"null", "", false},
		{"", "https://relief.example.org/requests", true},
		{"", "https://evil.example/x", false},
		{"", "", true},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/api/v1/requests", nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if c.referer != "" {
			r.Header.Set("Referer", c.referer)
		}
		if got := s.originAllowed(r); got != c.want {
			t.Errorf("origin=%q referer=%q got %v want %v", c.origin, c.referer, got, c.want)
		}
	}
}
