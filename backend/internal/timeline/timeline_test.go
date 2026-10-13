package timeline_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/timeline"
	"incident-room-backend/internal/user"
)

var fixedNow = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

var (
	engineer    = user.User{ID: "u-eng", Name: "Eva", Role: user.RoleIngeniero}
	ownOncall   = user.User{ID: "oncall-1", Name: "Oscar", Role: user.RoleOncall}
	otherOncall = user.User{ID: "oncall-2", Name: "Olga", Role: user.RoleOncall}
	admin       = user.User{ID: "u-admin", Name: "Ada", Role: user.RoleAdmin}
)

// fakeRepo keeps one incident per id and an append-only slice of events. Like
// timeline.Repository, it has no way to update or delete an event (BR-09).
type fakeRepo struct {
	incidents map[string]incident.Incident
	services  map[string]incident.ServiceInfo
	events    []incident.TimelineEvent
	names     map[string]string
}

func newFake() *fakeRepo {
	oncall := ownOncall.ID
	return &fakeRepo{
		incidents: map[string]incident.Incident{
			"i1": {ID: "i1", ServiceID: "payments", Severity: incident.SeveritySEV2, State: incident.StateMitigating, DeclaredAt: fixedNow},
		},
		services: map[string]incident.ServiceInfo{"payments": {Criticality: incident.CriticalityImportant, OncallUserID: &oncall}},
		names:    map[string]string{engineer.ID: engineer.Name, ownOncall.ID: ownOncall.Name, admin.ID: admin.Name},
	}
}

func (f *fakeRepo) Get(_ context.Context, id string) (incident.Incident, error) {
	inc, ok := f.incidents[id]
	if !ok {
		return incident.Incident{}, incident.ErrNotFound
	}
	return inc, nil
}

func (f *fakeRepo) FindService(_ context.Context, id string) (incident.ServiceInfo, error) {
	s, ok := f.services[id]
	if !ok {
		return incident.ServiceInfo{}, incident.ErrServiceNotFound
	}
	return s, nil
}

// ListEvents orders by occurred_at like the real repository (stable on ties).
func (f *fakeRepo) ListEvents(_ context.Context, incidentID string) ([]incident.EventView, error) {
	var out []incident.EventView
	for _, e := range f.events {
		if e.IncidentID != incidentID {
			continue
		}
		v := incident.EventView{TimelineEvent: e}
		if e.AuthorID != nil {
			name := f.names[*e.AuthorID]
			v.AuthorName = &name
		}
		out = append(out, v)
	}
	slices.SortStableFunc(out, func(a, b incident.EventView) int { return a.OccurredAt.Compare(b.OccurredAt) })
	return out, nil
}

func (f *fakeRepo) AppendEvent(_ context.Context, incidentID string, ev incident.TimelineEvent) (incident.TimelineEvent, error) {
	if _, ok := f.incidents[incidentID]; !ok {
		return incident.TimelineEvent{}, incident.ErrNotFound
	}
	ev.ID = "ev-" + string(rune('a'+len(f.events)))
	ev.IncidentID = incidentID
	f.events = append(f.events, ev)
	return ev, nil
}

// ChangeSeverity mirrors the conditional update of the real repository.
func (f *fakeRepo) ChangeSeverity(_ context.Context, id string, from, to incident.Severity, mutable []incident.State, ev incident.TimelineEvent) (incident.Incident, error) {
	inc, ok := f.incidents[id]
	if !ok {
		return incident.Incident{}, incident.ErrNotFound
	}
	if inc.Severity != from || !slices.Contains(mutable, inc.State) {
		return incident.Incident{}, incident.ErrSeverityLocked
	}
	inc.Severity = to
	f.incidents[id] = inc
	ev.IncidentID = id
	f.events = append(f.events, ev)
	return inc, nil
}

func newSvc(repo *fakeRepo, now time.Time) *timeline.Service {
	return timeline.NewService(repo, func() time.Time { return now })
}

func TestBR09_TimelineSoloInserta(t *testing.T) {
	// The repository contract only reads and inserts: no method may update or delete events.
	repoType := reflect.TypeFor[timeline.Repository]()
	for i := range repoType.NumMethod() {
		name := repoType.Method(i).Name
		for _, verb := range []string{"Update", "Delete", "Remove", "Edit", "Replace"} {
			if strings.Contains(name, verb) {
				t.Errorf("Repository exposes %s: timeline must be append-only", name)
			}
		}
	}

	// A correcting note is a new event: the original stays untouched.
	repo := newFake()
	svc := newSvc(repo, fixedNow)
	first, err := svc.AddNote(context.Background(), engineer, "i1", "Reinicié el pod equivocado")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddNote(context.Background(), engineer, "i1", "Corrección: reinicié payments-api-2"); err != nil {
		t.Fatal(err)
	}
	events, err := svc.List(context.Background(), engineer, "i1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].ID != first.ID || events[0].Body != "Reinicié el pod equivocado" {
		t.Errorf("events = %+v, want the original note kept first", events)
	}
}

