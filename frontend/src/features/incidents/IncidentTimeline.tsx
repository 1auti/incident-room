import { useEffect, useState, type FormEvent } from 'react'
import { addNote, getTimeline, type TimelineEntry } from '../../api/incidents'
import { Alert } from '../../ui/Alert'
import { Button } from '../../ui/Button'
import { Field, controlClass } from '../../ui/Field'

interface Props {
  incidentId: string
  // Changes whenever the incident changed elsewhere (severity, state) so the timeline reloads.
  reloadToken: number
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

// Structured changes ({"from":"SEV2","to":"SEV1"}) read better as "SEV2 → SEV1" than as JSON.
function contentOf(ev: TimelineEntry): string {
  const { from, to } = ev.data
  const change = from !== undefined && to !== undefined ? `${from} → ${to}` : ''
  return [ev.body, change].filter((part) => part !== '').join(' · ')
}

interface Loaded {
  incidentId: string
  events: TimelineEntry[]
  error: string | null
}

export function IncidentTimeline({ incidentId, reloadToken }: Props) {
  const [loaded, setLoaded] = useState<Loaded | null>(null)
  const [noteReloads, setNoteReloads] = useState(0)
  const [note, setNote] = useState('')
  const [noteError, setNoteError] = useState<string | null>(null)
  useEffect(() => {
    let active = true
    getTimeline(incidentId)
      .then((events) => {
        if (active) setLoaded({ incidentId, events, error: null })
      })
      .catch((err: unknown) => {
        if (active) setLoaded({ incidentId, events: [], error: messageOf(err) })
      })
    return () => {
      active = false
    }
  }, [incidentId, reloadToken, noteReloads])

  // A mistaken note is corrected with another note: events are never edited (BR-09).
  async function submitNote(e: FormEvent) {
    e.preventDefault()
    setNoteError(null)
    try {
      await addNote(incidentId, note)
      setNote('')
      setNoteReloads((n) => n + 1)
    } catch (err) {
      setNoteError(messageOf(err))
    }
  }

  // Within one incident, keep the previous events while a reload is in flight to avoid flicker.
  const current = loaded?.incidentId === incidentId ? loaded : null
  const events = current?.events ?? []

  return (
    <section data-testid="timeline" className="mt-4">
      <h3>Línea de tiempo</h3>
      {current === null && <p role="status">Cargando...</p>}
      {current?.error && (
        <Alert role="alert" data-testid="timeline-error">
          {current.error}
        </Alert>
      )}
      <ol
        data-testid="timeline-events"
        className="my-3 flex flex-col gap-3 border-l-2 border-slate-200 pl-4 dark:border-slate-700"
      >
        {events.map((ev) => (
          <li key={ev.id} data-testid="timeline-event" className="flex flex-wrap items-baseline gap-x-2 text-sm">
            <time
              data-testid="timeline-event-time"
              dateTime={ev.occurred_at}
              className="text-xs text-slate-500 dark:text-slate-400"
            >
              {new Date(ev.occurred_at).toLocaleString()}
            </time>
            <strong data-testid="timeline-event-type">{ev.type}</strong>
            <span data-testid="timeline-event-author" className="text-slate-600 dark:text-slate-300">
              {ev.author_id === null ? 'Sistema' : ev.author_name}
            </span>
            <span data-testid="timeline-event-content">{contentOf(ev)}</span>
          </li>
        ))}
      </ol>
      <form onSubmit={(e) => void submitNote(e)} className="flex flex-col gap-2">
        <Field>
          Nota
          <textarea
            data-testid="timeline-note-input"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className={controlClass}
          />
        </Field>
        <Button type="submit" data-testid="timeline-note-submit" className="self-start">
          Agregar nota
        </Button>
      </form>
      {noteError && (
        <Alert role="alert" data-testid="timeline-note-error" className="mt-2">
          {noteError}
        </Alert>
      )}
    </section>
  )
}
