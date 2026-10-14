import { useEffect, useMemo, useState } from 'react'
import type { User } from '../../api/auth'
import { listIncidents, type Incident, type Severity, type State } from '../../api/incidents'
import { getDashboard, type Dashboard } from '../../api/metrics'
import { listServices, type Service } from '../../api/services'
import { Alert } from '../../ui/Alert'
import { Badge, type BadgeTone } from '../../ui/Badge'
import { Button } from '../../ui/Button'
import { Card } from '../../ui/Card'
import { severityTone, stateTone } from '../incidents/tones'
import { computeKpis } from './kpis'

interface Props {
  user: User
  onOpenIncident: (id: string) => void
}

interface Data {
  incidents: Incident[]
  dashboard: Dashboard
  services: Service[]
}

const severities: Severity[] = ['SEV1', 'SEV2', 'SEV3']
const states: State[] = ['declarado', 'reconocido', 'mitigando', 'resuelto']

const barColor: Record<BadgeTone, string> = {
  neutral: 'bg-slate-500',
  info: 'bg-blue-500',
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  danger: 'bg-red-500',
}

const valueColor: Record<BadgeTone | 'accent', string> = {
  neutral: 'text-slate-100',
  info: 'text-blue-300',
  success: 'text-emerald-300',
  warning: 'text-amber-300',
  danger: 'text-red-300',
  accent: 'text-blue-400',
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

// Same formatting and "no data" convention as MetricsDashboard (null means no value, never 0).
function formatMinutes(seconds: number): string {
  return `${Number((seconds / 60).toFixed(1))} min`
}

function formatPercent(pct: number): string {
  return `${Number(pct.toFixed(1))} %`
}

function KpiCard({
  label,
  value,
  testId,
  tone,
}: {
  label: string
  value: string
  testId: string
  tone: BadgeTone | 'accent'
}) {
  // Non-numeric text ("Sin datos") is shown smaller and neutral so it never reads as a good value.
  const numeric = /^\d/.test(value)
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-900 p-4 shadow-sm">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-400">{label}</p>
      <p
        data-testid={testId}
        className={`mt-2 flex h-10 items-center tabular-nums ${
          numeric ? `text-4xl font-semibold ${valueColor[tone]}` : 'text-base font-medium text-slate-400'
        }`}
      >
        {value}
      </p>
    </div>
  )
}

interface BarRow {
  key: string
  label: string
  count: number
  tone: BadgeTone
}

function BarList({ rows, testId, empty }: { rows: BarRow[]; testId: string; empty: string }) {
  const max = Math.max(1, ...rows.map((r) => r.count))
  return (
    <div data-testid={testId} className="flex flex-col gap-2">
      {rows.length === 0 && <p className="text-sm text-slate-400">{empty}</p>}
      {rows.map((r) => (
        <div key={r.key} data-bar-key={r.key} className="flex items-center gap-3 text-sm">
          <span title={r.label} className="w-28 shrink-0 truncate text-slate-300">
            {r.label}
          </span>
          <div className="h-2.5 flex-1 overflow-hidden rounded-full bg-slate-800">
            <div className={`h-full rounded-full ${barColor[r.tone]}`} style={{ width: `${(r.count / max) * 100}%` }} />
          </div>
          <span data-testid={`${testId}-count`} className="w-8 text-right font-semibold tabular-nums text-slate-100">
            {r.count}
          </span>
        </div>
      ))}
    </div>
  )
}

