package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func env(m map[string]string) Getenv { return func(k string) string { return m[k] } }

func valid() map[string]string {
	return map[string]string{
		"RELIEFMESH_DATABASE_URL":         "postgres://x@localhost/db",
		"RELIEFMESH_PUBLIC_ORIGIN":        "https://relief.example.org",
		"RELIEFMESH_SECRET_KEY":           strings.Repeat("a", 32),
		"RELIEFMESH_DATA_ENCRYPTION_KEYS": "k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)),
	}
}

func TestLoadValidProductionConfig(t *testing.T) {
	c, err := LoadFrom(env(valid()))
	if err != nil {
		t.Fatal(err)
	}
	if !c.CookieSecure || c.Environment != EnvProduction || c.AllowDemoSeed {
		t.Fatalf("unsafe defaults: %+v", c)
	}
	if c.AllowedOrigins[0] != "https://relief.example.org" {
		t.Fatal("origin not set")
	}
}

func TestProductionRequiresSecrets(t *testing.T) {
	for _, missing := range []string{"RELIEFMESH_DATABASE_URL", "RELIEFMESH_PUBLIC_ORIGIN", "RELIEFMESH_SECRET_KEY", "RELIEFMESH_DATA_ENCRYPTION_KEYS"} {
		m := valid()
		delete(m, missing)
		if _, err := LoadFrom(env(m)); err == nil {
			t.Errorf("missing %s should fail", missing)
		}
	}
	m := valid()
	m["RELIEFMESH_SECRET_KEY"] = "short"
	if _, err := LoadFrom(env(m)); err == nil {
		t.Error("short secret should fail")
	}
}

func TestInsecureCookiesRejectedInProduction(t *testing.T) {
	m := valid()
	m["RELIEFMESH_COOKIE_SECURE"] = "false"
	if _, err := LoadFrom(env(m)); err == nil {
		t.Fatal("insecure cookies must be rejected for public production origins")
	}
	m["RELIEFMESH_ENV"] = "development"
	if _, err := LoadFrom(env(m)); err != nil {
		t.Fatalf("development may use insecure cookies: %v", err)
	}
}

func TestParseDataKeys(t *testing.T) {
	k := base64.StdEncoding.EncodeToString(make([]byte, 32))
	keys, err := ParseDataKeys("new:" + k + ",old:" + k)
	if err != nil || len(keys) != 2 || keys[0].ID != "new" {
		t.Fatalf("unexpected: %v %v", keys, err)
	}
	for _, bad := range []string{"", "nokey", "k1:" + base64.StdEncoding.EncodeToString(make([]byte, 16)), "a:" + k + ",a:" + k} {
		if _, err := ParseDataKeys(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestTrustedProxies(t *testing.T) {
	m := valid()
	m["RELIEFMESH_TRUSTED_PROXIES"] = "10.0.0.0/8, 172.18.0.2"
	c, err := LoadFrom(env(m))
	if err != nil || len(c.TrustedProxies) != 2 {
		t.Fatalf("unexpected: %v %v", c, err)
	}
	m["RELIEFMESH_TRUSTED_PROXIES"] = "not-an-ip"
	if _, err := LoadFrom(env(m)); err == nil {
		t.Fatal("invalid CIDR should fail")
	}
}
