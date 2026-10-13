package runbook_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"incident-room-backend/internal/dbtest"
	"incident-room-backend/internal/runbook"
	"incident-room-backend/internal/service"
)

func vec(first float32) []float32 {
	v := make([]float32, runbook.Dimensions)
	v[0], v[1] = first, -0.5
	return v
}

func TestPostgres_RunbookConEmbeddingYBajaFisica(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := runbook.NewPostgresRepository(pool)
	services := service.NewPostgresRepository(pool)

	var adminID, payments, search string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ada', 'ada@x.com', 'h', 'admin') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]*string{"payments": &payments, "search": &search} {
		if err := pool.QueryRow(c, `INSERT INTO services (name, criticality) VALUES ($1, 'critica') RETURNING id`, name).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

	created, err := repo.Create(c, runbook.Runbook{ServiceID: payments, Title: "Pool agotado", Content: "Paso 1", CreatedBy: adminID, UpdatedAt: at}, vec(0.25))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.ServiceID != payments || !created.UpdatedAt.Equal(at) || created.UpdatedAt.Location() != time.UTC {
		t.Errorf("created = %+v", created)
	}
	embeddingOf := func(id string) (float32, int) {
		t.Helper()
		var first float32
		var dims int
		if err := pool.QueryRow(c, `SELECT (embedding::real[])[1], vector_dims(embedding) FROM runbooks WHERE id = $1`, id).Scan(&first, &dims); err != nil {
			t.Fatal(err)
		}
		return first, dims
	}
	if first, dims := embeddingOf(created.ID); first != 0.25 || dims != runbook.Dimensions {
		t.Errorf("stored embedding = %v (%d dims), want 0.25 (%d)", first, dims, runbook.Dimensions)
	}
	for _, bad := range []string{"nope", "00000000-0000-0000-0000-000000000000"} {
		if _, err := repo.Create(c, runbook.Runbook{ServiceID: bad, Title: "t", Content: "c", CreatedBy: adminID, UpdatedAt: at}, vec(1)); !errors.Is(err, runbook.ErrServiceNotFound) {
			t.Errorf("Create on %q err = %v, want ErrServiceNotFound", bad, err)
		}
	}

	// Edit replaces content and embedding, keeping service and author.
	later := at.Add(time.Hour)
	updated, err := repo.Update(c, runbook.Runbook{ID: created.ID, Title: "Pool agotado v2", Content: "Paso 2", UpdatedAt: later}, vec(0.75))
	if err != nil || updated.Content != "Paso 2" || !updated.UpdatedAt.Equal(later) || updated.ServiceID != payments || updated.CreatedBy != adminID {
		t.Errorf("updated = %+v, %v", updated, err)
	}
	if first, _ := embeddingOf(created.ID); first != 0.75 {
		t.Errorf("embedding after edit = %v, want 0.75", first)
	}

	if _, err := repo.Create(c, runbook.Runbook{ServiceID: search, Title: "Índice", Content: "c", CreatedBy: adminID, UpdatedAt: at}, vec(1)); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListByService(c, payments)
	if err != nil || len(list) != 1 || list[0].ID != created.ID {
		t.Errorf("payments list = %+v, %v", list, err)
	}
	if list, err := repo.ListByService(c, "nope"); err != nil || len(list) != 0 {
		t.Errorf("malformed service list = %+v, %v", list, err)
	}

	// A service with runbooks cannot be removed (BR-20).
	if has, err := services.HasRunbooks(c, payments); err != nil || !has {
		t.Errorf("HasRunbooks(payments) = %v, %v; want true", has, err)
	}

	// Removal is physical: the row, and so its embedding, is gone (BR-12).
	if err := repo.Delete(c, created.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(c, `SELECT count(*) FROM runbooks WHERE id = $1`, created.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows after delete = %d, %v", n, err)
	}
	if has, err := services.HasRunbooks(c, payments); err != nil || has {
		t.Errorf("HasRunbooks after delete = %v, %v; want false", has, err)
	}
	for _, bad := range []string{created.ID, "nope"} {
		if err := repo.Delete(c, bad); !errors.Is(err, runbook.ErrNotFound) {
			t.Errorf("Delete %q err = %v, want ErrNotFound", bad, err)
		}
		if _, err := repo.Update(c, runbook.Runbook{ID: bad, Title: "t", Content: "c", UpdatedAt: at}, vec(1)); !errors.Is(err, runbook.ErrNotFound) {
			t.Errorf("Update %q err = %v, want ErrNotFound", bad, err)
		}
	}
}

// unit returns a 768-dimension vector pointing mostly at axis i (cosine similarity
// between unit(i) and unit(j) is 1 for i == j and ~0 otherwise).
func unit(i int, tilt float32) []float32 {
	v := make([]float32, runbook.Dimensions)
	v[i], v[i+1] = 1, tilt
	return v
}

func TestPostgres_BusquedaPorSimilitud(t *testing.T) {
	c := context.Background()
	pool := dbtest.Pool(t)
	repo := runbook.NewPostgresRepository(pool)
	var adminID, serviceID string
	if err := pool.QueryRow(c, `INSERT INTO users (name, email, password_hash, role) VALUES ('Ada', 'ada@x.com', 'h', 'admin') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(c, `INSERT INTO services (name, criticality) VALUES ('payments', 'critica') RETURNING id`).Scan(&serviceID); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	create := func(title string, v []float32) runbook.Runbook {
		t.Helper()
		rb, err := repo.Create(c, runbook.Runbook{ServiceID: serviceID, Title: title, Content: title, CreatedBy: adminID, UpdatedAt: at}, v)
		if err != nil {
			t.Fatal(err)
		}
		return rb
	}
	exact := create("Pool agotado", unit(0, 0))
	near := create("Pool lento", unit(0, 0.5))
	create("Disco lleno", unit(10, 0))

	got, err := repo.Search(c, unit(0, 0), 5, 0.5)
	if err != nil || len(got) != 2 || got[0].ID != exact.ID || got[1].ID != near.ID {
		t.Fatalf("Search = %+v, %v; want the two pool runbooks, most similar first", got, err)
	}
	if got[0].ServiceName != "payments" || got[0].Similarity < 0.99 || got[1].Similarity >= got[0].Similarity {
		t.Errorf("matches = %+v", got)
	}
	if got, _ := repo.Search(c, unit(0, 0), 1, 0.5); len(got) != 1 || got[0].ID != exact.ID {
		t.Errorf("k=1 = %+v, want only the closest", got)
	}
	if got, _ := repo.Search(c, unit(0, 0), 5, 0.95); len(got) != 1 {
		t.Errorf("high threshold = %+v, want only the exact one", got)
	}
	// Removed: no longer retrievable (BR-12).
	if err := repo.Delete(c, exact.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Search(c, unit(0, 0), 5, 0.5); len(got) != 1 || got[0].ID != near.ID {
		t.Errorf("after delete = %+v, want only %s", got, near.ID)
	}
}
