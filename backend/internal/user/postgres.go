package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgUniqueViolation     = "23505"
	pgInvalidTextRepresen = "22P02"
	userColumns           = "id, name, email, role, password_hash, created_at"
)

// PostgresRepository implements Repository on Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, u User) (User, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO users (name, email, role, password_hash) VALUES ($1, $2, $3, $4) RETURNING `+userColumns,
		u.Name, u.Email, string(u.Role), u.PasswordHash)
	created, err := scanUser(row)
	if pgErrCode(err) == pgUniqueViolation {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	return r.find(ctx, "email = $1", email)
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (User, error) {
	return r.find(ctx, "id = $1", id)
}

func (r *PostgresRepository) UpdateRole(ctx context.Context, id string, role Role) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET role = $1 WHERE id = $2`, string(role), id)
	if pgErrCode(err) == pgInvalidTextRepresen {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateRoleGuarded changes the role in one transaction (BR-19). The user row is
// locked FOR UPDATE first, so it serializes with SetOncall, which locks the same
// row FOR SHARE: an assignment committed before the lock is seen by the
// on-call check, and one that starts after waits and then sees the new role.
func (r *PostgresRepository) UpdateRoleGuarded(ctx context.Context, id string, role Role) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var locked string
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, id).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepresen {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	if role != RoleOncall {
		var assigned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM services WHERE oncall_user_id = $1)`, id).Scan(&assigned); err != nil {
			return fmt.Errorf("check on-call: %w", err)
		}
		if assigned {
			return ErrOncallAssigned
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET role = $1 WHERE id = $2`, string(role), id); err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ExistsAdmin(ctx context.Context) (bool, error) {
	var ok bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE role = 'admin')`).Scan(&ok); err != nil {
		return false, fmt.Errorf("check admin: %w", err)
	}
	return ok, nil
}

func (r *PostgresRepository) ListByRole(ctx context.Context, role Role) ([]User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE role = $1 ORDER BY lower(name), id`, string(role))
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return out, nil
}

// IsOncallOfAnyService reports whether the user is the on-call of some service;
// a malformed id is not on-call of anything.
func (r *PostgresRepository) IsOncallOfAnyService(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM services WHERE oncall_user_id = $1)`, id).Scan(&ok)
	if pgErrCode(err) == pgInvalidTextRepresen {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check on-call: %w", err)
	}
	return ok, nil
}

func (r *PostgresRepository) find(ctx context.Context, where string, arg any) (User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE `+where, arg))
	if errors.Is(err, pgx.ErrNoRows) || pgErrCode(err) == pgInvalidTextRepresen {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user: %w", err)
	}
	return u, nil
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	var role string
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &role, &u.PasswordHash, &u.CreatedAt); err != nil {
		return User{}, err
	}
	u.Role = Role(role)
	return u, nil
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
