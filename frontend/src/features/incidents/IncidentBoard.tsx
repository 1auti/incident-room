import { useEffect, useState } from 'react'
import { listIncidents, type Incident, type Severity, type State } from '../../api/incidents'
import { listServices, type Service } from '../../api/services'

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
  onOpen: (id: string) => void
}

export function IncidentBoard({ onOpen }: Props) {
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
    <section data-testid="incident-board">
      <h2>Tablero de incidentes</h2>
      <label>
        Severidad
        <select
          data-testid="board-filter-severity"
          value={severity}
          onChange={(e) => setSeverity(e.target.value as Severity | '')}
        >
          <option value="">Todos</option>
          {severities.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </label>
      <label>
        Servicio
        <select data-testid="board-filter-service" value={serviceId} onChange={(e) => setServiceId(e.target.value)}>
          <option value="">Todos</option>
          {services.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Estado
        <select data-testid="board-filter-state" value={state} onChange={(e) => setState(e.target.value as State | '')}>
          <option value="">Todos</option>
          {activeStates.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </label>
      {current === null && <p role="status">Cargando...</p>}
      {current?.error && (
        <p role="alert" data-testid="board-error">
          {current.error}
        </p>
      )}
      {current && current.error === null && current.incidents.length === 0 && (
        <p role="status" data-testid="board-empty">No hay incidentes activos que coincidan.</p>
      )}
      {current && current.incidents.length > 0 && (
        <ul>
          {current.incidents.map((inc) => (
            <li key={inc.id} data-testid="board-incident-row" data-incident-id={inc.id}>
              <strong>{inc.title}</strong>{' '}
              <span data-testid="board-incident-severity">{inc.severity}</span>{' '}
              <span data-testid="board-incident-state">{inc.state}</span>{' '}
              <span data-testid="board-incident-service">{serviceName(inc.service_id)}</span>
              {' '}
              {inc.escalated_at !== null && <strong data-testid="board-incident-escalated">Escalado</strong>}
              {' '}
              <button type="button" data-testid="board-incident-open" onClick={() => onOpen(inc.id)}>
                Ver detalle
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
