package incident_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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

// seededDB holds the ids of the user and service the SQL tests share.
type seededDB struct {
	userID, serviceID string
}

func seedBase(t *testing.T, pool *pgxpool.Pool) seededDB {
	t.Helper()
	c := context.Background()
	var s seededDB
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ana', 'ana@x.com', 'h', 'oncall') RETURNING id`).Scan(&s.userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(c, `INSERT INTO services (name, criticality) VALUES ('Pagos', 'importante') RETURNING id`).Scan(&s.serviceID); err != nil {
		t.Fatal(err)
	}
	return s
}

func insertIncident(t *testing.T, pool *pgxpool.Pool, s seededDB, state incident.State, sev incident.Severity, declaredAt time.Time) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by, declared_at)
		VALUES ('x', $1, 'menor', $2, $2, $3, $4, $5) RETURNING id`,
		s.serviceID, string(sev), string(state), s.userID, declaredAt).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func countEvents(t *testing.T, pool *pgxpool.Pool, incidentID string, typ incident.EventType) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM timeline_events WHERE incident_id = $1 AND type = $2`, incidentID, string(typ)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPostgres_TransicionFijaAcknowledgedAtYEvento(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := incident.NewPostgresRepository(pool)
	s := seedBase(t, pool)
	declared := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	id := insertIncident(t, pool, s, incident.StateDeclared, incident.SeveritySEV2, declared)

	got, err := repo.Get(c, id)
	if err != nil || got.State != incident.StateDeclared || got.AcknowledgedAt != nil || !got.DeclaredAt.Equal(declared) {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	for _, bad := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if _, err := repo.Get(c, bad); !errors.Is(err, incident.ErrNotFound) {
			t.Errorf("Get %q err = %v, want ErrNotFound", bad, err)
		}
	}

	at := declared.Add(14*time.Minute + 59*time.Second)
	ev := incident.TimelineEvent{Type: incident.EventStateChange, AuthorID: &s.userID, Data: map[string]string{"from": "declarado", "to": "reconocido"}, OccurredAt: at}
	updated, err := repo.UpdateState(c, id, incident.StateDeclared, incident.StateAcknowledged, incident.StateUpdate{AcknowledgedAt: &at}, ev)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != incident.StateAcknowledged || updated.AcknowledgedAt == nil || !updated.AcknowledgedAt.Equal(at) || updated.AcknowledgedAt.Location() != time.UTC {
		t.Errorf("updated = %+v", updated)
	}
	var from, to string
	var author *string
	if err := pool.QueryRow(c, `SELECT data->>'from', data->>'to', author_id FROM timeline_events WHERE incident_id = $1 AND type = 'cambio_estado'`, id).
		Scan(&from, &to, &author); err != nil || from != "declarado" || to != "reconocido" || author == nil || *author != s.userID {
		t.Errorf("cambio_estado row = %s %s %v, %v", from, to, author, err)
	}

	// The incident is no longer declarado: the conditional update touches nothing.
	if _, err := repo.UpdateState(c, id, incident.StateDeclared, incident.StateAcknowledged, incident.StateUpdate{AcknowledgedAt: &at}, ev); !errors.Is(err, incident.ErrInvalidTransition) {
		t.Errorf("second UpdateState err = %v, want ErrInvalidTransition", err)
	}
	if n := countEvents(t, pool, id, incident.EventStateChange); n != 1 {
		t.Errorf("cambio_estado events = %d, want 1 (rolled back)", n)
	}
	if _, err := repo.UpdateState(c, "00000000-0000-0000-0000-000000000000", incident.StateDeclared, incident.StateAcknowledged, incident.StateUpdate{AcknowledgedAt: &at}, ev); !errors.Is(err, incident.ErrNotFound) {
		t.Errorf("unknown id err = %v, want ErrNotFound", err)
	}
	if _, err := repo.UpdateState(c, "nope", incident.StateDeclared, incident.StateAcknowledged, incident.StateUpdate{AcknowledgedAt: &at}, ev); !errors.Is(err, incident.ErrNotFound) {
		t.Errorf("malformed id err = %v, want ErrNotFound", err)
	}
}

