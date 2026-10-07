package user_test

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"

	"incident-room-backend/internal/user"
)

type fakeRepo struct {
	users map[string]user.User
	next  int
	// oncallOf holds the ids of users that are on-call of some service (BR-19).
	oncallOf map[string]bool
}

func (f *fakeRepo) ListByRole(_ context.Context, role user.Role) ([]user.User, error) {
	out := []user.User{}
	for _, u := range f.users {
		if u.Role == role {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeRepo) IsOncallOfAnyService(_ context.Context, id string) (bool, error) {
	return f.oncallOf[id], nil
}

func newFakeRepo() *fakeRepo { return &fakeRepo{users: map[string]user.User{}} }

func (f *fakeRepo) Create(_ context.Context, u user.User) (user.User, error) {
	for _, existing := range f.users {
		if existing.Email == u.Email {
			return user.User{}, user.ErrEmailTaken
		}
	}
	f.next++
	u.ID = strconv.Itoa(f.next)
	f.users[u.ID] = u
	return u, nil
}

func (f *fakeRepo) FindByEmail(_ context.Context, email string) (user.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return user.User{}, user.ErrNotFound
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (user.User, error) {
	u, ok := f.users[id]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) UpdateRole(_ context.Context, id string, role user.Role) error {
	u, ok := f.users[id]
	if !ok {
		return user.ErrNotFound
	}
	u.Role = role
	f.users[id] = u
	return nil
}

func (f *fakeRepo) ExistsAdmin(_ context.Context) (bool, error) {
	for _, u := range f.users {
		if u.Role == user.RoleAdmin {
			return true, nil
		}
	}
	return false, nil
}

func TestBR19_RegistroCreaIngeniero(t *testing.T) {
	tests := []struct {
		name, userName, email, password string
	}{
		{"plain", "Ana", "ana@example.com", "s3cret-pass"},
		{"email is normalized", "Bob", "  Bob@Example.COM ", "another-pass"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := user.NewService(repo)

			got, err := svc.Register(context.Background(), tt.userName, tt.email, tt.password)
			if err != nil {
				t.Fatalf("Register: %v", err)
			}
			if got.Role != user.RoleIngeniero {
				t.Errorf("role = %q, want %q", got.Role, user.RoleIngeniero)
			}
			if got.PasswordHash == "" || got.PasswordHash == tt.password {
				t.Errorf("password must be stored hashed")
			}
			if got.Email != strings.ToLower(strings.TrimSpace(tt.email)) {
				t.Errorf("email = %q, want normalized", got.Email)
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "password") {
				t.Errorf("serialized user leaks password: %s", raw)
			}
		})
	}
}

func TestBR19_EmailUnico(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"same email", "ana@example.com"},
		{"same email different case", "ANA@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := user.NewService(repo)
			if _, err := svc.Register(context.Background(), "Ana", "ana@example.com", "pw-one"); err != nil {
				t.Fatalf("first Register: %v", err)
			}

			_, err := svc.Register(context.Background(), "Other", tt.email, "pw-two")
			if !errors.Is(err, user.ErrEmailTaken) {
				t.Fatalf("err = %v, want ErrEmailTaken", err)
			}
			if len(repo.users) != 1 {
				t.Errorf("users = %d, want 1", len(repo.users))
			}
		})
	}
}

func TestBR19_PrimerAdminDesdeEntorno(t *testing.T) {
	t.Run("creates admin when none exists", func(t *testing.T) {
		repo := newFakeRepo()
		svc := user.NewService(repo)
		if err := svc.EnsureAdmin(context.Background(), "root@example.com", "admin-pass"); err != nil {
			t.Fatalf("EnsureAdmin: %v", err)
		}
		u, err := repo.FindByEmail(context.Background(), "root@example.com")
		if err != nil {
			t.Fatal(err)
		}
		if u.Role != user.RoleAdmin {
			t.Errorf("role = %q, want admin", u.Role)
		}
	})
	t.Run("idempotent and no second admin", func(t *testing.T) {
		repo := newFakeRepo()
		svc := user.NewService(repo)
		for _, email := range []string{"root@example.com", "root@example.com", "other@example.com"} {
			if err := svc.EnsureAdmin(context.Background(), email, "admin-pass"); err != nil {
				t.Fatalf("EnsureAdmin(%s): %v", email, err)
			}
		}
		if len(repo.users) != 1 {
			t.Errorf("users = %d, want 1", len(repo.users))
		}
	})
}

