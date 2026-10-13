package postmortem_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"incident-room-backend/internal/dbtest"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/postmortem"
)

func TestPostgres_PostmortemCondicionalABorrador(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := postmortem.NewPostgresRepository(pool)
	incidents := incident.NewPostgresRepository(pool)

	var adminID, serviceID, incidentID string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ada', 'ada@x.com', 'h', 'admin') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(c, `INSERT INTO services (name, criticality) VALUES ('Pagos', 'critica') RETURNING id`).Scan(&serviceID); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	if err := pool.QueryRow(c, `INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by, declared_at)
		VALUES ('x', $1, 'caida_total', 'SEV1', 'SEV1', 'resuelto', $2, $3) RETURNING id`, serviceID, adminID, at).Scan(&incidentID); err != nil {
		t.Fatal(err)
	}
	ev := func(action string) incident.TimelineEvent {
		return incident.TimelineEvent{Type: incident.EventPostmortem, AuthorID: &adminID, Body: "Postmortem " + action, Data: map[string]string{"action": action}, OccurredAt: at}
	}
	countEvents := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(c, `SELECT count(*) FROM timeline_events WHERE incident_id = $1 AND type = 'postmortem'`, incidentID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if _, err := repo.Find(c, incidentID); !errors.Is(err, postmortem.ErrNotFound) {
		t.Errorf("Find before create err = %v, want ErrNotFound", err)
	}
	for _, id := range []string{incidentID, "nope"} {
		if _, err := repo.UpdateContent(c, id, "x", at); !errors.Is(err, postmortem.ErrNotFound) {
			t.Errorf("UpdateContent(%s) without postmortem err = %v, want ErrNotFound", id, err)
		}
	}

	draft := postmortem.Postmortem{IncidentID: incidentID, Content: "v1", Status: postmortem.StatusDraft, GeneratedByLLM: true, UpdatedAt: at}
	created, err := repo.CreateDraft(c, draft, ev("generado"))
	if err != nil || created.ID == "" || created.Status != postmortem.StatusDraft || !created.GeneratedByLLM || created.ApprovedBy != nil {
		t.Fatalf("created = %+v, %v", created, err)
	}
	// At most one per incident: a second creation is a conflict and rolls back its event.
	if _, err := repo.CreateDraft(c, draft, ev("generado")); !errors.Is(err, postmortem.ErrConflict) {
		t.Errorf("second CreateDraft err = %v, want ErrConflict", err)
	}
	if n := countEvents(); n != 1 {
		t.Errorf("postmortem events = %d, want 1", n)
	}

	later := at.Add(time.Hour)
	if pm, err := repo.ReplaceDraft(c, incidentID, "v2", later, ev("regenerado")); err != nil || pm.Content != "v2" || !pm.UpdatedAt.Equal(later) {
		t.Errorf("ReplaceDraft = %+v, %v", pm, err)
	}
	if pm, err := repo.UpdateContent(c, incidentID, "v3", later); err != nil || pm.Content != "v3" {
		t.Errorf("UpdateContent = %+v, %v", pm, err)
	}
	if n := countEvents(); n != 2 {
		t.Errorf("postmortem events = %d, want 2 (editing adds none)", n)
	}

	approvedAt := at.Add(2 * time.Hour)
	approved, err := repo.Approve(c, incidentID, adminID, approvedAt, ev("aprobado"))
	if err != nil || approved.Status != postmortem.StatusApproved || approved.ApprovedBy == nil || *approved.ApprovedBy != adminID ||
		approved.ApprovedAt == nil || !approved.ApprovedAt.Equal(approvedAt) || approved.ApprovedAt.Location() != time.UTC {
		t.Fatalf("approved = %+v, %v", approved, err)
	}
	// BR-07: the incident side now sees an approved postmortem.
	if ok, err := incidents.HasApprovedPostmortem(c, incidentID); err != nil || !ok {
		t.Errorf("HasApprovedPostmortem = %v, %v; want true", ok, err)
	}

	// Approved is immutable: every write is refused and leaves no event.
	if _, err := repo.ReplaceDraft(c, incidentID, "v4", later, ev("regenerado")); !errors.Is(err, postmortem.ErrApproved) {
		t.Errorf("ReplaceDraft approved err = %v, want ErrApproved", err)
	}
	if _, err := repo.UpdateContent(c, incidentID, "v4", later); !errors.Is(err, postmortem.ErrApproved) {
		t.Errorf("UpdateContent approved err = %v, want ErrApproved", err)
	}
	if _, err := repo.Approve(c, incidentID, adminID, later, ev("aprobado")); !errors.Is(err, postmortem.ErrApproved) {
		t.Errorf("second Approve err = %v, want ErrApproved", err)
	}
	if pm, _ := repo.Find(c, incidentID); pm.Content != "v3" || countEvents() != 3 {
		t.Errorf("approved postmortem changed: %+v, events %d", pm, countEvents())
	}
}

func TestPostgres_GenerarRequiereIncidenteResueltoEnLaTransaccion(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := postmortem.NewPostgresRepository(pool)
	var adminID, serviceID string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ada', 'ada@x.com', 'h', 'admin') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(c, `INSERT INTO services (name, criticality) VALUES ('Pagos', 'critica') RETURNING id`).Scan(&serviceID); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	resolved := func() string {
		t.Helper()
		var id string
		if err := pool.QueryRow(c, `INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by, declared_at)
			VALUES ('x', $1, 'menor', 'SEV3', 'SEV3', 'resuelto', $2, $3) RETURNING id`, serviceID, adminID, at).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	ev := incident.TimelineEvent{Type: incident.EventPostmortem, AuthorID: &adminID, Data: map[string]string{"action": "generado"}, OccurredAt: at}
	draft := func(id string) postmortem.Postmortem {
		return postmortem.Postmortem{IncidentID: id, Content: "v1", Status: postmortem.StatusDraft, GeneratedByLLM: true, UpdatedAt: at}
	}
	withDraft, withoutDraft := resolved(), resolved()
	if _, err := repo.CreateDraft(c, draft(withDraft), ev); err != nil {
		t.Fatal(err)
	}

	// Both incidents are closed between the service's check and the write (the LLM call).
	if _, err := pool.Exec(c, `UPDATE incidents SET state = 'cerrado' WHERE id IN ($1, $2)`, withDraft, withoutDraft); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceDraft(c, withDraft, "v2", at, ev); !errors.Is(err, postmortem.ErrIncidentNotResolved) {
		t.Errorf("ReplaceDraft on cerrado err = %v, want ErrIncidentNotResolved", err)
	}
	if _, err := repo.CreateDraft(c, draft(withoutDraft), ev); !errors.Is(err, postmortem.ErrIncidentNotResolved) {
		t.Errorf("CreateDraft on cerrado err = %v, want ErrIncidentNotResolved", err)
	}
	counts := func(id string) (pms, events int, content string) {
		t.Helper()
		if err := pool.QueryRow(c, `SELECT (SELECT count(*) FROM postmortems WHERE incident_id = $1),
			(SELECT count(*) FROM timeline_events WHERE incident_id = $1),
			coalesce((SELECT content FROM postmortems WHERE incident_id = $1), '')`, id).Scan(&pms, &events, &content); err != nil {
			t.Fatal(err)
		}
		return pms, events, content
	}
	if pms, events, content := counts(withDraft); pms != 1 || events != 1 || content != "v1" {
		t.Errorf("draft incident: %d postmortems, %d events, content %q; want the first draft only", pms, events, content)
	}
	if pms, events, _ := counts(withoutDraft); pms != 0 || events != 0 {
		t.Errorf("closed incident got %d postmortems and %d events", pms, events)
	}
	if _, err := repo.CreateDraft(c, draft("00000000-0000-0000-0000-000000000000"), ev); !errors.Is(err, incident.ErrNotFound) {
		t.Errorf("CreateDraft on unknown incident err = %v, want incident.ErrNotFound", err)
	}
}
