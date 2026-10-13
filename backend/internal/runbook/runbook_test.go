package runbook_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"incident-room-backend/internal/runbook"
	"incident-room-backend/internal/user"
)

var (
	fixedNow = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	admin    = user.User{ID: "u-admin", Role: user.RoleAdmin}
	engineer = user.User{ID: "u-eng", Role: user.RoleIngeniero}
	oncall   = user.User{ID: "u-oncall", Role: user.RoleOncall}
	visitor  = user.User{ID: "x", Role: "visitante"}
)

// stored is a runbook as the fake keeps it: the row and its embedding live and die together.
type stored struct {
	rb        runbook.Runbook
	embedding []float32
}

type fakeRepo struct {
	rows     map[string]stored
	services map[string]bool
	nextID   int
}

func newFake() *fakeRepo {
	return &fakeRepo{rows: map[string]stored{}, services: map[string]bool{"payments": true, "search": true}}
}

func (f *fakeRepo) ListByService(_ context.Context, serviceID string) ([]runbook.Runbook, error) {
	var out []runbook.Runbook
	for _, s := range f.rows {
		if s.rb.ServiceID == serviceID {
			out = append(out, s.rb)
		}
	}
	slices.SortFunc(out, func(a, b runbook.Runbook) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (f *fakeRepo) Create(_ context.Context, rb runbook.Runbook, embedding []float32) (runbook.Runbook, error) {
	if !f.services[rb.ServiceID] {
		return runbook.Runbook{}, runbook.ErrServiceNotFound
	}
	f.nextID++
	rb.ID = "rb-" + string(rune('0'+f.nextID))
	f.rows[rb.ID] = stored{rb: rb, embedding: embedding}
	return rb, nil
}

func (f *fakeRepo) Update(_ context.Context, rb runbook.Runbook, embedding []float32) (runbook.Runbook, error) {
	old, ok := f.rows[rb.ID]
	if !ok {
		return runbook.Runbook{}, runbook.ErrNotFound
	}
	rb.ServiceID, rb.CreatedBy = old.rb.ServiceID, old.rb.CreatedBy
	f.rows[rb.ID] = stored{rb: rb, embedding: embedding}
	return rb, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := f.rows[id]; !ok {
		return runbook.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

// fakeEmbedder returns a deterministic vector derived from the text and records every call.
type fakeEmbedder struct {
	calls []string
	err   error
	dims  int
}

func (e *fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.calls = append(e.calls, text)
	if e.err != nil {
		return nil, e.err
	}
	dims := e.dims
	if dims == 0 {
		dims = runbook.Dimensions
	}
	v := make([]float32, dims)
	v[0] = float32(len(text))
	return v, nil
}

func newSvc(repo *fakeRepo, emb *fakeEmbedder) *runbook.Service {
	return runbook.NewService(repo, emb, func() time.Time { return fixedNow })
}

// seed stores a runbook created by the admin and resets the embedder history.
func seed(t *testing.T, repo *fakeRepo, emb *fakeEmbedder, serviceID, title string) runbook.Runbook {
	t.Helper()
	rb, err := newSvc(repo, emb).Create(context.Background(), admin, serviceID, title, "Paso 1: reiniciar el pool")
	if err != nil {
		t.Fatal(err)
	}
	emb.calls = nil
	return rb
}

func TestBR12_SoloAdminGestionaRunbooks(t *testing.T) {
	for _, actor := range []user.User{admin, engineer, oncall} {
		t.Run(string(actor.Role), func(t *testing.T) {
			repo, emb := newFake(), &fakeEmbedder{}
			existing := seed(t, repo, emb, "payments", "Pool agotado")
			svc := newSvc(repo, emb)
			ctx := context.Background()

			_, createErr := svc.Create(ctx, actor, "payments", "Nuevo", "Contenido")
			_, updateErr := svc.Update(ctx, actor, existing.ID, "Pool agotado", "Paso 1: escalar réplicas")
			deleteErr := svc.Delete(ctx, actor, existing.ID)

			if actor.Role == user.RoleAdmin {
				if createErr != nil || updateErr != nil || deleteErr != nil {
					t.Fatalf("admin errs = %v, %v, %v", createErr, updateErr, deleteErr)
				}
				return
			}
			for name, err := range map[string]error{"create": createErr, "update": updateErr, "delete": deleteErr} {
				if !errors.Is(err, runbook.ErrForbidden) {
					t.Errorf("%s err = %v, want ErrForbidden", name, err)
				}
			}
			if len(repo.rows) != 1 || repo.rows[existing.ID].rb != existing || len(emb.calls) != 0 {
				t.Errorf("denied management had effects: rows %+v, embed calls %v", repo.rows, emb.calls)
			}
		})
	}
}

func TestBR12_TodoAutenticadoLeeRunbooks(t *testing.T) {
	repo, emb := newFake(), &fakeEmbedder{}
	pay1 := seed(t, repo, emb, "payments", "Pool agotado")
	pay2 := seed(t, repo, emb, "payments", "Latencia alta")
	seed(t, repo, emb, "search", "Índice corrupto")

	for _, actor := range []user.User{admin, engineer, oncall} {
		got, err := newSvc(repo, emb).List(context.Background(), actor, "payments")
		if err != nil {
			t.Fatalf("%s: %v", actor.Role, err)
		}
		// Only the runbooks of the requested service (UC-07.4).
		if len(got) != 2 || got[0].ID != pay1.ID || got[1].ID != pay2.ID {
			t.Errorf("%s: list = %+v, want only the payments runbooks", actor.Role, got)
		}
	}
}

func TestBR10_AccesoDenegadoSinEfectos(t *testing.T) {
	repo, emb := newFake(), &fakeEmbedder{}
	existing := seed(t, repo, emb, "payments", "Pool agotado")
	svc := newSvc(repo, emb)
	ctx := context.Background()

	_, listErr := svc.List(ctx, visitor, "payments")
	_, createErr := svc.Create(ctx, visitor, "payments", "Nuevo", "Contenido")
	_, updateErr := svc.Update(ctx, visitor, existing.ID, "x", "y")
	deleteErr := svc.Delete(ctx, visitor, existing.ID)
	for name, err := range map[string]error{"list": listErr, "create": createErr, "update": updateErr, "delete": deleteErr} {
		if !errors.Is(err, runbook.ErrForbidden) {
			t.Errorf("%s err = %v, want ErrForbidden", name, err)
		}
	}
	if len(repo.rows) != 1 || repo.rows[existing.ID].rb != existing || len(emb.calls) != 0 {
		t.Errorf("denied access had effects: rows %+v, embed calls %v", repo.rows, emb.calls)
	}
}

func TestRunbook_EditarRecalculaEmbedding(t *testing.T) {
	repo, emb := newFake(), &fakeEmbedder{}
	existing := seed(t, repo, emb, "payments", "Pool agotado")
	before := repo.rows[existing.ID].embedding
	later := fixedNow.Add(time.Hour)
	svc := runbook.NewService(repo, emb, func() time.Time { return later })

	content := "Paso 1: escalar réplicas a 6\nPaso 2: reiniciar el pool de conexiones"
	got, err := svc.Update(context.Background(), admin, existing.ID, "Pool agotado (v2)", content)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != content || got.Title != "Pool agotado (v2)" || !got.UpdatedAt.Equal(later) || got.ServiceID != "payments" {
		t.Errorf("updated = %+v", got)
	}
	// The embedding is derived from the content only, and recalculated on every edit.
	if !slices.Equal(emb.calls, []string{content}) {
		t.Errorf("embed calls = %q, want the new content", emb.calls)
	}
	if after := repo.rows[existing.ID].embedding; slices.Equal(after, before) {
		t.Errorf("embedding not recalculated: %v", after[:1])
	}
}

func TestBR12_BajaEliminaRunbookYEmbedding(t *testing.T) {
	repo, emb := newFake(), &fakeEmbedder{}
	gone := seed(t, repo, emb, "payments", "Pool agotado")
	kept := seed(t, repo, emb, "payments", "Latencia alta")
	svc := newSvc(repo, emb)

	if err := svc.Delete(context.Background(), admin, gone.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.rows[gone.ID]; ok {
		t.Errorf("runbook and its embedding still stored")
	}
	list, _ := svc.List(context.Background(), engineer, "payments")
	if len(list) != 1 || list[0].ID != kept.ID {
		t.Errorf("list = %+v, want only %s", list, kept.ID)
	}
	if err := svc.Delete(context.Background(), admin, gone.ID); !errors.Is(err, runbook.ErrNotFound) {
		t.Errorf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestUC07_ValidacionYFallosDelEmbedding(t *testing.T) {
	ctx := context.Background()
	for name, in := range map[string][2]string{
		"sin título":           {"", "Contenido"},
		"título solo espacios": {"  \t", "Contenido"},
		"sin contenido":        {"Título", ""},
		"contenido en blanco":  {"Título", " \n "},
	} {
		repo, emb := newFake(), &fakeEmbedder{}
		if _, err := newSvc(repo, emb).Create(ctx, admin, "payments", in[0], in[1]); !errors.Is(err, runbook.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
		if len(repo.rows) != 0 || len(emb.calls) != 0 {
			t.Errorf("%s: invalid input had effects", name)
		}
	}

	// Without an embedding nothing is stored: the copilot must never miss a runbook.
	for name, emb := range map[string]*fakeEmbedder{
		"proveedor caído":    {err: errors.New("connection refused")},
		"dimensión distinta": {dims: 1024},
	} {
		repo := newFake()
		if _, err := newSvc(repo, emb).Create(ctx, admin, "payments", "Título", "Contenido"); !errors.Is(err, runbook.ErrEmbedding) {
			t.Errorf("%s: err = %v, want ErrEmbedding", name, err)
		}
		if len(repo.rows) != 0 {
			t.Errorf("%s: runbook stored without a valid embedding", name)
		}
	}

	repo, emb := newFake(), &fakeEmbedder{}
	if _, err := newSvc(repo, emb).Create(ctx, admin, "nope", "Título", "Contenido"); !errors.Is(err, runbook.ErrServiceNotFound) {
		t.Errorf("unknown service err = %v, want ErrServiceNotFound", err)
	}
	if _, err := newSvc(repo, emb).Update(ctx, admin, "nope", "Título", "Contenido"); !errors.Is(err, runbook.ErrNotFound) {
		t.Errorf("unknown runbook err = %v, want ErrNotFound", err)
	}

	// The title is trimmed; the content keeps its indentation. Stamped with the injected clock (BR-05).
	got, err := newSvc(repo, emb).Create(ctx, admin, "payments", "  Pool agotado ", "  Paso 1\n    detalle")
	if err != nil || got.Title != "Pool agotado" || got.Content != "  Paso 1\n    detalle" || got.CreatedBy != admin.ID || !got.UpdatedAt.Equal(fixedNow) {
		t.Errorf("created = %+v, %v", got, err)
	}
}
