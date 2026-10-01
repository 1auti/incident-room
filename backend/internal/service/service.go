// Package service holds the service catalog and its business rules
// (BR-01, BR-10, BR-12, BR-20).
package service

import (
	"context"
	"errors"

	"incident-room-backend/internal/incident"
)

// Domain errors translated to HTTP by handlers.
var (
	ErrInvalid   = errors.New("invalid service data")
	ErrNameTaken = errors.New("service name already in use")
	// ErrInUse: the service has incidents or runbooks and cannot be removed (BR-20).
	ErrInUse     = errors.New("service has incidents or runbooks")
	ErrNotFound  = errors.New("service not found")
	ErrForbidden = errors.New("forbidden")
)

// Service is a service incidents are declared on. The on-call is assigned in
// UC-04; it is null until then.
type Service struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Criticality  incident.Criticality `json:"criticality"`
	OncallUserID *string              `json:"oncall_user_id"`
}

// Repository is the persistence the Manager needs.
// Create and Update return ErrNameTaken when the name is already used
// (case-insensitively). Update and Delete return ErrNotFound when no service
// matches; Delete returns ErrInUse if the store still references the service.
type Repository interface {
	List(ctx context.Context) ([]Service, error)
	Create(ctx context.Context, name string, criticality incident.Criticality) (Service, error)
	Update(ctx context.Context, id, name string, criticality incident.Criticality) (Service, error)
	Delete(ctx context.Context, id string) error
	HasIncidents(ctx context.Context, id string) (bool, error)
	HasRunbooks(ctx context.Context, id string) (bool, error)
}
