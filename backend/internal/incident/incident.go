// Package incident holds the incident model and its business rules (BR-01,
// BR-02, BR-05, BR-09, BR-10, BR-11).
package incident

import (
	"context"
	"errors"
	"time"
)

// State is the lifecycle state of an incident.
type State string

// StateDeclared is the initial state (UC-02). Later states arrive with UC-04/UC-06.
const StateDeclared State = "declarado"

// EventType is the kind of a timeline event.
type EventType string

const (
	EventDeclaration    EventType = "declaracion"
	EventSeverityChange EventType = "cambio_severidad"
)

// Domain errors translated to HTTP by handlers.
var (
	ErrInvalid         = errors.New("invalid incident data")
	ErrServiceNotFound = errors.New("service not found")
	ErrForbidden       = errors.New("forbidden")
)

// Incident is a declared incident.
type Incident struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	ServiceID         string    `json:"service_id"`
	Impact            Impact    `json:"impact"`
	SuggestedSeverity Severity  `json:"suggested_severity"`
	Severity          Severity  `json:"severity"`
	State             State     `json:"state"`
	DeclaredBy        string    `json:"declared_by"`
	AssignedTo        *string   `json:"assigned_to"`
	DeclaredAt        time.Time `json:"declared_at"`
}

// TimelineEvent is an append-only entry of an incident timeline (BR-09).
// Data is never nil so it serializes as {}.
type TimelineEvent struct {
	ID         string            `json:"id"`
	IncidentID string            `json:"incident_id"`
	Type       EventType         `json:"type"`
	AuthorID   *string           `json:"author_id"`
	Body       string            `json:"body"`
	Data       map[string]string `json:"data"`
	OccurredAt time.Time         `json:"occurred_at"`
}

// ServiceInfo is what declaring an incident needs to know about a service.
type ServiceInfo struct {
	Criticality  Criticality
	OncallUserID *string
}

// Repository is the persistence the Service needs. It is defined here because
// package service already imports incident.
// FindService returns ErrServiceNotFound when no service matches (including a
// malformed id). Create stores the incident and its events atomically, filling
// the generated ids, and returns ErrServiceNotFound if the service disappeared.
// There is deliberately no way to update or delete events (BR-09).
type Repository interface {
	FindService(ctx context.Context, id string) (ServiceInfo, error)
	Create(ctx context.Context, inc Incident, events []TimelineEvent) (Incident, []TimelineEvent, error)
}
