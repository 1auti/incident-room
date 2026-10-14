import { useState, type FormEvent } from 'react'
import { getMe, login, type User } from '../../api/auth'
import { Alert } from '../../ui/Alert'
import { Button } from '../../ui/Button'
import { Field, controlClass } from '../../ui/Field'

interface Props {
  onLoggedIn: (user: User) => void
  onShowRegister: () => void
  notice?: string
}

export function LoginForm({ onLoggedIn, onShowRegister, notice }: Props) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await login(email, password)
      const user = await getMe()
      if (user) onLoggedIn(user)
      else setError('No se pudo abrir la sesión')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error inesperado')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form
      data-testid="login-form"
      onSubmit={submit}
      className="flex w-full max-w-sm flex-col gap-3 rounded-lg border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      <h1>Iniciar sesión</h1>
      {notice && (
        <Alert tone="status" data-testid="register-success">
          {notice}
        </Alert>
      )}
      <Field>
        Email
        <input
          data-testid="login-email"
          autoComplete="email"
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
          data-testid="login-password"
          autoComplete="current-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          className={controlClass}
        />
      </Field>
      {error && (
        <Alert role="alert" data-testid="login-error">
          {error}
        </Alert>
      )}
      <Button data-testid="login-submit" type="submit" disabled={busy}>
        Entrar
      </Button>
      <Button data-testid="show-register" type="button" variant="secondary" onClick={onShowRegister}>
        Crear cuenta
      </Button>
    </form>
  )
}
