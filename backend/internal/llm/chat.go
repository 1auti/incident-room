package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// chatTimeout bounds a generation: long enough for a local model, short enough
// that a hung provider becomes an error (a 503) instead of an endless request.
const chatTimeout = 2 * time.Minute

// Chat generates text with the configured provider.
type Chat struct {
	model llms.Model
}

// NewOllamaChat builds a Chat for model on an Ollama server (POST /api/chat).
func NewOllamaChat(serverURL, model string) (*Chat, error) {
	client, err := ollama.New(ollama.WithServerURL(serverURL), ollama.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}
	return &Chat{model: client}, nil
}

// NewGroqChat builds a Chat for model on Groq, whose API is OpenAI-compatible
// (POST {baseURL}/chat/completions).
func NewGroqChat(baseURL, apiKey, model string) (*Chat, error) {
	client, err := openai.New(openai.WithBaseURL(baseURL), openai.WithToken(apiKey), openai.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("groq chat: %w", err)
	}
	return &Chat{model: client}, nil
}

// Generate answers a single prompt.
func (c *Chat) Generate(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()
	out, err := llms.GenerateFromSinglePrompt(ctx, c.model, prompt)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	return out, nil
}
