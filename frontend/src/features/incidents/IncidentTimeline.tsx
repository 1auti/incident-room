import { useEffect, useState, type FormEvent } from 'react'
import { addNote, getTimeline, type TimelineEntry } from '../../api/incidents'

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
    <section data-testid="timeline">
      <h3>Timeline</h3>
      {current === null && <p role="status">Cargando...</p>}
      {current?.error && (
        <p role="alert" data-testid="timeline-error">
          {current.error}
        </p>
      )}
      <ol data-testid="timeline-events">
        {events.map((ev) => (
          <li key={ev.id} data-testid="timeline-event">
            <time data-testid="timeline-event-time" dateTime={ev.occurred_at}>
              {new Date(ev.occurred_at).toLocaleString()}
            </time>{' '}
            <strong data-testid="timeline-event-type">{ev.type}</strong>{' '}
            <span data-testid="timeline-event-author">{ev.author_id === null ? 'Sistema' : ev.author_name}</span>{' '}
            <span data-testid="timeline-event-content">{contentOf(ev)}</span>
          </li>
        ))}
      </ol>
      <form onSubmit={(e) => void submitNote(e)}>
        <label>
          Nota
          <textarea data-testid="timeline-note-input" value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        <button type="submit" data-testid="timeline-note-submit">
          Agregar nota
        </button>
      </form>
      {noteError && (
        <p role="alert" data-testid="timeline-note-error">
          {noteError}
        </p>
      )}
    </section>
  )
}
