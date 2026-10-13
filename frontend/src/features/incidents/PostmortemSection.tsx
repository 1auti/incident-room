import { useEffect, useState } from 'react'
import { ApiError } from '../../api/client'
import {
  approvePostmortem,
  editPostmortem,
  generatePostmortem,
  getPostmortem,
  type Postmortem,
} from '../../api/postmortems'

interface Props {
  incidentId: string
  // Called after a change that adds a timeline event (generate, approve).
  onEvent: () => void
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

// The backend decides who generates, edits and approves, and in which states (BR-13); the UI shows its
// refusals as they come. Only an approved postmortem hides the actions: it is immutable.
export function PostmortemSection({ incidentId, onEvent }: Props) {
  const [loaded, setLoaded] = useState(false)
  const [postmortem, setPostmortem] = useState<Postmortem | null>(null)
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    getPostmortem(incidentId)
      .then((pm) => {
        if (!active) return
        setPostmortem(pm)
        setDraft(pm?.content ?? '')
        setLoaded(true)
      })
      .catch((err: unknown) => {
        if (active) setError(messageOf(err))
      })
    return () => {
      active = false
    }
  }, [incidentId])

  async function run(action: () => Promise<Postmortem>, addsEvent: boolean) {
    setError(null)
    setBusy(true)
    try {
      const pm = await action()
      setPostmortem(pm)
      setDraft(pm.content)
      if (addsEvent) onEvent()
    } catch (err) {
      setError(messageOf(err))
      // 409: someone else changed or approved it; show what is stored now.
      if (err instanceof ApiError && err.status === 409) {
        const current = await getPostmortem(incidentId).catch(() => postmortem)
        setPostmortem(current)
        setDraft(current?.content ?? '')
      }
    } finally {
      setBusy(false)
    }
  }

  const editable = postmortem?.status !== 'aprobado'

  return (
    <section data-testid="postmortem">
      <h3>Postmortem</h3>
      {!loaded && error === null && <p role="status">Cargando...</p>}
      {error && (
        <p role="alert" data-testid="postmortem-error">
          {error}
        </p>
      )}
      {loaded && postmortem === null && <p data-testid="postmortem-empty">Este incidente no tiene postmortem.</p>}
      {postmortem && (
        <p>
          Estado: <span data-testid="postmortem-status">{postmortem.status}</span>
          {postmortem.approved_at !== null && (
            <>
              {' '}
              (aprobado <time data-testid="postmortem-approved-at">{postmortem.approved_at}</time>)
            </>
          )}
        </p>
      )}
      {postmortem && (
        <textarea
          data-testid="postmortem-content"
          value={draft}
          readOnly={!editable}
          rows={16}
          onChange={(e) => setDraft(e.target.value)}
        />
      )}
      {loaded && editable && (
        <div>
          <button
            type="button"
            data-testid="postmortem-generate"
            disabled={busy}
            onClick={() => void run(() => generatePostmortem(incidentId), true)}
          >
            {postmortem === null ? 'Generar borrador' : 'Regenerar borrador'}
          </button>
          {postmortem && (
            <>
              <button
                type="button"
                data-testid="postmortem-save"
                disabled={busy}
                onClick={() => void run(() => editPostmortem(incidentId, draft), false)}
              >
                Guardar
              </button>
              <button
                type="button"
                data-testid="postmortem-approve"
                disabled={busy}
                onClick={() => void run(() => approvePostmortem(incidentId), true)}
              >
                Aprobar
              </button>
            </>
          )}
          {busy && <span role="status">Procesando...</span>}
        </div>
      )}
    </section>
  )
}