func TestPostgres_EscalarEsCondicionalYUnico(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := incident.NewPostgresRepository(pool)
	s := seedBase(t, pool)
	declared := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	overdue := insertIncident(t, pool, s, incident.StateDeclared, incident.SeveritySEV2, declared)
	acked := insertIncident(t, pool, s, incident.StateAcknowledged, incident.SeveritySEV2, declared)

	pending, err := repo.ListPendingEscalation(c, incident.StateDeclared)
	if err != nil || len(pending) != 1 || pending[0].ID != overdue {
		t.Fatalf("pending = %+v, %v; want only the declarado one", pending, err)
	}

	at := declared.Add(15*time.Minute + time.Second)
	ev := incident.TimelineEvent{Type: incident.EventEscalation, Data: map[string]string{}, OccurredAt: at}

	// A stale view of the severity (it was raised meanwhile) does not escalate.
	stale := pending[0]
	stale.Severity = incident.SeveritySEV3
	if ok, err := repo.Escalate(c, stale, ev); err != nil || ok {
		t.Fatalf("stale severity Escalate = %v, %v; want false", ok, err)
	}
	if n := countEvents(t, pool, overdue, incident.EventEscalation); n != 0 {
		t.Fatalf("events after stale escalate = %d, want 0", n)
	}

	if ok, err := repo.Escalate(c, pending[0], ev); err != nil || !ok {
		t.Fatalf("Escalate = %v, %v; want true", ok, err)
	}
	// Second and concurrent-looking attempts add nothing.
	if ok, err := repo.Escalate(c, pending[0], ev); err != nil || ok {
		t.Fatalf("second Escalate = %v, %v; want false", ok, err)
	}
	// Not declarado: never escalates.
	if ok, err := repo.Escalate(c, incident.Incident{ID: acked, Severity: incident.SeveritySEV2}, ev); err != nil || ok {
		t.Fatalf("Escalate on reconocido = %v, %v; want false", ok, err)
	}

	if n := countEvents(t, pool, overdue, incident.EventEscalation); n != 1 {
		t.Errorf("escalado events = %d, want exactly 1", n)
	}
	if n := countEvents(t, pool, acked, incident.EventEscalation); n != 0 {
		t.Errorf("escalado events on reconocido = %d, want 0", n)
	}
	var author *string
	var escalatedAt time.Time
	if err := pool.QueryRow(c, `SELECT e.author_id, i.escalated_at FROM timeline_events e JOIN incidents i ON i.id = e.incident_id WHERE e.incident_id = $1 AND e.type = 'escalado'`, overdue).
		Scan(&author, &escalatedAt); err != nil || author != nil || !escalatedAt.Equal(at) {
		t.Errorf("escalado row author = %v escalated_at = %v, %v; want NULL and %v", author, escalatedAt, err, at)
	}
	if pending, err := repo.ListPendingEscalation(c, incident.StateDeclared); err != nil || len(pending) != 0 {
		t.Errorf("pending after escalation = %+v, %v; want none", pending, err)
	}
}

