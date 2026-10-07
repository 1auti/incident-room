package main

import (
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
