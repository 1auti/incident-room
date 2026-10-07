// Package service holds the service catalog and its business rules
// (BR-01, BR-10, BR-12, BR-20).
package service

import (
	"context"
	"errors"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

// Domain errors translated to HTTP by handlers.
var (
	ErrInvalid   = errors.New("invalid service data")
	ErrNameTaken = errors.New("service name already in use")
	// ErrInUse: the service has incidents or runbooks and cannot be removed (BR-20).
	ErrInUse     = errors.New("service has incidents or runbooks")
	ErrNotFound  = errors.New("service not found")
	ErrForbidden = errors.New("forbidden")
	// ErrInvalidOncall: the user to assign does not exist or lacks the oncall role (BR-11).
	ErrInvalidOncall = errors.New("on-call must be an existing user with role oncall")
)

// Users is the user lookup the Manager needs to validate an on-call (BR-11).
type Users interface {
	FindByID(ctx context.Context, id string) (user.User, error)
}

// Service is a service incidents are declared on. The on-call is assigned by
// the admin (BR-11); it is null until then.
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
// SetOncall, in one transaction, sets the service on-call and moves every
// incident of the service whose state is in active to the new on-call, storing
// one copy of ev per moved incident with data "from" (the previous assignee, ""
// if none) added. If userID is already the on-call it changes nothing and stores
// no event. It returns ErrNotFound when no service matches and ErrInvalidOncall
// when the user does not exist.
type Repository interface {
	List(ctx context.Context) ([]Service, error)
	Create(ctx context.Context, name string, criticality incident.Criticality) (Service, error)
	Update(ctx context.Context, id, name string, criticality incident.Criticality) (Service, error)
	Delete(ctx context.Context, id string) error
	HasIncidents(ctx context.Context, id string) (bool, error)
	HasRunbooks(ctx context.Context, id string) (bool, error)
	SetOncall(ctx context.Context, id, userID string, active []incident.State, ev incident.TimelineEvent) (Service, error)
}
