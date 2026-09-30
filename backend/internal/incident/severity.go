package incident

import (
	"errors"
	"fmt"
)

type Criticality string
type Impact string
type Severity string

const (
	CriticalityCritical  Criticality = "critica"
	CriticalityImportant Criticality = "importante"
	CriticalityStandard  Criticality = "estandar"
)

const (
	ImpactTotalOutage Impact = "caida_total"
	ImpactDegradation Impact = "degradacion"
	ImpactMinor       Impact = "menor"
)

const (
	SeveritySEV1 Severity = "SEV1"
	SeveritySEV2 Severity = "SEV2"
	SeveritySEV3 Severity = "SEV3"
)

var ErrInvalidSeverityInput = errors.New("criticidad o impacto desconocido")

func SuggestSeverity(c Criticality, i Impact) (Severity, error) {
	switch c {
	case CriticalityCritical:
		switch i {
		case ImpactTotalOutage:
			return SeveritySEV1, nil
		case ImpactDegradation:
			return SeveritySEV2, nil
		case ImpactMinor:
			return SeveritySEV3, nil
		}
	case CriticalityImportant:
		switch i {
		case ImpactTotalOutage:
			return SeveritySEV2, nil
		case ImpactDegradation:
			return SeveritySEV2, nil
		case ImpactMinor:
			return SeveritySEV3, nil
		}
	case CriticalityStandard:
		switch i {
		case ImpactTotalOutage:
			return SeveritySEV2, nil
		case ImpactDegradation:
			return SeveritySEV3, nil
		case ImpactMinor:
			return SeveritySEV3, nil
		}
	}

	switch c {
	case CriticalityCritical, CriticalityImportant, CriticalityStandard:
		return "", fmt.Errorf("impacto %q: %w", i, ErrInvalidSeverityInput)
	default:
		return "", fmt.Errorf("criticidad %q: %w", c, ErrInvalidSeverityInput)
	}
}
