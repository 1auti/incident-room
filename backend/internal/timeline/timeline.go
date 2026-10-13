// Package timeline holds the rules of an incident timeline: reading it,
// adding notes and changing the severity after declaring (BR-02, BR-05, BR-09,
// BR-10, BR-11).
package timeline

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

// ErrInvalid: an empty note, or a severity that is not SEV1-SEV3 or equals the
// current one.
var ErrInvalid = errors.New("invalid timeline input")

// mutableSeverityStates are the states where the severity may still change;
// from resuelto on it is fixed (BR-02).
var mutableSeverityStates = []incident.State{incident.StateDeclared, incident.StateAcknowledged, incident.StateMitigating}

// Repository is the persistence the Service needs. Events can only be listed
// and inserted: there is deliberately no way to update or delete one (BR-09).
// ChangeSeverity updates the incident, never an event: it sets the severity
// from -> to only if the incident is still in one of the mutable states, and
// stores ev in the same transaction (ErrSeverityLocked otherwise).
type Repository interface {
	Get(ctx context.Context, id string) (incident.Incident, error)
	FindService(ctx context.Context, id string) (incident.ServiceInfo, error)
	ListEvents(ctx context.Context, incidentID string) ([]incident.EventView, error)
	AppendEvent(ctx context.Context, incidentID string, ev incident.TimelineEvent) (incident.TimelineEvent, error)
	ChangeSeverity(ctx context.Context, id string, from, to incident.Severity, mutable []incident.State, ev incident.TimelineEvent) (incident.Incident, error)
}

// Service applies the timeline rules. It never reads the system clock: now is
// injected (BR-05).
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService builds a Service.
func NewService(repo Repository, now func() time.Time) *Service {
	return &Service{repo: repo, now: now}
}

// List returns the timeline of an incident in chronological order; every
// authenticated role sees every timeline (BR-09, BR-10).
func (s *Service) List(ctx context.Context, actor user.User, incidentID string) ([]incident.EventView, error) {
	if err := incident.Authorize(actor); err != nil {
		return nil, err
	}
	// Get first so an unknown incident is a 404 and not an empty timeline.
	if _, err := s.repo.Get(ctx, incidentID); err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	events, err := s.repo.ListEvents(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return events, nil
}

// AddNote appends a nota event by actor at the injected instant. Every
// authenticated role may add notes, in any state (BR-10); a mistake is
// corrected with another note, never by editing (BR-09).
func (s *Service) AddNote(ctx context.Context, actor user.User, incidentID, body string) (incident.TimelineEvent, error) {
	if err := incident.Authorize(actor); err != nil {
		return incident.TimelineEvent{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return incident.TimelineEvent{}, ErrInvalid
	}
	author := actor.ID
	ev, err := s.repo.AppendEvent(ctx, incidentID, incident.TimelineEvent{
		Type:       incident.EventNote,
		AuthorID:   &author,
		Body:       body,
		Data:       map[string]string{},
		OccurredAt: s.now().UTC(),
	})
	if err != nil {
		return incident.TimelineEvent{}, fmt.Errorf("append note: %w", err)
	}
	return ev, nil
}

// ChangeSeverity sets a new severity after declaring: only the service's
// on-call or the admin (BR-02, BR-11), and only while declarado, reconocido or
// mitigando. It adds a cambio_severidad event with from, to and author.
func (s *Service) ChangeSeverity(ctx context.Context, actor user.User, incidentID string, to incident.Severity) (incident.Incident, error) {
	if err := incident.Authorize(actor); err != nil {
		return incident.Incident{}, err
	}
	if !incident.ValidSeverity(to) {
		return incident.Incident{}, ErrInvalid
	}
	inc, err := s.repo.Get(ctx, incidentID)
	if err != nil {
		return incident.Incident{}, fmt.Errorf("get incident: %w", err)
	}
	info, err := s.repo.FindService(ctx, inc.ServiceID)
	if err != nil {
		return incident.Incident{}, fmt.Errorf("find service: %w", err)
	}
	if !incident.CanOperate(actor, info) {
		return incident.Incident{}, incident.ErrForbidden
	}
	// The repository re-checks the state atomically; this early check keeps
	// the error precise without a write.
	if !slices.Contains(mutableSeverityStates, inc.State) {
		return incident.Incident{}, incident.ErrSeverityLocked
	}
	if inc.Severity == to {
		return incident.Incident{}, ErrInvalid
	}

	author := actor.ID
	ev := incident.TimelineEvent{
		Type:       incident.EventSeverityChange,
		AuthorID:   &author,
		Data:       map[string]string{"from": string(inc.Severity), "to": string(to)},
		OccurredAt: s.now().UTC(),
	}
	updated, err := s.repo.ChangeSeverity(ctx, incidentID, inc.Severity, to, mutableSeverityStates, ev)
	if err != nil {
		return incident.Incident{}, fmt.Errorf("change severity: %w", err)
	}
	return updated, nil
}
