import type { Severity, State } from '../../api/incidents'
import type { BadgeTone } from '../../ui/Badge'

// Presentation only: maps a value to a badge colour; it carries no business rule.
export const severityTone: Record<Severity, BadgeTone> = {
  SEV1: 'danger',
  SEV2: 'warning',
  SEV3: 'info',
}

export const stateTone: Record<State, BadgeTone> = {
  declarado: 'danger',
  reconocido: 'warning',
  mitigando: 'info',
  resuelto: 'success',
  cerrado: 'neutral',
}
