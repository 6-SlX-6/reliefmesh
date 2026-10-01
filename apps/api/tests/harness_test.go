// Package tests contains integration tests that run the full HTTP API
// against a real PostgreSQL database.
//
// Set RELIEFMESH_TEST_DATABASE_URL to a dedicated, disposable database; the
// tests DROP and recreate its public schema. Without the variable the tests
// are skipped.
package tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/auth"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/config"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	apihttp "github.com/6-slx-6/reliefmesh/apps/api/internal/http"
	"github.com/6-slx-6/reliefmesh/apps/api/migrations"
)

const testPassword = "integration test passphrase"

var (
	hashOnce   sync.Once
	sharedHash string
)

type env struct {
	t      *testing.T
	pool   *pgxpool.Pool
	app    *apihttp.App
	server *httptest.Server
	org    uuid.UUID
	cfg    *config.Config
}

func dbURL(t *testing.T) string {
	url := os.Getenv("RELIEFMESH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("RELIEFMESH_TEST_DATABASE_URL not set; skipping integration test")
	}
	return url
}

var dbMu sync.Mutex

func TestMain(m *testing.M) {
	if os.Getenv("RELIEFMESH_TEST_LOGS") == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
	os.Exit(m.Run())
}

// newEnv resets the database and starts an API server.
func newEnv(t *testing.T) *env {
	t.Helper()
	url := dbURL(t)
	dbMu.Lock()
	t.Cleanup(dbMu.Unlock)
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := database.Migrate(ctx, pool, migrations.FS, nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	cfg, err := config.LoadFrom(func(k string) string {
		return map[string]string{
			"RELIEFMESH_DATABASE_URL":          url,
			"RELIEFMESH_ENV":                   "development",
			"RELIEFMESH_PUBLIC_ORIGIN":         "http://reliefmesh.test",
			"RELIEFMESH_SECRET_KEY":            strings.Repeat("s", 40),
			"RELIEFMESH_DATA_ENCRYPTION_KEYS":  "k1:" + key,
			"RELIEFMESH_ALLOW_DEMO_SEED":       "true",
			"RELIEFMESH_LOGIN_RATE_PER_MINUTE": "1000",
			"RELIEFMESH_LOGIN_BURST":           "1000",
		}[k]
	})
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	app, err := apihttp.NewApp(cfg, pool)
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	srv := httptest.NewServer(app.Router())
	e := &env{t: t, pool: pool, app: app, server: srv, cfg: cfg, org: uuid.New()}
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Test Org')`, e.org); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})
	return e
}

// createUser inserts an active user with the shared test password.
func (e *env) createUser(username string, roles ...string) uuid.UUID {
	e.t.Helper()
	hashOnce.Do(func() {
		h, err := auth.HashPassword(context.Background(), testPassword)
		if err != nil {
			panic(err)
		}
		sharedHash = h
	})
	id := uuid.New()
	availability := "unavailable"
	for _, r := range roles {
		if r == "volunteer" {
			availability = "available"
		}
	}
	_, err := e.pool.Exec(context.Background(), `INSERT INTO users (id, organization_id, username, display_name, roles,
		password_hash, availability) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, e.org, username, "Name of "+username, roles, sharedHash, availability)
	if err != nil {
		e.t.Fatalf("create user: %v", err)
	}
	return id
}

type client struct {
	t    *testing.T
	e    *env
	http *http.Client
	csrf string
	user map[string]any
}

func (e *env) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: e.t, e: e, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

// login creates a user (if roles are given) and returns a logged-in client.
func (e *env) login(username string, roles ...string) *client {
	e.t.Helper()
	if len(roles) > 0 {
		e.createUser(username, roles...)
	}
	c := e.anon()
	var res map[string]any
	status := c.do("POST", "/api/v1/auth/login", map[string]any{"username": username, "password": testPassword}, &res)
	if status != 200 {
		e.t.Fatalf("login %s failed: %d %v", username, status, res)
	}
	c.csrf = res["csrf_token"].(string)
	c.user = res["user"].(map[string]any)
	return c
}

func (c *client) userID() string { return c.user["id"].(string) }

// do performs a request and decodes the JSON response into out (if non-nil).
func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	return c.doWith(method, path, body, out, nil)
}

func (c *client) doWith(method, path string, body any, out any, headers map[string]string) int {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.e.server.URL+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" && method != "GET" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil && len(data) > 0 {
		if sp, ok := out.(*string); ok {
			*sp = string(data)
		} else if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, string(data), err)
		}
	}
	return resp.StatusCode
}

// mustStatus asserts the status code.
func (c *client) must(want int, method, path string, body any) map[string]any {
	c.t.Helper()
	var out map[string]any
	got := c.do(method, path, body, &out)
	if got != want {
		c.t.Fatalf("%s %s: status %d, want %d: %v", method, path, got, want, out)
	}
	return out
}

func errCode(m map[string]any) string {
	if e, ok := m["error"].(map[string]any); ok {
		return fmt.Sprint(e["code"])
	}
	return ""
}

// baseRequest returns a valid request payload.
func baseRequest(overrides map[string]any) map[string]any {
	m := map[string]any{
		"category":                  "drinking_water",
		"urgency":                   "normal",
		"title":                     "Water for shelter",
		"description":               "Bottled water for 20 people",
		"requested_quantity":        40,
		"requested_unit":            "litres",
		"estimated_people_affected": 20,
		"location":                  map[string]any{"mode": "area_only", "area_label": "North district"},
	}
	for k, v := range overrides {
		m[k] = v
	}
	return m
}

// verifiedRequest creates a request as the given client and verifies it with
// the coordinator.
func verifiedRequest(t *testing.T, creator, coord *client, overrides map[string]any) map[string]any {
	t.Helper()
	r := creator.must(201, "POST", "/api/v1/requests", baseRequest(overrides))
	id := r["id"].(string)
	return coord.must(200, "POST", "/api/v1/requests/"+id+"/status", map[string]any{"status": "verified"})
}
