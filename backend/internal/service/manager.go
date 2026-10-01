package service

import (
	"context"
	"fmt"
	"strings"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

// Manager applies the service catalog rules.
type Manager struct {
	repo Repository
}

// NewManager builds a Manager.
func NewManager(repo Repository) *Manager {
	return &Manager{repo: repo}
}

// List returns every service. Any authenticated user may read it (BR-10,
// enforced by the HTTP layer).
func (m *Manager) List(ctx context.Context) ([]Service, error) {
	return m.repo.List(ctx)
}

// Create adds a service without on-call (BR-12, BR-01).
func (m *Manager) Create(ctx context.Context, actor user.User, name string, criticality incident.Criticality) (Service, error) {
	if actor.Role != user.RoleAdmin {
		return Service{}, ErrForbidden
	}
	name, err := validate(name, criticality)
	if err != nil {
		return Service{}, err
	}
	return m.repo.Create(ctx, name, criticality)
}

// Update changes name and criticality; it never touches the on-call (UC-04).
func (m *Manager) Update(ctx context.Context, actor user.User, id, name string, criticality incident.Criticality) (Service, error) {
	if actor.Role != user.RoleAdmin {
		return Service{}, ErrForbidden
	}
	name, err := validate(name, criticality)
	if err != nil {
		return Service{}, err
	}
	return m.repo.Update(ctx, id, name, criticality)
}

// Delete removes a service only if it has no incidents and no runbooks
// (BR-20). A failure checking dependencies aborts the removal.
func (m *Manager) Delete(ctx context.Context, actor user.User, id string) error {
	if actor.Role != user.RoleAdmin {
		return ErrForbidden
	}
	hasIncidents, err := m.repo.HasIncidents(ctx, id)
	if err != nil {
		return fmt.Errorf("check incidents: %w", err)
	}
	if hasIncidents {
		return ErrInUse
	}
	hasRunbooks, err := m.repo.HasRunbooks(ctx, id)
	if err != nil {
		return fmt.Errorf("check runbooks: %w", err)
	}
	if hasRunbooks {
		return ErrInUse
	}
	return m.repo.Delete(ctx, id)
}

func validate(name string, c incident.Criticality) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name is required: %w", ErrInvalid)
	}
	switch c {
	case incident.CriticalityCritical, incident.CriticalityImportant, incident.CriticalityStandard:
	default:
		return "", fmt.Errorf("criticality must be critica, importante or estandar: %w", ErrInvalid)
	}
	return name, nil
}
