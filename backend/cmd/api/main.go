// Command api is the entry point of the Incident Room backend.
//
// Configuration comes from the environment: DATABASE_URL (required), PORT
// (default 8080), ADMIN_EMAIL and ADMIN_PASSWORD (optional; when both are set
// the first admin is created if none exists, BR-19), SLA_SEV1, SLA_SEV2 and
// SLA_SEV3 (acknowledgement deadlines as Go durations, defaults 5m, 15m and 60m,
// BR-03) and ESCALATION_INTERVAL (how often overdue incidents are escalated,
// default 10s, BR-04). Pending SQL migrations are applied at startup.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/httpapi"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/service"
	"incident-room-backend/internal/user"
	"incident-room-backend/migrations"
)

const (
	sessionTTL                = 24 * time.Hour
	defaultEscalationInterval = 10 * time.Second
	shutdownTimeout           = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if err := migrations.Apply(ctx, pool); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	userRepo := user.NewPostgresRepository(pool)
	userSvc := user.NewService(userRepo)
	authSvc := auth.NewService(userRepo, auth.NewPostgresSessionRepository(pool), sessionTTL, time.Now)

	sla, err := slaFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	escalationInterval, err := escalationIntervalFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	serviceMgr := service.NewManager(service.NewPostgresRepository(pool), userRepo, time.Now)
	incidentSvc := incident.NewService(incident.NewPostgresRepository(pool), time.Now, sla)

	if email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD"); email != "" && password != "" {
		if err := userSvc.EnsureAdmin(ctx, email, password); err != nil {
			if errors.Is(err, user.ErrAdminEmailTaken) {
				return fmt.Errorf("ADMIN_EMAIL %q already belongs to a non-admin user and there is no admin: "+
					"existing users are not promoted automatically; use a different ADMIN_EMAIL or remove that user: %w", email, err)
			}
			return fmt.Errorf("ensure admin: %w", err)
		}
	}

	// Deferred after pool.Close, so it runs first: the ticker goroutine must
	// finish before the pool is closed.
	escalationDone := make(chan struct{})
	defer func() {
		stop()
		<-escalationDone
	}()
	go func() {
		defer close(escalationDone)
		runEscalation(ctx, incidentSvc, escalationInterval, logger)
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	srv := &http.Server{Addr: ":" + port, Handler: httpapi.New(userSvc, authSvc, serviceMgr, incidentSvc, logger), ReadHeaderTimeout: 10 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}

// overdueEscalator is the part of the incident service the ticker needs.
type overdueEscalator interface {
	EscalateOverdue(ctx context.Context) (int, error)
}

// runEscalation evaluates overdue incidents every interval until ctx is done.
// A failed evaluation is logged and never stops the server (BR-04). There is
// no evaluation before the first tick.
func runEscalation(ctx context.Context, svc overdueEscalator, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := svc.EscalateOverdue(ctx); err != nil {
				logger.Error("escalate overdue incidents", "err", err, "escalated", n)
			}
		}
	}
}

// slaFromEnv reads SLA_SEV1, SLA_SEV2 and SLA_SEV3 over the defaults (BR-03).
// An invalid or non-positive value is an error, so a typo fails at startup.
func slaFromEnv(getenv func(string) string) (incident.SLA, error) {
	sla := incident.DefaultSLA()
	for _, f := range []struct {
		name string
		dst  *time.Duration
	}{{"SLA_SEV1", &sla.SEV1}, {"SLA_SEV2", &sla.SEV2}, {"SLA_SEV3", &sla.SEV3}} {
		v := getenv(f.name)
		if v == "" {
			continue
		}
		d, err := parsePositiveDuration(f.name, v)
		if err != nil {
			return incident.SLA{}, err
		}
		*f.dst = d
	}
	return sla, nil
}

// escalationIntervalFromEnv reads ESCALATION_INTERVAL.
func escalationIntervalFromEnv(getenv func(string) string) (time.Duration, error) {
	v := getenv("ESCALATION_INTERVAL")
	if v == "" {
		return defaultEscalationInterval, nil
	}
	return parsePositiveDuration("ESCALATION_INTERVAL", v)
}

func parsePositiveDuration(name, v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a valid duration: %w", name, v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %q", name, v)
	}
	return d, nil
}
