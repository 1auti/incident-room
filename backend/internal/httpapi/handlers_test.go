package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/httpapi"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/service"
	"incident-room-backend/internal/user"
)

type fakeUsers struct {
	mu   sync.Mutex
	list []user.User
}

func (f *fakeUsers) Create(_ context.Context, u user.User) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.list {
		if x.Email == u.Email {
			return user.User{}, user.ErrEmailTaken
		}
	}
	u.ID = "id-" + u.Email
	f.list = append(f.list, u)
	return u, nil
}

func (f *fakeUsers) find(match func(user.User) bool) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.list {
		if match(x) {
			return x, nil
		}
	}
	return user.User{}, user.ErrNotFound
}

func (f *fakeUsers) FindByEmail(_ context.Context, e string) (user.User, error) {
	return f.find(func(u user.User) bool { return u.Email == e })
}

func (f *fakeUsers) FindByID(_ context.Context, id string) (user.User, error) {
	return f.find(func(u user.User) bool { return u.ID == id })
}

func (f *fakeUsers) UpdateRole(_ context.Context, id string, r user.Role) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.list {
		if f.list[i].ID == id {
			f.list[i].Role = r
			return nil
		}
	}
	return user.ErrNotFound
}

func (f *fakeUsers) ExistsAdmin(context.Context) (bool, error) {
	_, err := f.find(func(u user.User) bool { return u.Role == user.RoleAdmin })
	return err == nil, nil
}

type fakeSessions struct {
	mu sync.Mutex
	m  map[string]auth.Session
}

func (f *fakeSessions) Create(_ context.Context, s auth.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[s.TokenHash] = s
	return nil
}

func (f *fakeSessions) FindByTokenHash(_ context.Context, h string) (auth.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.m[h]
	if !ok {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	return s, nil
}

type fakeServiceRepo struct {
	mu        sync.Mutex
	list      []service.Service
	withDeps  map[string]bool // services that have incidents or runbooks (BR-20)
	nextIndex int
}

func (f *fakeServiceRepo) List(context.Context) ([]service.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]service.Service(nil), f.list...), nil
}

func (f *fakeServiceRepo) nameUsed(name, exceptID string) bool {
	for _, s := range f.list {
		if s.ID != exceptID && strings.EqualFold(s.Name, name) {
			return true
		}
	}
	return false
}

func (f *fakeServiceRepo) Create(_ context.Context, name string, c incident.Criticality) (service.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nameUsed(name, "") {
		return service.Service{}, service.ErrNameTaken
	}
	f.nextIndex++
	s := service.Service{ID: fmt.Sprintf("svc-%d", f.nextIndex), Name: name, Criticality: c}
	f.list = append(f.list, s)
	return s, nil
}

func (f *fakeServiceRepo) Update(_ context.Context, id, name string, c incident.Criticality) (service.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.list {
		if f.list[i].ID != id {
			continue
		}
		if f.nameUsed(name, id) {
			return service.Service{}, service.ErrNameTaken
		}
		f.list[i].Name, f.list[i].Criticality = name, c
		return f.list[i], nil
	}
	return service.Service{}, service.ErrNotFound
}

func (f *fakeServiceRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.list {
		if f.list[i].ID == id {
			f.list = append(f.list[:i], f.list[i+1:]...)
			return nil
		}
	}
	return service.ErrNotFound
}

func (f *fakeServiceRepo) HasIncidents(_ context.Context, id string) (bool, error) {
	return f.withDeps[id], nil
}

func (f *fakeServiceRepo) HasRunbooks(context.Context, string) (bool, error) { return false, nil }

// fakeIncidentRepo resolves services from the fake catalog and records what is stored.
type fakeIncidentRepo struct {
	mu        sync.Mutex
	services  *fakeServiceRepo
	incidents []incident.Incident
	events    []incident.TimelineEvent
}

func (f *fakeIncidentRepo) FindService(_ context.Context, id string) (incident.ServiceInfo, error) {
	f.services.mu.Lock()
	defer f.services.mu.Unlock()
	for _, s := range f.services.list {
		if s.ID == id {
			return incident.ServiceInfo{Criticality: s.Criticality, OncallUserID: s.OncallUserID}, nil
		}
	}
	return incident.ServiceInfo{}, incident.ErrServiceNotFound
}