func TestPostgres_TimelineSoloInsertaYOrdena(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := incident.NewPostgresRepository(pool)
	s := seedBase(t, pool)
	declared := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	id := insertIncident(t, pool, s, incident.StateMitigating, incident.SeveritySEV2, declared)

	// Inserted out of chronological order: the listing sorts by occurred_at (BR-09).
	late := incident.TimelineEvent{Type: incident.EventNote, AuthorID: &s.userID, Body: "tarde", OccurredAt: declared.Add(20 * time.Minute)}
	system := incident.TimelineEvent{Type: incident.EventEscalation, OccurredAt: declared.Add(15 * time.Minute)}
	for _, ev := range []incident.TimelineEvent{late, system} {
		if _, err := repo.AppendEvent(c, id, ev); err != nil {
			t.Fatal(err)
		}
	}
	events, err := repo.ListEvents(c, id)
	if err != nil || len(events) != 2 {
		t.Fatalf("ListEvents = %+v, %v", events, err)
	}
	if events[0].Type != incident.EventEscalation || events[0].AuthorID != nil || events[0].AuthorName != nil {
		t.Errorf("first = %+v, want the system escalado without author", events[0])
	}
	if events[1].Body != "tarde" || events[1].AuthorName == nil || *events[1].AuthorName != "Ana" ||
		!events[1].OccurredAt.Equal(late.OccurredAt) || events[1].OccurredAt.Location() != time.UTC || events[1].Data == nil {
		t.Errorf("second = %+v, want the note by Ana in UTC", events[1])
	}

	for _, bad := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if _, err := repo.AppendEvent(c, bad, late); !errors.Is(err, incident.ErrNotFound) {
			t.Errorf("AppendEvent %q err = %v, want ErrNotFound", bad, err)
		}
	}
	if _, err := repo.ListEvents(c, "nope"); !errors.Is(err, incident.ErrNotFound) {
		t.Errorf("ListEvents malformed id err = %v, want ErrNotFound", err)
	}
}

func TestPostgres_CambioSeveridadEsCondicional(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := incident.NewPostgresRepository(pool)
	s := seedBase(t, pool)
	declared := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	mutable := []incident.State{incident.StateDeclared, incident.StateAcknowledged, incident.StateMitigating}
	ev := incident.TimelineEvent{Type: incident.EventSeverityChange, AuthorID: &s.userID, Data: map[string]string{"from": "SEV2", "to": "SEV1"}, OccurredAt: declared.Add(time.Hour)}

	id := insertIncident(t, pool, s, incident.StateMitigating, incident.SeveritySEV2, declared)
	got, err := repo.ChangeSeverity(c, id, incident.SeveritySEV2, incident.SeveritySEV1, mutable, ev)
	if err != nil || got.Severity != incident.SeveritySEV1 || got.SuggestedSeverity != incident.SeveritySEV2 {
		t.Fatalf("ChangeSeverity = %+v, %v; want SEV1 keeping suggested SEV2", got, err)
	}
	if n := countEvents(t, pool, id, incident.EventSeverityChange); n != 1 {
		t.Errorf("cambio_severidad events = %d, want 1", n)
	}
	// Severity no longer SEV2: a stale change touches nothing.
	if _, err := repo.ChangeSeverity(c, id, incident.SeveritySEV2, incident.SeveritySEV3, mutable, ev); !errors.Is(err, incident.ErrSeverityLocked) {
		t.Errorf("stale from err = %v, want ErrSeverityLocked", err)
	}

	resolved := insertIncident(t, pool, s, incident.StateResolved, incident.SeveritySEV1, declared)
	if _, err := repo.ChangeSeverity(c, resolved, incident.SeveritySEV1, incident.SeveritySEV2, mutable, ev); !errors.Is(err, incident.ErrSeverityLocked) {
		t.Errorf("resuelto err = %v, want ErrSeverityLocked", err)
	}
	if inc, _ := repo.Get(c, resolved); inc.Severity != incident.SeveritySEV1 || countEvents(t, pool, resolved, incident.EventSeverityChange) != 0 {
		t.Errorf("resuelto changed: %+v", inc)
	}
	for _, bad := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if _, err := repo.ChangeSeverity(c, bad, incident.SeveritySEV2, incident.SeveritySEV1, mutable, ev); !errors.Is(err, incident.ErrNotFound) {
			t.Errorf("ChangeSeverity %q err = %v, want ErrNotFound", bad, err)
		}
	}
}

