import { useEffect, useState, type FormEvent } from 'react'
import type { User } from '../../api/auth'
import { createRunbook, deleteRunbook, listRunbooks, updateRunbook, type Runbook } from '../../api/runbooks'
import { listServices, type Service } from '../../api/services'

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

interface Props {
  user: User
}

// Every authenticated user reads runbooks; only the admin sees the management controls. The backend
// enforces it either way (BR-10, BR-12).
export function RunbooksPanel({ user }: Props) {
  const [services, setServices] = useState<Service[]>([])
  const [serviceId, setServiceId] = useState('')

  useEffect(() => {
    let active = true
    listServices()
      .then((list) => {
        if (active) setServices(list)
      })
      .catch(() => {
        // Without the catalog there is nothing to pick; the services screen shows the error.
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <section data-testid="runbooks-panel">
      <h2>Runbooks</h2>
      <label>
        Servicio
        <select data-testid="runbooks-service-select" value={serviceId} onChange={(e) => setServiceId(e.target.value)}>
          <option value="">Elegí un servicio</option>
          {services.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      {/* key: another service starts a fresh list and form, so no draft leaks between services. */}
      {serviceId !== '' && <ServiceRunbooks key={serviceId} serviceId={serviceId} isAdmin={user.role === 'admin'} />}
    </section>
  )
}

interface ServiceRunbooksProps {
  serviceId: string
  isAdmin: boolean
}

function ServiceRunbooks({ serviceId, isAdmin }: ServiceRunbooksProps) {
  const [runbooks, setRunbooks] = useState<Runbook[] | null>(null)
  const [reloads, setReloads] = useState(0)
  const [error, setError] = useState<string | null>(null)
  // editingId null = the form creates a new runbook.
  const [editingId, setEditingId] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')

  useEffect(() => {
    let active = true
    listRunbooks(serviceId)
      .then((list) => {
        if (active) setRunbooks(list)
      })
      .catch((err: unknown) => {
        if (active) setError(messageOf(err))
      })
    return () => {
      active = false
    }
  }, [serviceId, reloads])

  function resetForm() {
    setEditingId(null)
    setTitle('')
    setContent('')
  }

  function startEdit(rb: Runbook) {
    setError(null)
    setEditingId(rb.id)
    setTitle(rb.title)
    setContent(rb.content)
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      if (editingId === null) await createRunbook(serviceId, title, content)
      else await updateRunbook(editingId, title, content)
      resetForm()
      setReloads((n) => n + 1)
    } catch (err) {
      setError(messageOf(err))
    }
  }

  async function remove(id: string) {
    setError(null)
    try {
      await deleteRunbook(id)
      if (editingId === id) resetForm()
      setReloads((n) => n + 1)
    } catch (err) {
      setError(messageOf(err))
    }
  }

  return (
    <div>
      {error && (
        <p role="alert" data-testid="runbooks-error">
          {error}
        </p>
      )}
      {runbooks === null && <p role="status">Cargando...</p>}
      {runbooks?.length === 0 && (
        <p role="status" data-testid="runbooks-empty">
          Este servicio no tiene runbooks.
        </p>
      )}
      <ul>
        {runbooks?.map((rb) => (
          <li key={rb.id} data-testid="runbook-row" data-runbook-title={rb.title}>
            <h3 data-testid="runbook-title">{rb.title}</h3>
            <pre data-testid="runbook-content">{rb.content}</pre>
            {isAdmin && (
              <>
                <button type="button" data-testid="runbook-edit" onClick={() => startEdit(rb)}>
                  Editar
                </button>
                <button type="button" data-testid="runbook-delete" onClick={() => void remove(rb.id)}>
                  Eliminar
                </button>
              </>
            )}
          </li>
        ))}
      </ul>
      {isAdmin && (
        <form onSubmit={(e) => void submit(e)}>
          <h3>{editingId === null ? 'Nuevo runbook' : 'Editar runbook'}</h3>
          <label>
            Título
            <input data-testid="runbook-form-title" value={title} onChange={(e) => setTitle(e.target.value)} />
          </label>
          <label>
            Contenido
            <textarea data-testid="runbook-form-content" value={content} onChange={(e) => setContent(e.target.value)} />
          </label>
          <button type="submit" data-testid="runbook-form-submit">
            {editingId === null ? 'Crear' : 'Guardar'}
          </button>
          {editingId !== null && (
            <button type="button" data-testid="runbook-form-cancel" onClick={resetForm}>
              Cancelar
            </button>
          )}
        </form>
      )}
    </div>
  )
}
