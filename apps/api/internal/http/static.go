package http

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// StaticHandler serves the built web app (single-binary / demo deployments).
// In the production Compose setup the web app is served by its own
// container; both use the same Content-Security-Policy.
type StaticHandler struct {
	dir   string
	index string
	csp   string
}

// NewStaticHandler prepares static serving from dir.
func NewStaticHandler(dir string) (*StaticHandler, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	index := filepath.Join(abs, "index.html")
	if _, err := os.Stat(index); err != nil {
		return nil, fmt.Errorf("index.html not found in %s", abs)
	}
	var hashes []string
	for _, name := range []string{"index.html", "200.html"} {
		b, err := os.ReadFile(filepath.Join(abs, name))
		if err != nil {
			continue
		}
		for _, h := range InlineScriptHashes(string(b)) {
			if !contains(hashes, h) {
				hashes = append(hashes, h)
			}
		}
	}
	sort.Strings(hashes)
	return &StaticHandler{dir: abs, index: index, csp: WebCSP(hashes)}, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// WebCSP builds the Content-Security-Policy for the web app. No third-party
// origins are allowed: no CDNs, fonts, analytics or tracking.
func WebCSP(scriptHashes []string) string {
	script := "'self'"
	for _, h := range scriptHashes {
		script += " '" + h + "'"
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + script,
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"font-src 'self'",
		"connect-src 'self'",
		"manifest-src 'self'",
		"worker-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}

// InlineScriptHashes returns CSP sha256 sources for inline <script> blocks.
func InlineScriptHashes(html string) []string {
	var out []string
	lower := strings.ToLower(html)
	pos := 0
	for {
		i := strings.Index(lower[pos:], "<script")
		if i < 0 {
			return out
		}
		start := pos + i
		end := strings.Index(lower[start:], ">")
		if end < 0 {
			return out
		}
		tag := lower[start : start+end]
		bodyStart := start + end + 1
		close := strings.Index(lower[bodyStart:], "</script>")
		if close < 0 {
			return out
		}
		body := html[bodyStart : bodyStart+close]
		if !strings.Contains(tag, " src=") && strings.TrimSpace(body) != "" {
			sum := sha256.Sum256([]byte(body))
			out = append(out, "sha256-"+base64.StdEncoding.EncodeToString(sum[:]))
		}
		pos = bodyStart + close + len("</script>")
	}
}

func (s *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h := w.Header()
	h.Set("Content-Security-Policy", s.csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Permissions-Policy", "geolocation=(self), camera=(), microphone=(), payment=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")

	clean := path.Clean("/" + r.URL.Path)
	if strings.Contains(clean, "/.") {
		http.NotFound(w, r)
		return
	}
	file := filepath.Join(s.dir, filepath.FromSlash(clean))
	if st, err := os.Stat(file); err == nil && !st.IsDir() {
		switch {
		case strings.HasPrefix(clean, "/_nuxt/"):
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasSuffix(clean, ".html") || clean == "/sw.js" || strings.HasSuffix(clean, ".webmanifest"):
			h.Set("Cache-Control", "no-cache")
		default:
			h.Set("Cache-Control", "public, max-age=3600")
		}
		http.ServeFile(w, r, file)
		return
	}
	if strings.HasPrefix(clean, "/_nuxt/") || path.Ext(clean) != "" {
		http.NotFound(w, r)
		return
	}
	// SPA fallback: client-side routing handles the path.
	h.Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, s.index)
}
