package incident_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

var (
	serviceOncall = user.User{ID: "oncall-1", Role: user.RoleOncall}
	foreignOncall = user.User{ID: "oncall-2", Role: user.RoleOncall}
	adminUser     = user.User{ID: "u-admin", Role: user.RoleAdmin}
)

const rootCause = "Pool de conexiones agotado por fuga en v2.3"

// seedIn stores incident i1 of the service "important" (on-call oncall-1) in state st.
func seedIn(repo *fakeRepo, st incident.State, sev incident.Severity) {
	seedDeclared(repo, "i1", "important", sev)
	repo.seeded[0].State = st
}

// assertUnchanged fails when a rejected transition touched the incident or the timeline.
func assertUnchanged(t *testing.T, repo *fakeRepo, st incident.State) {
	t.Helper()
	inc := repo.seeded[0]
	if inc.State != st || len(repo.events) != 0 || inc.RootCause != nil || inc.ResolvedAt != nil || inc.ClosedAt != nil {
		t.Errorf("rejected transition changed the incident: %+v, events %+v", inc, repo.events)
	}
}

func TestBR08_SoloTransicionesT1aT4(t *testing.T) {
	for _, tc := range []struct {
		name    string
		from    incident.State
		to      incident.State
		wantErr error
	}{
		{"T1 declarado a reconocido", incident.StateDeclared, incident.StateAcknowledged, nil},
		{"T2 reconocido a mitigando", incident.StateAcknowledged, incident.StateMitigating, nil},
		{"T3 mitigando a resuelto", incident.StateMitigating, incident.StateResolved, nil},
		{"T4 resuelto a cerrado", incident.StateResolved, incident.StateClosed, nil},
		{"salto declarado a resuelto", incident.StateDeclared, incident.StateResolved, incident.ErrInvalidTransition},
		{"salto reconocido a cerrado", incident.StateAcknowledged, incident.StateClosed, incident.ErrInvalidTransition},
		{"retroceso mitigando a reconocido", incident.StateMitigating, incident.StateAcknowledged, incident.ErrInvalidTransition},
		{"reapertura resuelto a mitigando", incident.StateResolved, incident.StateMitigating, incident.ErrInvalidTransition},
		{"reapertura cerrado a resuelto", incident.StateClosed, incident.StateResolved, incident.ErrInvalidTransition},
		{"mismo estado", incident.StateMitigating, incident.StateMitigating, incident.ErrInvalidTransition},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			seedIn(repo, tc.from, incident.SeveritySEV2)
			svc, _ := clockedSvc(repo, fixedNow)
			_, err := svc.Transition(context.Background(), adminUser, "i1", tc.to, rootCause)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				assertUnchanged(t, repo, tc.from)
				return
			}
			if repo.seeded[0].State != tc.to || len(repo.events) != 1 {
				t.Fatalf("state = %s, events = %d; want %s, 1", repo.seeded[0].State, len(repo.events), tc.to)
			}
			ev := repo.events[0]
			if ev.Type != incident.EventStateChange || ev.Data["from"] != string(tc.from) || ev.Data["to"] != string(tc.to) ||
				ev.AuthorID == nil || *ev.AuthorID != adminUser.ID {
				t.Errorf("event = %+v, want cambio_estado %s -> %s by the admin", ev, tc.from, tc.to)
			}
		})
	}
}

func TestBR08_IncidenteInexistente(t *testing.T) {
	repo := newFake()
	seedIn(repo, incident.StateDeclared, incident.SeveritySEV2)
	svc, _ := clockedSvc(repo, fixedNow)
	if _, err := svc.Transition(context.Background(), adminUser, "nope", incident.StateAcknowledged, ""); !errors.Is(err, incident.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	assertUnchanged(t, repo, incident.StateDeclared)
}

func TestBR11_OncallSoloTransicionaSusServicios(t *testing.T) {
	for _, tc := range []struct {
		name    string
		actor   user.User
		wantErr error
	}{
		{"on-call del servicio", serviceOncall, nil},
		{"admin", adminUser, nil},
		{"on-call de otro servicio", foreignOncall, incident.ErrForbidden},
		{"ingeniero", engineer, incident.ErrForbidden},
		{"rol desconocido", user.User{ID: "x", Role: "visitante"}, incident.ErrForbidden},
	} {
		// T1-T4 follow the same rule (BR-11).
		for _, step := range []struct{ from, to incident.State }{
			{incident.StateDeclared, incident.StateAcknowledged},
			{incident.StateAcknowledged, incident.StateMitigating},
			{incident.StateMitigating, incident.StateResolved},
			{incident.StateResolved, incident.StateClosed},
		} {
			t.Run(tc.name+" "+string(step.to), func(t *testing.T) {
				repo := newFake()
				seedIn(repo, step.from, incident.SeveritySEV2)
				svc, _ := clockedSvc(repo, fixedNow)
				_, err := svc.Transition(context.Background(), tc.actor, "i1", step.to, rootCause)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					assertUnchanged(t, repo, step.from)
				}
			})
		}
	}
}

