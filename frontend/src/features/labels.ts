import type { Impact, State } from '../api/incidents'
import type { Criticality } from '../api/services'

// Presentation only: human labels for enum values. The values themselves are never changed.
export const impactLabel: Record<Impact, string> = {
  caida_total: 'Caída total',
  degradacion: 'Degradación',
  menor: 'Menor',
}

export const criticalityLabel: Record<Criticality, string> = {
  critica: 'Crítica',
  importante: 'Importante',
  estandar: 'Estándar',
}

export const stateLabel: Record<State, string> = {
  declarado: 'Declarado',
  reconocido: 'Reconocido',
  mitigando: 'Mitigando',
  resuelto: 'Resuelto',
  cerrado: 'Cerrado',
}

const eventTypeLabels: Record<string, string> = {
  declaracion: 'Declaración',
  escalado: 'Escalado',
  nota: 'Nota',
  cambio_severidad: 'Cambio de severidad',
  postmortem: 'Postmortem',
}

// Event types come from the backend as free text; unknown ones are shown as they are.
export function eventTypeText(type: string): string {
  return eventTypeLabels[type] ?? type
}
