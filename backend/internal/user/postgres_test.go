package user_test

import (
	"context"
	"errors"
	"testing"

	"incident-room-backend/internal/dbtest"
	"incident-room-backend/internal/user"
)

func TestPostgresRepository(t *testing.T) {
	ctx := context.Background()
	repo := user.NewPostgresRepository(dbtest.Pool(t))

	if ok, err := repo.ExistsAdmin(ctx); err != nil || ok {
		t.Fatalf("ExistsAdmin on empty = %v, %v", ok, err)
	}
	created, err := repo.Create(ctx, user.User{Name: "Ana", Email: "ana@x.com", Role: user.RoleIngeniero, PasswordHash: "h"})
	if err != nil || created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("Create = %+v, %v", created, err)
	}
	// BR-19: unique email maps to ErrEmailTaken and no second row is created.
	if _, err := repo.Create(ctx, user.User{Name: "Otra", Email: "ana@x.com", Role: user.RoleIngeniero, PasswordHash: "h"}); !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("duplicate err = %v, want ErrEmailTaken", err)
	}

	byEmail, err := repo.FindByEmail(ctx, "ana@x.com")
	if err != nil || byEmail.ID != created.ID || byEmail.PasswordHash != "h" {
		t.Fatalf("FindByEmail = %+v, %v", byEmail, err)
	}
	if _, err := repo.FindByID(ctx, created.ID); err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if _, err := repo.FindByEmail(ctx, "no@x.com"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("FindByEmail missing err = %v", err)
	}
	// A malformed id is "not found", not an internal error.
	if _, err := repo.FindByID(ctx, "nope"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("FindByID malformed err = %v", err)
	}

	if err := repo.UpdateRole(ctx, created.ID, user.RoleAdmin); err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if got, _ := repo.FindByID(ctx, created.ID); got.Role != user.RoleAdmin {
		t.Errorf("role = %s", got.Role)
	}
	if err := repo.UpdateRole(ctx, "00000000-0000-0000-0000-000000000000", user.RoleOncall); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("UpdateRole missing err = %v", err)
	}
	if ok, err := repo.ExistsAdmin(ctx); err != nil || !ok {
		t.Errorf("ExistsAdmin = %v, %v", ok, err)
	}
}
