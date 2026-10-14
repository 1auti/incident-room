import { useEffect, useRef, useState } from 'react'
import {
  changeSeverity,
  getIncident,
  transitionIncident,
  type Incident,
  type Severity,
  type State,
} from '../../api/incidents'
import { Alert } from '../../ui/Alert'
import { Badge } from '../../ui/Badge'
import { Button } from '../../ui/Button'
import { Card } from '../../ui/Card'
import { Field, controlClass } from '../../ui/Field'
import { IncidentTimeline } from './IncidentTimeline'
import { PostmortemSection } from './PostmortemSection'
import { severityTone, stateTone } from './tones'

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
  const rootRef = useRef<HTMLDivElement>(null)

  // Stacked layout (below lg): bring the freshly opened detail into view.
  useEffect(() => {
    rootRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }, [incidentId])

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
    <div ref={rootRef} className="lg:sticky lg:top-20 lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto">
      <Card data-testid="incident-detail">
        <div className="mb-3 flex items-center justify-between gap-3">
          <h2>Detalle del incidente</h2>
          <Button type="button" variant="secondary" data-testid="incident-detail-close" onClick={onClose}>
            Cerrar detalle
          </Button>
        </div>
        {current === null && <p role="status">Cargando...</p>}
        {current?.error && (
          <Alert role="alert" data-testid="incident-detail-error" className="mt-3">
            {current.error}
          </Alert>
        )}
        {actionError && (
          <Alert role="alert" data-testid="incident-detail-error" className="mt-3">
            {actionError}
          </Alert>
        )}
        {incident && (
          <div className="mt-3 flex flex-col gap-3 text-sm">
            <h3 data-testid="incident-detail-title" className="text-base">
              {incident.title}
            </h3>
            <p className="flex items-center gap-2">
              <span className="w-24 shrink-0 text-slate-400">Estado:</span>
              <Badge tone={stateTone[incident.state]} data-testid="incident-detail-state">
                {incident.state}
              </Badge>
            </p>
            <p className="flex items-center gap-2">
              <span className="w-24 shrink-0 text-slate-400">Severidad:</span>
              <Badge tone={severityTone[incident.severity]} data-testid="incident-detail-severity">
                {incident.severity}
              </Badge>
            </p>
            <Field className="flex-row items-center gap-2 rounded-md border border-slate-800 bg-slate-950/40 px-3 py-2">
              <span className="font-normal text-slate-400">Cambiar severidad</span>
              <select
                data-testid="incident-detail-severity-select"
                value={incident.severity}
                onChange={(e) => void updateSeverity(e.target.value as Severity)}
                className={`${controlClass} ml-auto`}
              >
                <option value="SEV1">SEV1</option>
                <option value="SEV2">SEV2</option>
                <option value="SEV3">SEV3</option>
              </select>
            </Field>
            <p className="flex items-center gap-2">
              <span className="w-24 shrink-0 text-slate-400">Asignado a:</span>
              <span data-testid="incident-detail-assigned-to">{incident.assigned_to ?? 'Sin asignar'}</span>
            </p>
            {incident.escalated_at !== null && (
              <div>
                <Badge tone="danger" data-testid="incident-detail-escalated">
                  Escalado
                </Badge>
              </div>
            )}
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
              <Button
                type="button"
                className="self-start"
                data-testid="incident-acknowledge"
                onClick={() => void transition('reconocido')}
              >
                Acknowledge
              </Button>
            )}
            {incident.state === 'reconocido' && (
              <Button
                type="button"
                className="self-start"
                data-testid="incident-mitigate"
                onClick={() => void transition('mitigando')}
              >
                Mitigar
              </Button>
            )}
            {incident.state === 'mitigando' && (
              <div className="flex flex-col gap-2">
                <Field>
                  Causa raíz
                  <textarea
                    data-testid="incident-root-cause-input"
                    value={rootCause}
                    onChange={(e) => setRootCause(e.target.value)}
                    className={controlClass}
                  />
                </Field>
                <Button
                  type="button"
                  className="self-start"
                  data-testid="incident-resolve"
                  onClick={() => void transition('resuelto')}
                >
                  Resolver
                </Button>
              </div>
            )}
            {incident.state === 'resuelto' && (
              <Button
                type="button"
                className="self-start"
                data-testid="incident-close"
                onClick={() => void transition('cerrado')}
              >
                Cerrar incidente
              </Button>
            )}
            <PostmortemSection incidentId={incidentId} onEvent={() => setTimelineReloads((n) => n + 1)} />
            <IncidentTimeline incidentId={incidentId} reloadToken={timelineReloads} />
          </div>
        )}
      </Card>
    </div>
  )
}