func TestBR09_OrdenCronologico(t *testing.T) {
	repo := newFake()
	author := engineer.ID
	// Inserted out of order on purpose: the listing sorts by occurred_at.
	repo.events = []incident.TimelineEvent{
		{ID: "e3", IncidentID: "i1", Type: incident.EventNote, AuthorID: &author, Body: "tarde", OccurredAt: fixedNow.Add(20 * time.Minute)},
		{ID: "e1", IncidentID: "i1", Type: incident.EventDeclaration, AuthorID: &author, OccurredAt: fixedNow},
		{ID: "e2", IncidentID: "i1", Type: incident.EventEscalation, AuthorID: nil, OccurredAt: fixedNow.Add(15*time.Minute + time.Second)},
		{ID: "other", IncidentID: "i2", Type: incident.EventNote, OccurredAt: fixedNow},
	}
	events, err := newSvc(repo, fixedNow).List(context.Background(), engineer, "i1")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range events {
		got = append(got, e.ID)
	}
	if !slices.Equal(got, []string{"e1", "e2", "e3"}) {
		t.Errorf("order = %v, want [e1 e2 e3]", got)
	}
	// The escalado is generated by the system: no author (BR-04).
	if events[1].AuthorID != nil || events[1].AuthorName != nil {
		t.Errorf("escalado author = %v %v, want none", events[1].AuthorID, events[1].AuthorName)
	}
	if events[0].AuthorName == nil || *events[0].AuthorName != engineer.Name {
		t.Errorf("declaracion author name = %v, want %s", events[0].AuthorName, engineer.Name)
	}
}

func TestBR05_NotaUsaRelojInyectable(t *testing.T) {
	for _, actor := range []user.User{engineer, ownOncall, otherOncall, admin} {
		t.Run(string(actor.Role)+"-"+actor.ID, func(t *testing.T) {
			repo := newFake()
			at := fixedNow.Add(42 * time.Minute)
			ev, err := newSvc(repo, at).AddNote(context.Background(), actor, "i1", "  Rollback a v2.2 aplicado  ")
			if err != nil {
				t.Fatal(err)
			}
			if !ev.OccurredAt.Equal(at) || ev.Type != incident.EventNote || ev.AuthorID == nil || *ev.AuthorID != actor.ID ||
				ev.Body != "Rollback a v2.2 aplicado" {
				t.Errorf("note = %+v, want nota by %s at %s with trimmed body", ev, actor.ID, at)
			}
		})
	}
}

// The session itself is enforced by RequireAuth (see httpapi); here an actor
// without a valid role, as a visitor, is denied with no effects.
func TestBR10_TimelineRequiereSesion(t *testing.T) {
	visitor := user.User{ID: "x", Role: "visitante"}
	repo := newFake()
	svc := newSvc(repo, fixedNow)
	if _, err := svc.List(context.Background(), visitor, "i1"); !errors.Is(err, incident.ErrForbidden) {
		t.Errorf("List err = %v, want ErrForbidden", err)
	}
	if _, err := svc.AddNote(context.Background(), visitor, "i1", "hola"); !errors.Is(err, incident.ErrForbidden) {
		t.Errorf("AddNote err = %v, want ErrForbidden", err)
	}
	if _, err := svc.ChangeSeverity(context.Background(), visitor, "i1", incident.SeveritySEV1); !errors.Is(err, incident.ErrForbidden) {
		t.Errorf("ChangeSeverity err = %v, want ErrForbidden", err)
	}
	if len(repo.events) != 0 || repo.incidents["i1"].Severity != incident.SeveritySEV2 {
		t.Errorf("denied access had effects: events %+v incident %+v", repo.events, repo.incidents["i1"])
	}
}

func TestUC05_NotaYTimelineValidan(t *testing.T) {
	repo := newFake()
	svc := newSvc(repo, fixedNow)
	for _, body := range []string{"", "   \n\t"} {
		if _, err := svc.AddNote(context.Background(), engineer, "i1", body); !errors.Is(err, timeline.ErrInvalid) {
			t.Errorf("AddNote(%q) err = %v, want ErrInvalid", body, err)
		}
	}
	if _, err := svc.AddNote(context.Background(), engineer, "nope", "hola"); !errors.Is(err, incident.ErrNotFound) {
		t.Errorf("AddNote unknown incident err = %v, want ErrNotFound", err)
	}
	if _, err := svc.List(context.Background(), engineer, "nope"); !errors.Is(err, incident.ErrNotFound) {
		t.Errorf("List unknown incident err = %v, want ErrNotFound", err)
	}
	if len(repo.events) != 0 {
		t.Errorf("rejected notes created events: %+v", repo.events)
	}
}

