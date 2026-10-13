package postmortem

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"incident-room-backend/internal/incident"
)

const (
	pgUniqueViolation = "23505"
	pgInvalidTextRepr = "22P02"
	postmortemColumns = `id, incident_id, content, status, generated_by_llm, approved_by, approved_at, updated_at`
)

// PostgresRepository implements Repository on Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Find returns the postmortem of an incident; a malformed id has none.
func (r *PostgresRepository) Find(ctx context.Context, incidentID string) (Postmortem, error) {
	pm, err := scanPostmortem(r.pool.QueryRow(ctx, `SELECT `+postmortemColumns+` FROM postmortems WHERE incident_id = $1`, incidentID))
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepr {
		return Postmortem{}, ErrNotFound
	}
	if err != nil {
		return Postmortem{}, fmt.Errorf("find postmortem: %w", err)
	}
	return pm, nil
}

// CreateDraft inserts the draft and its event; the unique incident_id turns a
// concurrent creation into ErrConflict (at most one per incident, BR-13).
func (r *PostgresRepository) CreateDraft(ctx context.Context, pm Postmortem, ev incident.TimelineEvent) (Postmortem, error) {
	return r.withEvent(ctx, pm.IncidentID, ev, func(tx pgx.Tx) (Postmortem, error) {
		if err := requireResolved(ctx, tx, pm.IncidentID); err != nil {
			return Postmortem{}, err
		}
		created, err := scanPostmortem(tx.QueryRow(ctx, `
			INSERT INTO postmortems (incident_id, content, status, generated_by_llm, updated_at)
			VALUES ($1, $2, $3, $4, $5) RETURNING `+postmortemColumns,
			pm.IncidentID, pm.Content, string(pm.Status), pm.GeneratedByLLM, pm.UpdatedAt))
		if pgErrCode(err) == pgUniqueViolation {
			return Postmortem{}, ErrConflict
		}
		return created, err
	})
}

// ReplaceDraft replaces the content of the draft and stores its event.
func (r *PostgresRepository) ReplaceDraft(ctx context.Context, incidentID, content string, at time.Time, ev incident.TimelineEvent) (Postmortem, error) {
	return r.withEvent(ctx, incidentID, ev, func(tx pgx.Tx) (Postmortem, error) {
		if err := requireResolved(ctx, tx, incidentID); err != nil {
			return Postmortem{}, err
		}
		return updateDraft(ctx, tx, incidentID, `content = $2, updated_at = $3`, content, at)
	})
}

// UpdateContent replaces the text of the draft; editing has no event (BR-09).
func (r *PostgresRepository) UpdateContent(ctx context.Context, incidentID, content string, at time.Time) (Postmortem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Postmortem{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	pm, err := updateDraft(ctx, tx, incidentID, `content = $2, updated_at = $3`, content, at)
	if err != nil {
		return Postmortem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Postmortem{}, fmt.Errorf("commit: %w", err)
	}
	return pm, nil
}

// Approve makes the draft final, recording who and when, and stores its event.
func (r *PostgresRepository) Approve(ctx context.Context, incidentID, approvedBy string, at time.Time, ev incident.TimelineEvent) (Postmortem, error) {
	return r.withEvent(ctx, incidentID, ev, func(tx pgx.Tx) (Postmortem, error) {
		return updateDraft(ctx, tx, incidentID, `status = 'aprobado', approved_by = $2, approved_at = $3, updated_at = $3`, approvedBy, at)
	})
}

// withEvent runs change and inserts ev in the same transaction.
func (r *PostgresRepository) withEvent(ctx context.Context, incidentID string, ev incident.TimelineEvent, change func(pgx.Tx) (Postmortem, error)) (Postmortem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Postmortem{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	pm, err := change(tx)
	if err != nil {
		return Postmortem{}, err
	}
	if _, err := incident.InsertEvent(ctx, tx, incidentID, ev); err != nil {
		return Postmortem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Postmortem{}, fmt.Errorf("commit: %w", err)
	}
	return pm, nil
}

// requireResolved re-checks, inside the write's transaction, that the incident
// is still resuelto: the LLM answers outside any transaction and the incident
// may have been closed meanwhile (BR-13). FOR SHARE makes a concurrent
// transition wait until this transaction ends.
func requireResolved(ctx context.Context, tx pgx.Tx, incidentID string) error {
	var state string
	err := tx.QueryRow(ctx, `SELECT state FROM incidents WHERE id = $1 FOR SHARE`, incidentID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepr {
		return incident.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock incident: %w", err)
	}
	if incident.State(state) != incident.StateResolved {
		return ErrIncidentNotResolved
	}
	return nil
}

// updateDraft applies set to the incident's postmortem only while it is a
// borrador; when nothing matches it tells ErrNotFound from ErrApproved.
func updateDraft(ctx context.Context, tx pgx.Tx, incidentID, set string, a, b any) (Postmortem, error) {
	pm, err := scanPostmortem(tx.QueryRow(ctx, `UPDATE postmortems SET `+set+`
		WHERE incident_id = $1 AND status = 'borrador' RETURNING `+postmortemColumns, incidentID, a, b))
	if pgErrCode(err) == pgInvalidTextRepr {
		return Postmortem{}, ErrNotFound
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM postmortems WHERE incident_id = $1)`, incidentID).Scan(&exists); err != nil {
			return Postmortem{}, fmt.Errorf("check postmortem: %w", err)
		}
		if !exists {
			return Postmortem{}, ErrNotFound
		}
		return Postmortem{}, ErrApproved
	}
	if err != nil {
		return Postmortem{}, fmt.Errorf("update postmortem: %w", err)
	}
	return pm, nil
}

func scanPostmortem(row pgx.Row) (Postmortem, error) {
	var pm Postmortem
	var status string
	if err := row.Scan(&pm.ID, &pm.IncidentID, &pm.Content, &status, &pm.GeneratedByLLM, &pm.ApprovedBy, &pm.ApprovedAt, &pm.UpdatedAt); err != nil {
		return Postmortem{}, err
	}
	pm.Status, pm.UpdatedAt = Status(status), pm.UpdatedAt.UTC()
	if pm.ApprovedAt != nil {
		t := pm.ApprovedAt.UTC()
		pm.ApprovedAt = &t
	}
	return pm, nil
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
