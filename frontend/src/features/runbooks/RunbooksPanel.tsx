import { useEffect, useState, type FormEvent } from 'react'
import type { User } from '../../api/auth'
import { createRunbook, deleteRunbook, listRunbooks, updateRunbook, type Runbook } from '../../api/runbooks'
import { listServices, type Service } from '../../api/services'
import { Alert } from '../../ui/Alert'
import { Button } from '../../ui/Button'
import { Card } from '../../ui/Card'
import { Field, controlClass } from '../../ui/Field'

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
    <Card data-testid="runbooks-panel" title="Runbooks">
      <Field className="mb-3 max-w-xs">
        Servicio
        <select
          data-testid="runbooks-service-select"
          value={serviceId}
          onChange={(e) => setServiceId(e.target.value)}
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
      {/* key: another service starts a fresh list and form, so no draft leaks between services. */}
      {serviceId !== '' && <ServiceRunbooks key={serviceId} serviceId={serviceId} isAdmin={user.role === 'admin'} />}
    </Card>
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
    <div className="flex flex-col gap-3">
      {error && (
        <Alert role="alert" data-testid="runbooks-error">
          {error}
        </Alert>
      )}
      {runbooks === null && <p role="status">Cargando...</p>}
      {runbooks?.length === 0 && (
        <p role="status" data-testid="runbooks-empty" className="text-sm text-slate-600 dark:text-slate-300">
          Este servicio no tiene runbooks.
        </p>
      )}
      <ul className="flex flex-col gap-3">
        {runbooks?.map((rb) => (
          <li
            key={rb.id}
            data-testid="runbook-row"
            data-runbook-title={rb.title}
            className="rounded-md border border-slate-200 p-3 dark:border-slate-700"
          >
            <h3 data-testid="runbook-title" className="font-semibold">
              {rb.title}
            </h3>
            <pre
              data-testid="runbook-content"
              className="my-2 overflow-x-auto whitespace-pre-wrap rounded-md bg-slate-100 p-2 text-sm dark:bg-slate-900"
            >
              {rb.content}
            </pre>
            {isAdmin && (
              <div className="flex gap-2">
                <Button type="button" variant="secondary" data-testid="runbook-edit" onClick={() => startEdit(rb)}>
                  Editar
                </Button>
                <Button type="button" variant="danger" data-testid="runbook-delete" onClick={() => void remove(rb.id)}>
                  Eliminar
                </Button>
              </div>
            )}
          </li>
        ))}
      </ul>
      {isAdmin && (
        <form onSubmit={(e) => void submit(e)} className="flex flex-col gap-3">
          <h3 className="font-semibold">{editingId === null ? 'Nuevo runbook' : 'Editar runbook'}</h3>
          <Field>
            Título
            <input
              data-testid="runbook-form-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className={controlClass}
            />
          </Field>
          <Field>
            Contenido
            <textarea
              data-testid="runbook-form-content"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className={`${controlClass} min-h-24`}
            />
          </Field>
          <div className="flex gap-2">
            <Button type="submit" data-testid="runbook-form-submit">
              {editingId === null ? 'Crear' : 'Guardar'}
            </Button>
            {editingId !== null && (
              <Button type="button" variant="secondary" data-testid="runbook-form-cancel" onClick={resetForm}>
                Cancelar
              </Button>
            )}
          </div>
        </form>
      )}
    </div>
  )
}
