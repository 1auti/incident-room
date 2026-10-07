package incident_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

type fakeRepo struct {
	services map[string]incident.ServiceInfo
	created  int
	lastInc  incident.Incident
	lastEvs  []incident.TimelineEvent

	seeded    []incident.Incident
	listed    int
	lastQuery incident.ListQuery

	// events collects what Escalate and UpdateState append (BR-09).
	events []incident.TimelineEvent
}

func (f *fakeRepo) indexOf(id string) int {
	for i := range f.seeded {
		if f.seeded[i].ID == id {
			return i
		}
	}
	return -1
}

func (f *fakeRepo) Get(_ context.Context, id string) (incident.Incident, error) {
	i := f.indexOf(id)
	if i < 0 {
		return incident.Incident{}, incident.ErrNotFound
	}
	return f.seeded[i], nil
}

func (f *fakeRepo) ListPendingEscalation(_ context.Context, state incident.State) ([]incident.Incident, error) {
	var out []incident.Incident
	for _, inc := range f.seeded {
		if inc.State == state && inc.EscalatedAt == nil {
			out = append(out, inc)
		}
	}
	return out, nil
}

// Escalate mirrors the conditional update of the real repository.
func (f *fakeRepo) Escalate(_ context.Context, inc incident.Incident, ev incident.TimelineEvent) (bool, error) {
	i := f.indexOf(inc.ID)
	if i < 0 || f.seeded[i].State != incident.StateDeclared || f.seeded[i].EscalatedAt != nil || f.seeded[i].Severity != inc.Severity {
		return false, nil
	}
	at := ev.OccurredAt
	f.seeded[i].EscalatedAt = &at
	ev.IncidentID = inc.ID
	f.events = append(f.events, ev)
	return true, nil
}

func (f *fakeRepo) UpdateState(_ context.Context, id string, from, to incident.State, ackAt *time.Time, ev incident.TimelineEvent) (incident.Incident, error) {
	i := f.indexOf(id)
	if i < 0 {
		return incident.Incident{}, incident.ErrNotFound
	}
	if f.seeded[i].State != from {
		return incident.Incident{}, incident.ErrInvalidTransition
	}
	f.seeded[i].State = to
	if ackAt != nil {
		f.seeded[i].AcknowledgedAt = ackAt
	}
	ev.IncidentID = id
	f.events = append(f.events, ev)
	return f.seeded[i], nil
}

func (f *fakeRepo) FindService(_ context.Context, id string) (incident.ServiceInfo, error) {
	s, ok := f.services[id]
	if !ok {
		return incident.ServiceInfo{}, incident.ErrServiceNotFound
	}
	return s, nil
}

func (f *fakeRepo) Create(_ context.Context, inc incident.Incident, evs []incident.TimelineEvent) (incident.Incident, []incident.TimelineEvent, error) {
	f.created++
	inc.ID = "inc-1"
	out := make([]incident.TimelineEvent, len(evs))
	for i, e := range evs {
		e.ID = "ev-" + string(rune('1'+i))
		e.IncidentID = inc.ID
		out[i] = e
	}
	f.lastInc, f.lastEvs = inc, out
	return inc, out, nil
}

// List applies q over the seeded incidents, like the real repository does.
func (f *fakeRepo) List(_ context.Context, q incident.ListQuery) ([]incident.Incident, error) {
	f.listed++
	f.lastQuery = q
	var out []incident.Incident
	for _, inc := range f.seeded {
		if !slices.Contains(q.States, inc.State) ||
			(q.Severity != "" && inc.Severity != q.Severity) ||
			(q.ServiceID != "" && inc.ServiceID != q.ServiceID) {
			continue
		}
		out = append(out, inc)
	}
	return out, nil
}

var fixedNow = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

func newFake() *fakeRepo {
	oncall := "oncall-1"
	return &fakeRepo{services: map[string]incident.ServiceInfo{
		"critical":  {Criticality: incident.CriticalityCritical},
		"standard":  {Criticality: incident.CriticalityStandard},
		"important": {Criticality: incident.CriticalityImportant, OncallUserID: &oncall},
	}}
}

func newSvc(repo *fakeRepo) *incident.Service {
	return incident.NewService(repo, func() time.Time { return fixedNow }, incident.DefaultSLA())
}

