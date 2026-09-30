package incident

import (
	"errors"
	"testing"
)

func TestBR01_SeveridadSugeridaPorCriticidadEImpacto(t *testing.T) {
	tests := []struct {
		name        string
		criticality Criticality
		impact      Impact
		want        Severity
		wantErr     bool
	}{
		// 9 combinaciones de la tabla BR-01
		{"critica caida_total -> SEV1", CriticalityCritical, ImpactTotalOutage, SeveritySEV1, false},
		{"critica degradacion -> SEV2", CriticalityCritical, ImpactDegradation, SeveritySEV2, false},
		{"critica menor -> SEV3", CriticalityCritical, ImpactMinor, SeveritySEV3, false},
		{"importante caida_total -> SEV2", CriticalityImportant, ImpactTotalOutage, SeveritySEV2, false},
		{"importante degradacion -> SEV2", CriticalityImportant, ImpactDegradation, SeveritySEV2, false},
		{"importante menor -> SEV3", CriticalityImportant, ImpactMinor, SeveritySEV3, false},
		{"estandar caida_total -> SEV2", CriticalityStandard, ImpactTotalOutage, SeveritySEV2, false},
		{"estandar degradacion -> SEV3", CriticalityStandard, ImpactDegradation, SeveritySEV3, false},
		{"estandar menor -> SEV3", CriticalityStandard, ImpactMinor, SeveritySEV3, false},
		// Ejemplos de UC-02.1
		{"UC-02.1 payments critica caida_total -> SEV1", CriticalityCritical, ImpactTotalOutage, SeveritySEV1, false},
		{"UC-02.1 reports estandar degradacion -> SEV3", CriticalityStandard, ImpactDegradation, SeveritySEV3, false},
		// Entradas invalidas
		{"criticidad invalida", Criticality("urgente"), ImpactTotalOutage, "", true},
		{"impacto invalido", CriticalityCritical, Impact("parcial"), "", true},
		{"criticidad vacia", Criticality(""), ImpactTotalOutage, "", true},
		{"impacto vacio", CriticalityCritical, Impact(""), "", true},
		{"ambos vacios", Criticality(""), Impact(""), "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SuggestSeverity(tc.criticality, tc.impact)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidSeverityInput) {
					t.Fatalf("SuggestSeverity(%q, %q) error = %v, want ErrInvalidSeverityInput", tc.criticality, tc.impact, err)
				}
				if got != "" {
					t.Fatalf("SuggestSeverity(%q, %q) = %q, want \"\"", tc.criticality, tc.impact, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("SuggestSeverity(%q, %q) unexpected error: %v", tc.criticality, tc.impact, err)
			}
			if got != tc.want {
				t.Fatalf("SuggestSeverity(%q, %q) = %q, want %q", tc.criticality, tc.impact, got, tc.want)
			}
		})
	}
}