func (f *fakeIncidentRepo) Create(_ context.Context, inc incident.Incident, evs []incident.TimelineEvent) (incident.Incident, []incident.TimelineEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc.ID = fmt.Sprintf("inc-%d", len(f.incidents)+1)
	out := make([]incident.TimelineEvent, len(evs))
	for i, e := range evs {
		e.ID = fmt.Sprintf("ev-%d", len(f.events)+i+1)
		e.IncidentID = inc.ID
		out[i] = e
	}
	f.incidents = append(f.incidents, inc)
	f.events = append(f.events, out...)
	return inc, out, nil
}

var fixedNow = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

type env struct {
	h         http.Handler
	users     *fakeUsers
	sessions  *fakeSessions
	usersSvc  *user.Service
	services  *fakeServiceRepo
	incidents *fakeIncidentRepo
}

func newEnv(t *testing.T) *env {
	t.Helper()
	users := &fakeUsers{}
	sessions := &fakeSessions{m: map[string]auth.Session{}}
	services := &fakeServiceRepo{withDeps: map[string]bool{}}
	us := user.NewService(users)
	as := auth.NewService(users, sessions, time.Hour, time.Now)
	incidents := &fakeIncidentRepo{services: services}
	is := incident.NewService(incidents, func() time.Time { return fixedNow })
	h := httpapi.New(us, as, service.NewManager(services), is, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &env{h: h, users: users, sessions: sessions, usersSvc: us, services: services, incidents: incidents}
}

func (e *env) do(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func (e *env) loginCookie(t *testing.T, email, password string) *http.Cookie {
	t.Helper()
	w := e.do("POST", "/api/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, body %s", w.Code, w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func (e *env) register(t *testing.T, name, email string) {
	t.Helper()
	body := `{"name":"` + name + `","email":"` + email + `","password":"secret-pass"}`
	if w := e.do("POST", "/api/auth/register", body, nil); w.Code != http.StatusCreated {
		t.Fatalf("register status = %d", w.Code)
	}
}

func TestUC011_RegistroCreaIngenieroSinHash(t *testing.T) {
	e := newEnv(t)
	w := e.do("POST", "/api/auth/register", `{"name":"Ana","email":"Ana@x.com","password":"secret-pass"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["role"] != "ingeniero" {
		t.Errorf("role = %v", got["role"])
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "password") {
		t.Errorf("body leaks password data: %s", w.Body)
	}
}

func TestUC012_EmailDuplicadoRechazado(t *testing.T) {
	e := newEnv(t)
	e.register(t, "Ana", "ana@x.com")
	w := e.do("POST", "/api/auth/register", `{"name":"Otra","email":"ANA@x.com","password":"secret-pass"}`, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d", w.Code)
	}
	if len(e.users.list) != 1 {
		t.Errorf("users = %d, want 1", len(e.users.list))
	}
}

func TestUC011_FormatoInvalido(t *testing.T) {
	e := newEnv(t)
	for _, body := range []string{`{`, `{"name":"","email":"a@x.com","password":"secret-pass"}`, `{"name":"A","email":"","password":"secret-pass"}`, `{"name":"A","email":"a@x.com","password":""}`} {
		if w := e.do("POST", "/api/auth/register", body, nil); w.Code != http.StatusBadRequest {
			t.Errorf("body %q status = %d, want 400", body, w.Code)
		}
	}
	if len(e.users.list) != 0 {
		t.Errorf("users created on invalid input")
	}
}

func TestUC013_Login(t *testing.T) {
	e := newEnv(t)
	e.register(t, "Ana", "ana@x.com")
	c := e.loginCookie(t, "ana@x.com", "secret-pass")
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags: httpOnly=%v sameSite=%v", c.HttpOnly, c.SameSite)
	}
	w := e.do("GET", "/api/me", "", c)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "password") {
		t.Errorf("me: status %d body %s", w.Code, w.Body)
	}

	before := len(e.sessions.m)
	for _, body := range []string{`{"email":"ana@x.com","password":"wrong"}`, `{"email":"nadie@x.com","password":"secret-pass"}`} {
		w := e.do("POST", "/api/auth/login", body, nil)
		if w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
			t.Errorf("bad login: status %d cookies %d", w.Code, len(w.Result().Cookies()))
		}
	}
	if len(e.sessions.m) != before {
		t.Errorf("session created on failed login")
	}
}

func TestBR10_RutasProtegidasSinSesion(t *testing.T) {
	e := newEnv(t)
	e.register(t, "Ana", "ana@x.com")
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/me", ""},
		{"PATCH", "/api/users/id-ana@x.com/role", `{"role":"admin"}`},
		{"POST", "/api/incidents", `{"title":"x","service_id":"svc-1","impact":"menor"}`},
		{"GET", "/api/incidents/suggested-severity?service_id=svc-1&impact=menor", ""},
	} {
		w := e.do(tc.method, tc.path, tc.body, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want 401", tc.method, tc.path, w.Code)
		}
	}
	if e.users.list[0].Role != user.RoleIngeniero {
		t.Errorf("role changed without session")
	}
	if len(e.incidents.incidents) != 0 || len(e.incidents.events) != 0 {
		t.Errorf("incident or events created without session")
	}
}

func TestBR12_SoloAdminCambiaRol(t *testing.T) {
	e := newEnv(t)
	if err := e.usersSvc.EnsureAdmin(context.Background(), "root@x.com", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	e.register(t, "Ana", "ana@x.com")
	e.register(t, "Bruno", "bruno@x.com")
	target := "/api/users/id-bruno@x.com/role"

	// A non-admin is forbidden and the role stays the same.
	ana := e.loginCookie(t, "ana@x.com", "secret-pass")
	if w := e.do("PATCH", target, `{"role":"admin"}`, ana); w.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d, want 403", w.Code)
	}
	if got, _ := e.users.FindByID(context.Background(), "id-bruno@x.com"); got.Role != user.RoleIngeniero {
		t.Fatalf("role changed by non-admin: %s", got.Role)
	}

	// The admin can change it; invalid roles are 400; unknown users are 404.
	admin := e.loginCookie(t, "root@x.com", "admin-pass")
	if w := e.do("PATCH", target, `{"role":"oncall"}`, admin); w.Code != http.StatusOK {
		t.Fatalf("admin status = %d", w.Code)
	}
	if got, _ := e.users.FindByID(context.Background(), "id-bruno@x.com"); got.Role != user.RoleOncall {
		t.Errorf("role = %s, want oncall", got.Role)
	}
	if w := e.do("PATCH", target, `{"role":"root"}`, admin); w.Code != http.StatusBadRequest {
		t.Errorf("invalid role status = %d, want 400", w.Code)
	}
	if w := e.do("PATCH", "/api/users/nope/role", `{"role":"oncall"}`, admin); w.Code != http.StatusNotFound {
		t.Errorf("unknown user status = %d, want 404", w.Code)
	}
}

// cookies registers one user per role and returns their session cookies.
func (e *env) cookies(t *testing.T) (admin, eng, oncall *http.Cookie) {
	t.Helper()
	ctx := context.Background()
	if err := e.usersSvc.EnsureAdmin(ctx, "root@x.com", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	e.register(t, "Ana", "ana@x.com")
	e.register(t, "Oli", "oli@x.com")
	if err := e.users.UpdateRole(ctx, "id-oli@x.com", user.RoleOncall); err != nil {
		t.Fatal(err)
	}
	return e.loginCookie(t, "root@x.com", "admin-pass"), e.loginCookie(t, "ana@x.com", "secret-pass"), e.loginCookie(t, "oli@x.com", "secret-pass")
}

func (e *env) listServices(t *testing.T, c *http.Cookie) []map[string]any {
	t.Helper()
	w := e.do("GET", "/api/services", "", c)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *env) createService(t *testing.T, c *http.Cookie, name string) string {
	t.Helper()
	w := e.do("POST", "/api/services", `{"name":"`+name+`","criticality":"importante"}`, c)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", w.Code, w.Body)
	}
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	return s["id"].(string)
}

func TestUC111_CrearServicio(t *testing.T) {
	e := newEnv(t)
	admin, _, _ := e.cookies(t)
	w := e.do("POST", "/api/services", `{"name":"Pagos","criticality":"critica"}`, admin)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["name"] != "Pagos" || got["criticality"] != "critica" {
		t.Errorf("body = %v", got)
	}
	if v, ok := got["oncall_user_id"]; !ok || v != nil {
		t.Errorf("oncall_user_id = %v (present %v), want explicit null", v, ok)
	}
}

func TestUC112_ValidacionAlCrear(t *testing.T) {
	e := newEnv(t)
	admin, _, _ := e.cookies(t)
	e.createService(t, admin, "Pagos")
	for body, want := range map[string]int{
		`{"name":"","criticality":"critica"}`:  http.StatusBadRequest,
		`{"name":"X","criticality":"CRITICA"}`: http.StatusBadRequest,
		`{"name":"X"}`:                         http.StatusBadRequest,
		`{`:                                    http.StatusBadRequest,
		`{"name":"pagos","criticality":"estandar"}`: http.StatusConflict,
	} {
		w := e.do("POST", "/api/services", body, admin)
		if w.Code != want || !strings.Contains(w.Body.String(), `"error"`) {
			t.Errorf("body %q status = %d (%s), want %d with JSON error", body, w.Code, w.Body, want)
		}
	}
	if got := e.listServices(t, admin); len(got) != 1 {
		t.Errorf("services = %d, want 1", len(got))
	}
}

func TestUC113_EditarServicio(t *testing.T) {
	e := newEnv(t)
	admin, _, _ := e.cookies(t)
	id := e.createService(t, admin, "Pagos")
	e.createService(t, admin, "Auth")
	path := "/api/services/" + id

	w := e.do("PUT", path, `{"name":"Cobros","criticality":"estandar"}`, admin)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"Cobros"`) {
		t.Fatalf("update status = %d, body %s", w.Code, w.Body)
	}
	if w := e.do("PUT", path, `{"name":"auth","criticality":"estandar"}`, admin); w.Code != http.StatusConflict {
		t.Errorf("duplicate status = %d, want 409", w.Code)
	}
	if w := e.do("PUT", path, `{"name":"Cobros","criticality":"alta"}`, admin); w.Code != http.StatusBadRequest {
		t.Errorf("invalid criticality status = %d, want 400", w.Code)
	}
	if w := e.do("PUT", "/api/services/nope", `{"name":"Z","criticality":"estandar"}`, admin); w.Code != http.StatusNotFound {
		t.Errorf("unknown id status = %d, want 404", w.Code)
	}
	for _, s := range e.listServices(t, admin) {
		if s["id"] == id && (s["name"] != "Cobros" || s["criticality"] != "estandar") {
			t.Errorf("service changed by rejected update: %v", s)
		}
	}
}

func TestUC114_SoloAdminGestiona(t *testing.T) {
	e := newEnv(t)
	admin, eng, oncall := e.cookies(t)
	id := e.createService(t, admin, "Pagos")
	for name, c := range map[string]*http.Cookie{"ingeniero": eng, "oncall": oncall, "sin sesion": nil} {
		want := http.StatusForbidden
		if c == nil {
			want = http.StatusUnauthorized
		}
		for _, tc := range []struct{ method, path, body string }{
			{"POST", "/api/services", `{"name":"Otro","criticality":"estandar"}`},
			{"PUT", "/api/services/" + id, `{"name":"Otro","criticality":"estandar"}`},
			{"DELETE", "/api/services/" + id, ""},
		} {
			if w := e.do(tc.method, tc.path, tc.body, c); w.Code != want {
				t.Errorf("%s %s %s status = %d, want %d", name, tc.method, tc.path, w.Code, want)
			}
		}
	}
	got := e.listServices(t, admin)
	if len(got) != 1 || got[0]["name"] != "Pagos" || got[0]["criticality"] != "importante" {
		t.Errorf("services changed by denied actions: %v", got)
	}
}

func TestUC115_BajaSinDependencias(t *testing.T) {
	e := newEnv(t)
	admin, _, _ := e.cookies(t)
	id := e.createService(t, admin, "Pagos")
	if w := e.do("DELETE", "/api/services/"+id, "", admin); w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if got := e.listServices(t, admin); len(got) != 0 {
		t.Errorf("services = %v, want none", got)
	}
}

func TestUC116_BajaRechazadaConDependencias(t *testing.T) {
	e := newEnv(t)
	admin, _, _ := e.cookies(t)
	id := e.createService(t, admin, "Pagos")
	e.services.withDeps[id] = true
	w := e.do("DELETE", "/api/services/"+id, "", admin)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"error"`) {
		t.Fatalf("status = %d (%s), want 409", w.Code, w.Body)
	}
	if got := e.listServices(t, admin); len(got) != 1 {
		t.Errorf("service removed despite dependencies")
	}
}

func TestUC117_ListaParaTodoAutenticado(t *testing.T) {
	e := newEnv(t)
	admin, eng, oncall := e.cookies(t)
	e.createService(t, admin, "Pagos")
	for name, c := range map[string]*http.Cookie{"admin": admin, "ingeniero": eng, "oncall": oncall} {
		got := e.listServices(t, c)
		if len(got) != 1 || got[0]["name"] != "Pagos" || got[0]["criticality"] != "importante" {
			t.Errorf("%s list = %v", name, got)
		}
	}
	if w := e.do("GET", "/api/services", "", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("no session status = %d, want 401", w.Code)
	}
}

func TestUC117_ListaVaciaEsArray(t *testing.T) {
	e := newEnv(t)
	admin, _, _ := e.cookies(t)
	if body := strings.TrimSpace(e.do("GET", "/api/services", "", admin).Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

func TestUC021_SugerenciaPorAPI(t *testing.T) {
	e := newEnv(t)
	admin, eng, _ := e.cookies(t)
	id := e.createService(t, admin, "Pagos") // importante
	w := e.do("GET", "/api/incidents/suggested-severity?service_id="+id+"&impact=caida_total", "", eng)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"suggested_severity":"SEV2"}` {
		t.Errorf("status = %d, body %s", w.Code, w.Body)
	}
	for _, q := range []string{"service_id=" + id, "service_id=" + id + "&impact=enorme", "impact=menor", "service_id=nope&impact=menor"} {
		w := e.do("GET", "/api/incidents/suggested-severity?"+q, "", eng)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"error"`) {
			t.Errorf("query %q status = %d (%s), want 400 with JSON error", q, w.Code, w.Body)
		}
	}
}

func TestUC022_DeclararDevuelve201ConTimeline(t *testing.T) {
	e := newEnv(t)
	admin, eng, _ := e.cookies(t)
	id := e.createService(t, admin, "Pagos") // importante + caida_total => SEV2
	body := `{"title":" Caida ","description":"d","service_id":"` + id + `","impact":"caida_total","severity":"SEV1","declared_by":"someone-else"}`
	w := e.do("POST", "/api/incidents", body, eng)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	var got struct {
		Incident map[string]any   `json:"incident"`
		Timeline []map[string]any `json:"timeline"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	inc := got.Incident
	if inc["id"] == "" || inc["title"] != "Caida" || inc["state"] != "declarado" || inc["suggested_severity"] != "SEV2" ||
		inc["severity"] != "SEV1" || inc["declared_by"] != "id-ana@x.com" || inc["service_id"] != id ||
		inc["impact"] != "caida_total" || inc["declared_at"] != "2026-10-01T10:00:00Z" {
		t.Errorf("incident = %v", inc)
	}
	if v, ok := inc["assigned_to"]; !ok || v != nil {
		t.Errorf("assigned_to = %v (present %v), want explicit null", v, ok)
	}
	if len(got.Timeline) != 2 || got.Timeline[0]["type"] != "declaracion" || got.Timeline[1]["type"] != "cambio_severidad" {
		t.Fatalf("timeline = %v", got.Timeline)
	}
	for _, ev := range got.Timeline {
		if ev["author_id"] != "id-ana@x.com" || ev["incident_id"] != inc["id"] || ev["occurred_at"] != "2026-10-01T10:00:00Z" {
			t.Errorf("event = %v", ev)
		}
	}
	if d, _ := got.Timeline[0]["data"].(map[string]any); d == nil || len(d) != 0 {
		t.Errorf("declaracion data = %v, want {}", got.Timeline[0]["data"])
	}
	if d, _ := got.Timeline[1]["data"].(map[string]any); d["from"] != "SEV2" || d["to"] != "SEV1" {
		t.Errorf("cambio_severidad data = %v", got.Timeline[1]["data"])
	}
	if e.incidents.incidents[0].DeclaredBy != "id-ana@x.com" {
		t.Errorf("stored declared_by = %q, want the session user", e.incidents.incidents[0].DeclaredBy)
	}
}

func TestUC025_ValidacionAlDeclarar(t *testing.T) {
	e := newEnv(t)
	admin, eng, _ := e.cookies(t)
	id := e.createService(t, admin, "Pagos")
	for name, body := range map[string]string{
		"json roto":      `{`,
		"sin titulo":     `{"title":"  ","service_id":"` + id + `","impact":"menor"}`,
		"sin service_id": `{"title":"x","impact":"menor"}`,
		"sin impact":     `{"title":"x","service_id":"` + id + `"}`,
		"servicio nope":  `{"title":"x","service_id":"nope","impact":"menor"}`,
		"severidad mala": `{"title":"x","service_id":"` + id + `","impact":"menor","severity":"SEV4"}`,
	} {
		w := e.do("POST", "/api/incidents", body, eng)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"error"`) {
			t.Errorf("%s: status = %d (%s), want 400 with JSON error", name, w.Code, w.Body)
		}
	}
	if len(e.incidents.incidents) != 0 || len(e.incidents.events) != 0 {
		t.Errorf("created %d incidents, %d events on invalid input", len(e.incidents.incidents), len(e.incidents.events))
	}
}
