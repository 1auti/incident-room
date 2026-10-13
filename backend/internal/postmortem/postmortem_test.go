package postmortem_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/postmortem"
	"incident-room-backend/internal/user"
)

var (
	fixedNow      = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	admin         = user.User{ID: "u-admin", Role: user.RoleAdmin}
	serviceOncall = user.User{ID: "oncall-1", Role: user.RoleOncall}
	foreignOncall = user.User{ID: "oncall-2", Role: user.RoleOncall}
	engineer      = user.User{ID: "u-eng", Role: user.RoleIngeniero}
)

// fakeStore backs both the incident side (incident, service, timeline) and the
// postmortems, so approving here is what a later T4 sees (BR-07).
type fakeStore struct {
	inc         incident.Incident
	service     incident.ServiceInfo
	events      []incident.TimelineEvent
	postmortems map[string]postmortem.Postmortem
}

func newStore(state incident.State, sev incident.Severity) *fakeStore {
	oncall := serviceOncall.ID
	ana := "u-ana"
	root := "Pool de conexiones agotado por fuga en v2.3"
	return &fakeStore{
		inc: incident.Incident{
			ID: "i1", Title: "Pagos caídos", ServiceID: "payments", Severity: sev, State: state,
			DeclaredAt: fixedNow.Add(-2 * time.Hour), RootCause: &root,
		},
		service: incident.ServiceInfo{Criticality: incident.CriticalityCritical, OncallUserID: &oncall},
		events: []incident.TimelineEvent{
			{IncidentID: "i1", Type: incident.EventDeclaration, AuthorID: &ana, Body: "Pagos devuelven 500", OccurredAt: fixedNow.Add(-2 * time.Hour)},
			{IncidentID: "i1", Type: incident.EventEscalation, OccurredAt: fixedNow.Add(-110 * time.Minute)},
			{IncidentID: "i1", Type: incident.EventStateChange, AuthorID: &oncall, Body: root, Data: map[string]string{"from": "mitigando", "to": "resuelto"}, OccurredAt: fixedNow.Add(-30 * time.Minute)},
		},
		postmortems: map[string]postmortem.Postmortem{},
	}
}

// Incidents side (postmortem.Incidents).
func (f *fakeStore) Get(_ context.Context, id string) (incident.Incident, error) {
	if id != f.inc.ID {
		return incident.Incident{}, incident.ErrNotFound
	}
	return f.inc, nil
}

func (f *fakeStore) FindService(context.Context, string) (incident.ServiceInfo, error) {
	return f.service, nil
}

func (f *fakeStore) ListEvents(_ context.Context, id string) ([]incident.EventView, error) {
	var out []incident.EventView
	for _, e := range f.events {
		if e.IncidentID == id {
			v := incident.EventView{TimelineEvent: e}
			if e.AuthorID != nil {
				name := "Ana"
				v.AuthorName = &name
			}
			out = append(out, v)
		}
	}
	return out, nil
}

// Postmortem side (postmortem.Repository): writes are conditional on borrador, like Postgres.
func (f *fakeStore) Find(_ context.Context, incidentID string) (postmortem.Postmortem, error) {
	pm, ok := f.postmortems[incidentID]
	if !ok {
		return postmortem.Postmortem{}, postmortem.ErrNotFound
	}
	return pm, nil
}

func (f *fakeStore) CreateDraft(_ context.Context, pm postmortem.Postmortem, ev incident.TimelineEvent) (postmortem.Postmortem, error) {
	if f.inc.State != incident.StateResolved {
		return postmortem.Postmortem{}, postmortem.ErrIncidentNotResolved
	}
	if _, ok := f.postmortems[pm.IncidentID]; ok {
		return postmortem.Postmortem{}, postmortem.ErrConflict
	}
	pm.ID = "pm-" + pm.IncidentID
	f.postmortems[pm.IncidentID] = pm
	f.events = append(f.events, ev)
	return pm, nil
}

func (f *fakeStore) update(incidentID string, ev *incident.TimelineEvent, change func(*postmortem.Postmortem)) (postmortem.Postmortem, error) {
	pm, ok := f.postmortems[incidentID]
	if !ok {
		return postmortem.Postmortem{}, postmortem.ErrNotFound
	}
	if pm.Status != postmortem.StatusDraft {
		return postmortem.Postmortem{}, postmortem.ErrApproved
	}
	change(&pm)
	f.postmortems[incidentID] = pm
	if ev != nil {
		f.events = append(f.events, *ev)
	}
	return pm, nil
}

