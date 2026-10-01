package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/httpapi"
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

type env struct {
	h        http.Handler
	users    *fakeUsers
	sessions *fakeSessions
	usersSvc *user.Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	users := &fakeUsers{}
	sessions := &fakeSessions{m: map[string]auth.Session{}}
	us := user.NewService(users)
	as := auth.NewService(users, sessions, time.Hour, time.Now)
	return &env{h: httpapi.New(us, as), users: users, sessions: sessions, usersSvc: us}
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
	} {
		w := e.do(tc.method, tc.path, tc.body, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want 401", tc.method, tc.path, w.Code)
		}
	}
	if e.users.list[0].Role != user.RoleIngeniero {
		t.Errorf("role changed without session")
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
