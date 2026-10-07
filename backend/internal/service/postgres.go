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

// HasIncidents reports whether any incident references the service (BR-20).
// A malformed id is ErrNotFound, like Update and Delete.
func (r *PostgresRepository) HasIncidents(ctx context.Context, id string) (bool, error) {
	var has bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM incidents WHERE service_id = $1)`, id).Scan(&has)
	if pgErrCode(err) == pgInvalidTextRepresen {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("check incidents: %w", err)
	}
	return has, nil
}

// HasRunbooks reports whether any runbook references the service. No table
// references services yet; UC-07 replaces this body with
// SELECT EXISTS (...) over runbooks.
func (r *PostgresRepository) HasRunbooks(context.Context, string) (bool, error) {
	return false, nil
}

// SetOncall changes the on-call and reassigns the active incidents in one
// transaction. The service row is locked, so two concurrent changes serialize.
// Event rows are only ever inserted (BR-09).
func (r *PostgresRepository) SetOncall(ctx context.Context, id, userID string, active []incident.State, ev incident.TimelineEvent) (Service, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Service{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanService(tx.QueryRow(ctx, `SELECT `+serviceColumns+` FROM services WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepresen {
		return Service{}, ErrNotFound
	}
	if err != nil {
		return Service{}, fmt.Errorf("lock service: %w", err)
	}
	if current.OncallUserID != nil && *current.OncallUserID == userID {
		return current, nil
	}

	// BR-11 vs BR-19: lock the target user FOR SHARE and check the role in this
	// transaction. A concurrent demotion locks the row FOR UPDATE, so it either
	// committed first (the role is seen here) or waits until this commit and
	// then sees the service as assigned.
	var role string
	err = tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 FOR SHARE`, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepresen {
		return Service{}, ErrInvalidOncall
	}
	if err != nil {
		return Service{}, fmt.Errorf("lock on-call user: %w", err)
	}
	if role != "oncall" {
		return Service{}, ErrInvalidOncall
	}

	updated, err := scanService(tx.QueryRow(ctx,
		`UPDATE services SET oncall_user_id = $1 WHERE id = $2 RETURNING `+serviceColumns, userID, id))
	if pgErrCode(err) == pgForeignKeyViolation || pgErrCode(err) == pgInvalidTextRepresen {
		return Service{}, ErrInvalidOncall
	}
	if err != nil {
		return Service{}, fmt.Errorf("update on-call: %w", err)
	}

	states := make([]string, len(active))
	for i, st := range active {
		states[i] = string(st)
	}
	rows, err := tx.Query(ctx, `
		UPDATE incidents i SET assigned_to = $1
		FROM (SELECT id, assigned_to FROM incidents WHERE service_id = $2 AND state = ANY($3) FOR UPDATE) old
		WHERE i.id = old.id
		RETURNING i.id, old.assigned_to`, userID, id, states)
	if err != nil {
		return Service{}, fmt.Errorf("reassign incidents: %w", err)
	}
	type moved struct {
		id   string
		from *string
	}
	var movedList []moved
	for rows.Next() {
		var m moved
		if err := rows.Scan(&m.id, &m.from); err != nil {
			rows.Close()
			return Service{}, fmt.Errorf("scan reassigned incident: %w", err)
		}
		movedList = append(movedList, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Service{}, fmt.Errorf("reassign incidents: %w", err)
	}

	for _, m := range movedList {
		data := map[string]string{"from": ""}
		for k, v := range ev.Data {
			data[k] = v
		}
		if m.from != nil {
			data["from"] = *m.from
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO timeline_events (incident_id, type, author_id, body, data, occurred_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			m.id, string(ev.Type), ev.AuthorID, ev.Body, data, ev.OccurredAt); err != nil {
			return Service{}, fmt.Errorf("insert timeline event: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Service{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
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