func (f *fakeStore) ReplaceDraft(_ context.Context, incidentID, content string, at time.Time, ev incident.TimelineEvent) (postmortem.Postmortem, error) {
	if f.inc.State != incident.StateResolved {
		return postmortem.Postmortem{}, postmortem.ErrIncidentNotResolved
	}
	return f.update(incidentID, &ev, func(pm *postmortem.Postmortem) { pm.Content, pm.UpdatedAt = content, at })
}

func (f *fakeStore) UpdateContent(_ context.Context, incidentID, content string, at time.Time) (postmortem.Postmortem, error) {
	return f.update(incidentID, nil, func(pm *postmortem.Postmortem) { pm.Content, pm.UpdatedAt = content, at })
}

func (f *fakeStore) Approve(_ context.Context, incidentID, by string, at time.Time, ev incident.TimelineEvent) (postmortem.Postmortem, error) {
	return f.update(incidentID, &ev, func(pm *postmortem.Postmortem) {
		pm.Status, pm.ApprovedBy, pm.ApprovedAt, pm.UpdatedAt = postmortem.StatusApproved, &by, &at, at
	})
}

// Remaining incident.Repository methods, so the same store drives incident.Service (BR-07).
func (f *fakeStore) HasApprovedPostmortem(_ context.Context, id string) (bool, error) {
	pm, ok := f.postmortems[id]
	return ok && pm.Status == postmortem.StatusApproved, nil
}

func (f *fakeStore) UpdateState(_ context.Context, _ string, from, to incident.State, upd incident.StateUpdate, ev incident.TimelineEvent) (incident.Incident, error) {
	if f.inc.State != from {
		return incident.Incident{}, incident.ErrInvalidTransition
	}
	f.inc.State, f.inc.ClosedAt = to, upd.ClosedAt
	f.events = append(f.events, ev)
	return f.inc, nil
}

func (f *fakeStore) List(context.Context, incident.ListQuery) ([]incident.Incident, error) {
	return nil, nil
}

func (f *fakeStore) ListPendingEscalation(context.Context, incident.State) ([]incident.Incident, error) {
	return nil, nil
}

func (f *fakeStore) Escalate(context.Context, incident.Incident, incident.TimelineEvent) (bool, error) {
	return false, nil
}

func (f *fakeStore) Create(context.Context, incident.Incident, []incident.TimelineEvent) (incident.Incident, []incident.TimelineEvent, error) {
	return incident.Incident{}, nil, nil
}

// fakeLLM records the prompts and answers a fixed analysis, or err. during,
// when set, runs while the LLM "thinks" (to change the world meanwhile).
type fakeLLM struct {
	prompts []string
	answer  string
	err     error
	during  func()
}

func (l *fakeLLM) Generate(_ context.Context, prompt string) (string, error) {
	l.prompts = append(l.prompts, prompt)
	if l.during != nil {
		l.during()
	}
	if l.err != nil {
		return "", l.err
	}
	return l.answer, nil
}

func newSvc(store *fakeStore, llm *fakeLLM, now time.Time) *postmortem.Service {
	return postmortem.NewService(store, store, llm, func() time.Time { return now })
}

// postmortemEvents returns the actions of the postmortem events, in order.
func postmortemEvents(store *fakeStore) []string {
	var out []string
	for _, e := range store.events {
		if e.Type == incident.EventPostmortem {
			out = append(out, e.Data["action"])
		}
	}
	return out
}

