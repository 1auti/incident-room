// Package user holds the user model and its business rules (BR-12, BR-19).
package user

import (
	"context"
	"errors"
	"time"
)

// Role is the permission level of a user.
type Role string

const (
	RoleIngeniero Role = "ingeniero"
	RoleOncall    Role = "oncall"
	RoleAdmin     Role = "admin"
)

// Domain errors translated to HTTP by handlers.
var (
	ErrEmailTaken = errors.New("email already registered")
	ErrNotFound   = errors.New("user not found")
	ErrForbidden  = errors.New("forbidden")
	// ErrAdminEmailTaken: the configured admin email belongs to an existing
	// non-admin user. Existing users are never promoted (BR-19).
	ErrAdminEmailTaken = errors.New("admin email already belongs to a non-admin user")
)

// User is an account. The password hash is never serialized.
type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Repository is the persistence the Service needs.
// Create must return ErrEmailTaken on a duplicate email and ErrNotFound
// is returned by the Find*/UpdateRole methods when no user matches.
type Repository interface {
	Create(ctx context.Context, u User) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	UpdateRole(ctx context.Context, id string, role Role) error
	ExistsAdmin(ctx context.Context) (bool, error)
}
