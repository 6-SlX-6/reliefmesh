// Command reliefmesh-api runs the ReliefMesh API server and maintenance
// commands.
//
//	reliefmesh-api [serve]           run the HTTP server (default)
//	reliefmesh-api migrate           apply database migrations
//	reliefmesh-api bootstrap-admin   create the first administrator account
//	reliefmesh-api reset-password    break-glass password reset (operators with shell access)
//	reliefmesh-api seed-demo         load a fictional exercise scenario (demo mode only)
//	reliefmesh-api apply-retention   apply the data retention policy (e.g. from cron)
//	reliefmesh-api healthcheck       probe /healthz (for container health checks)
//	reliefmesh-api version           print the version
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/audit"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/auth"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/config"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/database"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/health"
	apihttp "github.com/6-slx-6/reliefmesh/apps/api/internal/http"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/roles"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/users"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
	"github.com/6-slx-6/reliefmesh/apps/api/migrations"
)

func main() {
	cmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "migrate":
		err = migrate()
	case "bootstrap-admin":
		err = bootstrapAdmin(args)
	case "reset-password":
		err = resetPassword(args)
	case "seed-demo":
		err = seedDemo(args)
	case "apply-retention":
		err = applyRetention()
	case "healthcheck":
		err = healthcheck(args)
	case "version":
		fmt.Println(health.Version)
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		err = fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const usage = `usage: reliefmesh-api <command> [flags]

commands:
  serve             run the HTTP API server (default)
  migrate           apply pending database migrations
  bootstrap-admin   create the first administrator (--username, --display-name, --organization)
  reset-password    break-glass reset of a local password (--username)
  seed-demo         load a fictional exercise scenario (--scenario flood|shelter|power-outage)
  apply-retention   redact personal data of records past the retention period
  healthcheck       probe the local /healthz endpoint (--url)
  version           print the version`

func setupLogger(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func openDB(ctx context.Context, url string, migrateNow bool) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	// The database container may still be starting; retry for a while.
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err = database.Connect(ctx, url)
		if err == nil {
			break
		}
		slog.Warn("database not reachable yet", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		return nil, err
	}
	if migrateNow {
		if err := database.Migrate(ctx, pool, migrations.FS, slog.Default()); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return pool, nil
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogger(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := openDB(ctx, cfg.DatabaseURL, cfg.AutoMigrate)
	if err != nil {
		return err
	}
	defer pool.Close()

	app, err := apihttp.NewApp(cfg, pool)
	if err != nil {
		return err
	}
	go app.Auth.RunSessionJanitor(ctx, 15*time.Minute)
	go func() {
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := app.Sync.CleanupOld(ctx, 30*24*time.Hour); err != nil {
					slog.Error("sync cleanup failed", "error", err)
				}
			}
		}
	}()

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("reliefmesh api listening", "addr", cfg.ListenAddr, "version", health.Version,
			"environment", cfg.Environment, "demo_seed_enabled", cfg.AllowDemoSeed)
		if !cfg.CookieSecure {
			slog.Warn("session cookies are not marked Secure; use HTTPS outside local development")
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func migrate() error {
	setupLogger(slog.LevelInfo)
	url, err := config.LoadDatabaseOnly()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := openDB(ctx, url, true)
	if err != nil {
		return err
	}
	pool.Close()
	fmt.Println("migrations applied")
	return nil
}

func readPassword(envName string) (string, error) {
	if v := os.Getenv(envName); v != "" {
		return v, nil
	}
	st, _ := os.Stdin.Stat()
	if st != nil && (st.Mode()&os.ModeCharDevice) == 0 {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 1024))
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	return "", nil
}

func systemPrincipal(org uuid.UUID) roles.Principal {
	return roles.Principal{OrgID: org, System: true}
}

func bootstrapAdmin(args []string) error {
	fs := flag.NewFlagSet("bootstrap-admin", flag.ExitOnError)
	username := fs.String("username", "admin", "administrator username")
	display := fs.String("display-name", "Administrator", "display name")
	orgName := fs.String("organization", "", "organization name (created if no organization exists)")
	alsoCoordinator := fs.Bool("also-coordinator", false, "also grant the coordinator role (small teams)")
	_ = fs.Parse(args)
	setupLogger(slog.LevelWarn)

	url, err := config.LoadDatabaseOnly()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := openDB(ctx, url, true)
	if err != nil {
		return err
	}
	defer pool.Close()

	errs := validation.Errors{}
	uname := errs.Username("username", *username)
	dname := validation.CleanText(*display)
	errs.Text("display_name", dname, 1, 80, false)
	if err := errs.Err(); err != nil {
		return fmt.Errorf("invalid input: %v", errs)
	}

	var admins int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_active AND 'admin' = ANY(roles)`).Scan(&admins); err != nil {
		return err
	}
	if admins > 0 {
		return errors.New("an active administrator already exists; use the admin UI or `reset-password` instead")
	}

	password, err := readPassword("RELIEFMESH_BOOTSTRAP_PASSWORD")
	if err != nil {
		return err
	}
	generated := false
	if password == "" {
		if password, err = auth.GenerateTemporaryPassword(); err != nil {
			return err
		}
		generated = true
	} else if msg := auth.ValidatePassword(password, uname); msg != "" {
		return errors.New(msg)
	}
	hash, err := auth.HashPassword(ctx, password)
	if err != nil {
		return err
	}
	rs := []string{"admin"}
	if *alsoCoordinator {
		rs = append(rs, "coordinator")
	}

	return database.InTx(ctx, pool, func(tx pgx.Tx) error {
		var org uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM organizations ORDER BY created_at LIMIT 1`).Scan(&org)
		if database.IsNoRows(err) {
			name := validation.CleanText(*orgName)
			if name == "" {
				name = "ReliefMesh Organization"
			}
			org = uuid.New()
			if _, err := tx.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, org, name); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		id := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO users (id, organization_id, username, display_name, roles, password_hash,
			must_change_password) VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, org, uname, dname, rs, hash, generated); err != nil {
			if database.IsUniqueViolation(err, "users_username_key") {
				return fmt.Errorf("username %q is already taken", uname)
			}
			return err
		}
		ev := audit.ForActor(systemPrincipal(org), "user.created", audit.System).On("user", id).
			With("roles", rs).With("bootstrap", true)
		if err := audit.Record(ctx, tx, ev); err != nil {
			return err
		}
		fmt.Printf("Administrator %q created.\n", uname)
		if generated {
			fmt.Printf("Temporary password (shown once, must be changed at first login): %s\n", password)
		}
		return nil
	})
}

func resetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	username := fs.String("username", "", "username to reset")
	_ = fs.Parse(args)
	if *username == "" {
		return errors.New("--username is required")
	}
	setupLogger(slog.LevelWarn)
	url, err := config.LoadDatabaseOnly()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := openDB(ctx, url, false)
	if err != nil {
		return err
	}
	defer pool.Close()
	var id, org uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id, organization_id FROM users WHERE lower(username) = lower($1)`, *username).Scan(&id, &org); err != nil {
		if database.IsNoRows(err) {
			return fmt.Errorf("user %q not found", *username)
		}
		return err
	}
	us := &users.Service{DB: pool}
	temp, err := us.ResetPassword(ctx, systemPrincipal(org), id)
	if err != nil {
		return err
	}
	fmt.Printf("Temporary password for %q (must be changed at next login): %s\n", *username, temp)
	return nil
}

func fullApp(ctx context.Context) (*apihttp.App, *pgxpool.Pool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	setupLogger(slog.LevelWarn)
	pool, err := openDB(ctx, cfg.DatabaseURL, true)
	if err != nil {
		return nil, nil, err
	}
	app, err := apihttp.NewApp(cfg, pool)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	return app, pool, nil
}

func seedDemo(args []string) error {
	fs := flag.NewFlagSet("seed-demo", flag.ExitOnError)
	scenario := fs.String("scenario", "flood", "scenario: flood, shelter or power-outage")
	ifEmpty := fs.Bool("if-not-seeded", false, "exit successfully if the scenario was already loaded")
	_ = fs.Parse(args)
	ctx := context.Background()
	app, pool, err := fullApp(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	var org uuid.UUID
	_ = pool.QueryRow(ctx, `SELECT id FROM organizations ORDER BY created_at LIMIT 1`).Scan(&org)
	if err := app.Demo.Seed(ctx, systemPrincipal(org), *scenario); err != nil {
		if *ifEmpty && strings.Contains(err.Error(), "already_seeded") {
			fmt.Println("scenario already loaded")
			return nil
		}
		return err
	}
	fmt.Printf("Scenario %q loaded. Demo accounts use the password documented in docs/exercise-guide.md.\n", *scenario)
	return nil
}

func applyRetention() error {
	ctx := context.Background()
	app, pool, err := fullApp(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT id FROM organizations`)
	if err != nil {
		return err
	}
	var orgs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		orgs = append(orgs, id)
	}
	rows.Close()
	for _, org := range orgs {
		pv, err := app.Retention.Apply(ctx, systemPrincipal(org))
		if err != nil {
			return err
		}
		fmt.Printf("organization %s: redacted %d requests and %d offers (retention %d days)\n", org, pv.Requests, pv.Offers, pv.RetentionDays)
	}
	return nil
}

func healthcheck(args []string) error {
	fs := flag.NewFlagSet("healthcheck", flag.ExitOnError)
	url := fs.String("url", "http://127.0.0.1:8080/healthz", "health endpoint")
	_ = fs.Parse(args)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(*url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, bufio.NewReader(resp.Body))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}
	return nil
}
