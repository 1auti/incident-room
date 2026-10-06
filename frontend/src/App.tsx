import { AuthGate } from './features/auth/AuthGate'
import { DeclareIncidentForm } from './features/incidents/DeclareIncidentForm'
import { ServicesPage } from './features/services/ServicesPage'

export default function App() {
  return (
    <AuthGate>
      {(user) => (
        <main data-testid="app-home">
          <h1>Incident Room</h1>
          <p>
            <span data-testid="current-user-name">{user.name}</span> (
            <span data-testid="current-user-role">{user.role}</span>)
          </p>
          <ServicesPage user={user} />
          <DeclareIncidentForm />
        </main>
      )}
    </AuthGate>
  )
}
