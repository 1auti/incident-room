import { useState, type FormEvent } from 'react'
import { getMe, login, type User } from '../../api/auth'

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
    <form data-testid="login-form" onSubmit={submit}>
      <h1>Iniciar sesión</h1>
      {notice && <p data-testid="register-success">{notice}</p>}
      <label>
        Email
        <input data-testid="login-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
      </label>
      <label>
        Contraseña
        <input
          data-testid="login-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
      </label>
      {error && <p role="alert" data-testid="login-error">{error}</p>}
      <button data-testid="login-submit" type="submit" disabled={busy}>Entrar</button>
      <button data-testid="show-register" type="button" onClick={onShowRegister}>Crear cuenta</button>
    </form>
  )
}
