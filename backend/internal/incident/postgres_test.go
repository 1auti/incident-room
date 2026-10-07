package incident_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"incident-room-backend/internal/dbtest"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/service"
)

func TestPostgres_DeclararPersisteIncidenteYEventos(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	svcRepo := service.NewPostgresRepository(pool)
	repo := incident.NewPostgresRepository(pool)

	var userID string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ana', 'ana@x.com', 'h', 'ingeniero') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	svc, err := svcRepo.Create(c, "Pagos", incident.CriticalityImportant)
	if err != nil {
		t.Fatal(err)
	}
	info, err := repo.FindService(c, svc.ID)
	if err != nil || info.Criticality != incident.CriticalityImportant || info.OncallUserID != nil {
		t.Fatalf("FindService = %+v, %v", info, err)
	}

	at := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	inc := incident.Incident{
		Title: "DB caida", Description: "d", ServiceID: svc.ID, Impact: incident.ImpactTotalOutage,
		SuggestedSeverity: incident.SeveritySEV2, Severity: incident.SeveritySEV1, State: incident.StateDeclared,
		DeclaredBy: userID, DeclaredAt: at,
	}
	evs := []incident.TimelineEvent{
		{Type: incident.EventDeclaration, AuthorID: &userID, Data: map[string]string{}, OccurredAt: at},
		{Type: incident.EventSeverityChange, AuthorID: &userID, Data: map[string]string{"from": "SEV2", "to": "SEV1"}, OccurredAt: at},
	}
	got, gotEvs, err := repo.Create(c, inc, evs)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.AssignedTo != nil || !got.DeclaredAt.Equal(at) || got.Severity != incident.SeveritySEV1 {
		t.Errorf("incident = %+v", got)
	}
	if len(gotEvs) != 2 || gotEvs[0].Type != incident.EventDeclaration || gotEvs[1].Type != incident.EventSeverityChange ||
		gotEvs[0].IncidentID != got.ID || gotEvs[1].Data["from"] != "SEV2" || gotEvs[0].Data == nil {
		t.Errorf("events = %+v", gotEvs)
	}

	// Reread with plain SQL.
	var title, sev, sugg, state string
	var assigned *string
	var declaredAt time.Time
	if err := pool.QueryRow(c, `SELECT title, severity, suggested_severity, state, assigned_to, declared_at FROM incidents WHERE id = $1`, got.ID).
		Scan(&title, &sev, &sugg, &state, &assigned, &declaredAt); err != nil {
		t.Fatal(err)
	}
	if title != "DB caida" || sev != "SEV1" || sugg != "SEV2" || state != "declarado" || assigned != nil || !declaredAt.Equal(at) {
		t.Errorf("row = %s %s %s %s %v %v", title, sev, sugg, state, assigned, declaredAt)
	}
	var from, to, typ string
	if err := pool.QueryRow(c, `SELECT type, data->>'from', data->>'to' FROM timeline_events WHERE incident_id = $1 AND type = 'cambio_severidad'`, got.ID).
		Scan(&typ, &from, &to); err != nil || from != "SEV2" || to != "SEV1" {
		t.Errorf("cambio_severidad row = %s %s %s, %v", typ, from, to, err)
	}
	var declData string
	if err := pool.QueryRow(c, `SELECT data::text FROM timeline_events WHERE incident_id = $1 AND type = 'declaracion'`, got.ID).Scan(&declData); err != nil || declData != "{}" {
		t.Errorf("declaracion data = %q, %v; want {}", declData, err)
	}
}

func TestPostgres_ServicioInexistenteEsErrServiceNotFound(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := incident.NewPostgresRepository(pool)
	var userID string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ana', 'ana@x.com', 'h', 'ingeniero') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if _, err := repo.FindService(c, id); !errors.Is(err, incident.ErrServiceNotFound) {
			t.Errorf("FindService %q err = %v, want ErrServiceNotFound", id, err)
		}
		inc := incident.Incident{
			Title: "x", ServiceID: id, Impact: incident.ImpactMinor, SuggestedSeverity: incident.SeveritySEV3,
			Severity: incident.SeveritySEV3, State: incident.StateDeclared, DeclaredBy: userID, DeclaredAt: time.Now().UTC(),
		}
		if _, _, err := repo.Create(c, inc, nil); !errors.Is(err, incident.ErrServiceNotFound) {
			t.Errorf("Create %q err = %v, want ErrServiceNotFound", id, err)
		}
	}
	var n int
	if err := pool.QueryRow(c, `SELECT count(*) FROM incidents`).Scan(&n); err != nil || n != 0 {
		t.Errorf("incidents = %d, %v; want 0", n, err)
	}
}

