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

// Lifecycle states. StateDeclared is the initial one (UC-02); the transitions
// between them arrive with UC-04/UC-06.
const (
	StateDeclared     State = "declarado"
	StateAcknowledged State = "reconocido"
	StateMitigating   State = "mitigando"
	StateResolved     State = "resuelto"
	StateClosed       State = "cerrado"
)

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
	ErrInvalidFilter   = errors.New("invalid incident filter")
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
	// EscalatedAt is set when the incident was escalated by SLA (BR-04); nil otherwise.
	EscalatedAt *time.Time `json:"escalated_at"`
}

// ListQuery is what the repository filters on. States is always set by the
// service (BR-15); empty Severity or ServiceID means no filter.
type ListQuery struct {
	States    []State
	Severity  Severity
	ServiceID string
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
// List returns the incidents matching every filter of q, newest declared_at
// first (ties by id); never nil-significant. A malformed ServiceID yields
// ErrInvalidFilter; a well-formed unknown one yields an empty list.
// There is deliberately no way to update or delete events (BR-09).
type Repository interface {
	List(ctx context.Context, q ListQuery) ([]Incident, error)
	FindService(ctx context.Context, id string) (ServiceInfo, error)
	Create(ctx context.Context, inc Incident, events []TimelineEvent) (Incident, []TimelineEvent, error)
}
