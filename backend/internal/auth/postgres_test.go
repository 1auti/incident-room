package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/dbtest"
	"incident-room-backend/internal/user"
)

func TestPostgresSessionRepository(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	u, err := user.NewPostgresRepository(pool).Create(ctx, user.User{Name: "Ana", Email: "ana@x.com", Role: user.RoleIngeniero, PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	repo := auth.NewPostgresSessionRepository(pool)

	if _, err := repo.FindByTokenHash(ctx, "missing"); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	want := auth.Session{TokenHash: "abc", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)}
	if err := repo.Create(ctx, want); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.FindByTokenHash(ctx, "abc")
	if err != nil || got.UserID != u.ID || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("Find = %+v, %v", got, err)
	}
}
