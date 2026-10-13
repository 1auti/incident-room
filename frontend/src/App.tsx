import { useState } from 'react'
import { AuthGate } from './features/auth/AuthGate'
import { CopilotPanel } from './features/copilot/CopilotPanel'
import { IncidentBoard } from './features/incidents/IncidentBoard'
import { IncidentDetail } from './features/incidents/IncidentDetail'
import { DeclareIncidentForm } from './features/incidents/DeclareIncidentForm'
import { MetricsDashboard } from './features/metrics/MetricsDashboard'
import { RunbooksPanel } from './features/runbooks/RunbooksPanel'
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
            // key: opening another incident starts a fresh detail, so no draft (e.g. a root cause) leaks into it.
            <IncidentDetail
              key={selectedIncidentId}
              incidentId={selectedIncidentId}
              onClose={() => setSelectedIncidentId(null)}
            />
          )}
          <DeclareIncidentForm />
          <MetricsDashboard />
          <RunbooksPanel user={user} />
          <CopilotPanel />
        </main>
      )}
    </AuthGate>
  )
}
