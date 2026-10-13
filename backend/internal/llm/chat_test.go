package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"incident-room-backend/internal/copilot"
	"incident-room-backend/internal/llm"
)

// request is the part of an OpenAI chat completion request the tests inspect.
type request struct {
	Model    string `json:"model"`
	Messages []struct {
		Role       string `json:"role"`
		Content    any    `json:"content"`
		ToolCallID string `json:"tool_call_id"`
		ToolCalls  []struct {
			ID       string `json:"id"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"messages"`
	Tools []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}

func completion(message map[string]any) map[string]any {
	return map[string]any{"id": "x", "object": "chat.completion", "model": "m",
		"choices": []map[string]any{{"index": 0, "finish_reason": "stop", "message": message}}}
}

// fakeServer answers each request with the next reply and records what it got.
func fakeServer(t *testing.T, replies ...map[string]any) (*httptest.Server, *[]request, *[]string) {
	t.Helper()
	var got []request
	var meta []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req request
		_ = json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req)
		meta = append(meta, r.URL.Path+" "+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(completion(replies[len(got)-1]))
	}))
	t.Cleanup(srv.Close)
	return srv, &got, &meta
}

// Groq and Ollama (under /v1) are both reached through the OpenAI-compatible protocol.
func TestChat_GenerarPorProtocoloOpenAI(t *testing.T) {
	for name, suffix := range map[string]string{"groq": "/openai/v1", "ollama": "/v1"} {
		t.Run(name, func(t *testing.T) {
			srv, got, meta := fakeServer(t, map[string]any{"role": "assistant", "content": "análisis"})
			chat, err := llm.NewChat(srv.URL+suffix, "key", "llama3.1")
			if err != nil {
				t.Fatal(err)
			}
			out, err := chat.Generate(context.Background(), "redactá el postmortem")
			if err != nil || out != "análisis" {
				t.Fatalf("Generate = %q, %v", out, err)
			}
			if (*meta)[0] != suffix+"/chat/completions Bearer key" || (*got)[0].Model != "llama3.1" {
				t.Errorf("request = %s, model %s", (*meta)[0], (*got)[0].Model)
			}
		})
	}
}

func TestChat_ToolCallingIdaYVuelta(t *testing.T) {
	srv, got, _ := fakeServer(t,
		map[string]any{"role": "assistant", "content": "", "tool_calls": []map[string]any{{
			"id": "call-1", "type": "function",
			"function": map[string]any{"name": "listar_abiertos", "arguments": `{"severidad":"SEV1"}`},
		}}},
		map[string]any{"role": "assistant", "content": "Hay 1 incidente SEV1 abierto."},
	)
	chat, err := llm.NewChat(srv.URL+"/v1", "ollama", "llama3.1")
	if err != nil {
		t.Fatal(err)
	}
	tools := []copilot.ToolDef{{Name: "listar_abiertos", Description: "Lista los incidentes activos",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"severidad": map[string]any{"type": "string"}}}}}
	conv := []copilot.Message{
		{Role: copilot.RoleSystem, Content: "Sos el copiloto"},
		{Role: copilot.RoleUser, Content: "listá los SEV1 abiertos"},
	}

	first, err := chat.Converse(context.Background(), conv, tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0] != (copilot.ToolCall{ID: "call-1", Name: "listar_abiertos", Arguments: `{"severidad":"SEV1"}`}) {
		t.Fatalf("first = %+v", first)
	}
	if req := (*got)[0]; len(req.Tools) != 1 || req.Tools[0].Function.Name != "listar_abiertos" || len(req.Messages) != 2 {
		t.Errorf("first request = %+v", req)
	}

	conv = append(conv, first, copilot.Message{Role: copilot.RoleTool, ToolCallID: "call-1", Name: "listar_abiertos", Content: `[{"id":"i1"}]`})
	second, err := chat.Converse(context.Background(), conv, tools)
	if err != nil || second.Content != "Hay 1 incidente SEV1 abierto." || len(second.ToolCalls) != 0 {
		t.Fatalf("second = %+v, %v", second, err)
	}
	// The second request carries the assistant's tool call and the tool's answer.
	msgs := (*got)[1].Messages
	if len(msgs) != 4 || msgs[2].Role != "assistant" || len(msgs[2].ToolCalls) != 1 || msgs[2].ToolCalls[0].ID != "call-1" ||
		msgs[3].Role != "tool" || msgs[3].ToolCallID != "call-1" {
		t.Errorf("second request messages = %+v", msgs)
	}
}
