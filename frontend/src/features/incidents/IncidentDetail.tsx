import { useEffect, useState } from 'react'
import {
  changeSeverity,
  getIncident,
  transitionIncident,
  type Incident,
  type Severity,
  type State,
} from '../../api/incidents'
import { IncidentTimeline } from './IncidentTimeline'
import { PostmortemSection } from './PostmortemSection'

interface Props {
  incidentId: string
  onClose: () => void
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

interface Loaded {
  id: string
  incident: Incident | null
  error: string | null
}

export function IncidentDetail({ incidentId, onClose }: Props) {
  const [loaded, setLoaded] = useState<Loaded | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  // Bumped after every change to the incident so the timeline shows its new event.
  const [timelineReloads, setTimelineReloads] = useState(0)
  const [rootCause, setRootCause] = useState('')

  useEffect(() => {
    let active = true
    getIncident(incidentId)
      .then((incident) => {
        if (active) setLoaded({ id: incidentId, incident, error: null })
      })
      .catch((err: unknown) => {
        if (active) setLoaded({ id: incidentId, incident: null, error: messageOf(err) })
      })
    return () => {
      active = false
    }
  }, [incidentId])

  const current = loaded?.id === incidentId ? loaded : null
  const incident = current?.incident ?? null

  // The backend decides who may transition and its preconditions (BR-06, BR-07, BR-08, BR-11);
  // a refusal is shown as is.
  async function transition(to: State) {
    setActionError(null)
    try {
      const updated = await transitionIncident(incidentId, to, to === 'resuelto' ? rootCause : '')
      setLoaded({ id: incidentId, incident: updated, error: null })
      setTimelineReloads((n) => n + 1)
    } catch (err) {
      setActionError(messageOf(err))
    }
  }

  // The backend decides who may change the severity and until which state (BR-02, BR-11).
  async function updateSeverity(severity: Severity) {
    setActionError(null)
    try {
      const updated = await changeSeverity(incidentId, severity)
      setLoaded({ id: incidentId, incident: updated, error: null })
      setTimelineReloads((n) => n + 1)
    } catch (err) {
      setActionError(messageOf(err))
    }
  }

  return (
    <section data-testid="incident-detail">
      <h2>Detalle del incidente</h2>
      <button type="button" data-testid="incident-detail-close" onClick={onClose}>
        Cerrar detalle
      </button>
      {current === null && <p role="status">Cargando...</p>}
      {current?.error && (
        <p role="alert" data-testid="incident-detail-error">
          {current.error}
        </p>
      )}
      {actionError && (
        <p role="alert" data-testid="incident-detail-error">
          {actionError}
        </p>
      )}
      {incident && (
        <div>
          <h3 data-testid="incident-detail-title">{incident.title}</h3>
          <p>
            Estado: <span data-testid="incident-detail-state">{incident.state}</span>
          </p>
          <p>
            Severidad: <span data-testid="incident-detail-severity">{incident.severity}</span>{' '}
            <label>
              Cambiar severidad
              <select
                data-testid="incident-detail-severity-select"
                value={incident.severity}
                onChange={(e) => void updateSeverity(e.target.value as Severity)}
              >
                <option value="SEV1">SEV1</option>
                <option value="SEV2">SEV2</option>
                <option value="SEV3">SEV3</option>
              </select>
            </label>
          </p>
          <p>
            Asignado a: <span data-testid="incident-detail-assigned-to">{incident.assigned_to ?? 'Sin asignar'}</span>
          </p>
          {incident.escalated_at !== null && <strong data-testid="incident-detail-escalated">Escalado</strong>}
          {incident.root_cause !== null && (
            <p>
              Causa raíz: <span data-testid="incident-detail-root-cause">{incident.root_cause}</span>
            </p>
          )}
          {incident.resolved_at !== null && (
            <p>
              Resuelto: <time data-testid="incident-detail-resolved-at">{incident.resolved_at}</time>
            </p>
          )}
          {incident.closed_at !== null && (
            <p>
              Cerrado: <time data-testid="incident-detail-closed-at">{incident.closed_at}</time>
            </p>
          )}
          {/* One action per state: the flow is linear (T1-T4). */}
          {incident.state === 'declarado' && (
            <button type="button" data-testid="incident-acknowledge" onClick={() => void transition('reconocido')}>
              Acknowledge
            </button>
          )}
          {incident.state === 'reconocido' && (
            <button type="button" data-testid="incident-mitigate" onClick={() => void transition('mitigando')}>
              Mitigar
            </button>
          )}
          {incident.state === 'mitigando' && (
            <div>
              <label>
                Causa raíz
                <textarea
                  data-testid="incident-root-cause-input"
                  value={rootCause}
                  onChange={(e) => setRootCause(e.target.value)}
                />
              </label>
              <button type="button" data-testid="incident-resolve" onClick={() => void transition('resuelto')}>
                Resolver
              </button>
            </div>
          )}
          {incident.state === 'resuelto' && (
            <button type="button" data-testid="incident-close" onClick={() => void transition('cerrado')}>
              Cerrar incidente
            </button>
          )}
          <PostmortemSection incidentId={incidentId} onEvent={() => setTimelineReloads((n) => n + 1)} />
          <IncidentTimeline incidentId={incidentId} reloadToken={timelineReloads} />
        </div>
      )}
    </section>
  )
}