func TestBR06_ResolverRequiereCausaRaiz(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rootCause string
		wantErr   error
		wantSaved string
	}{
		{"vacía", "", incident.ErrRootCauseRequired, ""},
		{"solo espacios", "  \t\n ", incident.ErrRootCauseRequired, ""},
		{"con texto", rootCause, nil, rootCause},
		{"con espacios alrededor", "  " + rootCause + "\n", nil, rootCause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			seedIn(repo, incident.StateMitigating, incident.SeveritySEV2)
			svc, _ := clockedSvc(repo, fixedNow)
			got, err := svc.Transition(context.Background(), serviceOncall, "i1", incident.StateResolved, tc.rootCause)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				assertUnchanged(t, repo, incident.StateMitigating)
				return
			}
			if got.State != incident.StateResolved || got.RootCause == nil || *got.RootCause != tc.wantSaved {
				t.Errorf("incident = %+v, want resuelto with root cause %q", got, tc.wantSaved)
			}
			// The cambio_estado event carries the root cause as its readable body.
			if len(repo.events) != 1 || repo.events[0].Body != tc.wantSaved {
				t.Errorf("events = %+v, want one with body %q", repo.events, tc.wantSaved)
			}
		})
	}

	// Only T3 stores a root cause: other transitions ignore one sent along.
	repo := newFake()
	seedIn(repo, incident.StateAcknowledged, incident.SeveritySEV2)
	svc, _ := clockedSvc(repo, fixedNow)
	got, err := svc.Transition(context.Background(), adminUser, "i1", incident.StateMitigating, rootCause)
	if err != nil || got.RootCause != nil || repo.events[0].Body != "" {
		t.Errorf("T2 = %+v, %v; want no root cause stored", got, err)
	}
}

func TestBR07_CerrarSEV1RequierePostmortemAprobado(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sev      incident.Severity
		approved bool
		wantErr  error
	}{
		// A draft postmortem counts as "not approved": the repository only reports approved ones.
		{"SEV1 sin postmortem aprobado", incident.SeveritySEV1, false, incident.ErrPostmortemRequired},
		{"SEV1 con postmortem aprobado", incident.SeveritySEV1, true, nil},
		{"SEV2 sin postmortem", incident.SeveritySEV2, false, nil},
		{"SEV3 sin postmortem", incident.SeveritySEV3, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFake()
			seedIn(repo, incident.StateResolved, tc.sev)
			repo.approvedPostmortems = map[string]bool{"i1": tc.approved}
			svc, _ := clockedSvc(repo, fixedNow)
			got, err := svc.Transition(context.Background(), adminUser, "i1", incident.StateClosed, "")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				assertUnchanged(t, repo, incident.StateResolved)
				return
			}
			if got.State != incident.StateClosed || got.ClosedAt == nil {
				t.Errorf("incident = %+v, want cerrado with closed_at", got)
			}
		})
	}
}

func TestBR05_TransicionesUsanRelojInyectable(t *testing.T) {
	repo := newFake()
	seedIn(repo, incident.StateDeclared, incident.SeveritySEV2)
	svc, now := clockedSvc(repo, fixedNow)
	steps := []struct {
		to    incident.State
		after time.Duration
	}{
		{incident.StateAcknowledged, 3 * time.Minute},
		{incident.StateMitigating, 10 * time.Minute},
		{incident.StateResolved, 40 * time.Minute},
		{incident.StateClosed, 2 * time.Hour},
	}
	for _, step := range steps {
		*now = fixedNow.Add(step.after)
		if _, err := svc.Transition(context.Background(), serviceOncall, "i1", step.to, rootCause); err != nil {
			t.Fatalf("%s: %v", step.to, err)
		}
	}
	if len(repo.events) != len(steps) {
		t.Fatalf("events = %d, want one per transition (%d)", len(repo.events), len(steps))
	}
	inc := repo.seeded[0]
	at := func(d time.Duration) time.Time { return fixedNow.Add(d) }
	if inc.AcknowledgedAt == nil || !inc.AcknowledgedAt.Equal(at(3*time.Minute)) ||
		inc.ResolvedAt == nil || !inc.ResolvedAt.Equal(at(40*time.Minute)) ||
		inc.ClosedAt == nil || !inc.ClosedAt.Equal(at(2*time.Hour)) {
		t.Errorf("timestamps = ack %v resolved %v closed %v", inc.AcknowledgedAt, inc.ResolvedAt, inc.ClosedAt)
	}
	for i, ev := range repo.events {
		if !ev.OccurredAt.Equal(at(steps[i].after)) || ev.OccurredAt.Location() != time.UTC {
			t.Errorf("event %d occurred_at = %v, want %v UTC", i, ev.OccurredAt, at(steps[i].after))
		}
	}
}
