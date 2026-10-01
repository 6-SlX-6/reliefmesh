package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter is an in-memory token bucket per client IP. It is suitable
// for a single API instance; it is a first line of defence in addition to the
// per-account lockout stored in the database.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipEntry
	rate     rate.Limit
	burst    int
	now      func() time.Time
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter allows perMinute events per IP with the given burst.
func NewIPRateLimiter(perMinute, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: map[string]*ipEntry{},
		rate:     rate.Limit(float64(perMinute) / 60.0),
		burst:    burst,
		now:      time.Now,
	}
}

// Allow reports whether an event from ip is allowed now.
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.limiters[ip]
	if !ok {
		if len(l.limiters) > 10000 {
			l.cleanupLocked(now)
		}
		e = &ipEntry{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.limiters[ip] = e
	}
	e.lastSeen = now
	return e.limiter.AllowN(now, 1)
}

func (l *IPRateLimiter) cleanupLocked(now time.Time) {
	for ip, e := range l.limiters {
		if now.Sub(e.lastSeen) > 15*time.Minute {
			delete(l.limiters, ip)
		}
	}
}

// ClientIP returns the client address. X-Forwarded-For is honoured only when
// the direct peer is a configured trusted proxy; the right-most untrusted
// address is used so clients cannot spoof their IP by prepending entries.
func ClientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if len(trusted) == 0 || !ipIn(host, trusted) {
		return host
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		if net.ParseIP(candidate) == nil {
			continue
		}
		if !ipIn(candidate, trusted) {
			return candidate
		}
	}
	return host
}

func ipIn(ip string, nets []*net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}