func TestBR13_GeneraBorradorDesdeTimeline(t *testing.T) {
	for _, actor := range []user.User{serviceOncall, admin} {
		t.Run(string(actor.Role), func(t *testing.T) {
			store, llm := newStore(incident.StateResolved, incident.SeveritySEV1), &fakeLLM{answer: "El pool se agotó por una fuga."}
			pm, err := newSvc(store, llm, fixedNow).Generate(context.Background(), actor, "i1")
			if err != nil {
				t.Fatal(err)
			}
			if pm.Status != postmortem.StatusDraft || !pm.GeneratedByLLM || pm.IncidentID != "i1" || !pm.UpdatedAt.Equal(fixedNow) {
				t.Errorf("postmortem = %+v", pm)
			}
			// The LLM gets the incident, its root cause and every event of the timeline.
			prompt := llm.prompts[0]
			for _, want := range []string{"Pagos caídos", "Pool de conexiones agotado", "Pagos devuelven 500", "escalado"} {
				if !strings.Contains(prompt, want) {
					t.Errorf("prompt lacks %q:\n%s", want, prompt)
				}
			}
			// The content has the analysis and cites every timeline event (option A).
			if !strings.Contains(pm.Content, "El pool se agotó por una fuga.") {
				t.Errorf("content lacks the analysis:\n%s", pm.Content)
			}
			for _, want := range []string{"declaracion", "Pagos devuelven 500", "escalado · Sistema", "cambio_estado", "mitigando → resuelto"} {
				if !strings.Contains(pm.Content, want) {
					t.Errorf("content does not cite %q:\n%s", want, pm.Content)
				}
			}
			if got := postmortemEvents(store); len(got) != 1 || got[0] != "generado" {
				t.Errorf("postmortem events = %v, want [generado]", got)
			}
			ev := store.events[len(store.events)-1]
			if ev.AuthorID == nil || *ev.AuthorID != actor.ID || !ev.OccurredAt.Equal(fixedNow) {
				t.Errorf("event = %+v, want by %s at %s", ev, actor.ID, fixedNow)
			}
		})
	}
}

func TestBR13_SoloGeneraEnResuelto(t *testing.T) {
	for _, tc := range []struct {
		name    string
		state   incident.State
		actor   user.User
		wantErr error
	}{
		{"mitigando", incident.StateMitigating, admin, postmortem.ErrIncidentNotResolved},
		{"declarado", incident.StateDeclared, serviceOncall, postmortem.ErrIncidentNotResolved},
		{"cerrado", incident.StateClosed, admin, postmortem.ErrIncidentNotResolved},
		{"ingeniero", incident.StateResolved, engineer, postmortem.ErrForbidden},
		{"oncall ajeno", incident.StateResolved, foreignOncall, postmortem.ErrForbidden},
		{"rol desconocido", incident.StateResolved, user.User{Role: "visitante"}, postmortem.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, llm := newStore(tc.state, incident.SeveritySEV2), &fakeLLM{answer: "x"}
			if _, err := newSvc(store, llm, fixedNow).Generate(context.Background(), tc.actor, "i1"); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if len(store.postmortems) != 0 || len(postmortemEvents(store)) != 0 || len(llm.prompts) != 0 {
				t.Errorf("rejected generation had effects: %+v, events %v, LLM calls %d", store.postmortems, postmortemEvents(store), len(llm.prompts))
			}
		})
	}

	// A failing LLM stores nothing either.
	store := newStore(incident.StateResolved, incident.SeveritySEV2)
	for _, llm := range []*fakeLLM{{err: errors.New("connection refused")}, {answer: "   "}} {
		if _, err := newSvc(store, llm, fixedNow).Generate(context.Background(), admin, "i1"); !errors.Is(err, postmortem.ErrGeneration) {
			t.Errorf("err = %v, want ErrGeneration", err)
		}
	}
	if len(store.postmortems) != 0 || len(postmortemEvents(store)) != 0 {
		t.Errorf("failed generation had effects")
	}
}

func TestBR13_NoGeneraSiSeCierraMientrasEscribeElLLM(t *testing.T) {
	for _, existing := range []bool{false, true} {
		store := newStore(incident.StateResolved, incident.SeveritySEV2)
		if existing {
			if _, err := newSvc(store, &fakeLLM{answer: "Primera"}, fixedNow).Generate(context.Background(), admin, "i1"); err != nil {
				t.Fatal(err)
			}
		}
		before := store.postmortems["i1"]
		events := len(store.events)
		// T4 lands while the LLM answers: the write must not happen.
		llm := &fakeLLM{answer: "Tarde", during: func() { store.inc.State = incident.StateClosed }}
		if _, err := newSvc(store, llm, fixedNow).Generate(context.Background(), admin, "i1"); !errors.Is(err, postmortem.ErrIncidentNotResolved) {
			t.Errorf("existing=%v: err = %v, want ErrIncidentNotResolved", existing, err)
		}
		if store.postmortems["i1"] != before || len(store.events) != events {
			t.Errorf("existing=%v: generation after closing had effects", existing)
		}
	}
}