var engineer = user.User{ID: "u-eng", Role: user.RoleIngeniero}

func TestUC021_SugerenciaPorServicioEImpacto(t *testing.T) {
	for _, tc := range []struct {
		name, service string
		impact        incident.Impact
		want          incident.Severity
		wantErr       error
	}{
		{"critica+caida_total", "critical", incident.ImpactTotalOutage, incident.SeveritySEV1, nil},
		{"estandar+degradacion", "standard", incident.ImpactDegradation, incident.SeveritySEV3, nil},
		{"servicio inexistente", "nope", incident.ImpactTotalOutage, "", incident.ErrServiceNotFound},
		{"impacto invalido", "critical", "enorme", "", incident.ErrInvalid},
		{"impacto vacio", "critical", "", "", incident.ErrInvalid},
		{"servicio vacio", "", incident.ImpactMinor, "", incident.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newSvc(newFake()).Suggest(context.Background(), engineer, tc.service, tc.impact)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Errorf("Suggest = %q, %v; want %q, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestUC022_DeclararAceptandoSugerida(t *testing.T) {
	repo := newFake()
	in := incident.DeclareInput{Title: "  DB caida  ", Description: "d", ServiceID: "critical", Impact: incident.ImpactTotalOutage}
	inc, evs, err := newSvc(repo).Declare(context.Background(), engineer, in)
	if err != nil {
		t.Fatal(err)
	}
	if inc.Title != "DB caida" || inc.State != incident.StateDeclared ||
		inc.SuggestedSeverity != incident.SeveritySEV1 || inc.Severity != inc.SuggestedSeverity ||
		inc.DeclaredBy != "u-eng" || inc.ServiceID != "critical" || inc.Impact != incident.ImpactTotalOutage {
		t.Errorf("incident = %+v", inc)
	}
	if len(evs) != 1 || evs[0].Type != incident.EventDeclaration || evs[0].AuthorID == nil || *evs[0].AuthorID != "u-eng" ||
		evs[0].Data == nil || len(evs[0].Data) != 0 || evs[0].Body != "" {
		t.Errorf("events = %+v", evs)
	}
}

func TestBR05_DeclaredAtUsaRelojInyectable(t *testing.T) {
	repo := newFake()
	calls := 0
	local := time.FixedZone("UTC-3", -3*3600)
	svc := incident.NewService(repo, func() time.Time {
		calls++
		return fixedNow.In(local)
	}, incident.DefaultSLA())
	inc, evs, err := svc.Declare(context.Background(), engineer, incident.DeclareInput{
		Title: "x", ServiceID: "critical", Impact: incident.ImpactTotalOutage, Severity: incident.SeveritySEV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("clock calls = %d, want 1", calls)
	}
	if !inc.DeclaredAt.Equal(fixedNow) || inc.DeclaredAt.Location() != time.UTC {
		t.Errorf("declared_at = %v, want %v in UTC", inc.DeclaredAt, fixedNow)
	}
	if len(evs) != 2 {
		t.Fatalf("events = %d, want 2", len(evs))
	}
	for _, e := range evs {
		if !e.OccurredAt.Equal(fixedNow) || e.OccurredAt.Location() != time.UTC {
			t.Errorf("event %s occurred_at = %v, want %v in UTC", e.Type, e.OccurredAt, fixedNow)
		}
	}
}

func TestBR11_DeclararAsignaOncallDelServicio(t *testing.T) {
	for _, tc := range []struct {
		name, service string
		want          *string
	}{
		{"con on-call", "important", ptr("oncall-1")},
		{"sin on-call", "critical", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inc, _, err := newSvc(newFake()).Declare(context.Background(), engineer, incident.DeclareInput{
				Title: "x", ServiceID: tc.service, Impact: incident.ImpactMinor,
			})
			if err != nil {
				t.Fatal(err)
			}
			if (inc.AssignedTo == nil) != (tc.want == nil) || (tc.want != nil && *inc.AssignedTo != *tc.want) {
				t.Errorf("assigned_to = %v, want %v", inc.AssignedTo, tc.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestBR10_TodoRolPuedeDeclarar(t *testing.T) {
	for _, tc := range []struct {
		role    user.Role
		wantErr error
	}{
		{user.RoleIngeniero, nil},
		{user.RoleOncall, nil},
		{user.RoleAdmin, nil},
		{"", incident.ErrForbidden},
		{"visitante", incident.ErrForbidden},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			repo := newFake()
			actor := user.User{ID: "u", Role: tc.role}
			_, _, err := newSvc(repo).Declare(context.Background(), actor, incident.DeclareInput{
				Title: "x", ServiceID: "critical", Impact: incident.ImpactMinor,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if wantCreated := map[bool]int{true: 1, false: 0}[tc.wantErr == nil]; repo.created != wantCreated {
				t.Errorf("Create calls = %d, want %d", repo.created, wantCreated)
			}
		})
	}
}

func TestBR02_CambioSeveridadRegistraEvento(t *testing.T) {
	for _, tc := range []struct {
		name     string
		severity incident.Severity
		wantSev  incident.Severity
		wantErr  error
		change   bool
	}{
		{"vacia usa sugerida", "", incident.SeveritySEV2, nil, false},
		{"igual a la sugerida", incident.SeveritySEV2, incident.SeveritySEV2, nil, false},
		{"distinta registra evento", incident.SeveritySEV1, incident.SeveritySEV1, nil, true},
		{"invalida", "SEV4", "", incident.ErrInvalid, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			// important + caida_total => suggested SEV2.
			inc, evs, err := newSvc(repo).Declare(context.Background(), engineer, incident.DeclareInput{
				Title: "x", ServiceID: "important", Impact: incident.ImpactTotalOutage, Severity: tc.severity,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if repo.created != 0 {
					t.Errorf("Create calls = %d, want 0", repo.created)
				}
				return
			}
			if inc.SuggestedSeverity != incident.SeveritySEV2 || inc.Severity != tc.wantSev {
				t.Errorf("suggested/severity = %s/%s", inc.SuggestedSeverity, inc.Severity)
			}
			if !tc.change {
				if len(evs) != 1 || evs[0].Type != incident.EventDeclaration {
					t.Errorf("events = %+v, want only declaracion", evs)
				}
				return
			}
			if len(evs) != 2 || evs[0].Type != incident.EventDeclaration || evs[1].Type != incident.EventSeverityChange {
				t.Fatalf("events = %+v, want [declaracion, cambio_severidad]", evs)
			}
			ch := evs[1]
			if ch.Data["from"] != "SEV2" || ch.Data["to"] != "SEV1" || len(ch.Data) != 2 ||
				ch.AuthorID == nil || *ch.AuthorID != "u-eng" || ch.Body != "" {
				t.Errorf("cambio_severidad = %+v", ch)
			}
		})
	}
}

func TestUC025_SinCamposObligatoriosNoCrea(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   incident.DeclareInput
	}{
		{"titulo vacio", incident.DeclareInput{Title: "", ServiceID: "critical", Impact: incident.ImpactMinor}},
		{"titulo con espacios", incident.DeclareInput{Title: "   \t", ServiceID: "critical", Impact: incident.ImpactMinor}},
		{"sin service_id", incident.DeclareInput{Title: "x", Impact: incident.ImpactMinor}},
		{"sin impact", incident.DeclareInput{Title: "x", ServiceID: "critical"}},
		{"impact invalido", incident.DeclareInput{Title: "x", ServiceID: "critical", Impact: "enorme"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			_, _, err := newSvc(repo).Declare(context.Background(), engineer, tc.in)
			if !errors.Is(err, incident.ErrInvalid) {
				t.Errorf("err = %v, want ErrInvalid", err)
			}
			if repo.created != 0 {
				t.Errorf("Create calls = %d, want 0", repo.created)
			}
		})
	}
}

func TestUC025_ServicioInexistenteNoCrea(t *testing.T) {
	repo := newFake()
	_, _, err := newSvc(repo).Declare(context.Background(), engineer, incident.DeclareInput{
		Title: "x", ServiceID: "nope", Impact: incident.ImpactMinor,
	})
	if !errors.Is(err, incident.ErrServiceNotFound) || repo.created != 0 {
		t.Errorf("err = %v, created = %d", err, repo.created)
	}
}

// seedBoard stores one incident per lifecycle state plus extra ones that differ
// in severity and service, so filters have something to discriminate.
func seedBoard(repo *fakeRepo) {
	mk := func(id string, state incident.State, sev incident.Severity, svc string) incident.Incident {
		return incident.Incident{ID: id, State: state, Severity: sev, ServiceID: svc}
	}
	repo.seeded = []incident.Incident{
		mk("declared", incident.StateDeclared, incident.SeveritySEV1, "svc-a"),
		mk("acknowledged", incident.StateAcknowledged, incident.SeveritySEV2, "svc-a"),
		mk("mitigating", incident.StateMitigating, incident.SeveritySEV1, "svc-b"),
		mk("resolved", incident.StateResolved, incident.SeveritySEV3, "svc-b"),
		mk("closed", incident.StateClosed, incident.SeveritySEV1, "svc-a"),
	}
}

func ids(list []incident.Incident) []string {
	out := make([]string, 0, len(list))
	for _, inc := range list {
		out = append(out, inc.ID)
	}
	return out
}

func TestBR15_ActivoEsTodoLoNoCerrado(t *testing.T) {
	repo := newFake()
	seedBoard(repo)
	got, err := newSvc(repo).List(context.Background(), engineer, incident.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"declared", "acknowledged", "mitigating", "resolved"}
	if !slices.Equal(ids(got), want) {
		t.Errorf("ids = %v, want %v (resuelto stays active, cerrado is out)", ids(got), want)
	}

	// Filtering by state intersects with the active states; cerrado never lists.
	for _, tc := range []struct {
		state     incident.State
		want      []string
		wantCalls int
	}{
		{incident.StateResolved, []string{"resolved"}, 1},
		{incident.StateClosed, []string{}, 0},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			repo := newFake()
			seedBoard(repo)
			got, err := newSvc(repo).List(context.Background(), engineer, incident.ListFilter{State: tc.state})
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || !slices.Equal(ids(got), tc.want) {
				t.Errorf("ids = %v (nil %v), want %v non-nil", ids(got), got == nil, tc.want)
			}
			if repo.listed != tc.wantCalls {
				t.Errorf("repo List calls = %d, want %d", repo.listed, tc.wantCalls)
			}
			if slices.Contains(repo.lastQuery.States, incident.StateClosed) {
				t.Errorf("query States %v include cerrado", repo.lastQuery.States)
			}
		})
	}
}

func TestBR15_FiltrosPorSeveridadServicioYEstado(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filter  incident.ListFilter
		want    []string
		wantErr error
	}{
		{"severidad", incident.ListFilter{Severity: incident.SeveritySEV1}, []string{"declared", "mitigating"}, nil},
		{"servicio", incident.ListFilter{ServiceID: "svc-b"}, []string{"mitigating", "resolved"}, nil},
		{"estado", incident.ListFilter{State: incident.StateAcknowledged}, []string{"acknowledged"}, nil},
		{"servicio y estado", incident.ListFilter{ServiceID: "svc-b", State: incident.StateMitigating}, []string{"mitigating"}, nil},
		{"todos a la vez", incident.ListFilter{Severity: incident.SeveritySEV1, ServiceID: "svc-a", State: incident.StateDeclared}, []string{"declared"}, nil},
		{"sin coincidencias", incident.ListFilter{Severity: incident.SeveritySEV3, ServiceID: "svc-a"}, []string{}, nil},
		{"severidad fuera del enum", incident.ListFilter{Severity: "SEV9"}, nil, incident.ErrInvalidFilter},
		{"estado fuera del enum", incident.ListFilter{State: "dormido"}, nil, incident.ErrInvalidFilter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			seedBoard(repo)
			got, err := newSvc(repo).List(context.Background(), engineer, tc.filter)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if repo.listed != 0 {
					t.Errorf("repo List calls = %d, want 0", repo.listed)
				}
				return
			}
			if got == nil || !slices.Equal(ids(got), tc.want) {
				t.Errorf("ids = %v (nil %v), want %v non-nil", ids(got), got == nil, tc.want)
			}
		})
	}
}

func TestBR04_EscaladoSeExponeEnListado(t *testing.T) {
	repo := newFake()
	seedBoard(repo)
	at := fixedNow.Add(-time.Hour)
	repo.seeded[0].EscalatedAt = &at
	got, err := newSvc(repo).List(context.Background(), engineer, incident.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, inc := range got {
		escalated := inc.EscalatedAt != nil
		if escalated != (inc.ID == "declared") {
			t.Errorf("%s escalated_at = %v", inc.ID, inc.EscalatedAt)
		}
	}
	if got[0].EscalatedAt == nil || !got[0].EscalatedAt.Equal(at) {
		t.Errorf("escalated_at = %v, want %v", got[0].EscalatedAt, at)
	}
}

func TestBR10_TodoAutenticadoVeTodosLosIncidentes(t *testing.T) {
	for _, tc := range []struct {
		role    user.Role
		wantErr error
	}{
		{user.RoleIngeniero, nil},
		{user.RoleOncall, nil},
		{user.RoleAdmin, nil},
		{"", incident.ErrForbidden},
		{"visitante", incident.ErrForbidden},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			repo := newFake()
			seedBoard(repo)
			// The actor did not declare any seeded incident: visibility is not per owner.
			got, err := newSvc(repo).List(context.Background(), user.User{ID: "someone-else", Role: tc.role}, incident.ListFilter{})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if repo.listed != 0 {
					t.Errorf("repo List calls = %d, want 0", repo.listed)
				}
				return
			}
			if len(got) != 4 {
				t.Errorf("ids = %v, want the 4 active incidents", ids(got))
			}
		})
	}
}

// clockedSvc returns a service whose clock is the returned pointer, so tests
// can move time forward (BR-05).
func clockedSvc(repo *fakeRepo, at time.Time) (*incident.Service, *time.Time) {
	now := at
	return incident.NewService(repo, func() time.Time { return now }, incident.DefaultSLA()), &now
}

// seedDeclared stores a declared, unacknowledged incident declared at fixedNow.
func seedDeclared(repo *fakeRepo, id, service string, sev incident.Severity) {
	repo.seeded = append(repo.seeded, incident.Incident{
		ID: id, ServiceID: service, Severity: sev, State: incident.StateDeclared, DeclaredAt: fixedNow,
	})
}

func escalationEvents(repo *fakeRepo) []incident.TimelineEvent {
	var out []incident.TimelineEvent
	for _, e := range repo.events {
		if e.Type == incident.EventEscalation {
			out = append(out, e)
		}
	}
	return out
}

func TestBR03_PlazoSLAPorSeveridad(t *testing.T) {
	sla := incident.DefaultSLA()
	for _, tc := range []struct {
		sev      incident.Severity
		deadline time.Duration
	}{
		{incident.SeveritySEV1, 5 * time.Minute},
		{incident.SeveritySEV2, 15 * time.Minute},
		{incident.SeveritySEV3, 60 * time.Minute},
	} {
		t.Run(string(tc.sev), func(t *testing.T) {
			if got := sla.Deadline(tc.sev); got != tc.deadline {
				t.Fatalf("Deadline(%s) = %v, want %v", tc.sev, got, tc.deadline)
			}
			for _, step := range []struct {
				name string
				at   time.Duration
				want int
			}{
				{"exact deadline does not escalate", tc.deadline, 0},
				{"one second later escalates", tc.deadline + time.Second, 1},
			} {
				repo := newFake()
				seedDeclared(repo, "i1", "important", tc.sev)
				svc, now := clockedSvc(repo, fixedNow)
				*now = fixedNow.Add(step.at)
				n, err := svc.EscalateOverdue(context.Background())
				if err != nil || n != step.want || len(escalationEvents(repo)) != step.want {
					t.Errorf("%s: escalated = %d (%v), events = %d, want %d", step.name, n, err, len(escalationEvents(repo)), step.want)
				}
			}
		})
	}
}

func TestBR03_PlazoUsaSeveridadVigente(t *testing.T) {
	// SEV3 raised to SEV1 is evaluated with the SEV1 deadline measured from declared_at.
	repo := newFake()
	seedDeclared(repo, "i1", "important", incident.SeveritySEV1) // already raised to SEV1
	svc, now := clockedSvc(repo, fixedNow.Add(7*time.Minute))
	if n, err := svc.EscalateOverdue(context.Background()); err != nil || n != 1 {
		t.Fatalf("SEV1 at 10:07: escalated = %d, %v; want 1", n, err)
	}
	_ = now

	for _, tc := range []struct {
		name string
		at   time.Duration
		want int
	}{
		{"SEV3 at 10:59:59", 59*time.Minute + 59*time.Second, 0},
		{"SEV3 at 11:00:01", 60*time.Minute + time.Second, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			seedDeclared(repo, "i1", "important", incident.SeveritySEV3)
			svc, _ := clockedSvc(repo, fixedNow.Add(tc.at))
			if n, err := svc.EscalateOverdue(context.Background()); err != nil || n != tc.want {
				t.Errorf("escalated = %d, %v; want %d", n, err, tc.want)
			}
		})
	}
}

