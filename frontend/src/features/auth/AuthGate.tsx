import { useEffect, useState, type ReactNode } from 'react'
import { getMe, type User } from '../../api/auth'
import { Alert } from '../../ui/Alert'
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

  if (loading) return <p className="p-4 text-sm">Cargando...</p>
  if (loadError)
    return (
      <Alert role="alert" className="m-4">
        {loadError}
      </Alert>
    )
  if (user) return <>{children(user)}</>

  if (screen === 'register') {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
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
      </div>
    )
  }
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <LoginForm notice={notice} onLoggedIn={setUser} onShowRegister={() => setScreen('register')} />
    </div>
  )
}
