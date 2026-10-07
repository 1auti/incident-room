package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

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

func insertUser(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, name, role string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, email, password_hash, role) VALUES ($1, $2, 'h', $3) RETURNING id`,
		name, name+"@x.com", role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPostgres_SetOncallActualizaServicio(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := service.NewPostgresRepository(pool)
	ana := insertUser(t, pool, "ana", "oncall")
	s, err := repo.Create(c, "Pagos", incident.CriticalityStandard)
	if err != nil {
		t.Fatal(err)
	}
	author := insertUser(t, pool, "root", "admin")
	ev := incident.TimelineEvent{Type: incident.EventAssignment, AuthorID: &author, Data: map[string]string{"to": ana}, OccurredAt: time.Now().UTC()}

	got, err := repo.SetOncall(c, s.ID, ana, incident.ActiveStates(), ev)
	if err != nil || got.OncallUserID == nil || *got.OncallUserID != ana || got.Name != "Pagos" {
		t.Fatalf("SetOncall = %+v, %v", got, err)
	}
	var stored *string
	if err := pool.QueryRow(c, `SELECT oncall_user_id FROM services WHERE id = $1`, s.ID).Scan(&stored); err != nil || stored == nil || *stored != ana {
		t.Errorf("stored oncall = %v, %v", stored, err)
	}

	for _, id := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if _, err := repo.SetOncall(c, id, ana, incident.ActiveStates(), ev); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("SetOncall %q err = %v, want ErrNotFound", id, err)
		}
	}
	if _, err := repo.SetOncall(c, s.ID, "00000000-0000-0000-0000-000000000000", incident.ActiveStates(), ev); !errors.Is(err, service.ErrInvalidOncall) {
		t.Errorf("unknown user err = %v, want ErrInvalidOncall", err)
	}
	if err := pool.QueryRow(c, `SELECT oncall_user_id FROM services WHERE id = $1`, s.ID).Scan(&stored); err != nil || *stored != ana {
		t.Errorf("service changed by a rejected assignment: %v, %v", stored, err)
	}
}

func TestPostgres_SetOncallReasignaSoloActivos(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := service.NewPostgresRepository(pool)
	ana := insertUser(t, pool, "ana", "oncall")
	bruno := insertUser(t, pool, "bruno", "oncall")
	admin := insertUser(t, pool, "root", "admin")
	pagos, err := repo.Create(c, "Pagos", incident.CriticalityStandard)
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.Create(c, "Auth", incident.CriticalityStandard)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	ev := incident.TimelineEvent{Type: incident.EventAssignment, AuthorID: &admin, Data: map[string]string{"to": ana}, OccurredAt: at}
	if _, err := repo.SetOncall(c, pagos.ID, ana, incident.ActiveStates(), ev); err != nil {
		t.Fatal(err)
	}

	insert := func(svcID, state string, assigned *string) string {
		var id string
		if err := pool.QueryRow(c, `
			INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by, assigned_to, declared_at)
			VALUES ('x', $1, 'menor', 'SEV3', 'SEV3', $2, $3, $4, $5) RETURNING id`,
			svcID, state, admin, assigned, at).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	declared := insert(pagos.ID, "declarado", &ana)
	resolved := insert(pagos.ID, "resuelto", &ana)
	closed := insert(pagos.ID, "cerrado", &ana)
	unassigned := insert(pagos.ID, "mitigando", nil)
	foreign := insert(other.ID, "declarado", &ana) // another service: untouched

	changeAt := at.Add(time.Hour)
	ev = incident.TimelineEvent{Type: incident.EventAssignment, AuthorID: &admin, Data: map[string]string{"to": bruno}, OccurredAt: changeAt}
	if _, err := repo.SetOncall(c, pagos.ID, bruno, incident.ActiveStates(), ev); err != nil {
		t.Fatal(err)
	}

	assignedTo := func(id string) string {
		var a *string
		if err := pool.QueryRow(c, `SELECT assigned_to FROM incidents WHERE id = $1`, id).Scan(&a); err != nil {
			t.Fatal(err)
		}
		if a == nil {
			return ""
		}
		return *a
	}
	count := func(id string) int {
		var n int
		if err := pool.QueryRow(c, `SELECT count(*) FROM timeline_events WHERE incident_id = $1 AND type = 'asignacion'`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, id := range []string{declared, resolved, unassigned} {
		if assignedTo(id) != bruno || count(id) != 1 {
			t.Errorf("active incident %s assigned_to = %s events = %d, want Bruno and 1", id, assignedTo(id), count(id))
		}
	}
	for id, want := range map[string]string{closed: ana, foreign: ana} {
		if assignedTo(id) != want || count(id) != 0 {
			t.Errorf("incident %s assigned_to = %s events = %d, want untouched (%s, 0)", id, assignedTo(id), count(id), want)
		}
	}

	var from, to, author string
	var occurred time.Time
	if err := pool.QueryRow(c, `SELECT data->>'from', data->>'to', author_id, occurred_at FROM timeline_events WHERE incident_id = $1 AND type = 'asignacion'`, declared).
		Scan(&from, &to, &author, &occurred); err != nil || from != ana || to != bruno || author != admin || !occurred.Equal(changeAt) {
		t.Errorf("asignacion = from %s to %s author %s at %v, %v", from, to, author, occurred, err)
	}
	if err := pool.QueryRow(c, `SELECT data->>'from' FROM timeline_events WHERE incident_id = $1 AND type = 'asignacion'`, unassigned).Scan(&from); err != nil || from != "" {
		t.Errorf("unassigned from = %q, %v; want empty", from, err)
	}

	// Assigning the current on-call again is a no-op: no new events.
	if _, err := repo.SetOncall(c, pagos.ID, bruno, incident.ActiveStates(), ev); err != nil {
		t.Fatal(err)
	}
	if count(declared) != 1 {
		t.Errorf("same on-call added events: %d", count(declared))
	}
}