export function KpiDashboard({ user, onOpenIncident }: Props) {
  const [data, setData] = useState<Data | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    Promise.all([listIncidents(), getDashboard({ preset: '30d' }), listServices()])
      .then(([incidents, dashboard, services]) => {
        if (active) setData({ incidents, dashboard, services })
      })
      .catch((err: unknown) => {
        if (active) setError(messageOf(err))
      })
    return () => {
      active = false
    }
  }, [])

  const kpis = useMemo(() => (data ? computeKpis(data.incidents, user.id) : null), [data, user.id])

  if (error !== null) {
    return (
      <div data-testid="kpi-dashboard">
        <Alert role="alert" data-testid="kpi-error">
          {error}
        </Alert>
      </div>
    )
  }
  if (data === null || kpis === null) {
    return (
      <div data-testid="kpi-dashboard">
        <Alert tone="status" role="status" data-testid="kpi-loading">
          Cargando...
        </Alert>
      </div>
    )
  }

  const { dashboard, services } = data
  const serviceName = new Map(services.map((s) => [s.id, s.name]))
  const serviceRows: BarRow[] = Object.entries(kpis.byService)
    .map(([id, count]) => ({ key: id, label: serviceName.get(id) ?? id, count, tone: 'info' as const }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label))
  const severityRows: BarRow[] = severities.map((s) => ({
    key: s,
    label: s,
    count: kpis.bySeverity[s],
    tone: severityTone[s],
  }))
  const stateRows: BarRow[] = states.map((s) => ({ key: s, label: s, count: kpis.byState[s], tone: stateTone[s] }))

  return (
    <div data-testid="kpi-dashboard" className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        <KpiCard label="Incidentes activos" value={String(kpis.totalActive)} testId="kpi-active" tone="accent" />
        <KpiCard label="SEV1" value={String(kpis.bySeverity.SEV1)} testId="kpi-sev1" tone="danger" />
        <KpiCard label="Escalados" value={String(kpis.escalated)} testId="kpi-escalated" tone="warning" />
        <KpiCard label="Sin asignar" value={String(kpis.unassigned)} testId="kpi-unassigned" tone="neutral" />
        <KpiCard
          label="MTTR"
          value={dashboard.mttr_seconds === null ? 'Sin datos' : formatMinutes(dashboard.mttr_seconds)}
          testId="kpi-mttr"
          tone="neutral"
        />
        <KpiCard
          label="SLA"
          value={
            dashboard.sla_compliance_percent === null ? 'Sin datos' : formatPercent(dashboard.sla_compliance_percent)
          }
          testId="kpi-sla"
          tone="success"
        />
      </div>
      <div className="grid gap-4 lg:grid-cols-3">
        <Card title="Por severidad">
          <BarList rows={severityRows} testId="kpi-by-severity" empty="Sin incidentes activos" />
        </Card>
        <Card title="Por estado">
          <BarList rows={stateRows} testId="kpi-by-state" empty="Sin incidentes activos" />
        </Card>
        <Card title="Por servicio">
          <BarList rows={serviceRows} testId="kpi-by-service" empty="Sin incidentes activos" />
        </Card>
      </div>
      <Card title="Requieren atención">
        {kpis.attention.length === 0 ? (
          <p className="text-sm text-slate-400">Nada requiere atención</p>
        ) : (
          <ul className="flex flex-col divide-y divide-slate-800">
            {kpis.attention.map((inc) => (
              <li
                key={inc.id}
                data-testid="kpi-attention-row"
                data-incident-id={inc.id}
                className="grid gap-2 py-3 sm:grid-cols-[auto_minmax(0,1fr)_auto_auto] sm:items-center sm:gap-3 sm:py-2"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <Badge tone={severityTone[inc.severity]}>{inc.severity}</Badge>
                  <Badge tone={stateTone[inc.state]}>{inc.state}</Badge>
                </div>
                <span className="order-first min-w-0 text-sm break-words text-slate-100 sm:order-none sm:truncate">
                  {inc.title}
                </span>
                <div className="flex items-center justify-between gap-3 sm:contents">
                  <span className="min-w-0 truncate text-xs text-slate-400">
                    {serviceName.get(inc.service_id) ?? inc.service_id}
                  </span>
                  <Button variant="secondary" data-testid="kpi-attention-open" onClick={() => onOpenIncident(inc.id)}>
                    Abrir
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  )
}
