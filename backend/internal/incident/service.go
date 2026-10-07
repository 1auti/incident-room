package incident

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"incident-room-backend/internal/user"
)

// Service applies the incident rules. It never reads the system clock: now is
// injected (BR-05).
type Service struct {
	repo Repository
	now  func() time.Time
	sla  SLA
}

// NewService builds a Service.
func NewService(repo Repository, now func() time.Time, sla SLA) *Service {
	return &Service{repo: repo, now: now, sla: sla}
}

// DeclareInput is what the declarer provides. An empty Severity means "accept
// the suggested one" (BR-02).
type DeclareInput struct {
	Title       string
	Description string
	ServiceID   string
	Impact      Impact
	Severity    Severity
}

// Suggest returns the severity suggested for a service and impact (BR-01).
func (s *Service) Suggest(ctx context.Context, actor user.User, serviceID string, impact Impact) (Severity, error) {
	if err := authorize(actor); err != nil {
		return "", err
	}
	if serviceID == "" {
		return "", fmt.Errorf("service_id is required: %w", ErrInvalid)
	}
	if !validImpact(impact) {
		return "", fmt.Errorf("impact must be caida_total, degradacion or menor: %w", ErrInvalid)
	}
	info, err := s.repo.FindService(ctx, serviceID)
	if err != nil {
		return "", fmt.Errorf("find service: %w", err)
	}
	sev, err := SuggestSeverity(info.Criticality, impact)
	if err != nil {
		return "", fmt.Errorf("suggest severity: %w", err)
	}
	return sev, nil
}

// Declare creates an incident in state declarado with its declaracion event
// (BR-09) and, when the chosen severity differs from the suggested one, a
// cambio_severidad event (BR-02). Assigns the service on-call if any (BR-11).
// The clock is read once so declared_at and every event share the instant.
func (s *Service) Declare(ctx context.Context, actor user.User, in DeclareInput) (Incident, []TimelineEvent, error) {
	if err := authorize(actor); err != nil {
		return Incident{}, nil, err
	}
	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		return Incident{}, nil, fmt.Errorf("title is required: %w", ErrInvalid)
	case in.ServiceID == "":
		return Incident{}, nil, fmt.Errorf("service_id is required: %w", ErrInvalid)
	case !validImpact(in.Impact):
		return Incident{}, nil, fmt.Errorf("impact must be caida_total, degradacion or menor: %w", ErrInvalid)
	case in.Severity != "" && !validSeverity(in.Severity):
		return Incident{}, nil, fmt.Errorf("severity must be SEV1, SEV2 or SEV3: %w", ErrInvalid)
	}

	info, err := s.repo.FindService(ctx, in.ServiceID)
	if err != nil {
		return Incident{}, nil, fmt.Errorf("find service: %w", err)
	}
	suggested, err := SuggestSeverity(info.Criticality, in.Impact)
	if err != nil {
		return Incident{}, nil, fmt.Errorf("suggest severity: %w", err)
	}
	severity := in.Severity
	if severity == "" {
		severity = suggested
	}

	at := s.now().UTC()
	author := actor.ID
	inc := Incident{
		Title:             title,
		Description:       in.Description,
		ServiceID:         in.ServiceID,
		Impact:            in.Impact,
		SuggestedSeverity: suggested,
		Severity:          severity,
		State:             StateDeclared,
		DeclaredBy:        actor.ID,
		AssignedTo:        info.OncallUserID,
		DeclaredAt:        at,
	}
	events := []TimelineEvent{{Type: EventDeclaration, AuthorID: &author, Data: map[string]string{}, OccurredAt: at}}
	if severity != suggested {
		events = append(events, TimelineEvent{
			Type:       EventSeverityChange,
			AuthorID:   &author,
			Data:       map[string]string{"from": string(suggested), "to": string(severity)},
			OccurredAt: at,
		})
	}
	return s.repo.Create(ctx, inc, events)
}

// ListFilter is what the board user asks for; an empty field means no filter.
type ListFilter struct {
	Severity  Severity
	ServiceID string
	State     State
}

// ActiveStates returns all the states but cerrado (BR-15).
func ActiveStates() []State {
	return []State{StateDeclared, StateAcknowledged, StateMitigating, StateResolved}
}

// isActive reports whether an incident in state st is active (BR-15).
func isActive(st State) bool {
	return validState(st) && st != StateClosed
}

// List returns the active incidents (BR-15) matching every filter given, for any
// authenticated role (BR-10). Filtering by cerrado yields an empty list without
// touching the repository. The result is never nil.
func (s *Service) List(ctx context.Context, actor user.User, f ListFilter) ([]Incident, error) {
	if err := authorize(actor); err != nil {
		return nil, err
	}
	if f.Severity != "" && !validSeverity(f.Severity) {
		return nil, fmt.Errorf("severity must be SEV1, SEV2 or SEV3: %w", ErrInvalidFilter)
	}
	if f.State != "" && !validState(f.State) {
		return nil, fmt.Errorf("state must be declarado, reconocido, mitigando, resuelto or cerrado: %w", ErrInvalidFilter)
	}

	states := ActiveStates()
	if f.State != "" {
		if !isActive(f.State) {
			return []Incident{}, nil
		}
		states = []State{f.State}
	}
	list, err := s.repo.List(ctx, ListQuery{States: states, Severity: f.Severity, ServiceID: f.ServiceID})
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	if list == nil {
		list = []Incident{}
	}
	return list, nil
}

