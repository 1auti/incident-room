import { useCallback, useEffect, useState, type FormEvent } from 'react'
import type { User } from '../../api/auth'
import {
  createService,
  deleteService,
  listServices,
  setServiceOncall,
  updateService,
  type Criticality,
  type Service,
} from '../../api/services'
import { listUsers } from '../../api/users'

interface Props {
  user: User
}

const criticalities: Criticality[] = ['critica', 'importante', 'estandar']

function CriticalitySelect(props: {
  testId: string
  value: Criticality
  onChange: (c: Criticality) => void
  ariaLabel?: string
}) {
  return (
    <select
      data-testid={props.testId}
      aria-label={props.ariaLabel}
      value={props.value}
      onChange={(e) => props.onChange(e.target.value as Criticality)}
    >
      {criticalities.map((c) => (
        <option key={c} value={c}>
          {c}
        </option>
      ))}
    </select>
  )
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

export function ServicesPage({ user }: Props) {
  // Hiding admin actions is presentation only; the backend enforces BR-12.
  const isAdmin = user.role === 'admin'
  const [services, setServices] = useState<Service[]>([])
  const [error, setError] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [criticality, setCriticality] = useState<Criticality>('estandar')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const [editCriticality, setEditCriticality] = useState<Criticality>('estandar')
  const [oncallUsers, setOncallUsers] = useState<User[]>([])
  const [oncallChoice, setOncallChoice] = useState<Record<string, string>>({})

  const refresh = useCallback(async () => {
    try {
      setServices(await listServices())
    } catch (err) {
      setError(messageOf(err))
    }
  }, [])

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

  // Only the admin may assign an on-call (BR-12), so only the admin loads the candidates.
  useEffect(() => {
    if (!isAdmin) return
    let active = true
    listUsers('oncall')
      .then((list) => {
        if (active) setOncallUsers(list)
      })
      .catch((err: unknown) => {
        if (active) setError(messageOf(err))
      })
    return () => {
      active = false
    }
  }, [isAdmin])

  async function run(action: () => Promise<void>) {
    setError(null)
    try {
      await action()
      await refresh()
    } catch (err) {
      setError(messageOf(err))
    }
  }

  function submitCreate(e: FormEvent) {
    e.preventDefault()
    void run(async () => {
      await createService(name, criticality)
      setName('')
    })
  }

  function startEdit(s: Service) {
    setEditingId(s.id)
    setEditName(s.name)
    setEditCriticality(s.criticality)
  }

  function submitEdit(e: FormEvent, id: string) {
    e.preventDefault()
    void run(async () => {
      await updateService(id, editName, editCriticality)
      setEditingId(null)
    })
  }

  return (
    <section>
      <h2>Servicios</h2>
      {error && <p role="alert" data-testid="service-error">{error}</p>}
      {isAdmin && (
        <form data-testid="service-create-form" onSubmit={submitCreate}>
          <label>
            Nombre
            <input data-testid="service-name-input" value={name} onChange={(e) => setName(e.target.value)} />
          </label>
          <label>
            Criticidad
            <CriticalitySelect testId="service-criticality-select" value={criticality} onChange={setCriticality} />
          </label>
          <button data-testid="service-create-submit" type="submit">Crear servicio</button>
        </form>
      )}
      <table data-testid="services-list">
        <thead>
          <tr>
            <th>Nombre</th>
            <th>Criticidad</th>
            <th>On-call</th>
            {isAdmin && <th>Acciones</th>}
          </tr>
        </thead>
        <tbody>
          {services.map((s) => (
            <tr key={s.id} data-testid="service-row" data-service-name={s.name}>
              <td data-testid="service-name">{s.name}</td>
              <td data-testid="service-criticality">{s.criticality}</td>
              <td data-testid="service-oncall">{s.oncall_user_id ?? 'Sin on-call'}</td>
              {isAdmin && (
                <td>
                  {editingId === s.id ? (
                    <form data-testid="service-edit-form" onSubmit={(e) => submitEdit(e, s.id)}>
                      <input
                        data-testid="service-edit-name"
                        aria-label="Nombre del servicio"
                        value={editName}
                        onChange={(e) => setEditName(e.target.value)}
                      />
                      <CriticalitySelect
                        testId="service-edit-criticality"
                        ariaLabel="Criticidad del servicio"
                        value={editCriticality}
                        onChange={setEditCriticality}
                      />
                      <button data-testid="service-edit-submit" type="submit">Guardar</button>
                      <button type="button" onClick={() => setEditingId(null)}>Cancelar</button>
                    </form>
                  ) : (
                    <>
                      <select
                        data-testid="service-oncall-select"
                        aria-label="On-call del servicio"
                        value={oncallChoice[s.id] ?? s.oncall_user_id ?? ''}
                        onChange={(e) => setOncallChoice({ ...oncallChoice, [s.id]: e.target.value })}
                      >
                        <option value="">Seleccionar on-call</option>
                        {oncallUsers.map((u) => (
                          <option key={u.id} value={u.id}>
                            {u.name}
                          </option>
                        ))}
                      </select>
                      <button
                        data-testid="service-oncall-submit"
                        type="button"
                        disabled={(oncallChoice[s.id] ?? s.oncall_user_id ?? '') === ''}
                        onClick={() =>
                          void run(async () => {
                            await setServiceOncall(s.id, oncallChoice[s.id] ?? s.oncall_user_id ?? '')
                          })
                        }
                      >
                        Assign on-call
                      </button>
                      <button data-testid="service-edit" type="button" onClick={() => startEdit(s)}>Editar</button>
                      <button data-testid="service-delete" type="button" onClick={() => void run(() => deleteService(s.id))}>
                        Borrar
                      </button>
                    </>
                  )}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
