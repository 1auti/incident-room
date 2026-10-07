package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

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

func TestPostgres_ListByRoleYEsOncallDeAlgunServicio(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	repo := user.NewPostgresRepository(pool)
	ana, err := repo.Create(ctx, user.User{Name: "Ana", Email: "ana@x.com", Role: user.RoleOncall, PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, user.User{Name: "Bruno", Email: "bruno@x.com", Role: user.RoleOncall, PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, user.User{Name: "Carla", Email: "carla@x.com", Role: user.RoleIngeniero, PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListByRole(ctx, user.RoleOncall)
	if err != nil || len(got) != 2 || got[0].Name != "Ana" || got[1].Name != "Bruno" {
		t.Fatalf("ListByRole oncall = %+v, %v; want Ana, Bruno", got, err)
	}
	if got, err := repo.ListByRole(ctx, user.RoleAdmin); err != nil || got == nil || len(got) != 0 {
		t.Errorf("ListByRole admin = %#v, %v; want empty non-nil", got, err)
	}

	if ok, err := repo.IsOncallOfAnyService(ctx, ana.ID); err != nil || ok {
		t.Fatalf("IsOncallOfAnyService before assignment = %v, %v; want false", ok, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO services (name, criticality, oncall_user_id) VALUES ('Pagos', 'estandar', $1)`, ana.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.IsOncallOfAnyService(ctx, ana.ID); err != nil || !ok {
		t.Errorf("IsOncallOfAnyService after assignment = %v, %v; want true", ok, err)
	}
	if ok, err := repo.IsOncallOfAnyService(ctx, "nope"); err != nil || ok {
		t.Errorf("IsOncallOfAnyService malformed id = %v, %v; want false", ok, err)
	}
}

func newOncallWithService(t *testing.T, pool *pgxpool.Pool, repo *user.PostgresRepository) user.User {
	t.Helper()
	ana, err := repo.Create(context.Background(), user.User{Name: "Ana", Email: "ana@x.com", Role: user.RoleOncall, PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO services (name, criticality, oncall_user_id) VALUES ('Pagos', 'estandar', $1)`, ana.ID); err != nil {
		t.Fatal(err)
	}
	return ana
}

func TestBR19_PostgresUpdateRoleGuarded(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	repo := user.NewPostgresRepository(pool)
	ana := newOncallWithService(t, pool, repo)
	free, err := repo.Create(ctx, user.User{Name: "Bruno", Email: "bruno@x.com", Role: user.RoleOncall, PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.UpdateRoleGuarded(ctx, ana.ID, user.RoleAdmin); !errors.Is(err, user.ErrOncallAssigned) {
		t.Errorf("demote assigned on-call err = %v, want ErrOncallAssigned", err)
	}
	if got, _ := repo.FindByID(ctx, ana.ID); got.Role != user.RoleOncall {
		t.Errorf("role after rejected demotion = %s, want oncall", got.Role)
	}
	if err := repo.UpdateRoleGuarded(ctx, ana.ID, user.RoleOncall); err != nil {
		t.Errorf("keeping oncall err = %v, want nil", err)
	}
	if err := repo.UpdateRoleGuarded(ctx, free.ID, user.RoleAdmin); err != nil {
		t.Errorf("demote free on-call err = %v, want nil", err)
	}
	if got, _ := repo.FindByID(ctx, free.ID); got.Role != user.RoleAdmin {
		t.Errorf("role = %s, want admin", got.Role)
	}
	for _, id := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if err := repo.UpdateRoleGuarded(ctx, id, user.RoleAdmin); !errors.Is(err, user.ErrNotFound) {
			t.Errorf("missing user %q err = %v, want ErrNotFound", id, err)
		}
	}
}

// BR-19 vs BR-11: an assignment that is in flight (user locked FOR SHARE, service
// updated, not committed) makes a concurrent demotion wait, and once the
// assignment commits the demotion sees the service and fails.
func TestBR19_PostgresDegradacionEsperaAsignacionEnCurso(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	repo := user.NewPostgresRepository(pool)
	ana, err := repo.Create(ctx, user.User{Name: "Ana", Email: "ana@x.com", Role: user.RoleOncall, PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	var serviceID string
	if err := pool.QueryRow(ctx, `INSERT INTO services (name, criticality) VALUES ('Pagos', 'estandar') RETURNING id`).Scan(&serviceID); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int32
	var role string
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 FOR SHARE`, ana.ID).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE services SET oncall_user_id = $1 WHERE id = $2`, ana.ID, serviceID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- repo.UpdateRoleGuarded(ctx, ana.ID, user.RoleAdmin) }()
	dbtest.WaitBlockedBy(t, pool, pid)
	select {
	case err := <-done:
		t.Fatalf("demotion finished while the assignment was uncommitted: %v", err)
	default:
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, user.ErrOncallAssigned) {
		t.Fatalf("demotion err = %v, want ErrOncallAssigned", err)
	}
	if got, _ := repo.FindByID(ctx, ana.ID); got.Role != user.RoleOncall {
		t.Errorf("role = %s, want oncall", got.Role)
	}
}
