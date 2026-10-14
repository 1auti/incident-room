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
import { criticalityLabel } from '../labels'
import { Alert } from '../../ui/Alert'
import { Button } from '../../ui/Button'
import { Card } from '../../ui/Card'
import { Field, controlClass } from '../../ui/Field'
import { tableClass, tdClass, thClass } from '../../ui/table'

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
      className={controlClass}
    >
      {criticalities.map((c) => (
        <option key={c} value={c}>
          {criticalityLabel[c]}
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

  // Explains why "Asignar on-call" is disabled: no candidates at all, or none chosen yet.
  function assignHint(s: Service): string | undefined {
    if ((oncallChoice[s.id] ?? s.oncall_user_id ?? '') !== '') return undefined
    return oncallUsers.length === 0 ? 'No hay usuarios con rol on-call para asignar' : 'Seleccioná un on-call primero'
  }

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
    <Card title="Servicios">
      {error && (
        <Alert role="alert" data-testid="service-error" className="mb-3">
          {error}
        </Alert>
      )}
      {isAdmin && (
        <form data-testid="service-create-form" onSubmit={submitCreate} className="mb-4 flex flex-wrap items-end gap-3">
          <Field>
            Nombre
            <input
              data-testid="service-name-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={controlClass}
            />
          </Field>
          <Field>
            Criticidad
            <CriticalitySelect testId="service-criticality-select" value={criticality} onChange={setCriticality} />
          </Field>
          <Button data-testid="service-create-submit" type="submit">
            Crear servicio
          </Button>
        </form>
      )}
      {isAdmin && oncallUsers.length === 0 && (
        <p className="mb-3 text-sm text-slate-400">
          No hay usuarios con rol on-call: asignales ese rol para poder elegir un on-call.
        </p>
      )}
      <div className="overflow-x-auto">
        <table data-testid="services-list" className={tableClass}>
          <thead>
            <tr>
              <th className={thClass}>Nombre</th>
              <th className={thClass}>Criticidad</th>
              <th className={thClass}>On-call</th>
              {isAdmin && <th className={thClass}>Acciones</th>}
            </tr>
          </thead>
          <tbody>
            {services.map((s) => (
              <tr key={s.id} data-testid="service-row" data-service-name={s.name}>
                <td data-testid="service-name" className={tdClass}>
                  {s.name}
                </td>
                <td data-testid="service-criticality" title={criticalityLabel[s.criticality]} className={tdClass}>
                  {s.criticality}
                </td>
                <td data-testid="service-oncall" className={tdClass}>
                  {s.oncall_user_id ?? 'Sin on-call'}
                </td>
                {isAdmin && (
                  <td className={tdClass}>
                    {editingId === s.id ? (
                      <form
                        data-testid="service-edit-form"
                        onSubmit={(e) => submitEdit(e, s.id)}
                        className="flex flex-wrap items-center gap-2"
                      >
                        <input
                          data-testid="service-edit-name"
                          aria-label="Nombre del servicio"
                          value={editName}
                          onChange={(e) => setEditName(e.target.value)}
                          className={controlClass}
                        />
                        <CriticalitySelect
                          testId="service-edit-criticality"
                          ariaLabel="Criticidad del servicio"
                          value={editCriticality}
                          onChange={setEditCriticality}
                        />
                        <Button data-testid="service-edit-submit" type="submit">
                          Guardar
                        </Button>
                        <Button type="button" variant="secondary" onClick={() => setEditingId(null)}>
                          Cancelar
                        </Button>
                      </form>
                    ) : (
                      <div className="flex flex-wrap items-center gap-2">
                        <select
                          data-testid="service-oncall-select"
                          aria-label="On-call del servicio"
                          value={oncallChoice[s.id] ?? s.oncall_user_id ?? ''}
                          onChange={(e) => setOncallChoice({ ...oncallChoice, [s.id]: e.target.value })}
                          className={controlClass}
                        >
                          <option value="">Seleccionar on-call</option>
                          {oncallUsers.map((u) => (
                            <option key={u.id} value={u.id}>
                              {u.name}
                            </option>
                          ))}
                        </select>
                        <Button
                          data-testid="service-oncall-submit"
                          type="button"
                          disabled={(oncallChoice[s.id] ?? s.oncall_user_id ?? '') === ''}
                          title={assignHint(s)}
                          onClick={() =>
                            void run(async () => {
                              await setServiceOncall(s.id, oncallChoice[s.id] ?? s.oncall_user_id ?? '')
                            })
                          }
                        >
                          Asignar on-call
                        </Button>
                        <Button
                          data-testid="service-edit"
                          type="button"
                          variant="secondary"
                          onClick={() => startEdit(s)}
                        >
                          Editar
                        </Button>
                        <Button
                          data-testid="service-delete"
                          type="button"
                          variant="dangerGhost"
                          onClick={() => void run(() => deleteService(s.id))}
                        >
                          Borrar
                        </Button>
                      </div>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}
