package copilot

import "context"

// Role of a message in a conversation with the model.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn of the conversation. An assistant message may ask for
// tool calls; a tool message answers one of them (ToolCallID, Name).
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

// ToolCall is a tool invocation requested by the model; Arguments is JSON.
type ToolCall struct {
	ID, Name, Arguments string
}

// ToolDef describes a tool to the model; Parameters is a JSON Schema object.
type ToolDef struct {
	Name, Description string
	Parameters        map[string]any
}

// Model is a chat model with tool calling (an LLM).
type Model interface {
	Converse(ctx context.Context, messages []Message, tools []ToolDef) (Message, error)
}
