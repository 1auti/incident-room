import type { Incident, Severity, State } from '../../api/incidents'

// Active incidents never are "cerrado" (BR-15), so it has no counter.
export type ActiveState = Exclude<State, 'cerrado'>

export interface Kpis {
  totalActive: number
  bySeverity: Record<Severity, number>
  byState: Record<ActiveState, number>
  escalated: number
  unassigned: number
  assignedToMe: number
  byService: Record<string, number>
  attention: Incident[]
}

const severityRank: Record<Severity, number> = { SEV1: 0, SEV2: 1, SEV3: 2 }

// UI-01 (docs/domain.md, "Criterios de presentación"): "Requieren atención" is a presentation criterion
// only, not a business rule: active incidents that are SEV1, escalated or unassigned.
function needsAttention(inc: Incident): boolean {
  return inc.severity === 'SEV1' || inc.escalated_at !== null || inc.assigned_to === null
}

// Most urgent first: SEV1, then escalated, then oldest declared_at.
function compareUrgency(a: Incident, b: Incident): number {
  const sev1 = Number(b.severity === 'SEV1') - Number(a.severity === 'SEV1')
  if (sev1 !== 0) return sev1
  const esc = Number(b.escalated_at !== null) - Number(a.escalated_at !== null)
  if (esc !== 0) return esc
  const byAge = Date.parse(a.declared_at) - Date.parse(b.declared_at)
  if (byAge !== 0) return byAge
  return severityRank[a.severity] - severityRank[b.severity]
}

// Pure counting over the incidents shown on the board (BR-15: active means state is not "cerrado").
export function computeKpis(incidents: Incident[], userId: string): Kpis {
  const active = incidents.filter((inc): inc is Incident & { state: ActiveState } => inc.state !== 'cerrado')
  const kpis: Kpis = {
    totalActive: active.length,
    bySeverity: { SEV1: 0, SEV2: 0, SEV3: 0 },
    byState: { declarado: 0, reconocido: 0, mitigando: 0, resuelto: 0 },
    escalated: 0,
    unassigned: 0,
    assignedToMe: 0,
    byService: {},
    attention: [],
  }
  for (const inc of active) {
    kpis.bySeverity[inc.severity] += 1
    kpis.byState[inc.state] += 1
    if (inc.escalated_at !== null) kpis.escalated += 1
    if (inc.assigned_to === null) kpis.unassigned += 1
    if (inc.assigned_to === userId) kpis.assignedToMe += 1
    kpis.byService[inc.service_id] = (kpis.byService[inc.service_id] ?? 0) + 1
  }
  kpis.attention = active.filter(needsAttention).sort(compareUrgency)
  return kpis
}
