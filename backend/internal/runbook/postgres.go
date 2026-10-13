package runbook

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgForeignKeyViolation = "23503"
	pgInvalidTextRepr     = "22P02"
	serviceFKName         = "runbooks_service_id_fkey"
	runbookColumns        = `id, service_id, title, content, created_by, updated_at`
)

// PostgresRepository implements Repository on Postgres with pgvector.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ListByService returns a service's runbooks by title; a malformed id has none.
func (r *PostgresRepository) ListByService(ctx context.Context, serviceID string) ([]Runbook, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+runbookColumns+` FROM runbooks WHERE service_id = $1 ORDER BY lower(title), id`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("query runbooks: %w", err)
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Runbook, error) { return scanRunbook(row) })
	if pgErrCode(err) == pgInvalidTextRepr {
		return []Runbook{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan runbooks: %w", err)
	}
	return list, nil
}

// Search ranks runbooks by cosine distance (<=>, pgvector); similarity is
// 1 - distance.
func (r *PostgresRepository) Search(ctx context.Context, embedding []float32, k int, minSimilarity float64) ([]Match, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.service_id, r.title, r.content, r.created_by, r.updated_at, s.name,
			1 - (r.embedding OPERATOR(public.<=>) $1::public.vector) AS similarity
		FROM runbooks r JOIN services s ON s.id = r.service_id
		WHERE 1 - (r.embedding OPERATOR(public.<=>) $1::public.vector) >= $2
		ORDER BY r.embedding OPERATOR(public.<=>) $1::public.vector, r.id
		LIMIT $3`, vectorLiteral(embedding), minSimilarity, k)
	if err != nil {
		return nil, fmt.Errorf("search runbooks: %w", err)
	}
	matches, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Match, error) {
		var m Match
		err := row.Scan(&m.ID, &m.ServiceID, &m.Title, &m.Content, &m.CreatedBy, &m.UpdatedAt, &m.ServiceName, &m.Similarity)
		m.UpdatedAt = m.UpdatedAt.UTC()
		return m, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan matches: %w", err)
	}
	return matches, nil
}

// Create stores the runbook and its embedding in one row.
func (r *PostgresRepository) Create(ctx context.Context, rb Runbook, embedding []float32) (Runbook, error) {
	created, err := scanRunbook(r.pool.QueryRow(ctx, `
		INSERT INTO runbooks (service_id, title, content, embedding, created_by, updated_at)
		VALUES ($1, $2, $3, $4::public.vector, $5, $6) RETURNING `+runbookColumns,
		rb.ServiceID, rb.Title, rb.Content, vectorLiteral(embedding), rb.CreatedBy, rb.UpdatedAt))
	// A malformed service id (22P02) is the same client mistake as an unknown one.
	if isServiceFKViolation(err) || pgErrCode(err) == pgInvalidTextRepr {
		return Runbook{}, ErrServiceNotFound
	}
	if err != nil {
		return Runbook{}, fmt.Errorf("insert runbook: %w", err)
	}
	return created, nil
}

// Update replaces title, content, embedding and updated_at in one statement.
func (r *PostgresRepository) Update(ctx context.Context, rb Runbook, embedding []float32) (Runbook, error) {
	updated, err := scanRunbook(r.pool.QueryRow(ctx, `
		UPDATE runbooks SET title = $1, content = $2, embedding = $3::public.vector, updated_at = $4
		WHERE id = $5 RETURNING `+runbookColumns,
		rb.Title, rb.Content, vectorLiteral(embedding), rb.UpdatedAt, rb.ID))
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepr {
		return Runbook{}, ErrNotFound
	}
	if err != nil {
		return Runbook{}, fmt.Errorf("update runbook: %w", err)
	}
	return updated, nil
}

// Delete removes the row, and with it the embedding (BR-12).
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM runbooks WHERE id = $1`, id)
	if pgErrCode(err) == pgInvalidTextRepr {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete runbook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// vectorLiteral formats a vector in pgvector's text form, e.g. "[0.1,-2,3e-05]",
// so no pgvector client library is needed.
func vectorLiteral(v []float32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = strconv.FormatFloat(float64(x), 'g', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func scanRunbook(row pgx.Row) (Runbook, error) {
	var rb Runbook
	if err := row.Scan(&rb.ID, &rb.ServiceID, &rb.Title, &rb.Content, &rb.CreatedBy, &rb.UpdatedAt); err != nil {
		return Runbook{}, err
	}
	rb.UpdatedAt = rb.UpdatedAt.UTC()
	return rb, nil
}

func isServiceFKViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == serviceFKName
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