func TestPostgres_ResolverYCerrarGuardanCamposYEvento(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := incident.NewPostgresRepository(pool)
	s := seedBase(t, pool)
	declared := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	id := insertIncident(t, pool, s, incident.StateMitigating, incident.SeveritySEV2, declared)

	resolvedAt, cause := declared.Add(40*time.Minute), "Pool de conexiones agotado por fuga en v2.3"
	ev := incident.TimelineEvent{Type: incident.EventStateChange, AuthorID: &s.userID, Body: cause,
		Data: map[string]string{"from": "mitigando", "to": "resuelto"}, OccurredAt: resolvedAt}
	got, err := repo.UpdateState(c, id, incident.StateMitigating, incident.StateResolved,
		incident.StateUpdate{ResolvedAt: &resolvedAt, RootCause: &cause}, ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != incident.StateResolved || got.RootCause == nil || *got.RootCause != cause ||
		got.ResolvedAt == nil || !got.ResolvedAt.Equal(resolvedAt) || got.ResolvedAt.Location() != time.UTC || got.ClosedAt != nil {
		t.Errorf("resolved = %+v", got)
	}
	var body string
	if err := pool.QueryRow(c, `SELECT body FROM timeline_events WHERE incident_id = $1 AND type = 'cambio_estado'`, id).Scan(&body); err != nil || body != cause {
		t.Errorf("cambio_estado body = %q, %v", body, err)
	}

	// T4 leaves root_cause and resolved_at as they were (nil fields are untouched).
	closedAt := declared.Add(2 * time.Hour)
	ev.Body, ev.Data, ev.OccurredAt = "", map[string]string{"from": "resuelto", "to": "cerrado"}, closedAt
	got, err = repo.UpdateState(c, id, incident.StateResolved, incident.StateClosed, incident.StateUpdate{ClosedAt: &closedAt}, ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != incident.StateClosed || got.ClosedAt == nil || !got.ClosedAt.Equal(closedAt) ||
		got.RootCause == nil || *got.RootCause != cause || got.ResolvedAt == nil || !got.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("closed = %+v", got)
	}
	if n := countEvents(t, pool, id, incident.EventStateChange); n != 2 {
		t.Errorf("cambio_estado events = %d, want 2", n)
	}
}

func TestPostgres_PostmortemAprobado(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := incident.NewPostgresRepository(pool)
	s := seedBase(t, pool)
	declared := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	insertPostmortem := func(incidentID, status string) {
		t.Helper()
		var approvedBy *string
		var approvedAt *time.Time
		if status == "aprobado" {
			approvedBy, approvedAt = &s.userID, &declared
		}
		if _, err := pool.Exec(c, `INSERT INTO postmortems (incident_id, content, status, generated_by_llm, approved_by, approved_at, updated_at)
			VALUES ($1, 'pm', $2, true, $3, $4, $5)`, incidentID, status, approvedBy, approvedAt, declared); err != nil {
			t.Fatal(err)
		}
	}

	none := insertIncident(t, pool, s, incident.StateResolved, incident.SeveritySEV1, declared)
	draft := insertIncident(t, pool, s, incident.StateResolved, incident.SeveritySEV1, declared)
	approved := insertIncident(t, pool, s, incident.StateResolved, incident.SeveritySEV1, declared)
	insertPostmortem(draft, "borrador")
	insertPostmortem(approved, "aprobado")

	for id, want := range map[string]bool{none: false, draft: false, approved: true} {
		if got, err := repo.HasApprovedPostmortem(c, id); err != nil || got != want {
			t.Errorf("HasApprovedPostmortem(%s) = %v, %v; want %v", id, got, err, want)
		}
	}
	// At most one postmortem per incident (BR-13) and an approved one records who and when.
	if _, err := pool.Exec(c, `INSERT INTO postmortems (incident_id, content, status, generated_by_llm, updated_at)
		VALUES ($1, 'otro', 'borrador', true, $2)`, draft, declared); err == nil {
		t.Error("second postmortem for the same incident was accepted")
	}
	if _, err := pool.Exec(c, `INSERT INTO postmortems (incident_id, content, status, generated_by_llm, updated_at)
		VALUES ($1, 'pm', 'aprobado', true, $2)`, none, declared); err == nil {
		t.Error("approved postmortem without approved_by/approved_at was accepted")
	}
}