func TestBR13_UnPostmortemPorIncidente(t *testing.T) {
	store := newStore(incident.StateResolved, incident.SeveritySEV2)
	svc := newSvc(store, &fakeLLM{answer: "Primera versión"}, fixedNow)
	for range 3 {
		if _, err := svc.Generate(context.Background(), admin, "i1"); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.postmortems) != 1 {
		t.Errorf("postmortems = %d, want 1", len(store.postmortems))
	}
	if got := postmortemEvents(store); strings.Join(got, ",") != "generado,regenerado,regenerado" {
		t.Errorf("events = %v", got)
	}
}

func TestBR13_RegenerarReemplazaBorrador(t *testing.T) {
	store := newStore(incident.StateResolved, incident.SeveritySEV2)
	if _, err := newSvc(store, &fakeLLM{answer: "Primera versión"}, fixedNow).Generate(context.Background(), serviceOncall, "i1"); err != nil {
		t.Fatal(err)
	}
	later := fixedNow.Add(time.Hour)
	pm, err := newSvc(store, &fakeLLM{answer: "Segunda versión"}, later).Generate(context.Background(), serviceOncall, "i1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pm.Content, "Segunda versión") || strings.Contains(pm.Content, "Primera versión") ||
		!pm.UpdatedAt.Equal(later) || pm.Status != postmortem.StatusDraft {
		t.Errorf("regenerated = %+v", pm)
	}
}

func TestBR13_EditableSoloEnBorrador(t *testing.T) {
	store := newStore(incident.StateResolved, incident.SeveritySEV2)
	if _, err := newSvc(store, &fakeLLM{answer: "Borrador"}, fixedNow).Generate(context.Background(), admin, "i1"); err != nil {
		t.Fatal(err)
	}
	later := fixedNow.Add(time.Hour)
	svc := newSvc(store, &fakeLLM{}, later)
	events := len(store.events)

	for _, tc := range []struct {
		actor   user.User
		content string
		wantErr error
	}{
		{engineer, "x", postmortem.ErrForbidden},
		{foreignOncall, "x", postmortem.ErrForbidden},
		{serviceOncall, "  \n ", postmortem.ErrInvalid},
	} {
		if _, err := svc.Edit(context.Background(), tc.actor, "i1", tc.content); !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: err = %v, want %v", tc.actor.ID, err, tc.wantErr)
		}
	}
	// The text is stored as written; editing adds no event (BR-09).
	edited := "  ## Resumen\n    detalle con sangría"
	for _, actor := range []user.User{serviceOncall, admin} {
		pm, err := svc.Edit(context.Background(), actor, "i1", edited+" "+actor.ID)
		if err != nil || pm.Content != edited+" "+actor.ID || !pm.UpdatedAt.Equal(later) || pm.Status != postmortem.StatusDraft {
			t.Errorf("%s: edited = %+v, %v", actor.ID, pm, err)
		}
	}
	if len(store.events) != events {
		t.Errorf("editing added %d events, want 0", len(store.events)-events)
	}
	if _, err := newSvc(newStore(incident.StateResolved, incident.SeveritySEV2), &fakeLLM{}, fixedNow).Edit(context.Background(), admin, "i1", "x"); !errors.Is(err, postmortem.ErrNotFound) {
		t.Errorf("edit without postmortem err = %v, want ErrNotFound", err)
	}
}

func TestBR13_AprobarSoloAdmin(t *testing.T) {
	for _, actor := range []user.User{serviceOncall, engineer, foreignOncall} {
		store := newStore(incident.StateResolved, incident.SeveritySEV1)
		if _, err := newSvc(store, &fakeLLM{answer: "x"}, fixedNow).Generate(context.Background(), admin, "i1"); err != nil {
			t.Fatal(err)
		}
		if _, err := newSvc(store, &fakeLLM{}, fixedNow).Approve(context.Background(), actor, "i1"); !errors.Is(err, postmortem.ErrForbidden) {
			t.Errorf("%s: err = %v, want ErrForbidden", actor.ID, err)
		}
		if pm := store.postmortems["i1"]; pm.Status != postmortem.StatusDraft || pm.ApprovedBy != nil {
			t.Errorf("%s: postmortem = %+v, want still borrador", actor.ID, pm)
		}
	}
}

