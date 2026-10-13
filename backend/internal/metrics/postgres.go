package metrics

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"incident-room-backend/internal/incident"
)

// PostgresRepository implements Repository on Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ResolvedIn returns the incidents with resolved_at in [From, To).
func (r *PostgresRepository) ResolvedIn(ctx context.Context, p Period) ([]ResolvedRow, error) {
	rows, err := r.pool.Query(ctx, `SELECT declared_at, resolved_at FROM incidents
		WHERE resolved_at >= $1 AND resolved_at < $2`, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("query resolved: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ResolvedRow])
	if err != nil {
		return nil, fmt.Errorf("scan resolved: %w", err)
	}
	return out, nil
}

// DeclaredIn returns the incidents with declared_at in [From, To).
func (r *PostgresRepository) DeclaredIn(ctx context.Context, p Period) ([]DeclaredRow, error) {
	rows, err := r.pool.Query(ctx, `SELECT service_id, severity, declared_at, acknowledged_at FROM incidents
		WHERE declared_at >= $1 AND declared_at < $2`, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("query declared: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (DeclaredRow, error) {
		var d DeclaredRow
		var severity string
		err := row.Scan(&d.ServiceID, &severity, &d.DeclaredAt, &d.AcknowledgedAt)
		d.Severity = incident.Severity(severity)
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan declared: %w", err)
	}
	return out, nil
}

// Services returns every service by name, case-insensitive like its uniqueness.
func (r *PostgresRepository) Services(ctx context.Context) ([]ServiceRef, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM services ORDER BY lower(name), id`)
	if err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ServiceRef])
	if err != nil {
		return nil, fmt.Errorf("scan services: %w", err)
	}
	return out, nil
}
