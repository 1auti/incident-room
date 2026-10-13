import { useEffect, useMemo, useState } from 'react'
import { getDashboard, type Dashboard, type MetricsPeriod } from '../../api/metrics'

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
    <section data-testid="metrics-dashboard">
      <h2>Métricas</h2>
      <label>
        Período
        <select data-testid="metrics-period-select" value={mode} onChange={(e) => setMode(e.target.value as Mode)}>
          <option value="30d">Últimos 30 días</option>
          <option value="7d">Últimos 7 días</option>
          <option value="range">Rango libre</option>
        </select>
      </label>
      {mode === 'range' && (
        <span>
          <label>
            Desde
            <input
              type="date"
              data-testid="metrics-range-from"
              value={fromDate}
              onChange={(e) => setFromDate(e.target.value)}
            />
          </label>
          <label>
            Hasta (incluido)
            <input
              type="date"
              data-testid="metrics-range-to"
              value={toDate}
              onChange={(e) => setToDate(e.target.value)}
            />
          </label>
        </span>
      )}
      {period !== null && current === null && <p role="status">Cargando...</p>}
      {current?.error && (
        <p role="alert" data-testid="metrics-error">
          {current.error}
        </p>
      )}
      {dashboard && (
        <div>
          <p>
            Período (UTC): <time data-testid="metrics-period-from">{dashboard.from}</time> a{' '}
            <time data-testid="metrics-period-to">{dashboard.to}</time>
          </p>
          <p>
            MTTR:{' '}
            <span data-testid="metrics-mttr">
              {dashboard.mttr_seconds === null ? 'Sin datos' : formatMinutes(dashboard.mttr_seconds)}
            </span>
          </p>
          <p>
            Cumplimiento de SLA:{' '}
            <span data-testid="metrics-sla">
              {dashboard.sla_compliance_percent === null
                ? 'Sin datos'
                : formatPercent(dashboard.sla_compliance_percent)}
            </span>
          </p>
          <table>
            <thead>
              <tr>
                <th>Servicio</th>
                <th>Incidentes</th>
              </tr>
            </thead>
            <tbody>
              {dashboard.incidents_by_service.map((s) => (
                <tr key={s.service_id} data-testid="metrics-service-row" data-service-name={s.name}>
                  <td>{s.name}</td>
                  <td data-testid="metrics-service-count">{s.count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