func TestBR13_AprobarFijaAutorYFecha(t *testing.T) {
	store := newStore(incident.StateResolved, incident.SeveritySEV1)
	if _, err := newSvc(store, &fakeLLM{answer: "x"}, fixedNow).Generate(context.Background(), serviceOncall, "i1"); err != nil {
		t.Fatal(err)
	}
	approvedAt := fixedNow.Add(3 * time.Hour)
	pm, err := newSvc(store, &fakeLLM{}, approvedAt).Approve(context.Background(), admin, "i1")
	if err != nil {
		t.Fatal(err)
	}
	if pm.Status != postmortem.StatusApproved || pm.ApprovedBy == nil || *pm.ApprovedBy != admin.ID ||
		pm.ApprovedAt == nil || !pm.ApprovedAt.Equal(approvedAt) {
		t.Errorf("approved = %+v", pm)
	}
	if got := postmortemEvents(store); strings.Join(got, ",") != "generado,aprobado" {
		t.Errorf("events = %v", got)
	}
	if ev := store.events[len(store.events)-1]; *ev.AuthorID != admin.ID || !ev.OccurredAt.Equal(approvedAt) {
		t.Errorf("approval event = %+v", ev)
	}
}

func TestBR13_AprobadoEsInmutable(t *testing.T) {
	store := newStore(incident.StateResolved, incident.SeveritySEV1)
	svc := newSvc(store, &fakeLLM{answer: "Versión aprobada"}, fixedNow)
	if _, err := svc.Generate(context.Background(), admin, "i1"); err != nil {
		t.Fatal(err)
	}
	approved, err := svc.Approve(context.Background(), admin, "i1")
	if err != nil {
		t.Fatal(err)
	}
	events := len(store.events)
	llm := &fakeLLM{answer: "Otra versión"}
	svc = newSvc(store, llm, fixedNow.Add(time.Hour))
	if _, err := svc.Edit(context.Background(), admin, "i1", "cambio"); !errors.Is(err, postmortem.ErrApproved) {
		t.Errorf("edit err = %v, want ErrApproved", err)
	}
	if _, err := svc.Generate(context.Background(), admin, "i1"); !errors.Is(err, postmortem.ErrApproved) {
		t.Errorf("generate err = %v, want ErrApproved", err)
	}
	if _, err := svc.Approve(context.Background(), admin, "i1"); !errors.Is(err, postmortem.ErrApproved) {
		t.Errorf("second approve err = %v, want ErrApproved", err)
	}
	if store.postmortems["i1"].Content != approved.Content || len(store.events) != events || len(llm.prompts) != 0 {
		t.Errorf("approved postmortem changed or the LLM was called")
	}
}

func TestBR07_PostmortemAprobadoHabilitaCierreSEV1(t *testing.T) {
	store := newStore(incident.StateResolved, incident.SeveritySEV1)
	incidents := incident.NewService(store, func() time.Time { return fixedNow }, incident.DefaultSLA())
	pms := newSvc(store, &fakeLLM{answer: "x"}, fixedNow)

	if _, err := pms.Generate(context.Background(), serviceOncall, "i1"); err != nil {
		t.Fatal(err)
	}
	// A draft does not enable closing.
	if _, err := incidents.Transition(context.Background(), serviceOncall, "i1", incident.StateClosed, ""); !errors.Is(err, incident.ErrPostmortemRequired) {
		t.Fatalf("close with draft err = %v, want ErrPostmortemRequired", err)
	}
	if _, err := pms.Approve(context.Background(), admin, "i1"); err != nil {
		t.Fatal(err)
	}
	closed, err := incidents.Transition(context.Background(), serviceOncall, "i1", incident.StateClosed, "")
	if err != nil || closed.State != incident.StateClosed {
		t.Errorf("close after approval = %+v, %v", closed, err)
	}
}

func TestPostmortem_GetTodoAutenticado(t *testing.T) {
	store := newStore(incident.StateResolved, incident.SeveritySEV2)
	svc := newSvc(store, &fakeLLM{answer: "x"}, fixedNow)
	if _, err := svc.Get(context.Background(), engineer, "i1"); !errors.Is(err, postmortem.ErrNotFound) {
		t.Errorf("get without postmortem err = %v, want ErrNotFound", err)
	}
	if _, err := svc.Generate(context.Background(), admin, "i1"); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []user.User{engineer, serviceOncall, admin} {
		if pm, err := svc.Get(context.Background(), actor, "i1"); err != nil || pm.IncidentID != "i1" {
			t.Errorf("%s: get = %+v, %v", actor.Role, pm, err)
		}
	}
	if _, err := svc.Get(context.Background(), user.User{Role: "visitante"}, "i1"); !errors.Is(err, postmortem.ErrForbidden) {
		t.Errorf("visitor err = %v, want ErrForbidden", err)
	}
}
