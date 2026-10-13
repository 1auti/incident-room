package metrics_test

import (
	"context"
	"testing"
	"time"

	"incident-room-backend/internal/dbtest"
	"incident-room-backend/internal/metrics"
)

func TestPostgres_FiltrosDelPeriodo(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := metrics.NewPostgresRepository(pool)

	var userID, payments, search string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ana', 'ana@x.com', 'h', 'oncall') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	// Inserted out of name order: Services sorts case-insensitively.
	for name, id := range map[string]*string{"search": &search, "Payments": &payments} {
		if err := pool.QueryRow(c, `INSERT INTO services (name, criticality) VALUES ($1, 'importante') RETURNING id`, name).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	p := metrics.Period{From: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)}
	insert := func(serviceID string, declared time.Time, ack, resolved *time.Time) {
		t.Helper()
		if _, err := pool.Exec(c, `INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by, declared_at, acknowledged_at, resolved_at)
			VALUES ('x', $1, 'menor', 'SEV2', 'SEV2', 'resuelto', $2, $3, $4, $5)`, serviceID, userID, declared, ack, resolved); err != nil {
			t.Fatal(err)
		}
	}
	at := func(d time.Duration) *time.Time { v := p.From.Add(d); return &v }
	insert(payments, p.From, at(time.Minute), at(30*time.Minute))                // declared and resolved inside, at the start
	insert(payments, p.To, nil, at(25*time.Hour))                                // declared at the end (excluded), resolved after
	insert(search, p.From.Add(-time.Hour), nil, &p.To)                           // declared before, resolved at the end (excluded)
	insert(search, p.From.Add(-time.Hour), nil, at(23*time.Hour+59*time.Minute)) // declared before, resolved inside
	insert(search, p.From.Add(-time.Hour), nil, nil)                             // never resolved: no MTTR row

	resolved, err := repo.ResolvedIn(c, p)
	if err != nil || len(resolved) != 2 {
		t.Fatalf("ResolvedIn = %+v, %v; want 2 rows", resolved, err)
	}
	declared, err := repo.DeclaredIn(c, p)
	if err != nil || len(declared) != 1 {
		t.Fatalf("DeclaredIn = %+v, %v; want 1 row", declared, err)
	}
	d := declared[0]
	if d.ServiceID != payments || d.Severity != "SEV2" || !d.DeclaredAt.Equal(p.From) || d.AcknowledgedAt == nil || !d.AcknowledgedAt.Equal(*at(time.Minute)) {
		t.Errorf("declared row = %+v", d)
	}
	services, err := repo.Services(c)
	if err != nil || len(services) != 2 || services[0].Name != "Payments" || services[1].ID != search {
		t.Errorf("Services = %+v, %v; want Payments then search", services, err)
	}
}
