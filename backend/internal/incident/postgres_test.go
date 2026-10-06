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
