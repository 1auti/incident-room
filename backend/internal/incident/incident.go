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
	EventStateChange    EventType = "cambio_estado"
	EventSeverityChange EventType = "cambio_severidad"
	EventAssignment     EventType = "asignacion"
	EventEscalation     EventType = "escalado"
)

// Domain errors translated to HTTP by handlers.
var (
	ErrInvalid         = errors.New("invalid incident data")
	ErrServiceNotFound = errors.New("service not found")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidFilter   = errors.New("invalid incident filter")
	ErrNotFound        = errors.New("incident not found")
	// ErrInvalidTransition: the state change is not in the transition table or
	// the incident is no longer in the expected state (BR-08).
	ErrInvalidTransition = errors.New("invalid state transition")
)

// SLA holds the acknowledgement deadline per severity, measured from
// declared_at (BR-03).
type SLA struct {
	SEV1, SEV2, SEV3 time.Duration
}

// DefaultSLA returns the default deadlines: SEV1 5 min, SEV2 15 min, SEV3 60 min.
func DefaultSLA() SLA {
	return SLA{SEV1: 5 * time.Minute, SEV2: 15 * time.Minute, SEV3: 60 * time.Minute}
}

// Deadline returns the acknowledgement deadline for a severity; zero if unknown.
func (s SLA) Deadline(sev Severity) time.Duration {
	switch sev {
	case SeveritySEV1:
		return s.SEV1
	case SeveritySEV2:
		return s.SEV2
	case SeveritySEV3:
		return s.SEV3
	}
	return 0
}

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
	// AcknowledgedAt is set when the incident moves to reconocido; nil otherwise.
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
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
// Get returns ErrNotFound when no incident matches (including a malformed id).
// ListPendingEscalation returns the incidents in the given state that were never
// escalated. Escalate sets escalated_at to ev.OccurredAt and stores ev in one
// transaction, only if the incident is still declarado, not escalated and has the
// severity of inc; it reports whether it escalated. UpdateState moves an incident
// from one state to another, setting acknowledged_at when ackAt is not nil, and
// stores ev in the same transaction; it returns ErrInvalidTransition when the
// incident is no longer in state from, and ErrNotFound when it does not exist.
// There is deliberately no way to update or delete events (BR-09).
type Repository interface {
	List(ctx context.Context, q ListQuery) ([]Incident, error)
	Get(ctx context.Context, id string) (Incident, error)
	ListPendingEscalation(ctx context.Context, state State) ([]Incident, error)
	Escalate(ctx context.Context, inc Incident, ev TimelineEvent) (bool, error)
	UpdateState(ctx context.Context, id string, from, to State, ackAt *time.Time, ev TimelineEvent) (Incident, error)
	FindService(ctx context.Context, id string) (ServiceInfo, error)
	Create(ctx context.Context, inc Incident, events []TimelineEvent) (Incident, []TimelineEvent, error)
}
