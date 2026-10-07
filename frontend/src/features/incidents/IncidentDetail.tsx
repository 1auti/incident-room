import { useEffect, useState } from 'react'
import { getIncident, transitionIncident, type Incident } from '../../api/incidents'

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
            Severidad: <span data-testid="incident-detail-severity">{incident.severity}</span>
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
        </div>
      )}
    </section>
  )
}
