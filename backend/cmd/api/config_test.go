package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"incident-room-backend/internal/incident"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestBR03_SLAConfigurablePorEntorno(t *testing.T) {
	got, err := slaFromEnv(env(nil))
	if err != nil || got != incident.DefaultSLA() {
		t.Fatalf("defaults = %+v, %v; want %+v", got, err, incident.DefaultSLA())
	}
	got, err = slaFromEnv(env(map[string]string{"SLA_SEV1": "30s", "SLA_SEV3": "2h"}))
	want := incident.SLA{SEV1: 30 * time.Second, SEV2: 15 * time.Minute, SEV3: 2 * time.Hour}
	if err != nil || got != want {
		t.Fatalf("overrides = %+v, %v; want %+v", got, err, want)
	}
	for _, bad := range []map[string]string{
		{"SLA_SEV1": "abc"}, {"SLA_SEV2": "0s"}, {"SLA_SEV3": "-5m"},
	} {
		if _, err := slaFromEnv(env(bad)); err == nil {
			t.Errorf("slaFromEnv(%v) accepted an invalid value", bad)
		}
	}
}

func TestBR04_IntervaloDeEscaladoPorEntorno(t *testing.T) {
	if got, err := escalationIntervalFromEnv(env(nil)); err != nil || got != 10*time.Second {
		t.Errorf("default = %v, %v; want 10s", got, err)
	}
	if got, err := escalationIntervalFromEnv(env(map[string]string{"ESCALATION_INTERVAL": "1s"})); err != nil || got != time.Second {
		t.Errorf("1s = %v, %v", got, err)
	}
	for _, bad := range []string{"x", "0s", "-1s"} {
		if _, err := escalationIntervalFromEnv(env(map[string]string{"ESCALATION_INTERVAL": bad})); err == nil {
			t.Errorf("interval %q accepted", bad)
		}
	}
}

func TestLLM_ProveedorDeChatPorEntorno(t *testing.T) {
	got, err := chatConfigFromEnv(env(nil))
	if want := (chatConfig{provider: "ollama", baseURL: "http://localhost:11434/v1", apiKey: "ollama", model: "llama3.1"}); err != nil || got != want {
		t.Errorf("defaults = %+v, %v; want %+v", got, err, want)
	}
	got, err = chatConfigFromEnv(env(map[string]string{"OLLAMA_URL": "http://ollama:11434/", "LLM_MODEL": "qwen2.5"}))
	if want := (chatConfig{provider: "ollama", baseURL: "http://ollama:11434/v1", apiKey: "ollama", model: "qwen2.5"}); err != nil || got != want {
		t.Errorf("ollama overrides = %+v, %v; want %+v", got, err, want)
	}
	got, err = chatConfigFromEnv(env(map[string]string{"LLM_PROVIDER": "groq", "GROQ_API_KEY": "gsk"}))
	if want := (chatConfig{provider: "groq", baseURL: "https://api.groq.com/openai/v1", apiKey: "gsk", model: "openai/gpt-oss-20b"}); err != nil || got != want {
		t.Errorf("groq = %+v, %v; want %+v", got, err, want)
	}
	if strings.Contains(fmt.Sprintf("%v %+v", got, got), "gsk") {
		t.Errorf("the configuration prints the API key: %v", got)
	}
	for _, bad := range []map[string]string{
		{"LLM_PROVIDER": "groq"},
		{"LLM_PROVIDER": "openai"},
	} {
		if _, err := chatConfigFromEnv(env(bad)); err == nil {
			t.Errorf("chatConfigFromEnv(%v) accepted an invalid configuration", bad)
		}
	}
}

func TestCopiloto_UmbralDeSimilitudPorEntorno(t *testing.T) {
	if got, err := minSimilarityFromEnv(env(nil)); err != nil || got != 0.5 {
		t.Errorf("default = %v, %v; want 0.5", got, err)
	}
	if got, err := minSimilarityFromEnv(env(map[string]string{"RAG_MIN_SIMILARITY": "0.7"})); err != nil || got != 0.7 {
		t.Errorf("0.7 = %v, %v", got, err)
	}
	for _, bad := range []string{"x", "-0.1", "1.5"} {
		if _, err := minSimilarityFromEnv(env(map[string]string{"RAG_MIN_SIMILARITY": bad})); err == nil {
			t.Errorf("RAG_MIN_SIMILARITY=%q accepted", bad)
		}
	}
}
