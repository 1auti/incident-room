// Command api is the entry point of the Incident Room backend.
//
// Configuration comes from the environment: DATABASE_URL (required), PORT
// (default 8080), ADMIN_EMAIL and ADMIN_PASSWORD (optional; when both are set
// the first admin is created if none exists, BR-19). Pending SQL migrations
// are applied at startup.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/httpapi"
	"incident-room-backend/internal/user"
	"incident-room-backend/migrations"
)

const sessionTTL = 24 * time.Hour

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
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

	if email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD"); email != "" && password != "" {
		if err := userSvc.EnsureAdmin(ctx, email, password); err != nil {
			if errors.Is(err, user.ErrAdminEmailTaken) {
				return fmt.Errorf("ADMIN_EMAIL %q already belongs to a non-admin user and there is no admin: "+
					"existing users are not promoted automatically; use a different ADMIN_EMAIL or remove that user: %w", email, err)
			}
			return fmt.Errorf("ensure admin: %w", err)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	srv := &http.Server{Addr: ":" + port, Handler: httpapi.New(userSvc, authSvc, slog.New(slog.NewTextHandler(os.Stderr, nil))), ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}
