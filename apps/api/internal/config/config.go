// Package config loads ReliefMesh configuration from environment variables.
//
// Every secret can alternatively be provided through a file by appending
// _FILE to the variable name (for Docker/Kubernetes secrets), e.g.
// RELIEFMESH_SECRET_KEY_FILE=/run/secrets/reliefmesh_secret_key.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment names.
const (
	EnvProduction  = "production"
	EnvDevelopment = "development"
)

// DataKey is a named AES-256 key used to seal protected fields.
type DataKey struct {
	ID  string
	Key []byte
}

// Config is the fully validated runtime configuration.
type Config struct {
	Environment        string
	ListenAddr         string
	DatabaseURL        string
	PublicOrigin       string
	AllowedOrigins     []string
	CookieSecure       bool
	SessionTTL         time.Duration
	SessionIdleTimeout time.Duration
	SecretKey          []byte
	DataKeys           []DataKey
	TrustedProxies     []*net.IPNet
	WebDir             string
	AllowDemoSeed      bool
	AutoMigrate        bool
	LogLevel           slog.Level
	LoginRatePerMinute int
	LoginBurst         int
	MaxBodyBytes       int64
	SyncMaxBodyBytes   int64
}

// IsDevelopment reports whether the instance runs in development mode.
func (c *Config) IsDevelopment() bool { return c.Environment == EnvDevelopment }

// Getenv abstracts environment lookup for tests.
type Getenv func(string) string

// Load reads configuration from the process environment.
func Load() (*Config, error) { return LoadFrom(os.Getenv) }

// LoadDatabaseOnly reads just enough configuration for CLI commands that only
// need the database (migrate).
func LoadDatabaseOnly() (string, error) {
	v, err := lookupSecret(os.Getenv, "RELIEFMESH_DATABASE_URL")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("RELIEFMESH_DATABASE_URL is required")
	}
	return v, nil
}

