package http

import (
	"strings"
	"testing"
)

func TestInlineScriptHashes(t *testing.T) {
	html := `<html><head><script type="module" src="/_nuxt/entry.js"></script>
<script>window.__NUXT__={}</script><script type="application/json" id="d">[1]</script></head></html>`
	hashes := InlineScriptHashes(html)
	if len(hashes) != 2 {
		t.Fatalf("expected 2 inline hashes, got %d: %v", len(hashes), hashes)
	}
	for _, h := range hashes {
		if !strings.HasPrefix(h, "sha256-") {
			t.Fatalf("bad hash %s", h)
		}
	}
	csp := WebCSP(hashes)
	if !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, hashes[0]) {
		t.Fatalf("unexpected csp %s", csp)
	}
	if strings.Contains(csp, "http") {
		t.Fatal("CSP must not allow third-party origins")
	}
}
