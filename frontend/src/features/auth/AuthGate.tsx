import { useEffect, useState, type ReactNode } from 'react'
import { getMe, type User } from '../../api/auth'
import { LoginForm } from './LoginForm'
import { RegisterForm } from './RegisterForm'

interface Props {
  children: (user: User) => ReactNode
}

type Screen = 'login' | 'register'

export function AuthGate({ children }: Props) {
  const [loading, setLoading] = useState(true)
  const [user, setUser] = useState<User | null>(null)
  const [screen, setScreen] = useState<Screen>('login')
  const [notice, setNotice] = useState<string | undefined>()
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    getMe()
      .then(setUser)
      .catch((err: unknown) => setLoadError(err instanceof Error ? err.message : 'Error inesperado'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <p>Cargando...</p>
  if (loadError) return <p role="alert">{loadError}</p>
  if (user) return <>{children(user)}</>

  if (screen === 'register') {
    return (
      <RegisterForm
        onShowLogin={() => {
          setNotice(undefined)
          setScreen('login')
        }}
        onRegistered={() => {
          setNotice('Cuenta creada. Ya puedes iniciar sesión.')
          setScreen('login')
        }}
      />
    )
  }
  return <LoginForm notice={notice} onLoggedIn={setUser} onShowRegister={() => setScreen('register')} />
}
