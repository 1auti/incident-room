package service_test

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/service"
	"incident-room-backend/internal/user"
)

type fakeRepo struct {
	items     map[string]service.Service
	incidents map[string]bool
	runbooks  map[string]bool
	depErr    error
	calls     int
	seq       int

	setCalls  int
	setID     string
	setUser   string
	setStates []incident.State
	setEvent  incident.TimelineEvent
}

func (f *fakeRepo) SetOncall(_ context.Context, id, userID string, active []incident.State, ev incident.TimelineEvent) (service.Service, error) {
	f.calls++
	f.setCalls++
	f.setID, f.setUser, f.setStates, f.setEvent = id, userID, active, ev
	s, ok := f.items[id]
	if !ok {
		return service.Service{}, service.ErrNotFound
	}
	s.OncallUserID = &userID
	f.items[id] = s
	return s, nil
}

// fakeUsers resolves users by id like user.Repository.FindByID.
type fakeUsers map[string]user.User

func (f fakeUsers) FindByID(_ context.Context, id string) (user.User, error) {
	u, ok := f[id]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}

var (
	fixedNow = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	users    = fakeUsers{
		"u-oc":  {ID: "u-oc", Role: user.RoleOncall},
		"u-oc2": {ID: "u-oc2", Role: user.RoleOncall},
		"u-ing": {ID: "u-ing", Role: user.RoleIngeniero},
		"u-adm": {ID: "u-adm", Role: user.RoleAdmin},
	}
)

func newManager(repo *fakeRepo) *service.Manager {
	return service.NewManager(repo, users, func() time.Time { return fixedNow })
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: map[string]service.Service{}, incidents: map[string]bool{}, runbooks: map[string]bool{}}
}

func (f *fakeRepo) taken(name, exceptID string) bool {
	for id, s := range f.items {
		if id != exceptID && strings.EqualFold(s.Name, name) {
			return true
		}
	}
	return false
}