// Get returns one incident for any authenticated role (BR-10).
func (s *Service) Get(ctx context.Context, actor user.User, id string) (Incident, error) {
	if err := authorize(actor); err != nil {
		return Incident{}, err
	}
	inc, err := s.repo.Get(ctx, id)
	if err != nil {
		return Incident{}, fmt.Errorf("get incident: %w", err)
	}
	return inc, nil
}

// transitions is the only table of valid state changes (BR-08). UC-06 adds
// T2-T4 here.
var transitions = map[State][]State{
	StateDeclared: {StateAcknowledged},
}

// Transition moves an incident to the state to. Every transition goes through
// here. The ingeniero never transitions; an oncall only on incidents of the
// services they are on-call of; the admin always (BR-08, BR-11). T1 sets
// acknowledged_at and adds a cambio_estado event (BR-09).
func (s *Service) Transition(ctx context.Context, actor user.User, id string, to State) (Incident, error) {
	if err := authorize(actor); err != nil {
		return Incident{}, err
	}
	if actor.Role == user.RoleIngeniero {
		return Incident{}, ErrForbidden
	}
	inc, err := s.repo.Get(ctx, id)
	if err != nil {
		return Incident{}, fmt.Errorf("get incident: %w", err)
	}
	if actor.Role == user.RoleOncall {
		info, err := s.repo.FindService(ctx, inc.ServiceID)
		if err != nil {
			return Incident{}, fmt.Errorf("find service: %w", err)
		}
		if info.OncallUserID == nil || *info.OncallUserID != actor.ID {
			return Incident{}, ErrForbidden
		}
	}
	if !slices.Contains(transitions[inc.State], to) {
		return Incident{}, fmt.Errorf("%s to %s: %w", inc.State, to, ErrInvalidTransition)
	}

	at := s.now().UTC()
	author := actor.ID
	ev := TimelineEvent{
		Type:       EventStateChange,
		AuthorID:   &author,
		Data:       map[string]string{"from": string(inc.State), "to": string(to)},
		OccurredAt: at,
	}
	var ackAt *time.Time
	if inc.State == StateDeclared && to == StateAcknowledged {
		ackAt = &at
	}
	updated, err := s.repo.UpdateState(ctx, id, inc.State, to, ackAt, ev)
	if err != nil {
		return Incident{}, fmt.Errorf("update state: %w", err)
	}
	return updated, nil
}

// EscalateOverdue escalates every declarado incident whose deadline passed
// (BR-03, BR-04): now > declared_at + SLA(current severity). The escalation
// instant is the evaluation instant (BR-05). The repository guarantees a single
// escalation per incident. It returns how many incidents were escalated and
// keeps going when one of them fails.
func (s *Service) EscalateOverdue(ctx context.Context) (int, error) {
	candidates, err := s.repo.ListPendingEscalation(ctx, StateDeclared)
	if err != nil {
		return 0, fmt.Errorf("list pending escalation: %w", err)
	}
	now := s.now().UTC()
	escalated := 0
	var errs []error
	for _, inc := range candidates {
		deadline := s.sla.Deadline(inc.Severity)
		if deadline <= 0 || !now.After(inc.DeclaredAt.Add(deadline)) {
			continue
		}
		ev := TimelineEvent{Type: EventEscalation, AuthorID: nil, Data: map[string]string{}, OccurredAt: now}
		ok, err := s.repo.Escalate(ctx, inc, ev)
		if err != nil {
			errs = append(errs, fmt.Errorf("escalate %s: %w", inc.ID, err))
			continue
		}
		if ok {
			escalated++
		}
	}
	return escalated, errors.Join(errs...)
}

// ValidState reports whether st is one of the lifecycle states.
func ValidState(st State) bool { return validState(st) }

// authorize lets ingeniero, oncall and admin through (BR-10).
func authorize(actor user.User) error {
	switch actor.Role {
	case user.RoleIngeniero, user.RoleOncall, user.RoleAdmin:
		return nil
	}
	return ErrForbidden
}

func validImpact(i Impact) bool {
	switch i {
	case ImpactTotalOutage, ImpactDegradation, ImpactMinor:
		return true
	}
	return false
}

func validSeverity(s Severity) bool {
	switch s {
	case SeveritySEV1, SeveritySEV2, SeveritySEV3:
		return true
	}
	return false
}

func validState(st State) bool {
	switch st {
	case StateDeclared, StateAcknowledged, StateMitigating, StateResolved, StateClosed:
		return true
	}
	return false
}
