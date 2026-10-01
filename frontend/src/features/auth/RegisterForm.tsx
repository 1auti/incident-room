import { useState, type FormEvent } from 'react'
import { register } from '../../api/auth'

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
    <form data-testid="register-form" onSubmit={submit}>
      <h1>Crear cuenta</h1>
      <label>
        Nombre
        <input data-testid="register-name" value={name} onChange={(e) => setName(e.target.value)} required />
      </label>
      <label>
        Email
        <input data-testid="register-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
      </label>
      <label>
        Contraseña
        <input
          data-testid="register-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
      </label>
      {error && <p role="alert" data-testid="register-error">{error}</p>}
      <button data-testid="register-submit" type="submit" disabled={busy}>Registrarme</button>
      <button data-testid="show-login" type="button" onClick={onShowLogin}>Ya tengo cuenta</button>
    </form>
  )
}
