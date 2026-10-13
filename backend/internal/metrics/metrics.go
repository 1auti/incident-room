// Package metrics computes the dashboard metrics with their exact domain
// definitions: MTTR (BR-16), incidents per service (BR-17) and SLA compliance
// (BR-18), over a period [from, to) in UTC evaluated with an injected clock
// (BR-05).
package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

// ErrInvalidPeriod: a preset other than 7 or 30 days, a free range without
// both ends or with from >= to, or a preset mixed with a free range.
var ErrInvalidPeriod = errors.New("invalid metrics period")

const (
	day             = 24 * time.Hour
	defaultLastDays = 30
)

// Period is the half-open interval [From, To) in UTC.
type Period struct {
	From, To time.Time
}

// Query selects the period: LastDays (7 or 30) counts back from now, From/To
// is a free range; the zero Query means the last 30 days.
type Query struct {
	LastDays int
	From, To time.Time
}

// ResolvedRow is an incident resolved inside the period (MTTR, BR-16).
type ResolvedRow struct {
	DeclaredAt, ResolvedAt time.Time
}

// DeclaredRow is an incident declared inside the period (BR-17, BR-18).
type DeclaredRow struct {
	ServiceID      string
	Severity       incident.Severity
	DeclaredAt     time.Time
	AcknowledgedAt *time.Time
}

// ServiceRef names a service; every service appears in the per-service count.
type ServiceRef struct {
	ID, Name string
}

// Repository reads what the metrics need. ResolvedIn filters by resolved_at
// and DeclaredIn by declared_at, both within [From, To). Services returns every
// service in the order the dashboard shows them.
type Repository interface {
	ResolvedIn(ctx context.Context, p Period) ([]ResolvedRow, error)
	DeclaredIn(ctx context.Context, p Period) ([]DeclaredRow, error)
	Services(ctx context.Context) ([]ServiceRef, error)
}

// ServiceCount is the number of incidents declared in the period for a service.
type ServiceCount struct {
	ServiceID string `json:"service_id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
}

// Dashboard holds the metrics of a period. A nil metric has no value: no data
// is never reported as 0 (BR-16, BR-18).
type Dashboard struct {
	From                 time.Time      `json:"from"`
	To                   time.Time      `json:"to"`
	MTTRSeconds          *float64       `json:"mttr_seconds"`
	IncidentsByService   []ServiceCount `json:"incidents_by_service"`
	SLACompliancePercent *float64       `json:"sla_compliance_percent"`
}

// Service computes the dashboard. It never reads the system clock: now is
// injected (BR-05). sla is the same acknowledgement deadline table the
// escalation uses (BR-03).
type Service struct {
	repo Repository
	now  func() time.Time
	sla  incident.SLA
}

// NewService builds a Service.
func NewService(repo Repository, now func() time.Time, sla incident.SLA) *Service {
	return &Service{repo: repo, now: now, sla: sla}
}

// Dashboard returns the metrics for the period q selects. Every authenticated
// role sees it (BR-10).
func (s *Service) Dashboard(ctx context.Context, actor user.User, q Query) (Dashboard, error) {
	if err := incident.Authorize(actor); err != nil {
		return Dashboard{}, err
	}
	now := s.now().UTC()
	p, err := periodOf(q, now)
	if err != nil {
		return Dashboard{}, err
	}

	resolved, err := s.repo.ResolvedIn(ctx, p)
	if err != nil {
		return Dashboard{}, fmt.Errorf("resolved incidents: %w", err)
	}
	declared, err := s.repo.DeclaredIn(ctx, p)
	if err != nil {
		return Dashboard{}, fmt.Errorf("declared incidents: %w", err)
	}
	services, err := s.repo.Services(ctx)
	if err != nil {
		return Dashboard{}, fmt.Errorf("services: %w", err)
	}

	return Dashboard{
		From:                 p.From,
		To:                   p.To,
		MTTRSeconds:          mttr(resolved),
		IncidentsByService:   countByService(services, declared),
		SLACompliancePercent: s.slaCompliance(declared, now),
	}, nil
}

func periodOf(q Query, now time.Time) (Period, error) {
	free := !q.From.IsZero() || !q.To.IsZero()
	switch {
	case free && q.LastDays != 0:
		return Period{}, ErrInvalidPeriod
	case free:
		if q.From.IsZero() || q.To.IsZero() || !q.From.Before(q.To) {
			return Period{}, ErrInvalidPeriod
		}
		return Period{From: q.From.UTC(), To: q.To.UTC()}, nil
	case q.LastDays == 0:
		q.LastDays = defaultLastDays
	case q.LastDays != 7 && q.LastDays != 30:
		return Period{}, ErrInvalidPeriod
	}
	return Period{From: now.Add(-time.Duration(q.LastDays) * day), To: now}, nil
}

// mttr is the mean of resolved_at - declared_at (BR-16); nil without data.
func mttr(rows []ResolvedRow) *float64 {
	if len(rows) == 0 {
		return nil
	}
	var total time.Duration
	for _, r := range rows {
		total += r.ResolvedAt.Sub(r.DeclaredAt)
	}
	mean := (total / time.Duration(len(rows))).Seconds()
	return &mean
}

// countByService counts per service, including services with 0 (BR-17).
func countByService(services []ServiceRef, rows []DeclaredRow) []ServiceCount {
	counts := make(map[string]int, len(services))
	for _, r := range rows {
		counts[r.ServiceID]++
	}
	out := make([]ServiceCount, len(services))
	for i, svc := range services {
		out[i] = ServiceCount{ServiceID: svc.ID, Name: svc.Name, Count: counts[svc.ID]}
	}
	return out
}

// slaCompliance is complying / eligible as a percentage (BR-18). An incident is
// eligible when it was acknowledged or its deadline already passed (now >
// deadline, as in BR-04); it complies when acknowledged_at <= deadline. The
// deadline uses the current severity, measured from declared_at (BR-03).
func (s *Service) slaCompliance(rows []DeclaredRow, now time.Time) *float64 {
	var eligible, complying int
	for _, r := range rows {
		deadline := r.DeclaredAt.Add(s.sla.Deadline(r.Severity))
		switch {
		case r.AcknowledgedAt != nil:
			eligible++
			if !r.AcknowledgedAt.After(deadline) {
				complying++
			}
		case now.After(deadline):
			eligible++
		}
	}
	if eligible == 0 {
		return nil
	}
	pct := float64(complying) * 100 / float64(eligible)
	return &pct
}
