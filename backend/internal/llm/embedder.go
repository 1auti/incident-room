// Package llm adapts LangChainGo to the small interfaces the domain packages
// define, so they never import it directly. Provider URLs and models come from
// the environment (backend/AGENTS.md).
package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/ollama"
)

// embedTimeout bounds a call to Ollama, so a hung server turns into an error
// (a 503 for the client) instead of a request that never ends.
const embedTimeout = 30 * time.Second

// Embedder computes embeddings with an Ollama server (POST {serverURL}/api/embed).
type Embedder struct {
	impl *embeddings.EmbedderImpl
}

// NewOllamaEmbedder builds an Embedder for model at serverURL. It does not
// contact the server: a failure shows up on the first Embed.
func NewOllamaEmbedder(serverURL, model string) (*Embedder, error) {
	client, err := ollama.New(ollama.WithServerURL(serverURL), ollama.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("ollama client: %w", err)
	}
	impl, err := embeddings.NewEmbedder(client)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: %w", err)
	}
	return &Embedder{impl: impl}, nil
}

// Embed returns the embedding of text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()
	v, err := e.impl.EmbedQuery(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	return v, nil
}
