package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"incident-room-backend/internal/llm"
)

// The adapters are checked against fake servers speaking each provider's
// protocol: Groq cannot be reached from the tests (no API key, no network).
func TestChat_ProveedoresPorProtocolo(t *testing.T) {
	type seen struct{ path, auth, model, prompt string }
	for _, tc := range []struct {
		name  string
		build func(url string) (*llm.Chat, error)
		path  string
		reply func(w http.ResponseWriter)
		auth  string
	}{
		{
			name:  "ollama",
			build: func(url string) (*llm.Chat, error) { return llm.NewOllamaChat(url, "llama3.1") },
			path:  "/api/chat",
			reply: func(w http.ResponseWriter) {
				_ = json.NewEncoder(w).Encode(map[string]any{"model": "llama3.1", "done": true,
					"message": map[string]string{"role": "assistant", "content": "análisis"}})
			},
		},
		{
			name:  "groq",
			build: func(url string) (*llm.Chat, error) { return llm.NewGroqChat(url+"/openai/v1", "gsk-test", "llama3.1") },
			path:  "/openai/v1/chat/completions",
			auth:  "Bearer gsk-test",
			reply: func(w http.ResponseWriter) {
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "x", "object": "chat.completion", "model": "llama3.1",
					"choices": []map[string]any{{"index": 0, "finish_reason": "stop",
						"message": map[string]string{"role": "assistant", "content": "análisis"}}}})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got seen
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Model    string `json:"model"`
					Messages []struct {
						Content any `json:"content"`
					} `json:"messages"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				got = seen{path: r.URL.Path, auth: r.Header.Get("Authorization"), model: body.Model}
				if len(body.Messages) > 0 {
					b, _ := json.Marshal(body.Messages[0].Content)
					got.prompt = string(b)
				}
				w.Header().Set("Content-Type", "application/json")
				tc.reply(w)
			}))
			defer srv.Close()

			chat, err := tc.build(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			out, err := chat.Generate(context.Background(), "redactá el postmortem")
			if err != nil || out != "análisis" {
				t.Fatalf("Generate = %q, %v", out, err)
			}
			if got.path != tc.path || got.model != "llama3.1" || got.auth != tc.auth {
				t.Errorf("request = %+v, want path %s, model llama3.1, auth %q", got, tc.path, tc.auth)
			}
			if got.prompt == "" || !json.Valid([]byte(got.prompt)) {
				t.Errorf("prompt not sent: %q", got.prompt)
			}
		})
	}
}
