package metrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/metrics"
	"incident-room-backend/internal/user"
)

var (
	engineer = user.User{ID: "u-eng", Role: user.RoleIngeniero}
	// fixedNow is the injected clock; periods are evaluated against it (BR-05).
	fixedNow = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	day      = 24 * time.Hour
)

// fakeRepo applies [from, to) like the real repository: resolved rows by
// resolved_at and declared rows by declared_at.
type fakeRepo struct {
	resolved []metrics.ResolvedRow
	declared []metrics.DeclaredRow
	services []metrics.ServiceRef
	queried  []metrics.Period
}

func in(t time.Time, p metrics.Period) bool { return !t.Before(p.From) && t.Before(p.To) }

func (f *fakeRepo) ResolvedIn(_ context.Context, p metrics.Period) ([]metrics.ResolvedRow, error) {
	f.queried = append(f.queried, p)
	var out []metrics.ResolvedRow
	for _, r := range f.resolved {
		if in(r.ResolvedAt, p) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepo) DeclaredIn(_ context.Context, p metrics.Period) ([]metrics.DeclaredRow, error) {
	var out []metrics.DeclaredRow
	for _, r := range f.declared {
		if in(r.DeclaredAt, p) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepo) Services(context.Context) ([]metrics.ServiceRef, error) { return f.services, nil }

func newSvc(repo *fakeRepo) *metrics.Service {
	return metrics.NewService(repo, func() time.Time { return fixedNow }, incident.DefaultSLA())
}

// clock builds an instant on 2026-09-20 (inside the default 30-day period).
func clock(h, m int) time.Time { return time.Date(2026, 9, 20, h, m, 0, 0, time.UTC) }

func ptr(t time.Time) *time.Time { return &t }

func TestBR16_MTTRPromedioDelPeriodo(t *testing.T) {
	repo := &fakeRepo{resolved: []metrics.ResolvedRow{
		{DeclaredAt: clock(10, 0), ResolvedAt: clock(10, 30)},
		{DeclaredAt: clock(11, 0), ResolvedAt: clock(12, 0)},
	}}
	got, err := newSvc(repo).Dashboard(context.Background(), engineer, metrics.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got.MTTRSeconds == nil || *got.MTTRSeconds != (45*time.Minute).Seconds() {
		t.Errorf("MTTR = %v, want 45 min", got.MTTRSeconds)
	}
}

func TestBR16_SinDatosSinValor(t *testing.T) {
	// An unresolved incident never reaches the MTTR (it has no resolved_at row),
	// and still inside its deadline it is not eligible for the SLA either.
	repo := &fakeRepo{declared: []metrics.DeclaredRow{
		{ServiceID: "s1", Severity: incident.SeveritySEV3, DeclaredAt: fixedNow.Add(-10 * time.Minute)},
	}}
	got, err := newSvc(repo).Dashboard(context.Background(), engineer, metrics.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got.MTTRSeconds != nil {
		t.Errorf("MTTR = %v, want no value (not 0)", *got.MTTRSeconds)
	}
	if got.SLACompliancePercent != nil {
		t.Errorf("SLA = %v, want no value", *got.SLACompliancePercent)
	}
}

func TestBR16_FiltraPorResolvedAt(t *testing.T) {
	period := metrics.Query{From: clock(0, 0), To: clock(0, 0).Add(day)}
	repo := &fakeRepo{resolved: []metrics.ResolvedRow{
		// Declared the day before but resolved inside: counts.
		{DeclaredAt: clock(23, 0).Add(-day), ResolvedAt: clock(1, 0)},
		// Declared inside but resolved after the end (exclusive): does not count.
		{DeclaredAt: clock(22, 0), ResolvedAt: clock(0, 0).Add(day)},
	}}
	got, err := newSvc(repo).Dashboard(context.Background(), engineer, period)
	if err != nil {
		t.Fatal(err)
	}
	if got.MTTRSeconds == nil || *got.MTTRSeconds != (2*time.Hour).Seconds() {
		t.Errorf("MTTR = %v, want 2h (only the incident resolved inside)", got.MTTRSeconds)
	}
}

func TestBR17_IncidentesPorServicioIncluyeCeros(t *testing.T) {
	repo := &fakeRepo{
		services: []metrics.ServiceRef{{ID: "pay", Name: "payments"}, {ID: "srch", Name: "search"}, {ID: "inv", Name: "inventory"}},
	}
	for _, svc := range []string{"pay", "pay", "srch", "pay"} {
		repo.declared = append(repo.declared, metrics.DeclaredRow{ServiceID: svc, Severity: incident.SeveritySEV3, DeclaredAt: clock(10, 0)})
	}
	got, err := newSvc(repo).Dashboard(context.Background(), engineer, metrics.Query{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"payments": 3, "search": 1, "inventory": 0}
	if len(got.IncidentsByService) != len(want) {
		t.Fatalf("by service = %+v, want %v", got.IncidentsByService, want)
	}
	for i, c := range got.IncidentsByService {
		if c.Count != want[c.Name] || c.ServiceID != repo.services[i].ID {
			t.Errorf("%s = %d (%s), want %d keeping the repository order", c.Name, c.Count, c.ServiceID, want[c.Name])
		}
	}
}

func TestBR17_FiltraPorDeclaredAt(t *testing.T) {
	period := metrics.Query{From: clock(0, 0), To: clock(0, 0).Add(day)}
	repo := &fakeRepo{
		services: []metrics.ServiceRef{{ID: "pay", Name: "payments"}},
		declared: []metrics.DeclaredRow{
			{ServiceID: "pay", Severity: incident.SeveritySEV3, DeclaredAt: clock(0, 0)},          // start: included
			{ServiceID: "pay", Severity: incident.SeveritySEV3, DeclaredAt: clock(0, 0).Add(day)}, // end: excluded
			{ServiceID: "pay", Severity: incident.SeveritySEV3, DeclaredAt: clock(0, 0).Add(-time.Second)},
		},
	}
	got, err := newSvc(repo).Dashboard(context.Background(), engineer, period)
	if err != nil {
		t.Fatal(err)
	}
	if got.IncidentsByService[0].Count != 1 {
		t.Errorf("payments = %d, want 1 ([desde, hasta))", got.IncidentsByService[0].Count)
	}
}

func TestBR18_CumplimientoSLAElegibles(t *testing.T) {
	declared := clock(10, 0)
	// One incident at a time around the SEV1 deadline (5 min): ack <= deadline complies.
	for _, tc := range []struct {
		name string
		ack  *time.Time
		want float64
	}{
		{"reconocido justo en el plazo", ptr(declared.Add(5 * time.Minute)), 100},
		{"reconocido 1 s tarde", ptr(declared.Add(5*time.Minute + time.Second)), 0},
		{"sin reconocer y plazo vencido", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{declared: []metrics.DeclaredRow{
				{ServiceID: "s", Severity: incident.SeveritySEV1, DeclaredAt: declared, AcknowledgedAt: tc.ack},
			}}
			got, err := newSvc(repo).Dashboard(context.Background(), engineer, metrics.Query{})
			if err != nil || got.SLACompliancePercent == nil || *got.SLACompliancePercent != tc.want {
				t.Errorf("SLA = %v, %v; want %v", got.SLACompliancePercent, err, tc.want)
			}
		})
	}

	repo := &fakeRepo{declared: []metrics.DeclaredRow{
		// 3 acknowledged within the deadline (SEV1 5 min, SEV2 15 min, SEV3 60 min).
		{ServiceID: "s", Severity: incident.SeveritySEV1, DeclaredAt: declared, AcknowledgedAt: ptr(declared.Add(5 * time.Minute))},
		{ServiceID: "s", Severity: incident.SeveritySEV2, DeclaredAt: declared, AcknowledgedAt: ptr(declared.Add(14 * time.Minute))},
		{ServiceID: "s", Severity: incident.SeveritySEV3, DeclaredAt: declared, AcknowledgedAt: ptr(declared.Add(time.Hour))},
		// 1 escalated: never acknowledged and its deadline passed long ago.
		{ServiceID: "s", Severity: incident.SeveritySEV2, DeclaredAt: declared},
	}}
	got, err := newSvc(repo).Dashboard(context.Background(), engineer, metrics.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got.SLACompliancePercent == nil || *got.SLACompliancePercent != 75 {
		t.Errorf("SLA = %v, want 75", got.SLACompliancePercent)
	}
}

func TestBR18_PendienteNoEntra(t *testing.T) {
	declared := clock(10, 0)
	repo := &fakeRepo{declared: []metrics.DeclaredRow{
		{ServiceID: "s", Severity: incident.SeveritySEV2, DeclaredAt: declared, AcknowledgedAt: ptr(declared.Add(time.Minute))},
		{ServiceID: "s", Severity: incident.SeveritySEV2, DeclaredAt: declared, AcknowledgedAt: ptr(declared.Add(2 * time.Minute))},
		{ServiceID: "s", Severity: incident.SeveritySEV2, DeclaredAt: declared, AcknowledgedAt: ptr(declared.Add(3 * time.Minute))},
		{ServiceID: "s", Severity: incident.SeveritySEV1, DeclaredAt: declared},
		// Pending: declared 10 minutes before now, SEV3 (60 min), not acknowledged.
		{ServiceID: "s", Severity: incident.SeveritySEV3, DeclaredAt: fixedNow.Add(-10 * time.Minute)},
		// Exactly at its deadline is still within it (BR-04 escalates only when now > deadline).
		{ServiceID: "s", Severity: incident.SeveritySEV1, DeclaredAt: fixedNow.Add(-5 * time.Minute)},
	}}
	got, err := newSvc(repo).Dashboard(context.Background(), engineer, metrics.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got.SLACompliancePercent == nil || *got.SLACompliancePercent != 75 {
		t.Errorf("SLA = %v, want 75 (pending incidents out of both sides)", got.SLACompliancePercent)
	}
}

func TestMetricas_PeriodoPorDefecto30Dias(t *testing.T) {
	for _, tc := range []struct {
		name string
		q    metrics.Query
		want metrics.Period
	}{
		{"sin período", metrics.Query{}, metrics.Period{From: fixedNow.Add(-30 * day), To: fixedNow}},
		{"últimos 30 días", metrics.Query{LastDays: 30}, metrics.Period{From: fixedNow.Add(-30 * day), To: fixedNow}},
		{"últimos 7 días", metrics.Query{LastDays: 7}, metrics.Period{From: fixedNow.Add(-7 * day), To: fixedNow}},
		{"rango libre", metrics.Query{From: clock(0, 0), To: clock(0, 0).Add(day)}, metrics.Period{From: clock(0, 0), To: clock(0, 0).Add(day)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			got, err := newSvc(repo).Dashboard(context.Background(), engineer, tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if !got.From.Equal(tc.want.From) || !got.To.Equal(tc.want.To) || len(repo.queried) != 1 || repo.queried[0] != tc.want {
				t.Errorf("period = [%s, %s), queried %+v; want [%s, %s)", got.From, got.To, repo.queried, tc.want.From, tc.want.To)
			}
		})
	}
}

func TestMetricas_PeriodoInvalidoYRol(t *testing.T) {
	for name, q := range map[string]metrics.Query{
		"preset desconocido":   {LastDays: 14},
		"desde igual a hasta":  {From: clock(0, 0), To: clock(0, 0)},
		"desde después":        {From: clock(1, 0), To: clock(0, 0)},
		"solo desde":           {From: clock(0, 0)},
		"preset y rango libre": {LastDays: 7, From: clock(0, 0), To: clock(1, 0)},
	} {
		if _, err := newSvc(&fakeRepo{}).Dashboard(context.Background(), engineer, q); !errors.Is(err, metrics.ErrInvalidPeriod) {
			t.Errorf("%s: err = %v, want ErrInvalidPeriod", name, err)
		}
	}
	// Every authenticated role sees the dashboard; an unknown role does not (BR-10).
	for _, role := range []user.Role{user.RoleIngeniero, user.RoleOncall, user.RoleAdmin} {
		if _, err := newSvc(&fakeRepo{}).Dashboard(context.Background(), user.User{Role: role}, metrics.Query{}); err != nil {
			t.Errorf("%s: err = %v", role, err)
		}
	}
	if _, err := newSvc(&fakeRepo{}).Dashboard(context.Background(), user.User{Role: "visitante"}, metrics.Query{}); !errors.Is(err, incident.ErrForbidden) {
		t.Errorf("visitante err = %v, want ErrForbidden", err)
	}
}
