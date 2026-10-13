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
        <main data-testid="app-home" className="min-h-screen">
          <header className="border-b border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800">
            <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-3">
              <h1>Incident Room</h1>
              <p className="text-sm text-slate-600 dark:text-slate-300">
                <span data-testid="current-user-name">{user.name}</span> (
                <span data-testid="current-user-role">{user.role}</span>)
              </p>
            </div>
          </header>
          <div className="mx-auto grid max-w-7xl gap-4 px-4 py-4 lg:grid-cols-2">
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
          </div>
        </main>
      )}
    </AuthGate>
  )
}