func TestBR19_AdminEmailDeUsuarioExistenteFallaCerrado(t *testing.T) {
	repo := newFakeRepo()
	svc := user.NewService(repo)
	existing, err := svc.Register(context.Background(), "Ana", "root@example.com", "user-pass")
	if err != nil {
		t.Fatal(err)
	}

	err = svc.EnsureAdmin(context.Background(), "ROOT@example.com", "admin-pass")
	if !errors.Is(err, user.ErrAdminEmailTaken) {
		t.Fatalf("err = %v, want ErrAdminEmailTaken", err)
	}
	if errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("err must be distinguishable from ErrEmailTaken")
	}
	if ok, _ := repo.ExistsAdmin(context.Background()); ok {
		t.Errorf("no admin must be created")
	}
	got, _ := repo.FindByID(context.Background(), existing.ID)
	if got.Role != user.RoleIngeniero {
		t.Errorf("role = %q, want ingeniero (existing users are never promoted)", got.Role)
	}
	if len(repo.users) != 1 {
		t.Errorf("users = %d, want 1", len(repo.users))
	}
}

func TestBR12_SoloAdminCambiaRol(t *testing.T) {
	tests := []struct {
		name     string
		actor    user.Role
		newRole  user.Role
		wantErr  error
		wantRole user.Role
	}{
		{"admin promotes to oncall", user.RoleAdmin, user.RoleOncall, nil, user.RoleOncall},
		{"admin promotes to admin", user.RoleAdmin, user.RoleAdmin, nil, user.RoleAdmin},
		{"admin cannot set ingeniero", user.RoleAdmin, user.RoleIngeniero, user.ErrForbidden, user.RoleIngeniero},
		{"admin cannot set unknown role", user.RoleAdmin, user.Role("root"), user.ErrForbidden, user.RoleIngeniero},
		{"ingeniero forbidden", user.RoleIngeniero, user.RoleOncall, user.ErrForbidden, user.RoleIngeniero},
		{"oncall forbidden", user.RoleOncall, user.RoleAdmin, user.ErrForbidden, user.RoleIngeniero},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := user.NewService(repo)
			target, err := svc.Register(context.Background(), "Ana", "ana@example.com", "pw")
			if err != nil {
				t.Fatal(err)
			}

			err = svc.ChangeRole(context.Background(), user.User{ID: "actor", Role: tt.actor}, target.ID, tt.newRole)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			got, _ := repo.FindByID(context.Background(), target.ID)
			if got.Role != tt.wantRole {
				t.Errorf("role = %q, want %q", got.Role, tt.wantRole)
			}
		})
	}
}

func TestBR19_NoQuitarOncallAOncallDeServicio(t *testing.T) {
	for _, tc := range []struct {
		name      string
		role      user.Role
		newRole   user.Role
		isOncall  bool
		wantErr   error
		wantAfter user.Role
	}{
		{"oncall de un servicio no puede dejar de serlo", user.RoleOncall, user.RoleAdmin, true, user.ErrOncallAssigned, user.RoleOncall},
		{"oncall sin servicios pasa a admin", user.RoleOncall, user.RoleAdmin, false, nil, user.RoleAdmin},
		{"oncall de un servicio sigue oncall", user.RoleOncall, user.RoleOncall, true, nil, user.RoleOncall},
		{"ingeniero se promueve a oncall", user.RoleIngeniero, user.RoleOncall, false, nil, user.RoleOncall},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := user.NewService(repo)
			target, err := svc.Register(context.Background(), "Ana", "ana@example.com", "pw")
			if err != nil {
				t.Fatal(err)
			}
			target.Role = tc.role
			repo.users[target.ID] = target
			repo.oncallOf = map[string]bool{target.ID: tc.isOncall}

			err = svc.ChangeRole(context.Background(), user.User{ID: "actor", Role: user.RoleAdmin}, target.ID, tc.newRole)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got, _ := repo.FindByID(context.Background(), target.ID); got.Role != tc.wantAfter {
				t.Errorf("role = %q, want %q", got.Role, tc.wantAfter)
			}
		})
	}
}

func TestBR12_SoloAdminListaUsuariosPorRol(t *testing.T) {
	repo := newFakeRepo()
	svc := user.NewService(repo)
	repo.users["a"] = user.User{ID: "a", Name: "Ana", Role: user.RoleOncall}
	repo.users["b"] = user.User{ID: "b", Name: "Bruno", Role: user.RoleIngeniero}
	for _, tc := range []struct {
		name    string
		actor   user.Role
		role    user.Role
		wantErr error
		wantLen int
	}{
		{"admin lista oncall", user.RoleAdmin, user.RoleOncall, nil, 1},
		{"admin lista rol sin usuarios", user.RoleAdmin, user.RoleAdmin, nil, 0},
		{"rol invalido", user.RoleAdmin, "root", user.ErrInvalidRole, 0},
		{"ingeniero prohibido", user.RoleIngeniero, user.RoleOncall, user.ErrForbidden, 0},
		{"oncall prohibido", user.RoleOncall, user.RoleOncall, user.ErrForbidden, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.ListByRole(context.Background(), user.User{Role: tc.actor}, tc.role)
			if !errors.Is(err, tc.wantErr) || len(got) != tc.wantLen || (tc.wantErr == nil && got == nil) {
				t.Errorf("ListByRole = %v, %v; want %d users and %v", got, err, tc.wantLen, tc.wantErr)
			}
		})
	}
}