func TestBR04_EscalaUnaSolaVezAlVencer(t *testing.T) {
	repo := newFake()
	seedDeclared(repo, "i1", "important", incident.SeveritySEV2)
	svc, now := clockedSvc(repo, fixedNow.Add(15*time.Minute))
	if n, err := svc.EscalateOverdue(context.Background()); err != nil || n != 0 {
		t.Fatalf("at 10:15:00 escalated = %d, %v; want 0", n, err)
	}
	if repo.seeded[0].EscalatedAt != nil {
		t.Fatalf("escalated_at set before the deadline")
	}

	*now = fixedNow.Add(15*time.Minute + time.Second)
	if n, err := svc.EscalateOverdue(context.Background()); err != nil || n != 1 {
		t.Fatalf("at 10:15:01 escalated = %d, %v; want 1", n, err)
	}
	if got := repo.seeded[0].EscalatedAt; got == nil || !got.Equal(*now) {
		t.Errorf("escalated_at = %v, want %v", got, *now)
	}
	evs := escalationEvents(repo)
	if len(evs) != 1 || evs[0].AuthorID != nil || evs[0].IncidentID != "i1" || !evs[0].OccurredAt.Equal(*now) || evs[0].Data == nil {
		t.Fatalf("events = %+v, want exactly one escalado with nil author", repo.events)
	}

	*now = now.Add(time.Hour)
	if n, err := svc.EscalateOverdue(context.Background()); err != nil || n != 0 {
		t.Errorf("second evaluation escalated = %d, %v; want 0", n, err)
	}
	if len(escalationEvents(repo)) != 1 {
		t.Errorf("second evaluation added an event: %+v", repo.events)
	}
}

