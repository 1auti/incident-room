// Package runbook manages the runbooks of each service and the embedding the
// copilot retrieves them by (BR-10, BR-12, BR-14).
package runbook

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/user"
)

// Dimensions is the embedding size the runbooks table stores (vector(768),
// nomic-embed-text). Another model needs a new migration.
const Dimensions = 768

// Domain errors translated to HTTP by handlers.
var (
	ErrInvalid         = errors.New("invalid runbook: title and content are required")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("runbook not found")
	ErrServiceNotFound = errors.New("service not found")
	// ErrEmbedding: the embedding could not be computed (provider down) or has
	// the wrong size. Nothing is stored without its embedding.
	ErrEmbedding = errors.New("embedding unavailable")
)

// Runbook is a procedure of a service. Its embedding is never exposed: it only
// lives in the store, next to the row.
type Runbook struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Match is a runbook retrieved for a query, with its service name for citing
// and its cosine similarity to the query (1 = same direction).
type Match struct {
	Runbook
	ServiceName string  `json:"service_name"`
	Similarity  float64 `json:"similarity"`
}

// Embedder turns text into a vector.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Repository is the persistence the Service needs. A runbook is stored and
// updated together with its embedding; Delete is physical and takes the
// embedding with the row (BR-12). ListByService returns a service's runbooks
// by title (an unknown service has none). Create returns ErrServiceNotFound
// for an unknown or malformed service; Update and Delete return ErrNotFound.
// Update keeps service_id and created_by. Search returns up to k runbooks whose
// similarity to embedding is at least minSimilarity, most similar first; only
// stored runbooks exist, so a removed one is never found (BR-12, BR-14).
type Repository interface {
	Search(ctx context.Context, embedding []float32, k int, minSimilarity float64) ([]Match, error)
	ListByService(ctx context.Context, serviceID string) ([]Runbook, error)
	Create(ctx context.Context, rb Runbook, embedding []float32) (Runbook, error)
	Update(ctx context.Context, rb Runbook, embedding []float32) (Runbook, error)
	Delete(ctx context.Context, id string) error
}

// Service applies the runbook rules. It never reads the system clock: now is
// injected (BR-05).
type Service struct {
	repo     Repository
	embedder Embedder
	now      func() time.Time
}

// NewService builds a Service.
func NewService(repo Repository, embedder Embedder, now func() time.Time) *Service {
	return &Service{repo: repo, embedder: embedder, now: now}
}

// List returns the runbooks of a service; every authenticated role reads them (BR-10).
func (s *Service) List(ctx context.Context, actor user.User, serviceID string) ([]Runbook, error) {
	if incident.Authorize(actor) != nil {
		return nil, ErrForbidden
	}
	list, err := s.repo.ListByService(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list runbooks: %w", err)
	}
	return list, nil
}

// Search retrieves the runbooks most related to query for the copilot (RAG);
// every authenticated role reads runbooks (BR-10).
func (s *Service) Search(ctx context.Context, actor user.User, query string, k int, minSimilarity float64) ([]Match, error) {
	if incident.Authorize(actor) != nil {
		return nil, ErrForbidden
	}
	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbedding, err)
	}
	if len(embedding) != Dimensions {
		return nil, fmt.Errorf("%w: got %d dimensions, want %d", ErrEmbedding, len(embedding), Dimensions)
	}
	matches, err := s.repo.Search(ctx, embedding, k, minSimilarity)
	if err != nil {
		return nil, fmt.Errorf("search runbooks: %w", err)
	}
	return matches, nil
}

// Create stores a runbook of a service with the embedding of its content. Only
// the admin manages runbooks (BR-12).
func (s *Service) Create(ctx context.Context, actor user.User, serviceID, title, content string) (Runbook, error) {
	rb, embedding, err := s.prepare(ctx, actor, title, content)
	if err != nil {
		return Runbook{}, err
	}
	rb.ServiceID, rb.CreatedBy = serviceID, actor.ID
	created, err := s.repo.Create(ctx, rb, embedding)
	if err != nil {
		return Runbook{}, fmt.Errorf("create runbook: %w", err)
	}
	return created, nil
}

// Update replaces title and content and recalculates the embedding (UC-07.2).
func (s *Service) Update(ctx context.Context, actor user.User, id, title, content string) (Runbook, error) {
	rb, embedding, err := s.prepare(ctx, actor, title, content)
	if err != nil {
		return Runbook{}, err
	}
	rb.ID = id
	updated, err := s.repo.Update(ctx, rb, embedding)
	if err != nil {
		return Runbook{}, fmt.Errorf("update runbook: %w", err)
	}
	return updated, nil
}

// Delete removes a runbook physically, embedding included, so the copilot no
// longer retrieves it (BR-12, BR-14).
func (s *Service) Delete(ctx context.Context, actor user.User, id string) error {
	if actor.Role != user.RoleAdmin {
		return ErrForbidden
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete runbook: %w", err)
	}
	return nil
}

// prepare checks the admin role and the input, and embeds the content. It runs
// before any write, so a refusal or an embedding failure leaves no trace.
func (s *Service) prepare(ctx context.Context, actor user.User, title, content string) (Runbook, []float32, error) {
	if actor.Role != user.RoleAdmin {
		return Runbook{}, nil, ErrForbidden
	}
	// The content is kept as written: leading indentation can matter in a procedure.
	title = strings.TrimSpace(title)
	if title == "" || strings.TrimSpace(content) == "" {
		return Runbook{}, nil, ErrInvalid
	}
	embedding, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return Runbook{}, nil, fmt.Errorf("%w: %w", ErrEmbedding, err)
	}
	if len(embedding) != Dimensions {
		return Runbook{}, nil, fmt.Errorf("%w: got %d dimensions, want %d", ErrEmbedding, len(embedding), Dimensions)
	}
	return Runbook{Title: title, Content: content, UpdatedAt: s.now().UTC()}, embedding, nil
}
