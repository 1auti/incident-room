import { useEffect, useState, type FormEvent } from 'react'
import { declareIncident, suggestSeverity, type DeclareResult, type Impact, type Severity } from '../../api/incidents'
import { listServices, type Service } from '../../api/services'
import { Alert } from '../../ui/Alert'
import { Badge } from '../../ui/Badge'
import { Button } from '../../ui/Button'
import { Field, controlClass } from '../../ui/Field'
import { eventTypeText, impactLabel, stateLabel } from '../labels'
import { severityTone, stateTone } from './tones'

const impacts: Impact[] = ['caida_total', 'degradacion', 'menor']
const severities: Severity[] = ['SEV1', 'SEV2', 'SEV3']

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

export function DeclareIncidentForm({ onDeclared }: { onDeclared?: () => void }) {
  const [services, setServices] = useState<Service[]>([])
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [serviceId, setServiceId] = useState('')
  const [impact, setImpact] = useState<Impact | ''>('')
  const [suggested, setSuggested] = useState<Severity | ''>('')
  const [severity, setSeverity] = useState<Severity | ''>('')
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<DeclareResult | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let active = true
    listServices()
      .then((list) => {
        if (active) setServices(list)
      })
      .catch((err: unknown) => {
        if (active) setError(messageOf(err))
      })
    return () => {
      active = false
    }
  }, [])

  // The suggestion is computed by the backend (BR-01); the UI only shows it and preselects it.
  useEffect(() => {
    if (serviceId === '' || impact === '') return
    let active = true
    suggestSeverity(serviceId, impact)
      .then((sev) => {
        if (!active) return
        setSuggested(sev)
        setSeverity(sev)
      })
      .catch((err: unknown) => {
        if (active) setError(messageOf(err))
      })
    return () => {
      active = false
    }
  }, [serviceId, impact])

  function changeService(id: string) {
    setServiceId(id)
    setSuggested('')
    setSeverity('')
  }

  function changeImpact(value: Impact | '') {
    setImpact(value)
    setSuggested('')
    setSeverity('')
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setResult(null)
    setSubmitting(true)
    try {
      setResult(await declareIncident({ title, description, service_id: serviceId, impact, severity }))
      onDeclared?.()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setSubmitting(false)
    }
  }

  // Once declared, the form stays blocked until the user asks for another one: no accidental duplicates.
  function declareAnother() {
    setTitle('')
    setDescription('')
    setServiceId('')
    setImpact('')
    setSuggested('')
    setSeverity('')
    setError(null)
    setResult(null)
  }

  const declared = result?.incident

  return (
    <div data-testid="declare-incident-panel">
      <form
        data-testid="declare-incident-form"
        onSubmit={(e) => void submit(e)}
        className="flex min-w-0 flex-col gap-3"
      >
        <Field>
          Título
          <input
            data-testid="incident-title-input"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className={controlClass}
          />
        </Field>
        <Field>
          Descripción
          <textarea
            data-testid="incident-description-input"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className={controlClass}
          />
        </Field>
        <Field>
          Servicio
          <select
            data-testid="incident-service-select"
            value={serviceId}
            onChange={(e) => changeService(e.target.value)}
            className={controlClass}
          >
            <option value="">Elegí un servicio</option>
            {services.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </Field>
        <Field>
          Impacto
          <select
            data-testid="incident-impact-select"
            value={impact}
            onChange={(e) => changeImpact(e.target.value as Impact | '')}
            className={controlClass}
          >
            <option value="">Elegí un impacto</option>
            {impacts.map((i) => (
              <option key={i} value={i}>
                {impactLabel[i]}
              </option>
            ))}
          </select>
        </Field>
        <p hidden={suggested === ''} className="text-sm">
          Severidad sugerida:{' '}
          <span data-testid="incident-suggested-severity" className="font-semibold">
            {suggested}
          </span>
        </p>
        <Field>
          Severidad
          <select
            data-testid="incident-severity-select"
            value={severity}
            onChange={(e) => setSeverity(e.target.value as Severity | '')}
            className={controlClass}
          >
            <option value="">Usar la sugerida</option>
            {severities.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </Field>
        <Button data-testid="incident-declare-submit" type="submit" disabled={submitting || result !== null}>
          Declarar incidente
        </Button>
      </form>
      {error && (
        <Alert role="alert" data-testid="declare-error" className="mt-3">
          {error}
        </Alert>
      )}
      {result && declared && (
        <div
          data-testid="declared-incident"
          className="mt-4 flex min-w-0 flex-col gap-2 rounded-md border border-emerald-800 bg-emerald-950/30 p-3 text-sm"
        >
          <p className="text-xs font-semibold tracking-wide text-emerald-300 uppercase">Incidente declarado</p>
          <h3 className="text-base [overflow-wrap:anywhere]">{declared.title}</h3>
          <p className="flex items-center gap-2">
            <span className="w-24 shrink-0 text-slate-400">Estado:</span>
            <Badge tone={stateTone[declared.state]} title={stateLabel[declared.state]} data-testid="incident-state">
              {declared.state}
            </Badge>
          </p>
          <p className="flex items-center gap-2">
            <span className="w-24 shrink-0 text-slate-400">Impacto:</span>
            <span>{impactLabel[declared.impact]}</span>
          </p>
          <p className="flex items-center gap-2">
            <span className="w-24 shrink-0 text-slate-400">Severidad:</span>
            <Badge tone={severityTone[declared.severity]} data-testid="incident-severity">
              {declared.severity}
            </Badge>
          </p>
          <p className="flex items-center gap-2">
            <span className="w-24 shrink-0 text-slate-400">Asignado a:</span>
            <span data-testid="incident-assigned-to">{declared.assigned_to ?? 'Sin asignar'}</span>
          </p>
          <ul className="list-disc pl-5 text-slate-300">
            {result.timeline.map((ev) => (
              <li key={ev.id} data-testid="timeline-event" data-event-type={ev.type}>
                {eventTypeText(ev.type)}
              </li>
            ))}
          </ul>
          <Button
            type="button"
            variant="secondary"
            data-testid="declare-another"
            className="self-start"
            onClick={declareAnother}
          >
            Declarar otro
          </Button>
        </div>
      )}
    </div>
  )
}
