package incident

import (
	"context"
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
}

// NewService builds a Service.
func NewService(repo Repository, now func() time.Time) *Service {
	return &Service{repo: repo, now: now}
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

// activeStates are all the states but cerrado (BR-15).
var activeStates = []State{StateDeclared, StateAcknowledged, StateMitigating, StateResolved}

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

	states := slices.Clone(activeStates)
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