// LoadFrom reads configuration using the provided lookup function.
func LoadFrom(getenv Getenv) (*Config, error) {
	var errs []error
	c := &Config{}

	c.Environment = strings.ToLower(strDefault(getenv, "RELIEFMESH_ENV", EnvProduction))
	if c.Environment != EnvProduction && c.Environment != EnvDevelopment {
		errs = append(errs, fmt.Errorf("RELIEFMESH_ENV must be %q or %q", EnvProduction, EnvDevelopment))
	}
	c.ListenAddr = strDefault(getenv, "RELIEFMESH_LISTEN_ADDR", ":8080")

	dbURL, err := lookupSecret(getenv, "RELIEFMESH_DATABASE_URL")
	if err != nil {
		errs = append(errs, err)
	}
	if dbURL == "" {
		errs = append(errs, errors.New("RELIEFMESH_DATABASE_URL is required"))
	}
	c.DatabaseURL = dbURL

	c.PublicOrigin = strings.TrimRight(strDefault(getenv, "RELIEFMESH_PUBLIC_ORIGIN", ""), "/")
	if c.PublicOrigin == "" {
		if c.Environment == EnvDevelopment {
			c.PublicOrigin = "http://localhost:3000"
		} else {
			errs = append(errs, errors.New("RELIEFMESH_PUBLIC_ORIGIN is required in production (e.g. https://relief.example.org)"))
		}
	}
	if c.PublicOrigin != "" {
		if err := validateOrigin(c.PublicOrigin); err != nil {
			errs = append(errs, fmt.Errorf("RELIEFMESH_PUBLIC_ORIGIN: %w", err))
		}
	}
	c.AllowedOrigins = []string{c.PublicOrigin}
	for _, o := range splitList(getenv("RELIEFMESH_EXTRA_ALLOWED_ORIGINS")) {
		o = strings.TrimRight(o, "/")
		if err := validateOrigin(o); err != nil {
			errs = append(errs, fmt.Errorf("RELIEFMESH_EXTRA_ALLOWED_ORIGINS: %w", err))
			continue
		}
		c.AllowedOrigins = append(c.AllowedOrigins, o)
	}

	defaultSecure := !strings.HasPrefix(c.PublicOrigin, "http://")
	c.CookieSecure, err = boolDefault(getenv, "RELIEFMESH_COOKIE_SECURE", defaultSecure)
	if err != nil {
		errs = append(errs, err)
	}
	if !c.CookieSecure && c.Environment == EnvProduction && !strings.HasPrefix(c.PublicOrigin, "http://localhost") {
		errs = append(errs, errors.New("RELIEFMESH_COOKIE_SECURE=false is only allowed in development or for http://localhost origins"))
	}

	c.SessionTTL, err = durationDefault(getenv, "RELIEFMESH_SESSION_TTL", 24*time.Hour)
	if err != nil {
		errs = append(errs, err)
	}
	c.SessionIdleTimeout, err = durationDefault(getenv, "RELIEFMESH_SESSION_IDLE_TIMEOUT", 8*time.Hour)
	if err != nil {
		errs = append(errs, err)
	}

	secret, err := lookupSecret(getenv, "RELIEFMESH_SECRET_KEY")
	if err != nil {
		errs = append(errs, err)
	}
	if len(secret) < 32 {
		errs = append(errs, errors.New("RELIEFMESH_SECRET_KEY must be at least 32 characters (generate with: openssl rand -base64 48)"))
	}
	c.SecretKey = []byte(secret)

	keysRaw, err := lookupSecret(getenv, "RELIEFMESH_DATA_ENCRYPTION_KEYS")
	if err != nil {
		errs = append(errs, err)
	}
	c.DataKeys, err = ParseDataKeys(keysRaw)
	if err != nil {
		errs = append(errs, err)
	}

	for _, cidr := range splitList(getenv("RELIEFMESH_TRUSTED_PROXIES")) {
		if !strings.Contains(cidr, "/") {
			if strings.Contains(cidr, ":") {
				cidr += "/128"
			} else {
				cidr += "/32"
			}
		}
		_, n, perr := net.ParseCIDR(cidr)
		if perr != nil {
			errs = append(errs, fmt.Errorf("RELIEFMESH_TRUSTED_PROXIES: invalid CIDR %q", cidr))
			continue
		}
		c.TrustedProxies = append(c.TrustedProxies, n)
	}

	c.WebDir = getenv("RELIEFMESH_WEB_DIR")
	c.AllowDemoSeed, err = boolDefault(getenv, "RELIEFMESH_ALLOW_DEMO_SEED", false)
	if err != nil {
		errs = append(errs, err)
	}
	c.AutoMigrate, err = boolDefault(getenv, "RELIEFMESH_AUTO_MIGRATE", true)
	if err != nil {
		errs = append(errs, err)
	}

	switch strings.ToLower(strDefault(getenv, "RELIEFMESH_LOG_LEVEL", "info")) {
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "info":
		c.LogLevel = slog.LevelInfo
	case "warn", "warning":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	default:
		errs = append(errs, errors.New("RELIEFMESH_LOG_LEVEL must be debug, info, warn or error"))
	}

	c.LoginRatePerMinute, err = intDefault(getenv, "RELIEFMESH_LOGIN_RATE_PER_MINUTE", 10, 1, 1000)
	if err != nil {
		errs = append(errs, err)
	}
	c.LoginBurst, err = intDefault(getenv, "RELIEFMESH_LOGIN_BURST", 5, 1, 1000)
	if err != nil {
		errs = append(errs, err)
	}
	c.MaxBodyBytes = 1 << 20
	c.SyncMaxBodyBytes = 4 << 20

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

// ParseDataKeys parses "id:base64key[,id:base64key...]". The first key is the
// primary key used for new ciphertexts; the others remain available for
// decrypting data sealed before a key rotation.
func ParseDataKeys(raw string) ([]DataKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("RELIEFMESH_DATA_ENCRYPTION_KEYS is required (format: id:base64-32-byte-key, generate with: echo \"k1:$(openssl rand -base64 32)\")")
	}
	var keys []DataKey
	seen := map[string]bool{}
	for _, part := range splitList(raw) {
		id, b64, ok := strings.Cut(part, ":")
		if !ok || id == "" || b64 == "" {
			return nil, errors.New("RELIEFMESH_DATA_ENCRYPTION_KEYS: each entry must be id:base64key")
		}
		if len(id) > 32 {
			return nil, errors.New("RELIEFMESH_DATA_ENCRYPTION_KEYS: key id must be at most 32 characters")
		}
		if seen[id] {
			return nil, fmt.Errorf("RELIEFMESH_DATA_ENCRYPTION_KEYS: duplicate key id %q", id)
		}
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			key, err = base64.RawStdEncoding.DecodeString(b64)
		}
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("RELIEFMESH_DATA_ENCRYPTION_KEYS: key %q must be 32 bytes, base64-encoded", id)
		}
		seen[id] = true
		keys = append(keys, DataKey{ID: id, Key: key})
	}
	return keys, nil
}

func validateOrigin(o string) error {
	u, err := url.Parse(o)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" {
		return fmt.Errorf("%q must look like https://host[:port] without a path", o)
	}
	return nil
}

func lookupSecret(getenv Getenv, name string) (string, error) {
	if v := getenv(name); v != "" {
		return strings.TrimSpace(v), nil
	}
	if path := getenv(name + "_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("%s_FILE: %w", name, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return "", nil
}

func strDefault(getenv Getenv, name, def string) string {
	if v := strings.TrimSpace(getenv(name)); v != "" {
		return v
	}
	return def
}

func boolDefault(getenv Getenv, name string, def bool) (bool, error) {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def, fmt.Errorf("%s must be true or false", name)
	}
	return b, nil
}

func intDefault(getenv Getenv, name string, def, min, max int) (int, error) {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min || n > max {
		return def, fmt.Errorf("%s must be an integer between %d and %d", name, min, max)
	}
	return n, nil
}

func durationDefault(getenv Getenv, name string, def time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def, fmt.Errorf("%s must be a positive duration like 12h or 30m", name)
	}
	return d, nil
}

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