func TestBR04_ServicioSinOncallEscala(t *testing.T) {
	repo := newFake()
	seedDeclared(repo, "i1", "critical", incident.SeveritySEV2) // "critical" has no on-call
	svc, _ := clockedSvc(repo, fixedNow.Add(15*time.Minute+time.Second))
	if n, err := svc.EscalateOverdue(context.Background()); err != nil || n != 1 {
		t.Fatalf("escalated = %d, %v; want 1", n, err)
	}
	if repo.seeded[0].EscalatedAt == nil || repo.seeded[0].AssignedTo != nil {
		t.Errorf("incident = %+v, want escalated and still unassigned", repo.seeded[0])
	}
	if len(escalationEvents(repo)) != 1 {
		t.Errorf("events = %+v", repo.events)
	}
}

func TestBR04_ReconocidoATiempoNoEscala(t *testing.T) {
	repo := newFake()
	seedDeclared(repo, "i1", "important", incident.SeveritySEV2)
	svc, now := clockedSvc(repo, fixedNow.Add(14*time.Minute+59*time.Second))
	oncall := user.User{ID: "oncall-1", Role: user.RoleOncall}
	got, err := svc.Transition(context.Background(), oncall, "i1", incident.StateAcknowledged)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != incident.StateAcknowledged || got.AcknowledgedAt == nil || !got.AcknowledgedAt.Equal(*now) {
		t.Errorf("incident = %+v, want reconocido with acknowledged_at = 10:14:59", got)
	}
	if len(repo.events) != 1 || repo.events[0].Type != incident.EventStateChange || repo.events[0].AuthorID == nil ||
		*repo.events[0].AuthorID != "oncall-1" || repo.events[0].Data["from"] != "declarado" || repo.events[0].Data["to"] != "reconocido" ||
		!repo.events[0].OccurredAt.Equal(*now) {
		t.Errorf("events = %+v, want one cambio_estado declarado->reconocido", repo.events)
	}

	*now = fixedNow.Add(15*time.Minute + time.Second)
	if n, err := svc.EscalateOverdue(context.Background()); err != nil || n != 0 {
		t.Errorf("escalated = %d, %v; want 0", n, err)
	}
	if len(escalationEvents(repo)) != 0 || repo.seeded[0].EscalatedAt != nil {
		t.Errorf("acknowledged incident was escalated: %+v", repo.events)
	}
}

