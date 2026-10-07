package incident

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgForeignKeyViolation = "23503"
	pgInvalidTextRepresen = "22P02"
	serviceFKName         = "incidents_service_id_fkey"
	incidentColumns       = `id, title, description, service_id, impact, suggested_severity, severity, state,
		declared_by, assigned_to, declared_at, acknowledged_at, escalated_at`
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
	sql := `SELECT ` + incidentColumns + ` FROM incidents WHERE state = ANY($1)`
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
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
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

	created, err := scanIncident(tx.QueryRow(ctx, `
		INSERT INTO incidents (title, description, service_id, impact, suggested_severity, severity, state, declared_by, assigned_to, declared_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+incidentColumns,
		inc.Title, inc.Description, inc.ServiceID, string(inc.Impact), string(inc.SuggestedSeverity), string(inc.Severity),
		string(inc.State), inc.DeclaredBy, inc.AssignedTo, inc.DeclaredAt,
	))
	// A malformed service id (22P02) is the same client mistake as an unknown one.
	if isServiceFKViolation(err) || pgErrCode(err) == pgInvalidTextRepresen {
		return Incident{}, nil, ErrServiceNotFound
	}
	if err != nil {
		return Incident{}, nil, fmt.Errorf("insert incident: %w", err)
	}

	stored := make([]TimelineEvent, 0, len(events))
	for _, e := range events {
		out, err := insertEvent(ctx, tx, created.ID, e)
		if err != nil {
			return Incident{}, nil, err
		}
		stored = append(stored, out)
	}
	if err := tx.Commit(ctx); err != nil {
		return Incident{}, nil, fmt.Errorf("commit: %w", err)
	}
	return created, stored, nil
}

// Get returns one incident; a malformed id is ErrNotFound.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Incident, error) {
	inc, err := scanIncident(r.pool.QueryRow(ctx, `SELECT `+incidentColumns+` FROM incidents WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepresen {
		return Incident{}, ErrNotFound
	}
	if err != nil {
		return Incident{}, fmt.Errorf("get incident: %w", err)
	}
	return inc, nil
}

// ListPendingEscalation returns the never-escalated incidents in state, oldest first.
func (r *PostgresRepository) ListPendingEscalation(ctx context.Context, state State) ([]Incident, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+incidentColumns+` FROM incidents
		WHERE state = $1 AND escalated_at IS NULL ORDER BY declared_at, id`, string(state))
	if err != nil {
		return nil, fmt.Errorf("list pending escalation: %w", err)
	}
	defer rows.Close()
	out := []Incident{}
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		out = append(out, inc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pending escalation: %w", err)
	}
	return out, nil
}

// Escalate sets escalated_at and inserts ev in one transaction, only when the
// incident is still declarado, never escalated and has the severity the caller
// evaluated. The event is inserted only if the update touched a row, so each
// incident escalates at most once (BR-04, BR-09).
func (r *PostgresRepository) Escalate(ctx context.Context, inc Incident, ev TimelineEvent) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `UPDATE incidents SET escalated_at = $1
		WHERE id = $2 AND state = $3 AND escalated_at IS NULL AND severity = $4`,
		ev.OccurredAt, inc.ID, string(StateDeclared), string(inc.Severity))
	if err != nil {
		return false, fmt.Errorf("escalate incident: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := insertEvent(ctx, tx, inc.ID, ev); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

// UpdateState moves the incident from one state to another in one transaction,
// storing ev. The update is conditional on the current state; when no row
// matches, the incident either does not exist (ErrNotFound) or changed state
// meanwhile (ErrInvalidTransition).
func (r *PostgresRepository) UpdateState(ctx context.Context, id string, from, to State, ackAt *time.Time, ev TimelineEvent) (Incident, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Incident{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inc, err := scanIncident(tx.QueryRow(ctx, `UPDATE incidents SET state = $1, acknowledged_at = COALESCE($2, acknowledged_at)
		WHERE id = $3 AND state = $4 RETURNING `+incidentColumns, string(to), ackAt, id, string(from)))
	if pgErrCode(err) == pgInvalidTextRepresen {
		return Incident{}, ErrNotFound
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM incidents WHERE id = $1)`, id).Scan(&exists); err != nil {
			return Incident{}, fmt.Errorf("check incident: %w", err)
		}
		if !exists {
			return Incident{}, ErrNotFound
		}
		return Incident{}, ErrInvalidTransition
	}
	if err != nil {
		return Incident{}, fmt.Errorf("update state: %w", err)
	}
	if _, err := insertEvent(ctx, tx, id, ev); err != nil {
		return Incident{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Incident{}, fmt.Errorf("commit: %w", err)
	}
	return inc, nil
}

// insertEvent appends one timeline event; events are only ever inserted (BR-09).
func insertEvent(ctx context.Context, tx pgx.Tx, incidentID string, e TimelineEvent) (TimelineEvent, error) {
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
		incidentID, string(e.Type), e.AuthorID, e.Body, data, e.OccurredAt,
	).Scan(&out.ID, &out.IncidentID, &typ, &out.AuthorID, &out.Body, &out.Data, &out.OccurredAt)
	if err != nil {
		return TimelineEvent{}, fmt.Errorf("insert timeline event: %w", err)
	}
	out.Type = EventType(typ)
	if out.Data == nil {
		out.Data = map[string]string{}
	}
	out.OccurredAt = out.OccurredAt.UTC()
	return out, nil
}

// scanIncident reads a row selected with incidentColumns, instants in UTC.
func scanIncident(row pgx.Row) (Incident, error) {
	var inc Incident
	var impact, suggested, severity, state string
	if err := row.Scan(&inc.ID, &inc.Title, &inc.Description, &inc.ServiceID, &impact, &suggested, &severity, &state,
		&inc.DeclaredBy, &inc.AssignedTo, &inc.DeclaredAt, &inc.AcknowledgedAt, &inc.EscalatedAt); err != nil {
		return Incident{}, err
	}
	inc.Impact, inc.SuggestedSeverity, inc.Severity, inc.State = Impact(impact), Severity(suggested), Severity(severity), State(state)
	inc.DeclaredAt = inc.DeclaredAt.UTC()
	inc.AcknowledgedAt = utcPtr(inc.AcknowledgedAt)
	inc.EscalatedAt = utcPtr(inc.EscalatedAt)
	return inc, nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
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
