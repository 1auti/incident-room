import { useState } from 'react'
import { AuthGate } from './features/auth/AuthGate'
import { IncidentBoard } from './features/incidents/IncidentBoard'
import { IncidentDetail } from './features/incidents/IncidentDetail'
import { DeclareIncidentForm } from './features/incidents/DeclareIncidentForm'
import { ServicesPage } from './features/services/ServicesPage'

export default function App() {
  const [selectedIncidentId, setSelectedIncidentId] = useState<string | null>(null)
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
          <IncidentBoard onOpen={setSelectedIncidentId} />
          {selectedIncidentId !== null && (
            <IncidentDetail incidentId={selectedIncidentId} onClose={() => setSelectedIncidentId(null)} />
          )}
          <DeclareIncidentForm />
        </main>
      )}
    </AuthGate>
  )
}
