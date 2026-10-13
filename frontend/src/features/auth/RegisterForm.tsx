import { useState, type FormEvent } from 'react'
import { register } from '../../api/auth'
import { Alert } from '../../ui/Alert'
import { Button } from '../../ui/Button'
import { Field, controlClass } from '../../ui/Field'

interface Props {
  onRegistered: () => void
  onShowLogin: () => void
}

export function RegisterForm({ onRegistered, onShowLogin }: Props) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await register(name, email, password)
      onRegistered()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error inesperado')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form
      data-testid="register-form"
      onSubmit={submit}
      className="flex w-full max-w-sm flex-col gap-3 rounded-lg border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      <h1>Crear cuenta</h1>
      <Field>
        Nombre
        <input
          data-testid="register-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          className={controlClass}
        />
      </Field>
      <Field>
        Email
        <input
          data-testid="register-email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className={controlClass}
        />
      </Field>
      <Field>
        Contraseña
        <input
          data-testid="register-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          className={controlClass}
        />
      </Field>
      {error && (
        <Alert role="alert" data-testid="register-error">
          {error}
        </Alert>
      )}
      <Button data-testid="register-submit" type="submit" disabled={busy}>
        Registrarme
      </Button>
      <Button data-testid="show-login" type="button" variant="secondary" onClick={onShowLogin}>
        Ya tengo cuenta
      </Button>
    </form>
  )
}