func (f *fakeRepo) List(context.Context) ([]service.Service, error) {
	f.calls++
	out := []service.Service{}
	for _, s := range f.items {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *fakeRepo) Create(_ context.Context, name string, c incident.Criticality) (service.Service, error) {
	f.calls++
	if f.taken(name, "") {
		return service.Service{}, service.ErrNameTaken
	}
	f.seq++
	s := service.Service{ID: "svc-" + strconv.Itoa(f.seq), Name: name, Criticality: c}
	f.items[s.ID] = s
	return s, nil
}

func (f *fakeRepo) Update(_ context.Context, id, name string, c incident.Criticality) (service.Service, error) {
	f.calls++
	s, ok := f.items[id]
	if !ok {
		return service.Service{}, service.ErrNotFound
	}
	if f.taken(name, id) {
		return service.Service{}, service.ErrNameTaken
	}
	s.Name, s.Criticality = name, c
	f.items[id] = s
	return s, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	f.calls++
	if _, ok := f.items[id]; !ok {
		return service.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func (f *fakeRepo) HasIncidents(_ context.Context, id string) (bool, error) {
	f.calls++
	return f.incidents[id], f.depErr
}

func (f *fakeRepo) HasRunbooks(_ context.Context, id string) (bool, error) {
	f.calls++
	return f.runbooks[id], f.depErr
}

func (f *fakeRepo) snapshot() map[string]service.Service {
	out := map[string]service.Service{}
	for k, v := range f.items {
		out[k] = v
	}
	return out
}

func sameItems(a, b map[string]service.Service) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

var (
	admin    = user.User{ID: "u-admin", Role: user.RoleAdmin}
	ingenier = user.User{ID: "u-ing", Role: user.RoleIngeniero}
	oncall   = user.User{ID: "u-oc", Role: user.RoleOncall}
	ctx      = context.Background()
)

func seeded(t *testing.T) (*service.Manager, *fakeRepo, service.Service) {
	t.Helper()
	repo := newFakeRepo()
	m := newManager(repo)
	s, err := m.Create(ctx, admin, "Pagos", incident.CriticalityImportant)
	if err != nil {
		t.Fatal(err)
	}
	return m, repo, s
}

func TestBR12_SoloAdminGestionaServicios(t *testing.T) {
	for _, actor := range []user.User{ingenier, oncall} {
		for _, op := range []string{"create", "update", "delete"} {
			t.Run(string(actor.Role)+"/"+op, func(t *testing.T) {
				m, repo, s := seeded(t)
				before, calls := repo.snapshot(), repo.calls
				var err error
				switch op {
				case "create":
					_, err = m.Create(ctx, actor, "Nuevo", incident.CriticalityCritical)
				case "update":
					_, err = m.Update(ctx, actor, s.ID, "Otro", incident.CriticalityCritical)
				case "delete":
					err = m.Delete(ctx, actor, s.ID)
				}
				if !errors.Is(err, service.ErrForbidden) {
					t.Fatalf("err = %v, want ErrForbidden", err)
				}
				if repo.calls != calls || !sameItems(before, repo.items) {
					t.Errorf("forbidden action touched the repository")
				}
			})
		}
	}
	m, _, s := seeded(t)
	if _, err := m.Update(ctx, admin, s.ID, "Pagos 2", incident.CriticalityStandard); err != nil {
		t.Errorf("admin update: %v", err)
	}
	if err := m.Delete(ctx, admin, s.ID); err != nil {
		t.Errorf("admin delete: %v", err)
	}
}

func TestBR01_CriticidadValidaAlGestionarServicio(t *testing.T) {
	cases := []struct {
		crit incident.Criticality
		ok   bool
	}{
		{incident.CriticalityCritical, true},
		{incident.CriticalityImportant, true},
		{incident.CriticalityStandard, true},
		{"", false},
		{"alta", false},
		{"CRITICA", false},
	}
	for _, tc := range cases {
		t.Run("create/"+string(tc.crit), func(t *testing.T) {
			repo := newFakeRepo()
			s, err := newManager(repo).Create(ctx, admin, "Svc", tc.crit)
			if tc.ok {
				if err != nil || s.Criticality != tc.crit {
					t.Fatalf("Create = %+v, %v", s, err)
				}
				return
			}
			if !errors.Is(err, service.ErrInvalid) || len(repo.items) != 0 {
				t.Fatalf("err = %v items = %d, want ErrInvalid and none", err, len(repo.items))
			}
		})
		t.Run("update/"+string(tc.crit), func(t *testing.T) {
			m, repo, s := seeded(t)
			before := repo.snapshot()
			got, err := m.Update(ctx, admin, s.ID, s.Name, tc.crit)
			if tc.ok {
				if err != nil || got.Criticality != tc.crit {
					t.Fatalf("Update = %+v, %v", got, err)
				}
				return
			}
			if !errors.Is(err, service.ErrInvalid) || !sameItems(before, repo.items) {
				t.Fatalf("err = %v, want ErrInvalid and no change", err)
			}
		})
	}
}

func TestUC111_CrearSinOncall(t *testing.T) {
	m := newManager(newFakeRepo())
	s, err := m.Create(ctx, admin, "  Auth  ", incident.CriticalityCritical)
	if err != nil {
		t.Fatal(err)
	}
	if s.OncallUserID != nil || s.Name != "Auth" {
		t.Errorf("Create = %+v, want trimmed name and nil on-call", s)
	}
}

func TestUC112_NombreVacioODuplicado(t *testing.T) {
	m, repo, _ := seeded(t)
	for _, name := range []string{"", "   "} {
		if _, err := m.Create(ctx, admin, name, incident.CriticalityStandard); !errors.Is(err, service.ErrInvalid) {
			t.Errorf("name %q err = %v, want ErrInvalid", name, err)
		}
	}
	for _, name := range []string{"Pagos", "pagos", " PAGOS "} {
		if _, err := m.Create(ctx, admin, name, incident.CriticalityStandard); !errors.Is(err, service.ErrNameTaken) {
			t.Errorf("name %q err = %v, want ErrNameTaken", name, err)
		}
	}
	if len(repo.items) != 1 {
		t.Errorf("services = %d, want 1", len(repo.items))
	}
}

func TestUC113_EditarNombreDuplicadoNoCambia(t *testing.T) {
	m, repo, s := seeded(t)
	if _, err := m.Create(ctx, admin, "Auth", incident.CriticalityCritical); err != nil {
		t.Fatal(err)
	}
	before := repo.snapshot()
	if _, err := m.Update(ctx, admin, s.ID, "AUTH", incident.CriticalityImportant); !errors.Is(err, service.ErrNameTaken) {
		t.Fatalf("err = %v, want ErrNameTaken", err)
	}
	if _, err := m.Update(ctx, admin, s.ID, "  ", incident.CriticalityImportant); !errors.Is(err, service.ErrInvalid) {
		t.Fatalf("empty name err = %v, want ErrInvalid", err)
	}
	if !sameItems(before, repo.items) {
		t.Errorf("service changed on rejected update")
	}
	// Keeping its own name (different case) is allowed.
	if got, err := m.Update(ctx, admin, s.ID, "pagos", incident.CriticalityImportant); err != nil || got.Name != "pagos" {
		t.Errorf("rename to own name = %+v, %v", got, err)
	}
}

func TestBR20_BajaRechazadaConIncidentes(t *testing.T) {
	m, repo, s := seeded(t)
	repo.incidents[s.ID] = true
	if err := m.Delete(ctx, admin, s.ID); !errors.Is(err, service.ErrInUse) {
		t.Fatalf("err = %v, want ErrInUse", err)
	}
	if _, ok := repo.items[s.ID]; !ok {
		t.Errorf("service removed despite incidents")
	}
}

func TestBR20_BajaRechazadaConRunbooks(t *testing.T) {
	m, repo, s := seeded(t)
	repo.runbooks[s.ID] = true
	if err := m.Delete(ctx, admin, s.ID); !errors.Is(err, service.ErrInUse) {
		t.Fatalf("err = %v, want ErrInUse", err)
	}
	if _, ok := repo.items[s.ID]; !ok {
		t.Errorf("service removed despite runbooks")
	}
}

func TestBR20_BajaSinDependencias(t *testing.T) {
	m, _, s := seeded(t)
	if err := m.Delete(ctx, admin, s.ID); err != nil {
		t.Fatal(err)
	}
	list, err := m.List(ctx)
	if err != nil || len(list) != 0 {
		t.Errorf("List = %v, %v; want empty", list, err)
	}
}

func TestBR20_ErrorAlConsultarDependenciasNoBorra(t *testing.T) {
	m, repo, s := seeded(t)
	repo.depErr = errors.New("db down")
	err := m.Delete(ctx, admin, s.ID)
	if err == nil || errors.Is(err, service.ErrInUse) {
		t.Fatalf("err = %v, want a propagated dependency error", err)
	}
	if _, ok := repo.items[s.ID]; !ok {
		t.Errorf("service removed despite dependency check failure (must fail closed)")
	}
}

func TestBR12_SoloAdminAsignaOncall(t *testing.T) {
	for _, actor := range []user.User{ingenier, oncall, {ID: "x", Role: "visitante"}} {
		t.Run(string(actor.Role), func(t *testing.T) {
			m, repo, s := seeded(t)
			before, calls := repo.snapshot(), repo.calls
			if _, err := m.SetOncall(ctx, actor, s.ID, "u-oc"); !errors.Is(err, service.ErrForbidden) {
				t.Fatalf("err = %v, want ErrForbidden", err)
			}
			if repo.calls != calls || repo.setCalls != 0 || !sameItems(before, repo.items) {
				t.Errorf("forbidden action touched the repository")
			}
		})
	}
	m, _, s := seeded(t)
	got, err := m.SetOncall(ctx, admin, s.ID, "u-oc")
	if err != nil || got.OncallUserID == nil || *got.OncallUserID != "u-oc" {
		t.Errorf("admin SetOncall = %+v, %v", got, err)
	}
}

func TestBR11_SoloUsuarioOncallEsAsignable(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		wantErr      error
	}{
		{"usuario oncall", "u-oc", nil},
		{"ingeniero", "u-ing", service.ErrInvalidOncall},
		{"admin", "u-adm", service.ErrInvalidOncall},
		{"inexistente", "nope", service.ErrInvalidOncall},
		{"vacio", "", service.ErrInvalidOncall},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, repo, s := seeded(t)
			before := repo.snapshot()
			_, err := m.SetOncall(ctx, admin, s.ID, tc.target)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && (repo.setCalls != 0 || !sameItems(before, repo.items)) {
				t.Errorf("rejected assignment changed the service")
			}
		})
	}
}

