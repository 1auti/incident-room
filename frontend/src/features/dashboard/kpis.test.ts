import { describe, expect, it } from 'vitest'
import type { Incident } from '../../api/incidents'
import { computeKpis } from './kpis'

function make(over: Partial<Incident>): Incident {
  return {
    id: 'i',
    title: 't',
    description: '',
    service_id: 's1',
    impact: 'menor',
    suggested_severity: 'SEV3',
    severity: 'SEV3',
    state: 'declarado',
    declared_by: 'u0',
    assigned_to: 'u1',
    declared_at: '2026-01-02T00:00:00Z',
    acknowledged_at: null,
    escalated_at: null,
    root_cause: null,
    resolved_at: null,
    closed_at: null,
    ...over,
  }
}

describe('computeKpis', () => {
  it('returns zeros for no incidents', () => {
    const k = computeKpis([], 'u1')
    expect(k.totalActive).toBe(0)
    expect(k.bySeverity).toEqual({ SEV1: 0, SEV2: 0, SEV3: 0 })
    expect(k.attention).toEqual([])
    expect(k.byService).toEqual({})
  })

  it('counts by severity, state, service, escalated, unassigned and mine', () => {
    const k = computeKpis(
      [
        make({ id: 'a', severity: 'SEV1', state: 'reconocido', escalated_at: '2026-01-03T00:00:00Z' }),
        make({ id: 'b', severity: 'SEV2', assigned_to: null, service_id: 's2' }),
        make({ id: 'c', severity: 'SEV3', assigned_to: 'u2', state: 'resuelto' }),
        make({ id: 'd', state: 'cerrado' }),
      ],
      'u1',
    )
    expect(k.totalActive).toBe(3)
    expect(k.bySeverity).toEqual({ SEV1: 1, SEV2: 1, SEV3: 1 })
    expect(k.byState).toMatchObject({ declarado: 1, reconocido: 1, resuelto: 1, cerrado: 0 })
    expect(k.escalated).toBe(1)
    expect(k.unassigned).toBe(1)
    expect(k.assignedToMe).toBe(1)
    expect(k.byService).toEqual({ s1: 2, s2: 1 })
  })

  it('orders attention incidents: SEV1, escalated, then oldest', () => {
    const k = computeKpis(
      [
        make({ id: 'old-unassigned', assigned_to: null, declared_at: '2026-01-01T00:00:00Z' }),
        make({ id: 'new-unassigned', assigned_to: null, declared_at: '2026-01-05T00:00:00Z' }),
        make({ id: 'escalated', escalated_at: '2026-01-06T00:00:00Z', declared_at: '2026-01-04T00:00:00Z' }),
        make({ id: 'sev1', severity: 'SEV1', declared_at: '2026-01-07T00:00:00Z' }),
        make({ id: 'fine' }),
      ],
      'u1',
    )
    expect(k.attention.map((i) => i.id)).toEqual(['sev1', 'escalated', 'old-unassigned', 'new-unassigned'])
  })
})