func TestPostgres_ListarActivosConFiltros(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	svcRepo := service.NewPostgresRepository(pool)
	repo := incident.NewPostgresRepository(pool)

	var userID string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ana', 'ana@x.com', 'h', 'ingeniero') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	pagos, err := svcRepo.Create(c, "Pagos", incident.CriticalityImportant)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := svcRepo.Create(c, "Auth", incident.CriticalityStandard)
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	escalated := base.Add(20 * time.Minute)
	type row struct {
		title    string
		svc      string
		sev      incident.Severity
		state    incident.State
		minutes  int
		escalate *time.Time
	}
	rows := []row{
		{"a", pagos.ID, incident.SeveritySEV1, incident.StateDeclared, 0, &escalated},
		{"b", pagos.ID, incident.SeveritySEV2, incident.StateMitigating, 1, nil},
		{"c", auth.ID, incident.SeveritySEV1, incident.StateResolved, 2, nil},
		{"d", auth.ID, incident.SeveritySEV3, incident.StateAcknowledged, 3, nil},
		{"e", pagos.ID, incident.SeveritySEV1, incident.StateClosed, 4, nil},
	}
	idOf := map[string]string{}
	for _, r := range rows {
		var id string
		if err := pool.QueryRow(c, `
			INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by, declared_at, escalated_at)
			VALUES ($1, $2, 'menor', $3, $3, $4, $5, $6, $7) RETURNING id`,
			r.title, r.svc, string(r.sev), string(r.state), userID, base.Add(time.Duration(r.minutes)*time.Minute), r.escalate,
		).Scan(&id); err != nil {
			t.Fatal(err)
		}
		idOf[r.title] = id
	}
	active := []incident.State{incident.StateDeclared, incident.StateAcknowledged, incident.StateMitigating, incident.StateResolved}

	titles := func(list []incident.Incident) string {
		out := ""
		for _, inc := range list {
			out += inc.Title
		}
		return out
	}
	for _, tc := range []struct {
		name string
		q    incident.ListQuery
		want string
	}{
		{"activos, mas nuevo primero", incident.ListQuery{States: active}, "dcba"},
		{"un estado", incident.ListQuery{States: []incident.State{incident.StateResolved}}, "c"},
		{"severidad", incident.ListQuery{States: active, Severity: incident.SeveritySEV1}, "ca"},
		{"servicio", incident.ListQuery{States: active, ServiceID: auth.ID}, "dc"},
		{"servicio y estado", incident.ListQuery{States: []incident.State{incident.StateMitigating}, ServiceID: pagos.ID}, "b"},
		{"todos los filtros", incident.ListQuery{States: active, Severity: incident.SeveritySEV1, ServiceID: pagos.ID}, "a"},
		{"sin coincidencias", incident.ListQuery{States: active, Severity: incident.SeveritySEV3, ServiceID: pagos.ID}, ""},
		{"servicio inexistente", incident.ListQuery{States: active, ServiceID: "00000000-0000-0000-0000-000000000000"}, ""},
		{"sin estados", incident.ListQuery{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.List(c, tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || titles(got) != tc.want {
				t.Errorf("titles = %q (nil %v), want %q non-nil", titles(got), got == nil, tc.want)
			}
		})
	}

	got, err := repo.List(c, incident.ListQuery{States: active})
	if err != nil {
		t.Fatal(err)
	}
	for _, inc := range got {
		isEscalated := inc.EscalatedAt != nil
		if isEscalated != (inc.Title == "a") {
			t.Errorf("%s escalated_at = %v", inc.Title, inc.EscalatedAt)
		}
		if inc.ID != idOf[inc.Title] || inc.DeclaredAt.Location() != time.UTC {
			t.Errorf("incident = %+v", inc)
		}
	}
	if a := got[3]; a.EscalatedAt == nil || !a.EscalatedAt.Equal(escalated) || a.EscalatedAt.Location() != time.UTC {
		t.Errorf("a escalated_at = %v, want %v in UTC", a.EscalatedAt, escalated)
	}

	if _, err := repo.List(c, incident.ListQuery{States: active, ServiceID: "not-a-uuid"}); !errors.Is(err, incident.ErrInvalidFilter) {
		t.Errorf("malformed service id err = %v, want ErrInvalidFilter", err)
	}
}
