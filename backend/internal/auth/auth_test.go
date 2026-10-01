package auth_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/user"
)

type fakeUsers struct{ users map[string]user.User }

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (user.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return user.User{}, user.ErrNotFound
}

func (f *fakeUsers) FindByID(_ context.Context, id string) (user.User, error) {
	u, ok := f.users[id]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}

type fakeSessions struct{ sessions map[string]auth.Session }

func (f *fakeSessions) Create(_ context.Context, s auth.Session) error {
	f.sessions[s.TokenHash] = s
	return nil
}

func (f *fakeSessions) FindByTokenHash(_ context.Context, hash string) (auth.Session, error) {
	s, ok := f.sessions[hash]
	if !ok {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	return s, nil
}

var now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func setup(t *testing.T) (*auth.Service, *fakeSessions) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("right-pass"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUsers{users: map[string]user.User{
		"1": {ID: "1", Name: "Ana", Email: "ana@example.com", Role: user.RoleIngeniero, PasswordHash: string(hash)},
	}}
	sessions := &fakeSessions{sessions: map[string]auth.Session{}}
	svc := auth.NewService(users, sessions, time.Hour, func() time.Time { return now })
	return svc, sessions
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name, email, password string
		wantErr               error
		wantSessions          int
	}{
		{"correct credentials", "ana@example.com", "right-pass", nil, 1},
		{"email is case-insensitive", " ANA@example.com", "right-pass", nil, 1},
		{"wrong password", "ana@example.com", "wrong", auth.ErrInvalidCredentials, 0},
		{"unknown email", "nobody@example.com", "right-pass", auth.ErrInvalidCredentials, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sessions := setup(t)

			token, expires, err := svc.Login(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if len(sessions.sessions) != tt.wantSessions {
				t.Fatalf("sessions = %d, want %d", len(sessions.sessions), tt.wantSessions)
			}
			if tt.wantErr != nil {
				if token != "" {
					t.Errorf("token must be empty on failure")
				}
				return
			}
			if token == "" || !expires.Equal(now.Add(time.Hour)) {
				t.Errorf("token=%q expires=%v", token, expires)
			}
			if _, stored := sessions.sessions[token]; stored {
				t.Errorf("raw token must not be stored")
			}
			u, err := svc.Authenticate(context.Background(), token)
			if err != nil || u.ID != "1" {
				t.Errorf("Authenticate = %v, %v", u, err)
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	svc, sessions := setup(t)
	token, _, err := svc.Login(context.Background(), "ana@example.com", "right-pass")
	if err != nil {
		t.Fatal(err)
	}
	expired := auth.NewService(&fakeUsers{users: map[string]user.User{"1": {ID: "1"}}}, sessions, time.Hour,
		func() time.Time { return now.Add(2 * time.Hour) })

	tests := []struct {
		name    string
		svc     *auth.Service
		token   string
		wantErr error
	}{
		{"valid", svc, token, nil},
		{"empty token", svc, "", auth.ErrUnauthenticated},
		{"unknown token", svc, "nope", auth.ErrUnauthenticated},
		{"expired session", expired, token, auth.ErrUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.svc.Authenticate(context.Background(), tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBR10_RutasProtegidasSinSesion(t *testing.T) {
	svc, _ := setup(t)
	token, _, err := svc.Login(context.Background(), "ana@example.com", "right-pass")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		build      func(next http.Handler) http.Handler
		cookie     string
		wantStatus int
		wantCalled bool
	}{
		{"no cookie", auth.RequireAuth(svc, discardLogger()), "", http.StatusUnauthorized, false},
		{"invalid token", auth.RequireAuth(svc, discardLogger()), "bogus", http.StatusUnauthorized, false},
		{"valid session", auth.RequireAuth(svc, discardLogger()), token, http.StatusOK, true},
		{"role allowed", chain(auth.RequireAuth(svc, discardLogger()), auth.RequireRole(user.RoleIngeniero)), token, http.StatusOK, true},
		{"role not allowed", chain(auth.RequireAuth(svc, discardLogger()), auth.RequireRole(user.RoleAdmin)), token, http.StatusForbidden, false},
		{"role without session", auth.RequireRole(user.RoleAdmin), "", http.StatusUnauthorized, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			h := tt.build(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if u, ok := auth.UserFromContext(r.Context()); !ok || u.ID != "1" {
					t.Errorf("UserFromContext = %v, %v", u, ok)
				}
			}))
			req := httptest.NewRequest(http.MethodPost, "/anything", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tt.cookie})
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Errorf("handler called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}

func chain(outer, inner func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return outer(inner(next)) }
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)) }

type failingAuthenticator struct{ err error }

func (f failingAuthenticator) Authenticate(context.Context, string) (user.User, error) {
	return user.User{}, f.err
}

func TestRequireAuth_ErrorDeInfraestructura(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantLogged bool
	}{
		{"infrastructure error", errors.New("pq: connection refused to db-secret-host"), http.StatusInternalServerError, true},
		{"unauthenticated is not an error", auth.ErrUnauthenticated, http.StatusUnauthorized, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			called := false
			h := auth.RequireAuth(failingAuthenticator{tt.err}, logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
			req := httptest.NewRequest(http.MethodGet, "/anything", nil)
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "tok"})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus || called {
				t.Errorf("status = %d called = %v, want %d false", rec.Code, called, tt.wantStatus)
			}
			if strings.Contains(rec.Body.String(), "db-secret-host") {
				t.Errorf("response leaks the error: %q", rec.Body.String())
			}
			if got := strings.Contains(logs.String(), "level=ERROR"); got != tt.wantLogged {
				t.Errorf("error logged = %v, want %v (logs: %q)", got, tt.wantLogged, logs.String())
			}
			if tt.wantLogged && !strings.Contains(logs.String(), "db-secret-host") {
				t.Errorf("log must carry the error: %q", logs.String())
			}
		})
	}
}
