import { useEffect, useState } from 'react'
import { listIncidents, type Incident, type Severity, type State } from '../../api/incidents'
import { listServices, type Service } from '../../api/services'
import { Alert } from '../../ui/Alert'
import { Badge } from '../../ui/Badge'
import { Button } from '../../ui/Button'
import { Card } from '../../ui/Card'
import { Field, controlClass } from '../../ui/Field'
import { stateLabel } from '../labels'
import { severityTone, stateTone } from './tones'

const severities: Severity[] = ['SEV1', 'SEV2', 'SEV3']
// The board shows active incidents only (BR-15), so "cerrado" is not offered.
const activeStates: State[] = ['declarado', 'reconocido', 'mitigando', 'resuelto']

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

interface Loaded {
  key: string
  incidents: Incident[]
  error: string | null
}

interface Props {
  selectedId: string | null
  onOpen: (id: string) => void
}

export function IncidentBoard({ selectedId, onOpen }: Props) {
  const [services, setServices] = useState<Service[]>([])
  const [severity, setSeverity] = useState<Severity | ''>('')
  const [serviceId, setServiceId] = useState('')
  const [state, setState] = useState<State | ''>('')
  const [loaded, setLoaded] = useState<Loaded | null>(null)

  useEffect(() => {
    let active = true
    listServices()
      .then((list) => {
        if (active) setServices(list)
      })
      .catch(() => {
        // Without the catalog the service column falls back to the id; the board error covers the rest.
      })
    return () => {
      active = false
    }
  }, [])

  // The backend applies the filters (BR-15); every change asks again.
  const key = JSON.stringify([severity, serviceId, state])
  useEffect(() => {
    let active = true
    listIncidents({ severity, service_id: serviceId, state })
      .then((incidents) => {
        if (active) setLoaded({ key, incidents, error: null })
      })
      .catch((err: unknown) => {
        if (active) setLoaded({ key, incidents: [], error: messageOf(err) })
      })
    return () => {
      active = false
    }
  }, [key, severity, serviceId, state])

  const current = loaded?.key === key ? loaded : null
  const serviceName = (id: string) => services.find((s) => s.id === id)?.name ?? id

  return (
    <Card data-testid="incident-board" title="Tablero de incidentes">
      <div className="mb-3 flex flex-wrap gap-3">
        <Field>
          Severidad
          <select
            data-testid="board-filter-severity"
            value={severity}
            onChange={(e) => setSeverity(e.target.value as Severity | '')}
            className={controlClass}
          >
            <option value="">Todos</option>
            {severities.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </Field>
        <Field>
          Servicio
          <select
            data-testid="board-filter-service"
            value={serviceId}
            onChange={(e) => setServiceId(e.target.value)}
            className={controlClass}
          >
            <option value="">Todos</option>
            {services.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </Field>
        <Field>
          Estado
          <select
            data-testid="board-filter-state"
            value={state}
            onChange={(e) => setState(e.target.value as State | '')}
            className={controlClass}
          >
            <option value="">Todos</option>
            {activeStates.map((s) => (
              <option key={s} value={s}>
                {stateLabel[s]}
              </option>
            ))}
          </select>
        </Field>
      </div>
      {current === null && <p role="status">Cargando...</p>}
      {current?.error && (
        <Alert role="alert" data-testid="board-error">
          {current.error}
        </Alert>
      )}
      {current && current.error === null && current.incidents.length === 0 && (
        <p role="status" data-testid="board-empty" className="text-sm text-slate-600 dark:text-slate-300">
          No hay incidentes activos que coincidan.
        </p>
      )}
      {current && current.incidents.length > 0 && (
        <ul className="flex flex-col gap-2">
          {current.incidents.map((inc) => (
            <li
              key={inc.id}
              data-testid="board-incident-row"
              data-incident-id={inc.id}
              aria-current={inc.id === selectedId ? 'true' : undefined}
              className={`flex min-h-16 items-center justify-between gap-3 rounded-md border px-3 py-2 ${
                inc.id === selectedId ? 'border-blue-500 bg-blue-950/40 ring-1 ring-blue-500/60' : 'border-slate-700'
              }`}
            >
              <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                <strong className="min-w-0 text-slate-100 [overflow-wrap:anywhere]">{inc.title}</strong>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge tone={severityTone[inc.severity]} data-testid="board-incident-severity">
                    {inc.severity}
                  </Badge>
                  <Badge tone={stateTone[inc.state]} title={stateLabel[inc.state]} data-testid="board-incident-state">
                    {inc.state}
                  </Badge>
                  {inc.escalated_at !== null && (
                    <Badge tone="danger" data-testid="board-incident-escalated">
                      Escalado
                    </Badge>
                  )}
                  <span data-testid="board-incident-service" className="min-w-0 text-sm break-words text-slate-400">
                    {serviceName(inc.service_id)}
                  </span>
                </div>
              </div>
              <Button
                type="button"
                variant="secondary"
                data-testid="board-incident-open"
                onClick={() => onOpen(inc.id)}
              >
                Ver detalle
              </Button>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
