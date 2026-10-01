// Package dbtest provides a clean Postgres pool for repository tests.
package dbtest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"incident-room-backend/migrations"
)

// Pool connects using TEST DATABASE_URL from the environment, applies the
// migrations and empties the tables. It skips the test when DATABASE_URL is unset.
// Use a disposable database: tables are truncated.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE sessions, users CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}