func TestBR11_OncallAjenoNoReconoce(t *testing.T) {
	for _, tc := range []struct {
		name    string
		actor   user.User
		wantErr error
	}{
		{"on-call del servicio", user.User{ID: "oncall-1", Role: user.RoleOncall}, nil},
		{"oncall ajeno", user.User{ID: "oncall-2", Role: user.RoleOncall}, incident.ErrForbidden},
		{"ingeniero", user.User{ID: "u-eng", Role: user.RoleIngeniero}, incident.ErrForbidden},
		{"admin", user.User{ID: "u-admin", Role: user.RoleAdmin}, nil},
		{"rol desconocido", user.User{ID: "x", Role: "visitante"}, incident.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			seedDeclared(repo, "i1", "important", incident.SeveritySEV2)
			svc, _ := clockedSvc(repo, fixedNow.Add(time.Minute))
			_, err := svc.Transition(context.Background(), tc.actor, "i1", incident.StateAcknowledged)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			wantState, wantEvents := incident.StateDeclared, 0
			if tc.wantErr == nil {
				wantState, wantEvents = incident.StateAcknowledged, 1
			}
			if repo.seeded[0].State != wantState || len(repo.events) != wantEvents {
				t.Errorf("state = %s, events = %d; want %s, %d", repo.seeded[0].State, len(repo.events), wantState, wantEvents)
			}
		})
	}
}

