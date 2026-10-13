import { useEffect, useState, type FormEvent } from 'react'
import { declareIncident, suggestSeverity, type DeclareResult, type Impact, type Severity } from '../../api/incidents'
import { listServices, type Service } from '../../api/services'
import { Alert } from '../../ui/Alert'
import { Badge } from '../../ui/Badge'
import { Button } from '../../ui/Button'
import { Card } from '../../ui/Card'
import { Field, controlClass } from '../../ui/Field'
import { severityTone, stateTone } from './tones'

const impacts: Impact[] = ['caida_total', 'degradacion', 'menor']
const severities: Severity[] = ['SEV1', 'SEV2', 'SEV3']

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

export function DeclareIncidentForm() {
  const [services, setServices] = useState<Service[]>([])
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [serviceId, setServiceId] = useState('')
  const [impact, setImpact] = useState<Impact | ''>('')
  const [suggested, setSuggested] = useState<Severity | ''>('')
  const [severity, setSeverity] = useState<Severity | ''>('')
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<DeclareResult | null>(null)

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
    try {
      setResult(await declareIncident({ title, description, service_id: serviceId, impact, severity }))
    } catch (err) {
      setError(messageOf(err))
    }
  }

  const declared = result?.incident

  return (
    <Card title="Declarar incidente">
      <form data-testid="declare-incident-form" onSubmit={(e) => void submit(e)} className="flex flex-col gap-3">
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
                {i}
              </option>
            ))}
          </select>
        </Field>
        <p className="text-sm">
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
        <Button data-testid="incident-declare-submit" type="submit">
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
          className="mt-3 flex flex-col gap-1 border-t border-slate-200 pt-3 dark:border-slate-700"
        >
          <h3>{declared.title}</h3>
          <p>
            Estado:{' '}
            <Badge tone={stateTone[declared.state]} data-testid="incident-state">
              {declared.state}
            </Badge>
          </p>
          <p>
            Severidad:{' '}
            <Badge tone={severityTone[declared.severity]} data-testid="incident-severity">
              {declared.severity}
            </Badge>
          </p>
          <p>
            Asignado a: <span data-testid="incident-assigned-to">{declared.assigned_to ?? 'Sin asignar'}</span>
          </p>
          <ul className="list-disc pl-5 text-sm">
            {result.timeline.map((ev) => (
              <li key={ev.id} data-testid="timeline-event" data-event-type={ev.type}>
                {ev.type}
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  )
}
