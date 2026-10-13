import { useEffect, useState } from 'react'
import { changeSeverity, getIncident, transitionIncident, type Incident, type Severity } from '../../api/incidents'
import { IncidentTimeline } from './IncidentTimeline'

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

  // The backend decides who may acknowledge (BR-11); a refusal is shown as is.
  async function acknowledge() {
    setActionError(null)
    try {
      const updated = await transitionIncident(incidentId, 'reconocido')
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
          {incident.state === 'declarado' && (
            <button type="button" data-testid="incident-acknowledge" onClick={() => void acknowledge()}>
              Acknowledge
            </button>
          )}
          <IncidentTimeline incidentId={incidentId} reloadToken={timelineReloads} />
        </div>
      )}
    </section>
  )
}
