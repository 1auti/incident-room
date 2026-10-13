// Package postmortem holds the assisted postmortem of an incident: an LLM
// drafts it from the timeline, the on-call of the service or the admin edits or
// regenerates it, and only the admin approves it (BR-05, BR-07, BR-09, BR-10,
// BR-11, BR-13).
package postmortem

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

// Status is the lifecycle of a postmortem; aprobado is final (BR-13).
type Status string

const (
	StatusDraft    Status = "borrador"
	StatusApproved Status = "aprobado"
)

// Domain errors translated to HTTP by handlers.
var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("postmortem not found")
	ErrInvalid   = errors.New("invalid postmortem: content is required")
	// ErrIncidentNotResolved: a draft is only generated with the incident resuelto.
	ErrIncidentNotResolved = errors.New("incident is not resuelto")
	// ErrApproved: an approved postmortem is immutable.
	ErrApproved = errors.New("postmortem is approved")
	// ErrConflict: another request created the postmortem meanwhile.
	ErrConflict = errors.New("postmortem changed meanwhile")
	// ErrGeneration: the LLM failed or answered nothing. Nothing is stored.
	ErrGeneration = errors.New("postmortem generation failed")
)

// Postmortem of an incident; at most one per incident (BR-13).
type Postmortem struct {
	ID             string     `json:"id"`
	IncidentID     string     `json:"incident_id"`
	Content        string     `json:"content"`
	Status         Status     `json:"status"`
	GeneratedByLLM bool       `json:"generated_by_llm"`
	ApprovedBy     *string    `json:"approved_by"`
	ApprovedAt     *time.Time `json:"approved_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Generator writes text from a prompt (an LLM).
type Generator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Incidents reads the incident side: the incident, its service and its timeline.
type Incidents interface {
	Get(ctx context.Context, id string) (incident.Incident, error)
	FindService(ctx context.Context, id string) (incident.ServiceInfo, error)
	ListEvents(ctx context.Context, incidentID string) ([]incident.EventView, error)
}

// Repository persists postmortems. Every write that comes with a timeline event
// stores both in one transaction. CreateDraft and ReplaceDraft re-check in that
// transaction that the incident is still resuelto (ErrIncidentNotResolved).
// ReplaceDraft, UpdateContent and Approve only
// touch a borrador (ErrApproved otherwise, ErrNotFound when there is none);
// CreateDraft returns ErrConflict when the incident already has one.
type Repository interface {
	Find(ctx context.Context, incidentID string) (Postmortem, error)
	CreateDraft(ctx context.Context, pm Postmortem, ev incident.TimelineEvent) (Postmortem, error)
	ReplaceDraft(ctx context.Context, incidentID, content string, at time.Time, ev incident.TimelineEvent) (Postmortem, error)
	UpdateContent(ctx context.Context, incidentID, content string, at time.Time) (Postmortem, error)
	Approve(ctx context.Context, incidentID, approvedBy string, at time.Time, ev incident.TimelineEvent) (Postmortem, error)
}

// Service applies the postmortem rules. It never reads the system clock: now
// is injected (BR-05).
type Service struct {
	incidents Incidents
	repo      Repository
	llm       Generator
	now       func() time.Time
}

// NewService builds a Service.
func NewService(incidents Incidents, repo Repository, llm Generator, now func() time.Time) *Service {
	return &Service{incidents: incidents, repo: repo, llm: llm, now: now}
}

// Get returns the postmortem of an incident; every authenticated role reads it (BR-10).
func (s *Service) Get(ctx context.Context, actor user.User, incidentID string) (Postmortem, error) {
	if incident.Authorize(actor) != nil {
		return Postmortem{}, ErrForbidden
	}
	pm, err := s.repo.Find(ctx, incidentID)
	if err != nil {
		return Postmortem{}, fmt.Errorf("find postmortem: %w", err)
	}
	return pm, nil
}

// Generate drafts the postmortem with the LLM from the timeline, or replaces
// the content of the existing draft. Only with the incident resuelto, by its
// service's on-call or the admin (BR-13, BR-11). It adds a postmortem event.
func (s *Service) Generate(ctx context.Context, actor user.User, incidentID string) (Postmortem, error) {
	inc, err := s.authorizeOperator(ctx, actor, incidentID)
	if err != nil {
		return Postmortem{}, err
	}
	if inc.State != incident.StateResolved {
		return Postmortem{}, ErrIncidentNotResolved
	}
	existing, err := s.repo.Find(ctx, incidentID)
	exists := err == nil
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Postmortem{}, fmt.Errorf("find postmortem: %w", err)
	}
	if exists && existing.Status == StatusApproved {
		return Postmortem{}, ErrApproved
	}

	events, err := s.incidents.ListEvents(ctx, incidentID)
	if err != nil {
		return Postmortem{}, fmt.Errorf("list events: %w", err)
	}
	// The LLM is called before any write: a failure leaves no trace.
	analysis, err := s.llm.Generate(ctx, prompt(inc, events))
	if err != nil {
		return Postmortem{}, fmt.Errorf("%w: %w", ErrGeneration, err)
	}
	analysis = strings.TrimSpace(analysis)
	if analysis == "" {
		return Postmortem{}, fmt.Errorf("%w: empty answer", ErrGeneration)
	}
	content := draft(analysis, events)

	at := s.now().UTC()
	if !exists {
		pm := Postmortem{IncidentID: incidentID, Content: content, Status: StatusDraft, GeneratedByLLM: true, UpdatedAt: at}
		created, err := s.repo.CreateDraft(ctx, pm, event(actor, at, "generado"))
		if err != nil {
			return Postmortem{}, fmt.Errorf("create postmortem: %w", err)
		}
		return created, nil
	}
	replaced, err := s.repo.ReplaceDraft(ctx, incidentID, content, at, event(actor, at, "regenerado"))
	if err != nil {
		return Postmortem{}, fmt.Errorf("replace postmortem: %w", err)
	}
	return replaced, nil
}

// Edit replaces the text of the draft as written. Same roles as Generate;
// editing adds no event (BR-09).
func (s *Service) Edit(ctx context.Context, actor user.User, incidentID, content string) (Postmortem, error) {
	if _, err := s.authorizeOperator(ctx, actor, incidentID); err != nil {
		return Postmortem{}, err
	}
	if strings.TrimSpace(content) == "" {
		return Postmortem{}, ErrInvalid
	}
	pm, err := s.repo.UpdateContent(ctx, incidentID, content, s.now().UTC())
	if err != nil {
		return Postmortem{}, fmt.Errorf("edit postmortem: %w", err)
	}
	return pm, nil
}

// Approve makes the draft final. Only the admin; it records who and when and
// adds a postmortem event. An approved postmortem enables closing a SEV1 (BR-07).
func (s *Service) Approve(ctx context.Context, actor user.User, incidentID string) (Postmortem, error) {
	if actor.Role != user.RoleAdmin {
		return Postmortem{}, ErrForbidden
	}
	at := s.now().UTC()
	pm, err := s.repo.Approve(ctx, incidentID, actor.ID, at, event(actor, at, "aprobado"))
	if err != nil {
		return Postmortem{}, fmt.Errorf("approve postmortem: %w", err)
	}
	return pm, nil
}

// authorizeOperator lets through the admin and the on-call of the incident's
// service (BR-11, BR-13); an ingeniero never works the postmortem.
func (s *Service) authorizeOperator(ctx context.Context, actor user.User, incidentID string) (incident.Incident, error) {
	if actor.Role != user.RoleAdmin && actor.Role != user.RoleOncall {
		return incident.Incident{}, ErrForbidden
	}
	inc, err := s.incidents.Get(ctx, incidentID)
	if err != nil {
		return incident.Incident{}, fmt.Errorf("get incident: %w", err)
	}
	info, err := s.incidents.FindService(ctx, inc.ServiceID)
	if err != nil {
		return incident.Incident{}, fmt.Errorf("find service: %w", err)
	}
	if !incident.CanOperate(actor, info) {
		return incident.Incident{}, ErrForbidden
	}
	return inc, nil
}

func event(actor user.User, at time.Time, action string) incident.TimelineEvent {
	author := actor.ID
	return incident.TimelineEvent{
		Type:       incident.EventPostmortem,
		AuthorID:   &author,
		Body:       "Postmortem " + action,
		Data:       map[string]string{"action": action},
		OccurredAt: at,
	}
}

// prompt asks the LLM for the analysis; the timeline it is given is the same
// one the draft cites.
func prompt(inc incident.Incident, events []incident.EventView) string {
	rootCause := ""
	if inc.RootCause != nil {
		rootCause = *inc.RootCause
	}
	return fmt.Sprintf(`Sos un ingeniero SRE. Escribí en español el análisis de un postmortem sin culpables (blameless)
para el incidente de abajo: resumen, impacto, causa raíz y acciones para evitar que se repita.
Usá solo los hechos de la timeline; no inventes eventos. No repitas la timeline: se agrega aparte.

Incidente: %s
Severidad: %s
Causa raíz: %s

Timeline:
%s`, inc.Title, inc.Severity, rootCause, timeline(events))
}

// draft joins the LLM analysis with a timeline section built here, so the draft
// cites every event no matter what the LLM wrote (UC-08.1).
func draft(analysis string, events []incident.EventView) string {
	return "## Análisis\n\n" + analysis + "\n\n## Timeline\n\n" + timeline(events)
}

func timeline(events []incident.EventView) string {
	var b strings.Builder
	for _, e := range events {
		author := "Sistema"
		if e.AuthorID != nil && e.AuthorName != nil {
			author = *e.AuthorName
		}
		parts := []string{e.OccurredAt.UTC().Format(time.RFC3339), string(e.Type), author}
		if e.Body != "" {
			parts = append(parts, e.Body)
		}
		if from, to := e.Data["from"], e.Data["to"]; from != "" && to != "" {
			parts = append(parts, from+" → "+to)
		}
		b.WriteString("- " + strings.Join(parts, " · ") + "\n")
	}
	return b.String()
}
