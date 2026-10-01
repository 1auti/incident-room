package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"incident-room-backend/internal/incident"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgInvalidTextRepresen = "22P02"
	serviceColumns        = "id, name, criticality, oncall_user_id"
)

// PostgresRepository implements Repository on Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]Service, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serviceColumns+` FROM services ORDER BY lower(name), name`)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) Create(ctx context.Context, name string, c incident.Criticality) (Service, error) {
	s, err := scanService(r.pool.QueryRow(ctx,
		`INSERT INTO services (name, criticality) VALUES ($1, $2) RETURNING `+serviceColumns, name, string(c)))
	if pgErrCode(err) == pgUniqueViolation {
		return Service{}, ErrNameTaken
	}
	if err != nil {
		return Service{}, fmt.Errorf("insert service: %w", err)
	}
	return s, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id, name string, c incident.Criticality) (Service, error) {
	s, err := scanService(r.pool.QueryRow(ctx,
		`UPDATE services SET name = $1, criticality = $2 WHERE id = $3 RETURNING `+serviceColumns, name, string(c), id))
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepresen {
		return Service{}, ErrNotFound
	}
	if pgErrCode(err) == pgUniqueViolation {
		return Service{}, ErrNameTaken
	}
	if err != nil {
		return Service{}, fmt.Errorf("update service: %w", err)
	}
	return s, nil
}

// Delete maps a foreign key violation to ErrInUse: it is the database-level
// backstop of BR-20 for any table that references services.
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	switch {
	case pgErrCode(err) == pgInvalidTextRepresen:
		return ErrNotFound
	case pgErrCode(err) == pgForeignKeyViolation:
		return ErrInUse
	case err != nil:
		return fmt.Errorf("delete service: %w", err)
	case tag.RowsAffected() == 0:
		return ErrNotFound
	}
	return nil
}

// HasIncidents reports whether any incident references the service. No table
// references services yet; UC-02 replaces this body with
// SELECT EXISTS (...) over incidents.
func (r *PostgresRepository) HasIncidents(context.Context, string) (bool, error) {
	return false, nil
}

// HasRunbooks reports whether any runbook references the service. No table
// references services yet; UC-07 replaces this body with
// SELECT EXISTS (...) over runbooks.
func (r *PostgresRepository) HasRunbooks(context.Context, string) (bool, error) {
	return false, nil
}

func scanService(row pgx.Row) (Service, error) {
	var s Service
	var crit string
	if err := row.Scan(&s.ID, &s.Name, &crit, &s.OncallUserID); err != nil {
		return Service{}, err
	}
	s.Criticality = incident.Criticality(crit)
	return s, nil
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
