package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"

	"incident-room-backend/internal/copilot"
)

// chatTimeout bounds a model call: long enough for a local model, short enough
// that a hung provider becomes an error (a 503) instead of an endless request.
const chatTimeout = 2 * time.Minute

// Chat talks to an OpenAI-compatible chat API: Groq, or Ollama under /v1.
// One protocol serves both plain generation (UC-08) and tool calling (UC-10);
// LangChainGo's native Ollama client has no tool calling.
type Chat struct {
	model llms.Model
}

// NewChat builds a Chat for model at baseURL (e.g. https://api.groq.com/openai/v1
// or http://localhost:11434/v1). Ollama ignores the API key but the client
// requires one, so any non-empty value works there.
func NewChat(baseURL, apiKey, model string) (*Chat, error) {
	client, err := openai.New(openai.WithBaseURL(baseURL), openai.WithToken(apiKey), openai.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("chat client: %w", err)
	}
	return &Chat{model: client}, nil
}

// Generate answers a single prompt (the postmortem draft, UC-08).
func (c *Chat) Generate(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()
	out, err := llms.GenerateFromSinglePrompt(ctx, c.model, prompt)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	return out, nil
}

// Converse sends the conversation with the available tools and returns the
// model's next message: text, tool calls, or both (the copilot, UC-10).
func (c *Chat) Converse(ctx context.Context, messages []copilot.Message, tools []copilot.ToolDef) (copilot.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()
	defs := make([]llms.Tool, len(tools))
	for i, t := range tools {
		defs[i] = llms.Tool{Type: "function", Function: &llms.FunctionDefinition{
			Name: t.Name, Description: t.Description, Parameters: t.Parameters,
		}}
	}
	resp, err := c.model.GenerateContent(ctx, toContent(messages), llms.WithTools(defs))
	if err != nil {
		return copilot.Message{}, fmt.Errorf("converse: %w", err)
	}
	if len(resp.Choices) == 0 {
		return copilot.Message{}, fmt.Errorf("converse: empty response")
	}
	choice := resp.Choices[0]
	out := copilot.Message{Role: copilot.RoleAssistant, Content: choice.Content}
	for _, tc := range choice.ToolCalls {
		if tc.FunctionCall != nil {
			out.ToolCalls = append(out.ToolCalls, copilot.ToolCall{ID: tc.ID, Name: tc.FunctionCall.Name, Arguments: tc.FunctionCall.Arguments})
		}
	}
	return out, nil
}

// toContent maps the conversation to LangChainGo's message parts.
func toContent(messages []copilot.Message) []llms.MessageContent {
	out := make([]llms.MessageContent, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case copilot.RoleSystem:
			out = append(out, llms.TextParts(llms.ChatMessageTypeSystem, m.Content))
		case copilot.RoleUser:
			out = append(out, llms.TextParts(llms.ChatMessageTypeHuman, m.Content))
		case copilot.RoleAssistant:
			mc := llms.MessageContent{Role: llms.ChatMessageTypeAI}
			if m.Content != "" {
				mc.Parts = append(mc.Parts, llms.TextContent{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				mc.Parts = append(mc.Parts, llms.ToolCall{ID: tc.ID, Type: "function",
					FunctionCall: &llms.FunctionCall{Name: tc.Name, Arguments: tc.Arguments}})
			}
			out = append(out, mc)
		case copilot.RoleTool:
			out = append(out, llms.MessageContent{Role: llms.ChatMessageTypeTool, Parts: []llms.ContentPart{
				llms.ToolCallResponse{ToolCallID: m.ToolCallID, Name: m.Name, Content: m.Content},
			}})
		}
	}
	return out
}
