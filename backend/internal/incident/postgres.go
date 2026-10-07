package incident

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgForeignKeyViolation = "23503"
	pgInvalidTextRepresen = "22P02"
	serviceFKName         = "incidents_service_id_fkey"
)

// PostgresRepository implements Repository on Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindService(ctx context.Context, id string) (ServiceInfo, error) {
	var info ServiceInfo
	var crit string
	err := r.pool.QueryRow(ctx, `SELECT criticality, oncall_user_id FROM services WHERE id = $1`, id).
		Scan(&crit, &info.OncallUserID)
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepresen {
		return ServiceInfo{}, ErrServiceNotFound
	}
	if err != nil {
		return ServiceInfo{}, fmt.Errorf("find service: %w", err)
	}
	info.Criticality = Criticality(crit)
	return info, nil
}

// List returns the incidents matching q, ordered by declared_at DESC, id. The
// WHERE is built conditionally because casting ” to uuid fails. A malformed
// service id (22P02) is ErrInvalidFilter; a well-formed unknown one matches nothing.
func (r *PostgresRepository) List(ctx context.Context, q ListQuery) ([]Incident, error) {
	states := make([]string, len(q.States))
	for i, st := range q.States {
		states[i] = string(st)
	}
	sql := `SELECT id, title, description, service_id, impact, suggested_severity, severity, state,
		declared_by, assigned_to, declared_at, escalated_at
		FROM incidents WHERE state = ANY($1)`
	args := []any{states}
	if q.Severity != "" {
		args = append(args, string(q.Severity))
		sql += fmt.Sprintf(" AND severity = $%d", len(args))
	}
	if q.ServiceID != "" {
		args = append(args, q.ServiceID)
		sql += fmt.Sprintf(" AND service_id = $%d::uuid", len(args))
	}
	sql += " ORDER BY declared_at DESC, id"

	rows, err := r.pool.Query(ctx, sql, args...)
	if pgErrCode(err) == pgInvalidTextRepresen {
		return nil, ErrInvalidFilter
	}
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	out := []Incident{}
	for rows.Next() {
		var inc Incident
		var impact, suggested, severity, state string
		if err := rows.Scan(&inc.ID, &inc.Title, &inc.Description, &inc.ServiceID, &impact, &suggested, &severity, &state,
			&inc.DeclaredBy, &inc.AssignedTo, &inc.DeclaredAt, &inc.EscalatedAt); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		inc.Impact, inc.SuggestedSeverity, inc.Severity, inc.State = Impact(impact), Severity(suggested), Severity(severity), State(state)
		inc.DeclaredAt = inc.DeclaredAt.UTC()
		if inc.EscalatedAt != nil {
			at := inc.EscalatedAt.UTC()
			inc.EscalatedAt = &at
		}
		out = append(out, inc)
	}
	if err := rows.Err(); err != nil {
		if pgErrCode(err) == pgInvalidTextRepresen {
			return nil, ErrInvalidFilter
		}
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	return out, nil
}

// Create inserts the incident and its events in one transaction. Event rows
// are only ever inserted (BR-09).
func (r *PostgresRepository) Create(ctx context.Context, inc Incident, events []TimelineEvent) (Incident, []TimelineEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Incident{}, nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var impact, suggested, severity, state string
	err = tx.QueryRow(ctx, `
		INSERT INTO incidents (title, description, service_id, impact, suggested_severity, severity, state, declared_by, assigned_to, declared_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, title, description, service_id, impact, suggested_severity, severity, state, declared_by, assigned_to, declared_at`,
		inc.Title, inc.Description, inc.ServiceID, string(inc.Impact), string(inc.SuggestedSeverity), string(inc.Severity),
		string(inc.State), inc.DeclaredBy, inc.AssignedTo, inc.DeclaredAt,
	).Scan(&inc.ID, &inc.Title, &inc.Description, &inc.ServiceID, &impact, &suggested, &severity, &state,
		&inc.DeclaredBy, &inc.AssignedTo, &inc.DeclaredAt)
	// A malformed service id (22P02) is the same client mistake as an unknown one.
	if isServiceFKViolation(err) || pgErrCode(err) == pgInvalidTextRepresen {
		return Incident{}, nil, ErrServiceNotFound
	}
	if err != nil {
		return Incident{}, nil, fmt.Errorf("insert incident: %w", err)
	}
	inc.Impact, inc.SuggestedSeverity, inc.Severity, inc.State = Impact(impact), Severity(suggested), Severity(severity), State(state)
	inc.DeclaredAt = inc.DeclaredAt.UTC()

	stored := make([]TimelineEvent, 0, len(events))
	for _, e := range events {
		data := e.Data
		if data == nil {
			data = map[string]string{}
		}
		var typ string
		out := TimelineEvent{}
		err := tx.QueryRow(ctx, `
			INSERT INTO timeline_events (incident_id, type, author_id, body, data, occurred_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, incident_id, type, author_id, body, data, occurred_at`,
			inc.ID, string(e.Type), e.AuthorID, e.Body, data, e.OccurredAt,
		).Scan(&out.ID, &out.IncidentID, &typ, &out.AuthorID, &out.Body, &out.Data, &out.OccurredAt)
		if err != nil {
			return Incident{}, nil, fmt.Errorf("insert timeline event: %w", err)
		}
		out.Type = EventType(typ)
		if out.Data == nil {
			out.Data = map[string]string{}
		}
		out.OccurredAt = out.OccurredAt.UTC()
		stored = append(stored, out)
	}
	if err := tx.Commit(ctx); err != nil {
		return Incident{}, nil, fmt.Errorf("commit: %w", err)
	}
	return inc, stored, nil
}

// isServiceFKViolation is true only for the service foreign key: any other
// 23503 (e.g. an unknown user) is an internal error, not a client mistake.
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
