package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Service implements user business rules.
type Service struct {
	repo Repository
}

// NewService builds a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// NormalizeEmail returns the canonical form used for storage and lookup.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Register creates a public account. The role is always ingeniero (BR-19).
func (s *Service) Register(ctx context.Context, name, email, password string) (User, error) {
	return s.create(ctx, name, email, password, RoleIngeniero)
}

// ChangeRole lets only an admin set a user's role to oncall or admin (BR-12, BR-19).
func (s *Service) ChangeRole(ctx context.Context, actor User, targetID string, role Role) error {
	if actor.Role != RoleAdmin {
		return ErrForbidden
	}
	if role != RoleOncall && role != RoleAdmin {
		return ErrForbidden
	}
	target, err := s.repo.FindByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	// BR-19: an on-call of a service keeps the role until the service has another on-call.
	if target.Role == RoleOncall && role != RoleOncall {
		assigned, err := s.repo.IsOncallOfAnyService(ctx, targetID)
		if err != nil {
			return fmt.Errorf("check on-call: %w", err)
		}
		if assigned {
			return ErrOncallAssigned
		}
	}
	if err := s.repo.UpdateRole(ctx, targetID, role); err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	return nil
}

// ListByRole lists the users with a role; only an admin may (BR-12). The result is never nil.
func (s *Service) ListByRole(ctx context.Context, actor User, role Role) ([]User, error) {
	if actor.Role != RoleAdmin {
		return nil, ErrForbidden
	}
	switch role {
	case RoleIngeniero, RoleOncall, RoleAdmin:
	default:
		return nil, fmt.Errorf("role must be ingeniero, oncall or admin: %w", ErrInvalidRole)
	}
	list, err := s.repo.ListByRole(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	if list == nil {
		list = []User{}
	}
	return list, nil
}

// EnsureAdmin creates the first admin only when none exists (BR-19). It is idempotent.
// It fails closed with ErrAdminEmailTaken when the email already belongs to a
// user: promoting existing accounts would let anyone pre-register that email.
func (s *Service) EnsureAdmin(ctx context.Context, email, password string) error {
	exists, err := s.repo.ExistsAdmin(ctx)
	if err != nil {
		return fmt.Errorf("check admin: %w", err)
	}
	if exists {
		return nil
	}
	if _, err := s.create(ctx, "Admin", email, password, RoleAdmin); err != nil {
		if errors.Is(err, ErrEmailTaken) {
			return fmt.Errorf("create admin: %w", ErrAdminEmailTaken)
		}
		return fmt.Errorf("create admin: %w", err)
	}
	return nil
}

func (s *Service) create(ctx context.Context, name, email, password string, role Role) (User, error) {
	email = NormalizeEmail(email)
	if _, err := s.repo.FindByEmail(ctx, email); err == nil {
		return User{}, ErrEmailTaken
	} else if !errors.Is(err, ErrNotFound) {
		return User{}, fmt.Errorf("find user by email: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	created, err := s.repo.Create(ctx, User{Name: name, Email: email, Role: role, PasswordHash: string(hash)})
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return created, nil
}