func TestBR11_CambioOncallReasignaIncidentesActivos(t *testing.T) {
	m, repo, s := seeded(t)
	if _, err := m.SetOncall(ctx, admin, s.ID, "u-oc2"); err != nil {
		t.Fatal(err)
	}
	if repo.setCalls != 1 || repo.setID != s.ID || repo.setUser != "u-oc2" {
		t.Fatalf("SetOncall calls = %d id = %q user = %q", repo.setCalls, repo.setID, repo.setUser)
	}
	// Every state but cerrado is active (BR-15): resuelto is reassigned, cerrado is not.
	want := []incident.State{incident.StateDeclared, incident.StateAcknowledged, incident.StateMitigating, incident.StateResolved}
	if len(repo.setStates) != len(want) {
		t.Fatalf("active states = %v, want %v", repo.setStates, want)
	}
	for i, st := range want {
		if repo.setStates[i] != st {
			t.Errorf("active states = %v, want %v", repo.setStates, want)
		}
	}
	ev := repo.setEvent
	if ev.Type != incident.EventAssignment || ev.AuthorID == nil || *ev.AuthorID != "u-admin" ||
		!ev.OccurredAt.Equal(fixedNow) || ev.Data["to"] != "u-oc2" {
		t.Errorf("event template = %+v, want asignacion by the admin at the injected clock to u-oc2", ev)
	}
}
