import { useEffect, useMemo, useState } from 'react'
import { getDashboard, type Dashboard, type MetricsPeriod } from '../../api/metrics'
import { Alert } from '../../ui/Alert'
import { Card } from '../../ui/Card'
import { Field, controlClass } from '../../ui/Field'
import { tableClass, tdClass, thClass } from '../../ui/table'

type Mode = '30d' | '7d' | 'range'

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

// The range is picked as whole UTC days with an inclusive end, so "hasta" is sent as the next
// day at 00:00 to keep the backend's half-open [desde, hasta).
function rangeOf(fromDate: string, toDate: string): MetricsPeriod {
  const end = new Date(`${toDate}T00:00:00Z`)
  end.setUTCDate(end.getUTCDate() + 1)
  return { from: `${fromDate}T00:00:00Z`, to: end.toISOString().replace('.000Z', 'Z') }
}

function formatMinutes(seconds: number): string {
  return `${Number((seconds / 60).toFixed(1))} min`
}

function formatPercent(pct: number): string {
  return `${Number(pct.toFixed(1))} %`
}

const dayFormat = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })

function formatDay(iso: string): string {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? iso : dayFormat.format(date)
}

// The backend range is half-open: the shown end day is the last one included.
function formatLastDay(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return dayFormat.format(new Date(date.getTime() - 1))
}

function Tile({ label, testId, value }: { label: string; testId: string; value: string | null }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950/40 p-4">
      <p className="text-xs font-medium tracking-wide text-slate-400 uppercase">{label}</p>
      <p
        data-testid={testId}
        className={`mt-2 flex h-10 items-center tabular-nums ${
          value === null ? 'text-base font-medium text-slate-400' : 'text-3xl font-semibold text-slate-100'
        }`}
      >
        {value ?? 'Sin datos'}
      </p>
    </div>
  )
}

interface Loaded {
  key: string
  dashboard: Dashboard | null
  error: string | null
}

export function MetricsDashboard() {
  const [mode, setMode] = useState<Mode>('30d')
  const [fromDate, setFromDate] = useState('')
  const [toDate, setToDate] = useState('')
  const [loaded, setLoaded] = useState<Loaded | null>(null)

  // A free range waits until both days are chosen.
  const period = useMemo<MetricsPeriod | null>(() => {
    if (mode !== 'range') return { preset: mode }
    return fromDate && toDate ? rangeOf(fromDate, toDate) : null
  }, [mode, fromDate, toDate])
  const key = JSON.stringify(period)

  useEffect(() => {
    if (period === null) return
    let active = true
    getDashboard(period)
      .then((dashboard) => {
        if (active) setLoaded({ key, dashboard, error: null })
      })
      .catch((err: unknown) => {
        if (active) setLoaded({ key, dashboard: null, error: messageOf(err) })
      })
    return () => {
      active = false
    }
  }, [period, key])

  const current = loaded?.key === key ? loaded : null
  const dashboard = current?.dashboard ?? null

  return (
    <Card data-testid="metrics-dashboard" title="Métricas">
      <div className="mb-3 flex flex-wrap items-end gap-3">
        <Field>
          Período
          <select
            data-testid="metrics-period-select"
            value={mode}
            onChange={(e) => setMode(e.target.value as Mode)}
            className={controlClass}
          >
            <option value="30d">Últimos 30 días</option>
            <option value="7d">Últimos 7 días</option>
            <option value="range">Rango libre</option>
          </select>
        </Field>
        {mode === 'range' && (
          <span className="flex flex-wrap items-end gap-3">
            <Field>
              Desde
              <input
                type="date"
                data-testid="metrics-range-from"
                value={fromDate}
                onChange={(e) => setFromDate(e.target.value)}
                className={controlClass}
              />
            </Field>
            <Field>
              Hasta (incluido)
              <input
                type="date"
                data-testid="metrics-range-to"
                value={toDate}
                onChange={(e) => setToDate(e.target.value)}
                className={controlClass}
              />
            </Field>
          </span>
        )}
      </div>
      {period !== null && current === null && <p role="status">Cargando...</p>}
      {current?.error && (
        <Alert role="alert" data-testid="metrics-error">
          {current.error}
        </Alert>
      )}
      {dashboard && (
        <div className="flex flex-col gap-3">
          <div className="text-sm text-slate-300">
            <p>
              Período: <span className="font-medium text-slate-100">{formatDay(dashboard.from)}</span> al{' '}
              <span className="font-medium text-slate-100">{formatLastDay(dashboard.to)}</span>
            </p>
            <p className="text-xs text-slate-500">
              UTC: <time data-testid="metrics-period-from">{dashboard.from}</time> a{' '}
              <time data-testid="metrics-period-to">{dashboard.to}</time>
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Tile
              label="MTTR"
              testId="metrics-mttr"
              value={dashboard.mttr_seconds === null ? null : formatMinutes(dashboard.mttr_seconds)}
            />
            <Tile
              label="Cumplimiento de SLA"
              testId="metrics-sla"
              value={dashboard.sla_compliance_percent === null ? null : formatPercent(dashboard.sla_compliance_percent)}
            />
          </div>
          <table className={tableClass}>
            <thead>
              <tr>
                <th className={thClass}>Servicio</th>
                <th className={thClass}>Incidentes</th>
              </tr>
            </thead>
            <tbody>
              {dashboard.incidents_by_service.map((s) => (
                <tr key={s.service_id} data-testid="metrics-service-row" data-service-name={s.name}>
                  <td className={tdClass}>{s.name}</td>
                  <td data-testid="metrics-service-count" className={tdClass}>
                    {s.count}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  )
}