func TestBR08_TransicionesInvalidasSeRechazan(t *testing.T) {
	admin := user.User{ID: "u-admin", Role: user.RoleAdmin}
	for _, tc := range []struct {
		name    string
		state   incident.State
		to      incident.State
		id      string
		wantErr error
	}{
		{"salto declarado a resuelto", incident.StateDeclared, incident.StateResolved, "i1", incident.ErrInvalidTransition},
		{"ya reconocido", incident.StateAcknowledged, incident.StateAcknowledged, "i1", incident.ErrInvalidTransition},
		{"retroceso", incident.StateAcknowledged, incident.StateDeclared, "i1", incident.ErrInvalidTransition},
		{"inexistente", incident.StateDeclared, incident.StateAcknowledged, "nope", incident.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			seedDeclared(repo, "i1", "important", incident.SeveritySEV2)
			repo.seeded[0].State = tc.state
			svc, _ := clockedSvc(repo, fixedNow)
			if _, err := svc.Transition(context.Background(), admin, tc.id, tc.to); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if repo.seeded[0].State != tc.state || len(repo.events) != 0 || repo.seeded[0].AcknowledgedAt != nil {
				t.Errorf("rejected transition changed the incident: %+v events %+v", repo.seeded[0], repo.events)
			}
		})
	}
}

func TestUC04_GetRequiereRolValido(t *testing.T) {
	repo := newFake()
	seedDeclared(repo, "i1", "important", incident.SeveritySEV2)
	svc := newSvc(repo)
	if got, err := svc.Get(context.Background(), engineer, "i1"); err != nil || got.ID != "i1" {
		t.Errorf("Get = %+v, %v", got, err)
	}
	if _, err := svc.Get(context.Background(), user.User{Role: "visitante"}, "i1"); !errors.Is(err, incident.ErrForbidden) {
		t.Errorf("unknown role err = %v, want ErrForbidden", err)
	}
	if _, err := svc.Get(context.Background(), engineer, "nope"); !errors.Is(err, incident.ErrNotFound) {
		t.Errorf("missing err = %v, want ErrNotFound", err)
	}
}
