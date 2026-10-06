package service_test

import (
	"context"
	"errors"
	"testing"

	"incident-room-backend/internal/dbtest"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/service"
)

func TestPostgres_CrearListarEditarBorrar(t *testing.T) {
	c := context.Background()
	repo := service.NewPostgresRepository(dbtest.Pool(t))

	if list, err := repo.List(c); err != nil || list == nil || len(list) != 0 {
		t.Fatalf("empty List = %#v, %v; want empty non-nil", list, err)
	}
	b, err := repo.Create(c, "beta", incident.CriticalityStandard)
	if err != nil || b.ID == "" || b.OncallUserID != nil {
		t.Fatalf("Create = %+v, %v", b, err)
	}
	if _, err := repo.Create(c, "Alfa", incident.CriticalityCritical); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(c)
	if err != nil || len(list) != 2 || list[0].Name != "Alfa" || list[1].Name != "beta" {
		t.Fatalf("List = %+v, %v; want ordered by name ignoring case", list, err)
	}

	got, err := repo.Update(c, b.ID, "Gamma", incident.CriticalityImportant)
	if err != nil || got.Name != "Gamma" || got.Criticality != incident.CriticalityImportant {
		t.Fatalf("Update = %+v, %v", got, err)
	}
	if err := repo.Delete(c, b.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ := repo.List(c); len(list) != 1 {
		t.Errorf("services after delete = %d, want 1", len(list))
	}
}

func TestPostgres_NombreDuplicadoEsErrNameTaken(t *testing.T) {
	c := context.Background()
	repo := service.NewPostgresRepository(dbtest.Pool(t))
	if _, err := repo.Create(c, "Pagos", incident.CriticalityStandard); err != nil {
		t.Fatal(err)
	}
	other, err := repo.Create(c, "Auth", incident.CriticalityStandard)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Pagos", "PAGOS"} {
		if _, err := repo.Create(c, name, incident.CriticalityStandard); !errors.Is(err, service.ErrNameTaken) {
			t.Errorf("Create %q err = %v, want ErrNameTaken", name, err)
		}
		if _, err := repo.Update(c, other.ID, name, incident.CriticalityStandard); !errors.Is(err, service.ErrNameTaken) {
			t.Errorf("Update %q err = %v, want ErrNameTaken", name, err)
		}
	}
	if list, _ := repo.List(c); len(list) != 2 {
		t.Errorf("services = %d, want 2", len(list))
	}
}

func TestPostgres_IDInvalidoOInexistenteEsErrNotFound(t *testing.T) {
	c := context.Background()
	repo := service.NewPostgresRepository(dbtest.Pool(t))
	for _, id := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if _, err := repo.Update(c, id, "X", incident.CriticalityStandard); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("Update %q err = %v, want ErrNotFound", id, err)
		}
		if err := repo.Delete(c, id); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("Delete %q err = %v, want ErrNotFound", id, err)
		}
	}
}

func TestPostgres_FKBloqueaBajaEsErrInUse(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := service.NewPostgresRepository(pool)
	s, err := repo.Create(c, "Pagos", incident.CriticalityStandard)
	if err != nil {
		t.Fatal(err)
	}
	// Stand-in for the incidents/runbooks tables of UC-02 and UC-07.
	if _, err := pool.Exec(c, `CREATE TABLE dep (service_id UUID NOT NULL REFERENCES services (id))`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(c, `INSERT INTO dep (service_id) VALUES ($1)`, s.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(c, s.ID); !errors.Is(err, service.ErrInUse) {
		t.Fatalf("Delete err = %v, want ErrInUse", err)
	}
	if list, _ := repo.List(c); len(list) != 1 {
		t.Errorf("service removed despite reference")
	}
}

func TestPostgres_HasIncidentsDetectaIncidentes(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := service.NewPostgresRepository(pool)
	s, err := repo.Create(c, "Pagos", incident.CriticalityStandard)
	if err != nil {
		t.Fatal(err)
	}
	if has, err := repo.HasIncidents(c, s.ID); err != nil || has {
		t.Fatalf("HasIncidents without incidents = %v, %v; want false", has, err)
	}

	var userID string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ana', 'ana@x.com', 'h', 'ingeniero') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(c, `INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by, declared_at)
		VALUES ('x', $1, 'menor', 'SEV3', 'SEV3', 'declarado', $2, now())`, s.ID, userID); err != nil {
		t.Fatal(err)
	}
	if has, err := repo.HasIncidents(c, s.ID); err != nil || !has {
		t.Errorf("HasIncidents with incident = %v, %v; want true", has, err)
	}
	if _, err := repo.HasIncidents(c, "nope"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("HasIncidents nope err = %v, want ErrNotFound", err)
	}
}
