package tests

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/config"
	apihttp "github.com/6-slx-6/reliefmesh/apps/api/internal/http"
)

type openAPIDoc struct {
	Paths      map[string]map[string]any `yaml:"paths"`
	Components map[string]map[string]any `yaml:"components"`
}

func loadSpec(t *testing.T) (*openAPIDoc, []byte) {
	t.Helper()
	raw, err := os.ReadFile("../openapi/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("openapi.yaml is not valid YAML: %v", err)
	}
	return &doc, raw
}

func routerWithoutDB(t *testing.T) chi.Router {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	cfg, err := config.LoadFrom(func(k string) string {
		return map[string]string{
			"RELIEFMESH_DATABASE_URL":         "postgres://unused",
			"RELIEFMESH_ENV":                  "development",
			"RELIEFMESH_SECRET_KEY":           strings.Repeat("x", 32),
			"RELIEFMESH_DATA_ENCRYPTION_KEYS": "k1:" + key,
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	app, err := apihttp.NewApp(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return app.Router()
}

// TestOpenAPIMatchesRouter is an API contract test: every registered route
// must be documented and every documented operation must exist.
func TestOpenAPIMatchesRouter(t *testing.T) {
	doc, _ := loadSpec(t)
	documented := map[string]bool{}
	for path, ops := range doc.Paths {
		for method := range ops {
			switch method {
			case "get", "post", "put", "patch", "delete":
				documented[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	registered := map[string]bool{}
	err := chi.Walk(routerWithoutDB(t), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasSuffix(route, "/*") {
			return nil
		}
		route = strings.ReplaceAll(route, "/*/", "/")
		if len(route) > 1 {
			route = strings.TrimSuffix(route, "/")
		}
		registered[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(registered) < 50 || len(documented) < 50 {
		t.Fatalf("suspiciously few routes: registered=%d documented=%d", len(registered), len(documented))
	}
	var missingDoc, missingRoute []string
	for r := range registered {
		if !documented[r] {
			missingDoc = append(missingDoc, r)
		}
	}
	for d := range documented {
		if !registered[d] {
			missingRoute = append(missingRoute, d)
		}
	}
	sort.Strings(missingDoc)
	sort.Strings(missingRoute)
	if len(missingDoc) > 0 {
		t.Errorf("routes missing from openapi.yaml:\n  %s", strings.Join(missingDoc, "\n  "))
	}
	if len(missingRoute) > 0 {
		t.Errorf("documented operations without a route:\n  %s", strings.Join(missingRoute, "\n  "))
	}
}

// TestOpenAPIRefsResolve ensures every $ref points to a defined component.
func TestOpenAPIRefsResolve(t *testing.T) {
	doc, raw := loadSpec(t)
	for _, line := range strings.Split(string(raw), "\n") {
		for _, part := range strings.Split(line, `$ref: "#/components/`)[1:] {
			ref := part[:strings.Index(part, `"`)]
			kind, name, _ := strings.Cut(ref, "/")
			if _, ok := doc.Components[kind][name]; !ok {
				t.Errorf("unresolved $ref #/components/%s", ref)
			}
		}
	}
}

// TestStateChangingOperationsDocumentCSRF ensures authenticated mutating
// operations document the CSRF header.
func TestStateChangingOperationsDocumentCSRF(t *testing.T) {
	doc, _ := loadSpec(t)
	for path, ops := range doc.Paths {
		for method, op := range ops {
			if method == "get" || method == "parameters" || path == "/api/v1/auth/login" {
				continue
			}
			m, _ := op.(map[string]any)
			params, _ := m["parameters"].([]any)
			found := false
			for _, p := range params {
				if pm, ok := p.(map[string]any); ok && pm["$ref"] == "#/components/parameters/CSRF" {
					found = true
				}
			}
			if !found {
				t.Errorf("%s %s does not document X-CSRF-Token", strings.ToUpper(method), path)
			}
		}
	}
}