func TestBR02_CambioSeveridadSoloOncallOAdminHastaMitigando(t *testing.T) {
	for _, tc := range []struct {
		name    string
		actor   user.User
		state   incident.State
		wantErr error
	}{
		{"on-call del servicio en declarado", ownOncall, incident.StateDeclared, nil},
		{"on-call del servicio en reconocido", ownOncall, incident.StateAcknowledged, nil},
		{"admin en mitigando", admin, incident.StateMitigating, nil},
		{"ingeniero", engineer, incident.StateMitigating, incident.ErrForbidden},
		{"on-call ajeno", otherOncall, incident.StateMitigating, incident.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			inc := repo.incidents["i1"]
			inc.State = tc.state
			repo.incidents["i1"] = inc
			at := fixedNow.Add(30 * time.Minute)

			got, err := newSvc(repo, at).ChangeSeverity(context.Background(), tc.actor, "i1", incident.SeveritySEV1)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if repo.incidents["i1"].Severity != incident.SeveritySEV2 || len(repo.events) != 0 {
					t.Errorf("rejected change had effects: %+v events %+v", repo.incidents["i1"], repo.events)
				}
				return
			}
			if got.Severity != incident.SeveritySEV1 || len(repo.events) != 1 {
				t.Fatalf("severity = %s, events = %d; want SEV1, 1", got.Severity, len(repo.events))
			}
			ev := repo.events[0]
			want := map[string]string{"from": "SEV2", "to": "SEV1"}
			if ev.Type != incident.EventSeverityChange || !reflect.DeepEqual(ev.Data, want) ||
				ev.AuthorID == nil || *ev.AuthorID != tc.actor.ID || !ev.OccurredAt.Equal(at) {
				t.Errorf("event = %+v, want cambio_severidad %v by %s at %s", ev, want, tc.actor.ID, at)
			}
		})
	}
}

func TestBR02_SeveridadFijaDesdeResuelto(t *testing.T) {
	for _, tc := range []struct {
		name    string
		state   incident.State
		to      incident.Severity
		wantErr error
	}{
		{"admin baja un SEV1 resuelto", incident.StateResolved, incident.SeveritySEV2, incident.ErrSeverityLocked},
		{"admin sube un cerrado", incident.StateClosed, incident.SeveritySEV1, incident.ErrSeverityLocked},
		{"misma severidad", incident.StateMitigating, incident.SeveritySEV1, timeline.ErrInvalid},
		{"severidad inválida", incident.StateMitigating, "SEV9", timeline.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			inc := repo.incidents["i1"]
			inc.State, inc.Severity = tc.state, incident.SeveritySEV1
			repo.incidents["i1"] = inc
			if _, err := newSvc(repo, fixedNow).ChangeSeverity(context.Background(), admin, "i1", tc.to); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if repo.incidents["i1"].Severity != incident.SeveritySEV1 || len(repo.events) != 0 {
				t.Errorf("rejected change had effects: %+v events %+v", repo.incidents["i1"], repo.events)
			}
		})
	}
}

func TestBR11_OncallAjenoNoCambiaSeveridad(t *testing.T) {
	repo := newFake()
	// A service without on-call: no oncall may change its incidents, the admin may.
	repo.services["search"] = incident.ServiceInfo{Criticality: incident.CriticalityStandard}
	repo.incidents["i2"] = incident.Incident{ID: "i2", ServiceID: "search", Severity: incident.SeveritySEV3, State: incident.StateDeclared}
	svc := newSvc(repo, fixedNow)

	for _, tc := range []struct {
		actor   user.User
		id      string
		wantErr error
	}{
		{otherOncall, "i1", incident.ErrForbidden},
		{ownOncall, "i2", incident.ErrForbidden},
		{admin, "i2", nil},
	} {
		if _, err := svc.ChangeSeverity(context.Background(), tc.actor, tc.id, incident.SeveritySEV1); !errors.Is(err, tc.wantErr) {
			t.Errorf("%s on %s: err = %v, want %v", tc.actor.ID, tc.id, err, tc.wantErr)
		}
	}
	if repo.incidents["i1"].Severity != incident.SeveritySEV2 || len(repo.events) != 1 {
		t.Errorf("i1 = %+v, events = %d; want SEV2 untouched and only the admin's event", repo.incidents["i1"], len(repo.events))
	}
}
