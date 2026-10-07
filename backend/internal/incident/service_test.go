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
	return incident.NewService(repo, func() time.Time { return fixedNow })
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
	})
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
