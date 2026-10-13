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
          <p className="text-sm text-slate-600 dark:text-slate-300">
            Período (UTC): <time data-testid="metrics-period-from">{dashboard.from}</time> a{' '}
            <time data-testid="metrics-period-to">{dashboard.to}</time>
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            <p className="rounded-md border border-slate-200 p-3 text-sm dark:border-slate-700">
              MTTR:{' '}
              <span data-testid="metrics-mttr" className="block text-2xl font-semibold">
                {dashboard.mttr_seconds === null ? 'Sin datos' : formatMinutes(dashboard.mttr_seconds)}
              </span>
            </p>
            <p className="rounded-md border border-slate-200 p-3 text-sm dark:border-slate-700">
              Cumplimiento de SLA:{' '}
              <span data-testid="metrics-sla" className="block text-2xl font-semibold">
                {dashboard.sla_compliance_percent === null
                  ? 'Sin datos'
                  : formatPercent(dashboard.sla_compliance_percent)}
              </span>
            </p>
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
